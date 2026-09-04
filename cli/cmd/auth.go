package cmd

import (
	"fmt"
	"os"

	"kuro/cli/internal/config"
)

// RunAuth handles the `kuro auth <api-key>` command.
func RunAuth(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: kuro auth <api-key>")
		os.Exit(1)
	}

	apiKey := args[0]
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "Error: API key cannot be empty")
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	cfg.APIKey = apiKey

	if err := config.Save(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	path, _ := config.ResolvePath()
	fmt.Printf("API key saved to %s\n", path)
}
