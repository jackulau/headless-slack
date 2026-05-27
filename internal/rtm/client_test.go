package rtm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/jacklau/headless-slack/internal/api"
)

func TestBus_PublishSubscribe(t *testing.T) {
	b := NewBus()
	c1 := b.Subscribe(4)
	c2 := b.Subscribe(4)
	defer b.Unsubscribe(c1)
	defer b.Unsubscribe(c2)

	b.Publish(Event{Type: EventMessage, Text: "hi"})

	for _, ch := range []chan Event{c1, c2} {
		select {
		case ev := <-ch:
			if ev.Type != EventMessage || ev.Text != "hi" {
				t.Errorf("got %+v", ev)
			}
		case <-time.After(time.Second):
			t.Fatal("subscriber missed event")
		}
	}
}

func TestBus_DropsSlowSubscribers(t *testing.T) {
	b := NewBus()
	ch := b.Subscribe(1)
	defer b.Unsubscribe(ch)
	for i := 0; i < 50; i++ {
		b.Publish(Event{Type: EventMessage})
	}
	// Drain — bus must not deadlock even though buffer is 1.
	// We accept some drops; just ensure publish returned.
}

func TestDecode(t *testing.T) {
	ev, err := Decode([]byte(`{"type":"message","channel":"C1","user":"U1","text":"hello","ts":"1.000"}`))
	if err != nil {
		t.Fatal(err)
	}
	if ev.Type != "message" || ev.Channel != "C1" || ev.Text != "hello" {
		t.Errorf("bad event: %+v", ev)
	}
	if len(ev.Raw) == 0 {
		t.Error("raw should be populated")
	}
}

// stubConnector implements Connector + serves a fake WS endpoint.
type stubConnector struct {
	url string
}

func (s stubConnector) RTMConnect(ctx context.Context) (api.RTMConnectResp, error) {
	return api.RTMConnectResp{URL: s.url}, nil
}

// fakeWS spins up a websocket server that:
//
//  1. accepts a connection
//  2. sends "hello"
//  3. sends one "message" frame
//  4. sends "goodbye" (forces client to disconnect and reconnect)
func fakeWS(t *testing.T) (string, chan struct{}) {
	t.Helper()
	served := make(chan struct{}, 8)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		up := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
		conn, err := up.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer conn.Close()
		send := func(v any) {
			b, _ := json.Marshal(v)
			_ = conn.WriteMessage(websocket.TextMessage, b)
		}
		send(map[string]string{"type": "hello"})
		send(map[string]string{"type": "message", "channel": "C1", "user": "U1", "text": "yo", "ts": "1.000"})
		send(map[string]string{"type": "goodbye"})
		// Drain any pings briefly before closing.
		conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		_, _, _ = conn.ReadMessage()
		select {
		case served <- struct{}{}:
		default:
		}
	}))
	t.Cleanup(srv.Close)
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	return wsURL, served
}

func TestClient_ReceivesEvents(t *testing.T) {
	wsURL, served := fakeWS(t)
	bus := NewBus()
	sub := bus.Subscribe(16)
	defer bus.Unsubscribe(sub)

	c := NewClient(stubConnector{url: wsURL}, bus)
	c.PingInterval = 10 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	go func() { _ = c.Run(ctx) }()

	got := map[string]bool{}
	deadline := time.After(2 * time.Second)
	for len(got) < 3 {
		select {
		case ev := <-sub:
			got[ev.Type] = true
		case <-deadline:
			t.Fatalf("timeout; got types so far: %v", got)
		}
	}
	if !got["hello"] || !got["message"] || !got["goodbye"] {
		t.Errorf("missing types: %v", got)
	}
	select {
	case <-served:
	case <-time.After(time.Second):
		// Server may not have signaled yet; not fatal.
	}
}
