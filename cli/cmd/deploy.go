package cmd

import (
	"fmt"
	"os"
)

// RunDeploy handles the `kuro deploy` command.
// In Kuro Core, multi-tenant server deployments belong to Kuro Enterprise.
func RunDeploy(args []string) {
	fmt.Fprintln(os.Stderr, "Error: 'kuro deploy' belongs to Kuro Enterprise.")
	fmt.Fprintln(os.Stderr, "Kuro Core is a local-first, standalone gate and does not require server deployments.")
	fmt.Fprintln(os.Stderr, "For server deployments, multi-tenant APIs, and dashboards, see:")
	fmt.Fprintln(os.Stderr, "  https://github.com/Haiagari/kuro-enterprise")
	os.Exit(1)
}
