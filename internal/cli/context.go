package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jacklau/headless-slack/internal/api"
	"github.com/jacklau/headless-slack/internal/auth"
	"github.com/jacklau/headless-slack/internal/store"
)

// session bundles the live Slack client + local store.
type session struct {
	Team   string
	Client *api.Client
	Store  *store.Store
}

// open loads tokens from the keychain, builds an API client, opens the store.
// If --team wasn't provided, falls back to SLACK_TEAM env, then errors.
func open(ctx context.Context) (*session, error) {
	t := strings.TrimSpace(team)
	if t == "" {
		t = strings.TrimSpace(os.Getenv("SLACK_TEAM"))
	}
	if t == "" {
		return nil, errors.New("no team — pass --team <subdomain> or set SLACK_TEAM (run 'slk login' first)")
	}
	tok, err := auth.Load(t)
	if err != nil {
		return nil, err
	}
	s, err := store.Open(filepath.Join(cfgDir, "cache.sqlite"))
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}
	return &session{
		Team:   t,
		Client: api.New(t, tok.XOXC, tok.XOXD),
		Store:  s,
	}, nil
}

func (s *session) close() {
	if s != nil && s.Store != nil {
		_ = s.Store.Close()
	}
}
