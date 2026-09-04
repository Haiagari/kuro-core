package main

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

// ScannerStage represents an individual scanner running in the pipeline.
type ScannerStage struct {
	Name        string
	Category    string // "Secrets", "SAST", "SCA", "IaC"
	Status      string // "pending", "running", "passed", "blocked"
	Duration    time.Duration
	Findings    int
}

// LiveTelemetryReporter streams real-time terminal progress to the developer during git push.
type LiveTelemetryReporter struct {
	mu      sync.Mutex
	writer  io.Writer
	stages  []ScannerStage
	useANSI bool
}

// NewLiveTelemetryReporter creates a progress reporter.
func NewLiveTelemetryReporter(w io.Writer, useANSI bool) *LiveTelemetryReporter {
	return &LiveTelemetryReporter{
		writer:  w,
		useANSI: useANSI,
		stages: []ScannerStage{
			{Name: "Gitleaks", Category: "Secrets", Status: "pending"},
			{Name: "Semgrep", Category: "SAST", Status: "pending"},
			{Name: "Trivy", Category: "SCA / CVE", Status: "pending"},
			{Name: "Checkov", Category: "IaC / Cloud", Status: "pending"},
		},
	}
}

// FormatSidebandPacket constructs a Git Smart HTTP sideband pkt-line (channel 2 = progress message).
func FormatSidebandPacket(channel byte, message string) []byte {
	line := fmt.Sprintf("%c%s", channel, message)
	length := len(line) + 4
	return []byte(fmt.Sprintf("%04x%s", length, line))
}

// EmitBanner prints the Kuro Security Pipeline banner.
func (r *LiveTelemetryReporter) EmitBanner(repoPath, commitSHA, branch string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	shortSHA := commitSHA
	if len(shortSHA) > 8 {
		shortSHA = shortSHA[:8]
	}

	r.writeRemote("╭────────────────────────────────────────────────────────────╮")
	r.writeRemote("│             KURO Zero-Trust Security Pipeline              │")
	r.writeRemote(fmt.Sprintf("│  Repo: %-25s Commit: %-15s │", truncate(repoPath, 25), shortSHA))
	r.writeRemote("╰────────────────────────────────────────────────────────────╯")
	r.writeRemote("⏳ Orchestrating multi-scanner analysis...")
}

// UpdateStage updates the state of a scanner and renders progress.
func (r *LiveTelemetryReporter) UpdateStage(name, status string, duration time.Duration, findings int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := range r.stages {
		if strings.EqualFold(r.stages[i].Name, name) {
			r.stages[i].Status = status
			r.stages[i].Duration = duration
			r.stages[i].Findings = findings

			var symbol, color string
			switch status {
			case "passed":
				symbol = "✔"
				color = "\033[32m" // green
			case "blocked":
				symbol = "✖"
				color = "\033[31m" // red
			case "running":
				symbol = "⟳"
				color = "\033[33m" // yellow
			default:
				symbol = "•"
				color = "\033[90m"
			}

			if r.useANSI {
				r.writeRemote(fmt.Sprintf("  %s[%s] %-10s (%-10s) - %s (%.2fs)\033[0m",
					color, symbol, r.stages[i].Name, r.stages[i].Category, statusText(status, findings), duration.Seconds()))
			} else {
				r.writeRemote(fmt.Sprintf("  [%s] %-10s (%-10s) - %s (%.2fs)",
					symbol, r.stages[i].Name, r.stages[i].Category, statusText(status, findings), duration.Seconds()))
			}
			break
		}
	}
}

// EmitVerdict renders the final policy decision to the git terminal.
func (r *LiveTelemetryReporter) EmitVerdict(approved bool, totalFindings int, totalDuration time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if approved {
		if r.useANSI {
			r.writeRemote(fmt.Sprintf("\n\033[32m🛡️  [KURO VERDICT] APPROVED: All security gates satisfied in %.2fs\033[0m", totalDuration.Seconds()))
			r.writeRemote("\033[32m   Attestation: Cryptographically signed & verified\033[0m\n")
		} else {
			r.writeRemote(fmt.Sprintf("\n[KURO VERDICT] APPROVED: All security gates satisfied in %.2fs", totalDuration.Seconds()))
			r.writeRemote("   Attestation: Cryptographically signed & verified\n")
		}
	} else {
		if r.useANSI {
			r.writeRemote(fmt.Sprintf("\n\033[31m🚫 [KURO VERDICT] BLOCKED: %d policy violation(s) detected in %.2fs\033[0m", totalFindings, totalDuration.Seconds()))
			r.writeRemote("\033[33m   Run 'kuro fix' in your terminal to remediate or suppress.\033[0m\n")
		} else {
			r.writeRemote(fmt.Sprintf("\n[KURO VERDICT] BLOCKED: %d policy violation(s) detected in %.2fs", totalFindings, totalDuration.Seconds()))
			r.writeRemote("   Run 'kuro fix' in your terminal to remediate or suppress.\n")
		}
	}
}

func (r *LiveTelemetryReporter) writeRemote(msg string) {
	pkt := FormatSidebandPacket(2, "remote: "+msg+"\n")
	_, _ = r.writer.Write(pkt)
}

func statusText(status string, findings int) string {
	switch status {
	case "passed":
		return "0 findings"
	case "blocked":
		return fmt.Sprintf("%d findings", findings)
	case "running":
		return "analyzing..."
	default:
		return "pending"
	}
}

func truncate(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen-3] + "..."
	}
	return s
}
