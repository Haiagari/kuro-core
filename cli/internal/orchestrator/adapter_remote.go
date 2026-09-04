package orchestrator


import (
	"context"
	"fmt"
	"time"

	"kuro/cli/internal/client"
)

// ── RemoteAdapter ─────────────────────────────────────────

// RemoteAdapter sends scans to the Kuro server via HTTP and waits for results.
type RemoteAdapter struct {
	apiClient *client.Client
}

// NewRemoteAdapter creates a remote adapter with the given HTTP client.
func NewRemoteAdapter(apiClient *client.Client) *RemoteAdapter {
	return &RemoteAdapter{apiClient: apiClient}
}

func (a *RemoteAdapter) Name() string { return "remote" }

// Fetch starts a remote scan and returns the scan ID.
func (a *RemoteAdapter) Fetch(ctx context.Context, target string) (string, error) {
	resp, err := a.apiClient.TriggerScan(target, "")
	if err != nil {
		return "", fmt.Errorf("trigger scan: %w", err)
	}
	return resp.ScanID, nil
}

// Scope in remote mode is not executed locally — the server decides.
func (a *RemoteAdapter) Scope(ctx context.Context, target string) ([]string, error) {
	return nil, nil
}

// Run waits for the remote scan to finish and returns the findings.
func (a *RemoteAdapter) Run(ctx context.Context, target string, _ []string) (RunResult, error) {
	scanID := target
	startTime := time.Now()

	for {
		result, err := a.apiClient.GetScanStatus(scanID)
		if err != nil {
			if time.Since(startTime) < 30*time.Second {
				time.Sleep(5 * time.Second)
				continue
			}
			return RunResult{}, fmt.Errorf("get status: %w", err)
		}

		if isTerminal(result.Status) {
			return RunResult{Findings: convertFindings(result)}, nil
		}

		if time.Since(startTime) > 30*time.Minute {
			return RunResult{}, fmt.Errorf("scan did not complete within 30 minutes")
		}

		time.Sleep(5 * time.Second)
	}
}

func isTerminal(status string) bool {
	switch status {
	case "completed", "failed", "blocked":
		return true
	default:
		return false
	}
}

func convertFindings(r *client.ScanResult) []Finding {
	var findings []Finding
	for _, f := range r.TopFindings {
		findings = append(findings, Finding{
			Scanner:    f.Scanner,
			Severity:   f.Severity,
			Title:      f.Title,
			FilePath:   f.File,
			LineNumber: f.Line,
		})
	}
	return findings
}
