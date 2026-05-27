package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jacklau/headless-slack/internal/api"
)

// resolveChannel turns a user-supplied channel identifier into a Slack ID.
//
// Accepts:
//   - "C0123456789" / "D0123..." / "G0123..." — returned as-is
//   - "#general" or "general" — looked up in the local store, then via
//     conversations.list if not cached
func resolveChannel(ctx context.Context, s *session, input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", errors.New("channel required")
	}
	if isChannelID(input) {
		return input, nil
	}
	name := strings.TrimPrefix(input, "#")

	if c, err := s.Store.FindChannelByName(ctx, name); err == nil && c.ID != "" {
		return c.ID, nil
	}
	// Cold cache — list everything once and try again.
	chans, err := fetchAllChannels(ctx, s.Client)
	if err != nil {
		return "", err
	}
	for _, c := range chans {
		_ = s.Store.PutChannel(ctx, c)
		if c.Name == name {
			return c.ID, nil
		}
	}
	return "", fmt.Errorf("no channel named %q (try 'slk channels' to list)", name)
}

// resolveUser turns "@name" / "name" / "U0123…" into a user ID.
func resolveUser(ctx context.Context, s *session, input string) (api.User, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return api.User{}, errors.New("user required")
	}
	if isUserID(input) {
		return s.Client.UsersInfo(ctx, input)
	}
	name := strings.TrimPrefix(input, "@")
	// Fast path: query stored users.
	// Cold-load all users on first call — caches in store.
	users, err := s.Client.UsersList(ctx)
	if err != nil {
		return api.User{}, err
	}
	for _, u := range users {
		_ = s.Store.PutUser(ctx, u)
		if u.Name == name || u.Profile.DisplayName == name || u.RealName == name {
			return u, nil
		}
	}
	return api.User{}, fmt.Errorf("no user matched %q", input)
}

func isChannelID(s string) bool {
	if len(s) < 2 {
		return false
	}
	switch s[0] {
	case 'C', 'D', 'G':
		return allUpperAlnum(s)
	}
	return false
}

func isUserID(s string) bool {
	if len(s) < 2 {
		return false
	}
	switch s[0] {
	case 'U', 'W':
		return allUpperAlnum(s)
	}
	return false
}

func allUpperAlnum(s string) bool {
	for _, r := range s {
		if !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}
