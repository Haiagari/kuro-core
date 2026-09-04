package output

import (
	"kuro/cli/internal/client"
	"kuro/cli/internal/output/formats"
)

// PrintScanResult dispatches to the appropriate output format based on jsonFlag.
func PrintScanResult(r *client.ScanResult, jsonFlag bool) {
	if jsonFlag {
		formats.PrintScanResultJSON(r)
	} else {
		formats.PrintScanResult(r)
	}
}
