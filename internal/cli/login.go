package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/jacklau/headless-slack/internal/auth"
)

func runLogin() error {
	ctx := context.Background()
	_ = ctx

	teamHint := strings.TrimSpace(team)
	if teamHint == "" {
		teamHint = strings.TrimSpace(os.Getenv("SLACK_TEAM"))
	}

	xoxd := strings.TrimSpace(os.Getenv("SLACK_XOXD"))
	if xoxd == "" {
		fmt.Fprintln(os.Stderr, "→ Reading Slack session cookie from Chrome...")
		got, err := auth.ExtractXOXDFromChrome()
		if err != nil {
			fmt.Fprintln(os.Stderr, "  Chrome extraction failed:", err)
			fmt.Fprintln(os.Stderr, "  Falling back to manual paste.")
			xoxd = promptLine("  Paste xoxd cookie (begins with xoxd-): ")
		} else {
			xoxd = got
			fmt.Fprintln(os.Stderr, "  ✓ Got xoxd")
		}
	}
	if !strings.HasPrefix(xoxd, "xoxd-") {
		return errors.New("xoxd not in expected format")
	}

	// Build candidate workspace list.
	var teams []string
	if teamHint != "" {
		sub, err := auth.TeamFromURL(teamHint)
		if err == nil {
			teams = []string{sub}
		}
	}
	if len(teams) == 0 {
		fmt.Fprintln(os.Stderr, "→ Detecting workspaces from Chrome cookies...")
		detected, err := auth.ScanSlackTeamsFromChrome()
		if err == nil && len(detected) > 0 {
			teams = detected
			fmt.Fprintf(os.Stderr, "  ✓ Found %d: %s\n", len(detected), strings.Join(detected, ", "))
		}
	}
	if len(teams) == 0 {
		teamInput := promptLine("  Workspace URL/subdomain (e.g. myco): ")
		sub, err := auth.TeamFromURL(teamInput)
		if err != nil {
			return err
		}
		teams = []string{sub}
	}

	// Fetch xoxc per team. Skip teams that fail (session not bound there).
	saved, failed := 0, 0
	for _, t := range teams {
		fmt.Fprintf(os.Stderr, "→ %s.slack.com ... ", t)
		xoxc, err := auth.ExtractXOXC(t, xoxd)
		if err != nil {
			fmt.Fprintln(os.Stderr, "skip ("+err.Error()+")")
			failed++
			continue
		}
		if !strings.HasPrefix(xoxc, "xoxc-") {
			fmt.Fprintln(os.Stderr, "skip (token format)")
			failed++
			continue
		}
		if err := auth.Save(auth.Tokens{Team: t, XOXC: xoxc, XOXD: xoxd}); err != nil {
			fmt.Fprintln(os.Stderr, "save failed:", err)
			failed++
			continue
		}
		fmt.Fprintln(os.Stderr, "✓ saved")
		saved++
	}

	if saved == 0 {
		return fmt.Errorf("no workspaces enrolled (%d failed)", failed)
	}
	fmt.Fprintf(os.Stderr, "\nSaved %d workspace(s) to keychain.\n", saved)
	if saved == 1 {
		fmt.Fprintf(os.Stderr, "Try:  slk --team %s channels\n", teams[0])
		fmt.Fprintf(os.Stderr, "Or:   export SLACK_TEAM=%s\n", teams[0])
	} else {
		fmt.Fprintln(os.Stderr, "Run with --team <subdomain> or set SLACK_TEAM to choose.")
	}
	return nil
}

func promptLine(prompt string) string {
	fmt.Fprint(os.Stderr, prompt)
	r := bufio.NewReader(os.Stdin)
	s, _ := r.ReadString('\n')
	return strings.TrimSpace(s)
}
