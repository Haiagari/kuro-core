package tui


import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"kuro/cli/internal/orchestrator"
)

// ── Styles ──────────────────────────────────────────────────

var (
	styleBold  = lipgloss.NewStyle().Bold(true)
	styleGreen = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	styleRed   = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	styleDim   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

// phaseStatusIcon returns a status icon for a phase.
func phaseStatusIcon(status string) string {
	switch status {
	case "running":
		return "●"
	case "done":
		return "✓"
	case "skip":
		return "○"
	case "fail":
		return "✗"
	default:
		return "○"
	}
}

// phaseStatusColor returns a lipgloss style for a phase status.
func phaseStatusColor(status string, isActive bool) lipgloss.Style {
	if isActive {
		return styleBold
	}
	switch status {
	case "done":
		return styleGreen
	case "fail":
		return styleRed
	default:
		return lipgloss.NewStyle()
	}
}

// ── Progress bar ────────────────────────────────────────────

// renderProgressBar renders a progress bar as [████░░░░].
// width is the total character width of the bar (including brackets).
func renderProgressBar(fraction float64, width int) string {
	if width < 4 {
		return "[]"
	}
	inner := width - 2 // brackets
	filled := int(fraction * float64(inner))
	if filled > inner {
		filled = inner
	}
	if filled < 0 {
		filled = 0
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", inner-filled)
	if fraction >= 1.0 {
		return styleGreen.Render("[" + bar + "]")
	}
	return "[" + bar + "]"
}

// renderScannerProgress renders per-scanner progress lines.
func renderScannerProgress(progress []scannerProgress, barWidth int) string {
	if len(progress) == 0 {
		return ""
	}
	var lines []string
	for _, sp := range progress {
		fraction := 0.0
		if sp.Total > 0 {
			fraction = float64(sp.Done) / float64(sp.Total)
		}
		bar := renderProgressBar(fraction, barWidth)
		line := fmt.Sprintf("  %s %s  %s — %d/%d scanners",
			bar, styleDim.Render(sp.Name), styleDim.Render(fmt.Sprintf("%.1fs", sp.Elapsed.Seconds())), sp.Done, sp.Total)
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// ── Log panel ───────────────────────────────────────────────

// renderLogPanel renders the log panel.
func renderLogPanel(lines []logLine, maxLines int) string {
	if len(lines) == 0 {
		return ""
	}
	start := 0
	if len(lines) > maxLines {
		start = len(lines) - maxLines
	}
	var sb strings.Builder
	separator := styleDim.Render(strings.Repeat("─", 40))
	sb.WriteString("\n" + separator + "\n")
	for _, l := range lines[start:] {
		ts := l.Timestamp.Format("15:04:05")
		sb.WriteString(fmt.Sprintf("%s %s %s\n", styleDim.Render(ts), styleDim.Render("│"), l.Text))
	}
	return sb.String()
}

// ── Layout ──────────────────────────────────────────────────

const (
	maxLogLines   = 100
	progressWidth = 40
)

// renderLayout composes the full TUI view.
func renderLayout(model Model) string {
	var sb strings.Builder

	// Header
	version := "v1.1.0"
	header := styleBold.Render(fmt.Sprintf("🔍 Kuro — Security Gate %s", version))
	sb.WriteString(header + "\n")
	sb.WriteString(fmt.Sprintf("  Target: %s\n", model.target))
	sb.WriteString(fmt.Sprintf("  Mode:   %s\n", model.mode))
	sb.WriteString("\n")

	// Phase list
	allPhases := []orchestrator.Phase{
		orchestrator.PhaseFetch,
		orchestrator.PhaseScope,
		orchestrator.PhaseScan,
		orchestrator.PhaseAnalyze,
		orchestrator.PhaseDecide,
		orchestrator.PhaseReport,
	}
	for _, phase := range allPhases {
		ps, ok := model.phases[phase]
		if !ok {
			continue
		}
		isActive := phase == model.activePhase && ps.Status == "running"

		var prefix string
		if isActive {
			// ponytail: use a simple spinner frame; bubbletea spinner is
			// handled by the model's spinner field
			prefix = fmt.Sprintf(" %s ", model.spinnerFrame())
		} else {
			prefix = fmt.Sprintf(" %s ", phaseStatusIcon(ps.Status))
		}

		statusStyle := phaseStatusColor(ps.Status, isActive)
		line := fmt.Sprintf("%s%s%s %s",
			prefix,
			statusStyle.Render(string(phase)),
			styleDim.Render(fmt.Sprintf(" %s", ps.Status)),
			styleDim.Render(ps.Message),
		)
		sb.WriteString(line + "\n")

		// Scan sub-panel: expanded under the scan phase when active or done
		if phase == orchestrator.PhaseScan && len(model.progress) > 0 {
			sb.WriteString(renderScannerProgress(model.progress, progressWidth) + "\n")
		}
	}

	// Log panel
	sb.WriteString(renderLogPanel(model.logLines, maxLogLines))

	return sb.String()
}
