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
	reader := bufio.NewReader(os.Stdin)

	teamInput := strings.TrimSpace(team)
	if teamInput == "" {
		fmt.Print("Slack workspace URL or subdomain (e.g. myco or https://myco.slack.com): ")
		s, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		teamInput = strings.TrimSpace(s)
	}
	teamSub, err := auth.TeamFromURL(teamInput)
	if err != nil {
		return err
	}

	// xoxd — try Chrome first, fall back to env, then prompt.
	xoxd := strings.TrimSpace(os.Getenv("SLACK_XOXD"))
	if xoxd == "" {
		fmt.Fprintln(os.Stderr, "Reading 'd' cookie from Chrome (close Chrome if you see a lock error)...")
		got, err := auth.ExtractXOXDFromChrome()
		if err == nil {
			xoxd = got
		} else {
			fmt.Fprintln(os.Stderr, "Chrome extraction failed:", err)
			fmt.Print("Paste xoxd cookie value (begins with xoxd-): ")
			s, _ := reader.ReadString('\n')
			xoxd = strings.TrimSpace(s)
		}
	}
	if !strings.HasPrefix(xoxd, "xoxd-") {
		return errors.New("xoxd not in expected format")
	}

	// xoxc — fetch boot HTML using xoxd cookie.
	fmt.Fprintln(os.Stderr, "Fetching workspace bootstrap for xoxc token...")
	xoxc, err := auth.ExtractXOXC(teamSub, xoxd)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Boot fetch failed:", err)
		fmt.Print("Paste xoxc token (begins with xoxc-): ")
		s, _ := reader.ReadString('\n')
		xoxc = strings.TrimSpace(s)
	}
	if !strings.HasPrefix(xoxc, "xoxc-") {
		return errors.New("xoxc not in expected format")
	}

	tok := auth.Tokens{Team: teamSub, XOXC: xoxc, XOXD: xoxd}
	if err := auth.Save(tok); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Saved tokens for team %q to OS keychain.\n", teamSub)
	_ = ctx
	return nil
}
