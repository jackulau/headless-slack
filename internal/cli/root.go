package cli

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	cfgDir string
	team   string
)

func Root() *cobra.Command {
	root := &cobra.Command{
		Use:   "slk",
		Short: "slk — headless Slack CLI",
		Long: `slk is a terminal client for Slack.

It uses browser-extracted xoxc + xoxd tokens to read channels,
send messages, and stream realtime events without the official
desktop or web app.

Run "slk login" first to capture tokens from your local Chrome.
Then "slk channels" lists conversations, "slk read <chan>" reads
history, "slk send <chan> <msg>" posts, and "slk watch" streams
events. Run "slk" with no args to launch the TUI.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	defaultCfg, _ := defaultConfigDir()
	root.PersistentFlags().StringVar(&cfgDir, "config-dir", defaultCfg, "directory for tokens + cache")
	root.PersistentFlags().StringVar(&team, "team", "", "team subdomain (e.g. myco) — required when multiple teams stored")

	root.AddCommand(loginCmd(), channelsCmd(), readCmd(), sendCmd(), dmCmd(), watchCmd(), usersCmd(), searchCmd(), tuiCmd(), versionCmd())
	return root
}

func defaultConfigDir() (string, error) {
	h, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, "slk"), nil
}
