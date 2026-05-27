package main

import (
	"fmt"
	"os"

	"github.com/jacklau/headless-slack/internal/cli"
)

func main() {
	if err := cli.Root().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "slk:", err)
		os.Exit(1)
	}
}
