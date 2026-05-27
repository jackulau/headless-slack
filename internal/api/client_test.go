package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeServer returns a Slack-shaped mock server that responds to /api/<method>
// from a routing map.
func fakeServer(t *testing.T, routes map[string]any) (*Client, func()) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/")
		body, ok := routes[path]
		if !ok {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"ok":false,"error":"unknown_method"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(body)
	}))
	c := &Client{
		Auth:    &AuthClient{Inner: srv.Client(), XOXC: "xoxc-test", XOXD: "xoxd-test"},
		Team:    "myco",
		BaseURL: srv.URL,
	}
	return c, srv.Close
}

func TestConversationsList(t *testing.T) {
	c, stop := fakeServer(t, map[string]any{
		"conversations.list": map[string]any{
			"ok": true,
			"channels": []map[string]any{
				{"id": "C123", "name": "general", "is_channel": true},
				{"id": "C456", "name": "random", "is_channel": true},
			},
			"response_metadata": map[string]any{"next_cursor": ""},
		},
	})
	defer stop()
	chans, cur, err := c.ConversationsList(context.Background(), "", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(chans) != 2 || chans[0].Name != "general" || chans[1].ID != "C456" {
		t.Errorf("unexpected channels: %+v", chans)
	}
	if cur != "" {
		t.Errorf("cursor = %q want empty", cur)
	}
}

func TestConversationsHistory(t *testing.T) {
	c, stop := fakeServer(t, map[string]any{
		"conversations.history": map[string]any{
			"ok": true,
			"messages": []map[string]any{
				{"type": "message", "user": "U1", "text": "hi", "ts": "1.000"},
				{"type": "message", "user": "U2", "text": "yo", "ts": "2.000"},
			},
		},
	})
	defer stop()
	msgs, _, err := c.ConversationsHistory(context.Background(), "C123", 10, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 || msgs[0].Text != "hi" {
		t.Errorf("unexpected: %+v", msgs)
	}
}

func TestChatPostMessage(t *testing.T) {
	c, stop := fakeServer(t, map[string]any{
		"chat.postMessage": map[string]any{
			"ok":      true,
			"channel": "C123",
			"ts":      "1700000000.000100",
		},
	})
	defer stop()
	ch, ts, err := c.ChatPostMessage(context.Background(), "C123", "hello world")
	if err != nil {
		t.Fatal(err)
	}
	if ch != "C123" || ts != "1700000000.000100" {
		t.Errorf("got ch=%q ts=%q", ch, ts)
	}
}

func TestSlackError(t *testing.T) {
	c, stop := fakeServer(t, map[string]any{
		"users.info": map[string]any{
			"ok":    false,
			"error": "user_not_found",
		},
	})
	defer stop()
	_, err := c.UsersInfo(context.Background(), "Uxxx")
	if err == nil {
		t.Fatal("expected error")
	}
	if !IsAuthError(err) && err.Error() == "" {
		// just confirm it stringifies non-empty
		t.Errorf("err formatting odd: %q", err)
	}
}

func TestIsAuthError(t *testing.T) {
	c, stop := fakeServer(t, map[string]any{
		"users.info": map[string]any{"ok": false, "error": "invalid_auth"},
	})
	defer stop()
	_, err := c.UsersInfo(context.Background(), "U1")
	if !IsAuthError(err) {
		t.Errorf("expected IsAuthError true for invalid_auth, got %v", err)
	}
}

func TestRTMConnect(t *testing.T) {
	c, stop := fakeServer(t, map[string]any{
		"rtm.connect": map[string]any{
			"ok":   true,
			"url":  "wss://wss-primary.slack.com/?token=fake",
			"self": map[string]string{"id": "U1", "name": "me"},
			"team": map[string]string{"id": "T1", "name": "myco", "domain": "myco"},
		},
	})
	defer stop()
	r, err := c.RTMConnect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if r.URL == "" || r.Self.ID != "U1" {
		t.Errorf("got %+v", r)
	}
}
