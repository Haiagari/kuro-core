package cmd

import (
	"flag"
	"fmt"
	"os"

	"kuro/cli/internal/client"
	"kuro/cli/internal/config"
	"kuro/cli/internal/output"
)

// RunStatus handles the `kuro status <scan-id> [--json]` command.
func RunStatus(args []string) {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	jsonFlag := fs.Bool("json", false, "Output in JSON format")

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	scanID := fs.Arg(0)
	if scanID == "" {
		fmt.Fprintln(os.Stderr, "Usage: kuro status <scan-id> [--json]")
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if cfg.APIKey == "" {
		fmt.Fprintln(os.Stderr, "No API key configured. Run 'kuro auth <key>' first.")
		os.Exit(1)
	}

	cl := client.New(cfg.APIURL, cfg.APIKey)

	result, err := cl.GetScanStatus(scanID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if !scanTerminal(result.Status) {
		fmt.Fprintf(os.Stderr, "Scan %s is still %s.\n", scanID, result.Status)
		os.Exit(1)
	}

	output.PrintScanResult(result, *jsonFlag)
}

// scanTerminal returns true if the status indicates a completed scan.
func scanTerminal(status string) bool {
	switch status {
	case "completed", "failed", "blocked":
		return true
	default:
		return false
	}
}
