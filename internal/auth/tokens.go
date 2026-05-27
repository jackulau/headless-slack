package auth

import (
	"errors"
	"strings"
)

// Tokens holds the credentials needed to talk to Slack as a real user.
//
// XOXC is the workspace user token from boot_data.api_token. It begins with
// "xoxc-" and is per-team.
//
// XOXD is the value of the "d" session cookie on *.slack.com. It begins with
// "xoxd-" and is shared across all teams the user is signed into.
type Tokens struct {
	Team string // team subdomain, e.g. "myco" for myco.slack.com
	XOXC string
	XOXD string
}

func (t Tokens) Validate() error {
	if !strings.HasPrefix(t.XOXC, "xoxc-") {
		return errors.New("xoxc token missing or malformed (expected prefix 'xoxc-')")
	}
	if !strings.HasPrefix(t.XOXD, "xoxd-") {
		return errors.New("xoxd cookie missing or malformed (expected prefix 'xoxd-')")
	}
	if t.Team == "" {
		return errors.New("team subdomain empty")
	}
	return nil
}
