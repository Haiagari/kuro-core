package formats

import (
	"encoding/json"
	"fmt"
	"os"

	"kuro/cli/internal/client"
)

// PrintScanResultJSON renders a scan result as structured JSON to stdout.
func PrintScanResultJSON(r *client.ScanResult) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(r); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
		os.Exit(1)
	}
}
