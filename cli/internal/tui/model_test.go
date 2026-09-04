package tui


import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"kuro/cli/internal/orchestrator"
)

// newTestModel builds a model backed by its own context for tests that do not
// exercise cancellation.
func newTestModel(ch <-chan orchestrator.PhaseEvent) Model {
	ctx, cancel := context.WithCancel(context.Background())
	return NewModel("/test", "local", ch, ctx, cancel)
}

func TestLogBufferCap(t *testing.T) {
	ch := make(chan orchestrator.PhaseEvent, 200)
	m := newTestModel(ch)

	// Send 150 events (over the 100 cap)
	for i := 0; i < 150; i++ {
		m.handlePhaseEvent(orchestrator.PhaseEvent{
			Phase:     orchestrator.PhaseFetch,
			Status:    "running",
			Message:   "event",
			Timestamp: time.Now(),
		})
	}

	if len(m.logLines) > 100 {
		t.Errorf("log buffer exceeded 100 lines: got %d", len(m.logLines))
	}
	if len(m.logLines) == 0 {
		t.Error("log buffer should not be empty")
	}
}

func TestPhaseEventHandling(t *testing.T) {
	ch := make(chan orchestrator.PhaseEvent, 10)
	m := newTestModel(ch)

	phases := []orchestrator.Phase{
		orchestrator.PhaseFetch,
		orchestrator.PhaseScope,
		orchestrator.PhaseScan,
		orchestrator.PhaseAnalyze,
		orchestrator.PhaseDecide,
		orchestrator.PhaseReport,
	}

	// Send running events for all phases
	for _, p := range phases {
		m.handlePhaseEvent(orchestrator.PhaseEvent{
			Phase:  p,
			Status: "running",
		})
		if ps, ok := m.phases[p]; !ok {
			t.Errorf("phase %q not initialized after event", p)
		} else if ps.Status != "running" {
			t.Errorf("phase %q status = %q, want %q", p, ps.Status, "running")
		}
	}

	// Send done events for all phases
	for _, p := range phases {
		m.handlePhaseEvent(orchestrator.PhaseEvent{
			Phase:  p,
			Status: "done",
		})
		if ps, ok := m.phases[p]; !ok {
			t.Errorf("phase %q not found after done event", p)
		} else if ps.Status != "done" {
			t.Errorf("phase %q status = %q, want %q", p, ps.Status, "done")
		}
	}
}

func TestScannerProgress(t *testing.T) {
	ch := make(chan orchestrator.PhaseEvent, 10)
	m := newTestModel(ch)

	// Complete scan phase with done status
	m.handlePhaseEvent(orchestrator.PhaseEvent{
		Phase:  orchestrator.PhaseScan,
		Status: "done",
		Message: "5 findings",
	})

	if len(m.progress) == 0 {
		t.Fatal("expected scanner progress after scan done event")
	}
	if m.progress[0].Name != "scan" {
		t.Errorf("progress name = %q, want %q", m.progress[0].Name, "scan")
	}
}

func TestModelDoneOnReport(t *testing.T) {
	ch := make(chan orchestrator.PhaseEvent, 10)
	m := newTestModel(ch)

	if m.done {
		t.Fatal("model should not be done initially")
	}

	m.handlePhaseEvent(orchestrator.PhaseEvent{
		Phase:  orchestrator.PhaseReport,
		Status: "done",
	})

	if !m.done {
		t.Fatal("model should be done after report phase completed")
	}
}

func TestModelCtrlC(t *testing.T) {
	ch := make(chan orchestrator.PhaseEvent, 10)
	m := newTestModel(ch)

	// Simulate Ctrl+C
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})

	model := updated.(Model)
	if !model.done {
		t.Fatal("model should be done after Ctrl+C")
	}
	if cmd == nil {
		t.Fatal("expected tea.Quit command after Ctrl+C")
	}
}

func TestModelCtrlCCancelsSharedScanContext(t *testing.T) {
	ch := make(chan orchestrator.PhaseEvent, 10)
	ctx, cancel := context.WithCancel(context.Background())
	m := NewModel("/test", "local", ch, ctx, cancel)

	// Simulate Ctrl+C
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})

	if ctx.Err() != context.Canceled {
		t.Error("expected the shared scan context to be canceled after Ctrl+C")
	}
}

func TestProgressBarRendering(t *testing.T) {
	tests := []struct {
		name     string
		fraction float64
		width    int
	}{
		{"zero progress", 0.0, 20},
		{"half progress", 0.5, 20},
		{"complete", 1.0, 20},
		{"narrow bar", 0.5, 5},
		{"wide bar", 0.75, 60},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bar := renderProgressBar(tt.fraction, tt.width)
			if bar == "" {
				t.Fatal("bar should not be empty")
			}
			// Should start with [ or contain ANSI escape codes before it
			if !strings.HasPrefix(bar, "[") {
				t.Errorf("bar should start with [, got %q", bar)
			}
			// Should end with ] (assuming no trailing codes beyond what lipgloss adds)
		})
	}
}

func TestLogPanelCap(t *testing.T) {
	// Verify the maxLogLines constant
	if maxLogLines != 100 {
		t.Errorf("maxLogLines = %d, want 100", maxLogLines)
	}
}

func TestSpinnerFrame(t *testing.T) {
	ch := make(chan orchestrator.PhaseEvent, 10)
	m := newTestModel(ch)

	frame := m.spinnerFrame()
	if frame == "" {
		t.Error("spinner frame should not be empty")
	}

	// Spinner should cycle
	m.spinnerIndex = (m.spinnerIndex + 1) % len(spinnerFrames)
	frame2 := m.spinnerFrame()
	if frame2 == frame {
		t.Error("spinner should produce different frames on cycle")
	}
}

func TestPhaseStatusIcon(t *testing.T) {
	tests := []struct {
		status string
		want   string
	}{
		{"running", "●"},
		{"done", "✓"},
		{"skip", "○"},
		{"fail", "✗"},
		{"unknown", "○"},
	}

	for _, tt := range tests {
		got := phaseStatusIcon(tt.status)
		if got != tt.want {
			t.Errorf("phaseStatusIcon(%q) = %q, want %q", tt.status, got, tt.want)
		}
	}
}
