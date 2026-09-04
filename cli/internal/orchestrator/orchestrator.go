package orchestrator


import (
	"context"
	"fmt"
	"time"
)

// ── PhaseEvent ──────────────────────────────────────────────

// PhaseEvent is emitted by the orchestrator at each phase transition.
// Consumers (e.g. TUI) receive these through the events channel.
type PhaseEvent struct {
	Phase     Phase     `json:"phase"`
	Status    string    `json:"status"` // "running", "done", "skip", "fail"
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// ── Shared types ──────────────────────────────────────

// Finding is the result of a single scanner.
type Finding struct {
	Scanner     string `json:"scanner"`
	Severity    string `json:"severity"`
	Title       string `json:"title"`
	FilePath    string `json:"file_path"`
	LineNumber  int    `json:"line_number"`
	Description string `json:"description"`
	Verified    bool   `json:"verified"`
}

// ScannerTiming associates a scanner with its duration and finding count.
type ScannerTiming struct {
	Name     string        `json:"name"`
	Findings int           `json:"findings"`
	Duration time.Duration `json:"duration"`
}

// RunResult combines findings and per-scanner timing returned by an Adapter.
type RunResult struct {
	Findings []Finding
	Timings  []ScannerTiming
}

// ScanResult is the complete result of an orchestrated scan.
type ScanResult struct {
	Target             string          `json:"target"`
	Mode             string          `json:"mode"` // "local" or "remote"
	Status             string          `json:"status"`
	Decision           string          `json:"decision"`
	Findings           []Finding       `json:"findings"`
	ScannerTimings     []ScannerTiming `json:"scanner_timings,omitempty"`
	FindingsBySeverity map[string]int
	Duration           time.Duration
	StartedAt          time.Time
	FinishedAt         time.Time
}

// ── Orchestrator phases ──────────────────────────────────

// Phase represents a phase of the scan pipeline.
type Phase string

const (
	PhaseFetch    Phase = "fetch"
	PhaseScope    Phase = "scope"
	PhaseScan     Phase = "scan"
	PhaseAnalyze  Phase = "analyze"
	PhaseDecide   Phase = "decide"
	PhaseReport   Phase = "report"
)

// PhaseResult is the result of an individual phase.
type PhaseResult struct {
	Phase   Phase
	Status  string // "running", "done", "skip", "fail"
	Message string
	Error   error
}

// ── Adapter interface ──────────────────────────────────────

// Adapter is the interface that local and remote modes must implement.
type Adapter interface {
	// Name returns the adapter name ("local" or "remote").
	Name() string

	// Fetch prepares the target for scanning.
	// Local mode: verifies the path exists.
	// Remote mode: sends the target to the server.
	Fetch(ctx context.Context, target string) (string, error)

	// Scope determines which scanners to apply based on the target.
	Scope(ctx context.Context, target string) ([]string, error)

	// Run executes the scanners and returns findings with per-scanner timing.
	// Local mode: runs Docker/Podman containers.
	// Remote mode: waits for the server to finish.
	Run(ctx context.Context, target string, scanners []string) (RunResult, error)
}

// ── Orchestrator ────────────────────────────────────────────

// Orchestrator coordinates the scan phases.
type Orchestrator struct {
	adapter   Adapter
	verbose   bool
	eventsCh  chan<- PhaseEvent // nil in text/JSON mode, non-nil for TUI
}

// New creates an orchestrator with the given adapter.
func New(adapter Adapter, verbose bool) *Orchestrator {
	return &Orchestrator{
		adapter:  adapter,
		verbose:  verbose,
		eventsCh: nil,
	}
}

// NewWithEvents creates an orchestrator that sends phase events to eventsCh.
func NewWithEvents(adapter Adapter, verbose bool, eventsCh chan<- PhaseEvent) *Orchestrator {
	return &Orchestrator{
		adapter:  adapter,
		verbose:  verbose,
		eventsCh: eventsCh,
	}
}

// Run executes the complete scan pipeline.
func (o *Orchestrator) Run(ctx context.Context, target string) (*ScanResult, error) {
	result := &ScanResult{
		Target:    target,
		Mode:      o.adapter.Name(),
		StartedAt: time.Now(),
		Status:    "running",
	}

	// ── PHASE 1: FETCH ───────────────────────────────────────
	o.reportPhase(PhaseFetch, "running", "")
	fetchID, err := o.adapter.Fetch(ctx, target)
	if err != nil {
		o.reportPhase(PhaseFetch, "fail", err.Error())
		result.Status = "failed"
		return result, fmt.Errorf("fetch failed: %w", err)
	}
	o.reportPhase(PhaseFetch, "done", fmt.Sprintf("Target ready (%s)", truncate(fetchID, 40)))

	// ── PHASE 2: SCOPE ───────────────────────────────────────
	o.reportPhase(PhaseScope, "running", "")
	scanners, err := o.adapter.Scope(ctx, fetchID)
	if err != nil {
		o.reportPhase(PhaseScope, "fail", err.Error())
		result.Status = "failed"
		return result, fmt.Errorf("scope failed: %w", err)
	}
	if len(scanners) == 0 {
		o.reportPhase(PhaseScope, "skip", "No applicable scanners")
	} else {
		o.reportPhase(PhaseScope, "done", fmt.Sprintf("%d scanners: %s", len(scanners), joinScanners(scanners)))
	}

	// ── PHASE 3: SCAN ────────────────────────────────────────
	o.reportPhase(PhaseScan, "running", "")
	runResult, err := o.adapter.Run(ctx, fetchID, scanners)
	if err != nil {
		o.reportPhase(PhaseScan, "fail", err.Error())
		result.Status = "failed"
		return result, fmt.Errorf("scan failed: %w", err)
	}
	result.Findings = runResult.Findings
	result.ScannerTimings = runResult.Timings
	result.FindingsBySeverity = countBySeverity(runResult.Findings)
	o.reportPhase(PhaseScan, "done", fmt.Sprintf("%d findings", len(runResult.Findings)))

	// ── PHASE 4: ANALYZE ─────────────────────────────────────
	o.reportPhase(PhaseAnalyze, "running", "")
	decision := o.analyze(runResult.Findings)
	result.Decision = decision
	o.reportPhase(PhaseAnalyze, "done", fmt.Sprintf("Decision: %s", decision))

	// ── PHASE 5: DECIDE ──────────────────────────────────────
	o.reportPhase(PhaseDecide, "running", "")
	if decision == "pass" {
		o.reportPhase(PhaseDecide, "done", "✅ PASS — Clean code")
	} else if decision == "review" {
		o.reportPhase(PhaseDecide, "done", "⚠️ REVIEW — Review required")
	} else {
		o.reportPhase(PhaseDecide, "done", "❌ BLOCK — Would block the push")
	}

	// ── PHASE 6: REPORT ──────────────────────────────────────
	o.reportPhase(PhaseReport, "running", "")
	result.FinishedAt = time.Now()
	result.Duration = result.FinishedAt.Sub(result.StartedAt)
	result.Status = "completed"

	if o.verbose && len(result.ScannerTimings) > 0 {
		for _, st := range result.ScannerTimings {
			d := st.Duration.Round(time.Millisecond / 10)
			fmt.Printf("           ├─ %s... %d findings (%s)\n", st.Name, st.Findings, d)
		}
	}
	o.reportPhase(PhaseReport, "done", fmt.Sprintf("Scan completed in %s", result.Duration.Round(time.Millisecond)))

	return result, nil
}

// analyze applies the Policy Engine rules to the findings.
func (o *Orchestrator) analyze(findings []Finding) string {
	hasCritical := false
	hasHigh := false

	for _, f := range findings {
		switch f.Severity {
		case "CRITICAL":
			hasCritical = true
		case "HIGH":
			hasHigh = true
		}
	}

	switch {
	case hasCritical:
		return "block"
	case hasHigh:
		return "review"
	default:
		return "pass"
	}
}

// reportPhase displays the progress of a phase.
func (o *Orchestrator) reportPhase(phase Phase, status, msg string) {
	if o.eventsCh != nil {
		evt := PhaseEvent{
			Phase:     phase,
			Status:    status,
			Message:   msg,
			Timestamp: time.Now(),
		}
		select {
		case o.eventsCh <- evt:
		default:
			// drop if channel full — never block the pipeline
		}
	}
	if o.verbose {
		fmt.Printf("  %-8s │ %s\n", phase, status)
		if msg != "" {
			fmt.Printf("           %s\n", msg)
		}
	}
}

// ── Helpers ────────────────────────────────────────────────

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func joinScanners(scanners []string) string {
	s := ""
	for i, sc := range scanners {
		if i > 0 {
			s += ", "
		}
		s += sc
	}
	return s
}

func countBySeverity(findings []Finding) map[string]int {
	counts := make(map[string]int)
	for _, f := range findings {
		counts[f.Severity]++
	}
	return counts
}
