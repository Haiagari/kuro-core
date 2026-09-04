package formats

import (
	"fmt"
	"strings"
	"time"

	"kuro/cli/internal/client"
)

// severityOrder maps severity to a sortable integer (lower = more severe).
var severityOrder = map[string]int{
	"CRITICAL": 0,
	"HIGH":     1,
	"MEDIUM":   2,
	"LOW":      3,
}

// sortFindings sorts findings by severity (most severe first).
func sortFindings(items []client.TopFindingItem) {
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if severityOrder[items[i].Severity] > severityOrder[items[j].Severity] {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}

// severityEmoji returns a visual indicator for the severity level.
func severityEmoji(severity string) string {
	switch severity {
	case "CRITICAL":
		return "🔴"
	case "HIGH":
		return "🟠"
	case "MEDIUM":
		return "🟡"
	case "LOW":
		return "⚪"
	default:
		return "⚪"
	}
}

// PrintScanResult renders a scan result in human-readable terminal format.
func PrintScanResult(r *client.ScanResult) {
	repoName := r.RepositoryURL
	if repoName == "" {
		repoName = r.ScanID
	}

	// Extract short repo name from URL
	parts := strings.Split(strings.TrimRight(repoName, "/"), "/")
	shortName := parts[len(parts)-1]

	duration := formatDuration(r.Duration)

	policyLabel := r.PolicyDecision
	if policyLabel == "" {
		policyLabel = r.Decision
	}
	if policyLabel == "" {
		policyLabel = "PENDING"
	}

	fmt.Println()
	fmt.Printf("  Scan: %s (%s)\n", shortName, r.Branch)
	fmt.Printf("  Duration: %s\n", duration)
	fmt.Printf("  Policy: %s\n\n", policyLabel)

	// Findings by severity
	fmt.Println("  Findings by severity:")
	for _, sev := range []string{"CRITICAL", "HIGH", "MEDIUM", "LOW"} {
		count := 0
		if r.FindingsBySeverity != nil {
			count = r.FindingsBySeverity[sev]
		}
		fmt.Printf("    %s: %5d\n", sev, count)
	}

	// Top findings
	if len(r.TopFindings) > 0 {
		fmt.Println()
		fmt.Println("  Top findings:")
		sorted := make([]client.TopFindingItem, len(r.TopFindings))
		copy(sorted, r.TopFindings)
		sortFindings(sorted)
		for _, f := range sorted {
			emoji := severityEmoji(f.Severity)
			fmt.Printf("  %s [%s] %s\n", emoji, f.Severity, f.Title)
			fmt.Printf("    File: %s:%d\n", f.File, f.Line)
			fmt.Printf("    Scanner: %s\n", f.Scanner)
			fmt.Println()
		}
	}

	fmt.Println()
}

// formatDuration converts seconds to a human-readable duration string.
func formatDuration(seconds int) string {
	d := time.Duration(seconds) * time.Second
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60

	var parts []string
	if h > 0 {
		parts = append(parts, fmt.Sprintf("%dh", h))
	}
	if m > 0 {
		parts = append(parts, fmt.Sprintf("%dm", m))
	}
	parts = append(parts, fmt.Sprintf("%ds", s))

	return strings.Join(parts, " ")
}
