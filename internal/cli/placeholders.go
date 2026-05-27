package cli

import (
	"errors"

	"github.com/spf13/cobra"
)

var errNotYet = errors.New("command not yet implemented — see deliverable in GOAL.md")

func loginCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "extract xoxc + xoxd tokens from local Chrome",
		Long: `login extracts xoxc (workspace) and xoxd (session cookie) tokens
from your local Chrome profile and stores them in the OS keychain.

Chrome must be closed during extraction (it locks the cookie DB).`,
		RunE: func(*cobra.Command, []string) error { return runLogin() },
	}
}

func channelsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "channels",
		Short: "list channels and DMs",
		RunE:  func(*cobra.Command, []string) error { return runChannels() },
	}
}

func readCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "read <channel>",
		Short: "read recent messages from a channel",
		Args:  cobra.ExactArgs(1),
		RunE:  func(cmd *cobra.Command, a []string) error { return runRead(a[0], readLast) },
	}
	c.Flags().IntVarP(&readLast, "last", "n", 50, "number of messages")
	return c
}

func sendCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "send <channel> <message...>",
		Short: "send a message to a channel",
		Args:  cobra.MinimumNArgs(2),
		RunE:  func(_ *cobra.Command, a []string) error { return runSend(a[0], a[1:]) },
	}
}

func dmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "dm <user> <message...>",
		Short: "send a direct message to a user",
		Args:  cobra.MinimumNArgs(2),
		RunE:  func(_ *cobra.Command, a []string) error { return runDM(a[0], a[1:]) },
	}
}

func watchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "watch [channel]",
		Short: "stream realtime events (optionally filter to a channel)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, a []string) error {
			ch := ""
			if len(a) > 0 {
				ch = a[0]
			}
			return runWatch(ch)
		},
	}
}

func usersCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "users",
		Short: "list users in the workspace",
		RunE:  func(*cobra.Command, []string) error { return runUsers() },
	}
}

func searchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "search <query>",
		Short: "search messages",
		Args:  cobra.MinimumNArgs(1),
		RunE:  func(_ *cobra.Command, a []string) error { return runSearch(a) },
	}
}

func tuiCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "launch the interactive TUI (default when run without args)",
		RunE:  func(*cobra.Command, []string) error { return runTUI() },
	}
}

var readLast int

func runTUI() error { return errNotYet } // implemented in D8
