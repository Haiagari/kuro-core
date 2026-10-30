package output

import (
	"testing"

	"kuro/cli/internal/client"
)

func TestPrintScanResult(t *testing.T) {
	result := &client.ScanResult{
		ScanID:   "scan-test-1",
		Status:   "completed",
		Decision: "pass",
	}

	// Should not panic in either mode
	PrintScanResult(result, true)
	PrintScanResult(result, false)
}
