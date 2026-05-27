package store

import (
	"context"
	"testing"

	"github.com/jacklau/headless-slack/internal/api"
)

func openTest(t *testing.T) *Store {
	t.Helper()
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestUserRoundTrip(t *testing.T) {
	s := openTest(t)
	u := api.User{ID: "U1", Name: "alice", RealName: "Alice A"}
	u.Profile.DisplayName = "ali"
	u.Profile.Email = "alice@example.com"
	if err := s.PutUser(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetUser(context.Background(), "U1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "alice" || got.RealName != "Alice A" || got.Profile.DisplayName != "ali" {
		t.Errorf("got %+v", got)
	}
}

func TestChannelListing(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	_ = s.PutChannel(ctx, api.Channel{ID: "C1", Name: "general", IsChannel: true})
	_ = s.PutChannel(ctx, api.Channel{ID: "D1", Name: "", IsIM: true, User: "U2"})
	_ = s.PutChannel(ctx, api.Channel{ID: "G1", Name: "secret", IsGroup: true, IsPrivate: true})

	all, err := s.ListChannels(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("len=%d want 3: %+v", len(all), all)
	}

	chans, _ := s.ListChannels(ctx, "channel")
	if len(chans) != 1 || chans[0].ID != "C1" {
		t.Errorf("channel filter: %+v", chans)
	}
	ims, _ := s.ListChannels(ctx, "im")
	if len(ims) != 1 || ims[0].User != "U2" {
		t.Errorf("im filter: %+v", ims)
	}
}

func TestFindChannelByName(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	_ = s.PutChannel(ctx, api.Channel{ID: "C1", Name: "general", IsChannel: true})
	c, err := s.FindChannelByName(ctx, "general")
	if err != nil {
		t.Fatal(err)
	}
	if c.ID != "C1" || !c.IsChannel {
		t.Errorf("got %+v", c)
	}
}

func TestMessagesAndCursor(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	for _, ts := range []string{"3.000", "1.000", "2.000"} {
		m := api.Message{Type: "message", User: "U1", Text: "msg-" + ts, TS: ts}
		if err := s.PutMessage(ctx, "C1", m); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.RecentMessages(ctx, "C1", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d want 3", len(got))
	}
	// Oldest first after our reverse.
	if got[0].TS != "1.000" || got[2].TS != "3.000" {
		t.Errorf("order wrong: %+v", got)
	}

	// Upsert one with same ts (edit).
	if err := s.PutMessage(ctx, "C1", api.Message{TS: "2.000", User: "U1", Text: "edited"}); err != nil {
		t.Fatal(err)
	}
	got, _ = s.RecentMessages(ctx, "C1", 10)
	if len(got) != 3 {
		t.Fatalf("upsert should not duplicate")
	}

	if err := s.SetCursor(ctx, "C1", "3.000"); err != nil {
		t.Fatal(err)
	}
	cur, err := s.GetCursor(ctx, "C1")
	if err != nil || cur != "3.000" {
		t.Errorf("cursor: %q err=%v", cur, err)
	}
	missing, err := s.GetCursor(ctx, "C-nope")
	if err != nil || missing != "" {
		t.Errorf("missing cursor: %q err=%v", missing, err)
	}
}

func TestSearchMessages(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	_ = s.PutMessage(ctx, "C1", api.Message{TS: "1", User: "U1", Text: "hello world"})
	_ = s.PutMessage(ctx, "C1", api.Message{TS: "2", User: "U1", Text: "goodbye world"})
	_ = s.PutMessage(ctx, "C1", api.Message{TS: "3", User: "U1", Text: "unrelated"})
	got, err := s.SearchMessages(ctx, "world", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Errorf("got %d want 2: %+v", len(got), got)
	}
}
