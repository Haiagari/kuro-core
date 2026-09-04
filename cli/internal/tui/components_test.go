package tui

import (
	"testing"
	"time"

	"kuro/cli/internal/orchestrator"
)

func TestRenderProgressBar(t *testing.T) {
	t.Run("renders within bounds", func(t *testing.T) {
		bar := renderProgressBar(0.5, 12)
		if bar == "" {
			t.Error("bar should not be empty")
		}
	})

	t.Run("full bar is green", func(t *testing.T) {
		bar := renderProgressBar(1.0, 10)
		if bar == "" {
			t.Error("bar should not be empty")
		}
	})

	t.Run("zero width returns []", func(t *testing.T) {
		bar := renderProgressBar(0.5, 3)
		if bar != "[]" {
			t.Errorf("expected '[]', got %q", bar)
		}
	})

	t.Run("negative fraction clamped to 0", func(t *testing.T) {
		bar := renderProgressBar(-0.1, 12)
		if bar == "" {
			t.Error("bar should not be empty")
		}
	})

	t.Run("overflow fraction clamped to 1", func(t *testing.T) {
		bar := renderProgressBar(2.0, 12)
		if bar == "" {
			t.Error("bar should not be empty")
		}
	})
}

func TestRenderScannerProgress(t *testing.T) {
	t.Run("empty progress returns empty", func(t *testing.T) {
		out := renderScannerProgress([]scannerProgress{}, 20)
		if out != "" {
			t.Errorf("expected empty, got %q", out)
		}
	})

	t.Run("single scanner", func(t *testing.T) {
		prog := []scannerProgress{
			{Name: "gitleaks", Done: 3, Total: 5, Elapsed: 2 * time.Second},
		}
		out := renderScannerProgress(prog, 20)
		if out == "" {
			t.Error("expected non-empty output")
		}
	})
}

func TestRenderLogPanel(t *testing.T) {
	t.Run("empty log returns empty", func(t *testing.T) {
		out := renderLogPanel(nil, 10)
		if out != "" {
			t.Errorf("expected empty, got %q", out)
		}
	})

	t.Run("capped at max lines", func(t *testing.T) {
		lines := make([]logLine, 150)
		for i := range lines {
			lines[i] = logLine{Timestamp: time.Now(), Text: "test"}
		}
		out := renderLogPanel(lines, 100)
		if out == "" {
			t.Error("expected non-empty output")
		}
	})
}

func TestFmtLogLine(t *testing.T) {
	t.Run("with message", func(t *testing.T) {
		result := fmtLogLine(orchestrator.PhaseEvent{
			Phase:   orchestrator.PhaseFetch,
			Status:  "running",
			Message: "fetching...",
		})
		if result != "fetch: fetching..." {
			t.Errorf("got %q, want %q", result, "fetch: fetching...")
		}
	})

	t.Run("without message", func(t *testing.T) {
		result := fmtLogLine(orchestrator.PhaseEvent{
			Phase:  orchestrator.PhaseScan,
			Status: "done",
		})
		if result != "scan: done" {
			t.Errorf("got %q, want %q", result, "scan: done")
		}
	})
}
