package auth

import (
	"errors"
	"fmt"

	"github.com/zalando/go-keyring"
)

const keyringService = "slk-headless-slack"

// Save persists tokens for a team to the OS keychain.
// XOXC is stored per team (key: "xoxc:<team>"); XOXD is shared (key: "xoxd").
func Save(t Tokens) error {
	if err := t.Validate(); err != nil {
		return err
	}
	if err := keyring.Set(keyringService, "xoxc:"+t.Team, t.XOXC); err != nil {
		return fmt.Errorf("keyring set xoxc: %w", err)
	}
	if err := keyring.Set(keyringService, "xoxd", t.XOXD); err != nil {
		return fmt.Errorf("keyring set xoxd: %w", err)
	}
	return nil
}

// Load returns tokens for the given team, or an error if missing.
func Load(team string) (Tokens, error) {
	if team == "" {
		return Tokens{}, errors.New("team required")
	}
	xc, err := keyring.Get(keyringService, "xoxc:"+team)
	if err != nil {
		return Tokens{}, fmt.Errorf("no stored xoxc for team %q (run 'slk login'): %w", team, err)
	}
	xd, err := keyring.Get(keyringService, "xoxd")
	if err != nil {
		return Tokens{}, fmt.Errorf("no stored xoxd (run 'slk login'): %w", err)
	}
	return Tokens{Team: team, XOXC: xc, XOXD: xd}, nil
}

// Forget removes tokens for a team. Pass empty team to also forget xoxd.
func Forget(team string) error {
	if team != "" {
		_ = keyring.Delete(keyringService, "xoxc:"+team)
	} else {
		_ = keyring.Delete(keyringService, "xoxd")
	}
	return nil
}
