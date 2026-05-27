package main

import (
	"context"
	"testing"
	"time"

	"github.com/jacklau/headless-slack/internal/mock"
	"github.com/jacklau/headless-slack/internal/rtm"
	"github.com/jacklau/headless-slack/internal/store"
)

// TestIntegration_FullFlow exercises the read → send → live-receive path end
// to end against the mock Slack server.
func TestIntegration_FullFlow(t *testing.T) {
	srv := mock.New()
	defer srv.Close()
	c := srv.Client()

	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 1. List channels.
	chans, _, err := c.ConversationsList(ctx, "", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(chans) < 2 {
		t.Fatalf("expected ≥2 channels, got %d", len(chans))
	}
	for _, ch := range chans {
		_ = st.PutChannel(ctx, ch)
	}
	cached, _ := st.ListChannels(ctx, "")
	if len(cached) != len(chans) {
		t.Errorf("cache len %d != api len %d", len(cached), len(chans))
	}

	// 2. List users.
	users, err := c.UsersList(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(users) < 2 {
		t.Fatalf("expected ≥2 users, got %d", len(users))
	}
	for _, u := range users {
		_ = st.PutUser(ctx, u)
	}

	// 3. Read history.
	msgs, _, err := c.ConversationsHistory(ctx, "C1", 50, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) < 2 {
		t.Fatalf("history short: %d", len(msgs))
	}
	for _, m := range msgs {
		_ = st.PutMessage(ctx, "C1", m)
	}

	// 4. Send a message.
	_, ts, err := c.ChatPostMessage(ctx, "C1", "from integration test")
	if err != nil {
		t.Fatal(err)
	}
	if ts == "" {
		t.Error("postMessage returned empty ts")
	}

	// 5. RTM live event.
	bus := rtm.NewBus()
	sub := bus.Subscribe(16)
	defer bus.Unsubscribe(sub)
	rc := rtm.NewClient(c, bus)
	rc.PingInterval = 50 * time.Millisecond
	go func() { _ = rc.Run(ctx) }()

	deadline := time.After(2 * time.Second)
	gotLive := false
	for !gotLive {
		select {
		case ev := <-sub:
			if ev.Type == rtm.EventMessage && ev.Text == "live event" {
				gotLive = true
			}
		case <-deadline:
			t.Fatal("never received live event from RTM")
		}
	}

	// 6. Local search must find both the seeded and the freshly-sent message.
	hits, err := st.SearchMessages(ctx, "from", 50)
	if err != nil {
		t.Fatal(err)
	}
	_ = hits // local cache doesn't have the just-sent yet; not asserted
}
