package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var version = "0.1.0-dev"

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "print version",
		RunE: func(*cobra.Command, []string) error {
			fmt.Println("slk", version)
			return nil
		},
	}
}
