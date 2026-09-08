package cmd

import (
	"fmt"
	"os"
)

// RunUp handles the `kuro up` command.
// In Kuro Core, server orchestrations belong to Kuro Enterprise.
func RunUp(args []string) {
	fmt.Fprintln(os.Stderr, "Error: 'kuro up' belongs to Kuro Enterprise.")
	fmt.Fprintln(os.Stderr, "Kuro Core is a local-first, standalone gate and does not run server daemons.")
	fmt.Fprintln(os.Stderr, "For server deployments, multi-tenant APIs, and dashboards, see:")
	fmt.Fprintln(os.Stderr, "  https://github.com/Haiagari/kuro-enterprise")
	os.Exit(1)
}
