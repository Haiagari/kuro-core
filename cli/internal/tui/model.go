package tui


import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"kuro/cli/internal/orchestrator"
)

// ── Types ───────────────────────────────────────────────────

// phaseState tracks the status and message of a single pipeline phase.
type phaseState struct {
	Status  string
	Message string
}

// scannerProgress tracks per-scanner progress for the scan phase.
type scannerProgress struct {
	Name    string
	Done    int
	Total   int
	Elapsed time.Duration
}

// logLine is a single entry in the log buffer.
type logLine struct {
	Timestamp time.Time
	Text      string
}

// spinnerTick is sent periodically to animate the spinner.
type spinnerTick struct{}

// Model is the Bubbletea TUI model for the scan pipeline.
type Model struct {
	target string
	mode   string

	phases      map[orchestrator.Phase]*phaseState
	activePhase orchestrator.Phase
	progress    []scannerProgress
	logLines    []logLine

	eventsCh  <-chan orchestrator.PhaseEvent
	ctx       context.Context
	cancel    context.CancelFunc
	done      bool
	width     int
	height    int

	// Simple inline spinner state
	spinnerIndex int
}

// spinnerFrames is a simple dot animation sequence.
// ponytail: inline spinner instead of bubbles library dep
var spinnerFrames = []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"}

var allPhases = []orchestrator.Phase{
	orchestrator.PhaseFetch,
	orchestrator.PhaseScope,
	orchestrator.PhaseScan,
	orchestrator.PhaseAnalyze,
	orchestrator.PhaseDecide,
	orchestrator.PhaseReport,
}

// NewModel creates a new TUI model. The ctx and cancel are shared with the
// scan orchestrator, so Ctrl-C cancels the underlying scan rather than only
// quitting the TUI.
func NewModel(target, mode string, eventsCh <-chan orchestrator.PhaseEvent, ctx context.Context, cancel context.CancelFunc) Model {
	m := Model{
		target:   target,
		mode:     mode,
		phases:   make(map[orchestrator.Phase]*phaseState, len(allPhases)),
		progress: nil,
		logLines: make([]logLine, 0, maxLogLines),
		eventsCh: eventsCh,
		ctx:      ctx,
		cancel:   cancel,
		done:     false,
		width:    80,
		height:   24,
	}
	// Initialize all phases with "pending" status
	for _, p := range allPhases {
		m.phases[p] = &phaseState{Status: "pending", Message: ""}
	}
	return m
}

// Init initializes the model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		spinnerCmd(),
		waitForEvent(m.ctx, m.eventsCh),
	)
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case orchestrator.PhaseEvent:
		m.handlePhaseEvent(msg)
		if m.done {
			return m, tea.Quit
		}
		return m, waitForEvent(m.ctx, m.eventsCh)

	case nil:
		// Channel closed — orchestrator finished
		m.done = true
		return m, tea.Quit

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			m.cancel()
			m.done = true
			return m, tea.Quit
		}
		return m, nil

	case spinnerTick:
		m.spinnerIndex = (m.spinnerIndex + 1) % len(spinnerFrames)
		return m, spinnerCmd()

	default:
		return m, nil
	}
}

// View renders the model.
func (m Model) View() string {
	return renderLayout(m)
}

// spinnerFrame returns the current spinner frame.
func (m Model) spinnerFrame() string {
	return spinnerFrames[m.spinnerIndex]
}

// spinnerCmd returns a tea.Cmd that ticks for the spinner animation.
func spinnerCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return spinnerTick{}
	})
}

// handlePhaseEvent updates the model state from a PhaseEvent.
func (m *Model) handlePhaseEvent(evt orchestrator.PhaseEvent) {
	ps, ok := m.phases[evt.Phase]
	if !ok {
		ps = &phaseState{}
		m.phases[evt.Phase] = ps
	}
	ps.Status = evt.Status
	ps.Message = evt.Message

	m.activePhase = evt.Phase

	// Add to log buffer
	m.logLines = append(m.logLines, logLine{
		Timestamp: evt.Timestamp,
		Text:      fmtLogLine(evt),
	})
	// Cap log buffer
	if len(m.logLines) > maxLogLines {
		m.logLines = m.logLines[len(m.logLines)-maxLogLines:]
	}

	// Handle scan phase completion — populate scanner progress
	if evt.Phase == orchestrator.PhaseScan && evt.Status == "done" {
		// ponytail: per-scanner progress from timings; no streaming events
		// from the adapter yet
		m.progress = append(m.progress, scannerProgress{
			Name:    string(evt.Phase),
			Done:    1,
			Total:   1,
			Elapsed: 0,
		})
	}

	// Mark done when report phase completes
	if evt.Phase == orchestrator.PhaseReport && evt.Status == "done" {
		m.done = true
	}
}

// fmtLogLine formats a PhaseEvent as a log line.
func fmtLogLine(evt orchestrator.PhaseEvent) string {
	if evt.Message != "" {
		return string(evt.Phase) + ": " + evt.Message
	}
	return string(evt.Phase) + ": " + evt.Status
}
