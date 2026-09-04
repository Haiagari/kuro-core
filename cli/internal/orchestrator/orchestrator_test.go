package orchestrator


import (
	"context"
	"testing"
)

// ── Mock adapter for orchestrator tests ────────────────────

type mockAdapter struct {
	name      string
	fetchErr  error
	scopeRes  []string
	scopeErr  error
	runRes    []Finding
	runErr    error
}

func (m *mockAdapter) Name() string                         { return m.name }
func (m *mockAdapter) Fetch(_ context.Context, _ string) (string, error) { return "/tmp/test", m.fetchErr }
func (m *mockAdapter) Scope(_ context.Context, _ string) ([]string, error)  { return m.scopeRes, m.scopeErr }
func (m *mockAdapter) Run(_ context.Context, _ string, _ []string) (RunResult, error) {
	return RunResult{Findings: m.runRes}, m.runErr
}

// ── Tests ──────────────────────────────────────────────────

func TestNew(t *testing.T) {
	t.Run("New has nil eventsCh", func(t *testing.T) {
		o := New(&mockAdapter{}, false)
		if o.eventsCh != nil {
			t.Error("expected eventsCh to be nil from New()")
		}
	})
	t.Run("NewWithEvents sets eventsCh", func(t *testing.T) {
		ch := make(chan PhaseEvent, 64)
		o := NewWithEvents(&mockAdapter{}, false, ch)
		if o.eventsCh == nil {
			t.Error("expected eventsCh to be non-nil from NewWithEvents()")
		}
	})
	t.Run("NewWithEvents preserves adapter and verbose", func(t *testing.T) {
		ch := make(chan PhaseEvent, 64)
		o := NewWithEvents(&mockAdapter{name: "local"}, true, ch)
		if o.adapter.Name() != "local" {
			t.Errorf("expected adapter name 'local', got %q", o.adapter.Name())
		}
		if !o.verbose {
			t.Error("expected verbose=true")
		}
	})
	t.Run("creates orchestrator with local adapter", func(t *testing.T) {
		adapter := &mockAdapter{name: "local"}
		o := New(adapter, false)
		if o == nil {
			t.Fatal("New returned nil")
		}
		if o.adapter.Name() != "local" {
			t.Errorf("expected adapter name 'local', got %q", o.adapter.Name())
		}
		if o.verbose != false {
			t.Errorf("expected verbose=false, got %v", o.verbose)
		}
	})

	t.Run("creates orchestrator with remote adapter", func(t *testing.T) {
		adapter := &mockAdapter{name: "remote"}
		o := New(adapter, true)
		if o == nil {
			t.Fatal("New returned nil")
		}
		if o.adapter.Name() != "remote" {
			t.Errorf("expected adapter name 'remote', got %q", o.adapter.Name())
		}
		if o.verbose != true {
			t.Errorf("expected verbose=true, got %v", o.verbose)
		}
	})
}

func TestAnalyze(t *testing.T) {
	tests := []struct {
		name     string
		findings []Finding
		want     string
	}{
		{
			name:     "empty findings returns pass",
			findings: []Finding{},
			want:     "pass",
		},
		{
			name: "critical finding returns block",
			findings: []Finding{
				{Severity: "CRITICAL", Title: "CVE-2024-1234"},
			},
			want: "block",
		},
		{
			name: "high finding returns review",
			findings: []Finding{
				{Severity: "HIGH", Title: "XSS vulnerability"},
			},
			want: "review",
		},
		{
			name: "medium finding returns pass",
			findings: []Finding{
				{Severity: "MEDIUM", Title: "Style issue"},
			},
			want: "pass",
		},
		{
			name: "low severity returns pass",
			findings: []Finding{
				{Severity: "LOW", Title: "Minor warning"},
			},
			want: "pass",
		},
		{
			name: "mixed severity with critical returns block",
			findings: []Finding{
				{Severity: "MEDIUM", Title: "Style issue"},
				{Severity: "HIGH", Title: "XSS vulnerability"},
				{Severity: "CRITICAL", Title: "CVE-2024-1234"},
			},
			want: "block",
		},
		{
			name: "mixed severity with high but no critical returns review",
			findings: []Finding{
				{Severity: "LOW", Title: "Minor"},
				{Severity: "MEDIUM", Title: "Style"},
				{Severity: "HIGH", Title: "XSS vulnerability"},
			},
			want: "review",
		},
		{
			name: "multiple criticals still returns block",
			findings: []Finding{
				{Severity: "CRITICAL", Title: "CVE-1"},
				{Severity: "CRITICAL", Title: "CVE-2"},
			},
			want: "block",
		},
		{
			name: "multiple highs returns review",
			findings: []Finding{
				{Severity: "HIGH", Title: "Issue 1"},
				{Severity: "HIGH", Title: "Issue 2"},
			},
			want: "review",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := New(&mockAdapter{}, false)
			got := o.analyze(tt.findings)
			if got != tt.want {
				t.Errorf("analyze() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVerboseMode(t *testing.T) {
	t.Run("New with verbose=false", func(t *testing.T) {
		o := New(&mockAdapter{}, false)
		if o.verbose {
			t.Error("expected verbose=false")
		}
	})

	t.Run("New with verbose=true", func(t *testing.T) {
		o := New(&mockAdapter{}, true)
		if !o.verbose {
			t.Error("expected verbose=true")
		}
	})
}

// ── Helper tests ───────────────────────────────────────────

func TestTruncate(t *testing.T) {
	tests := []struct {
		name string
		s    string
		n    int
		want string
	}{
		{"short string unchanged", "hello", 10, "hello"},
		{"exact length unchanged", "hello", 5, "hello"},
		{"long string truncated", "hello world this is long", 10, "hello worl..."},
		{"empty string", "", 5, ""},
		{"zero length", "hello", 0, "..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.s, tt.n)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.s, tt.n, got, tt.want)
			}
		})
	}
}

func TestJoinScanners(t *testing.T) {
	tests := []struct {
		name     string
		scanners []string
		want     string
	}{
		{"multiple scanners", []string{"gitleaks", "semgrep", "trivy"}, "gitleaks, semgrep, trivy"},
		{"single scanner", []string{"gitleaks"}, "gitleaks"},
		{"empty slice", []string{}, ""},
		{"nil slice", nil, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := joinScanners(tt.scanners)
			if got != tt.want {
				t.Errorf("joinScanners(%v) = %q, want %q", tt.scanners, got, tt.want)
			}
		})
	}
}

func TestCountBySeverity(t *testing.T) {
	tests := []struct {
		name     string
		findings []Finding
		want     map[string]int
	}{
		{
			name: "mixed severities",
			findings: []Finding{
				{Severity: "CRITICAL"},
				{Severity: "HIGH"},
				{Severity: "HIGH"},
				{Severity: "MEDIUM"},
			},
			want: map[string]int{"CRITICAL": 1, "HIGH": 2, "MEDIUM": 1},
		},
		{
			name:     "empty findings",
			findings: []Finding{},
			want:     map[string]int{},
		},
		{
			name: "single severity",
			findings: []Finding{
				{Severity: "CRITICAL"},
				{Severity: "CRITICAL"},
			},
			want: map[string]int{"CRITICAL": 2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countBySeverity(tt.findings)
			if len(got) != len(tt.want) {
				t.Errorf("countBySeverity() len = %d, want %d", len(got), len(tt.want))
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("countBySeverity()[%q] = %d, want %d", k, got[k], v)
				}
			}
		})
	}
}

// ── Run tests (with mock adapter) ──────────────────────────

func TestRunSuccess(t *testing.T) {
	adapter := &mockAdapter{
		name:     "local",
		scopeRes: []string{"gitleaks"},
		runRes: []Finding{
			{Scanner: "gitleaks", Severity: "MEDIUM", Title: "test finding"},
		},
	}
	o := New(adapter, false)
	result, err := o.Run(context.Background(), "/tmp/test")
	if err != nil {
		t.Fatalf("Run() unexpected error: %v", err)
	}
	if result.Status != "completed" {
		t.Errorf("expected status 'completed', got %q", result.Status)
	}
	if result.Decision != "pass" {
		t.Errorf("expected decision 'pass', got %q", result.Decision)
	}
	if len(result.Findings) != 1 {
		t.Errorf("expected 1 finding, got %d", len(result.Findings))
	}
}

func TestRunFetchFailure(t *testing.T) {
	adapter := &mockAdapter{
		name:     "local",
		fetchErr: context.DeadlineExceeded,
	}
	o := New(adapter, false)
	_, err := o.Run(context.Background(), "/bad/path")
	if err == nil {
		t.Fatal("expected error from fetch failure")
	}
}

func TestRunScopeFailure(t *testing.T) {
	adapter := &mockAdapter{
		name:     "local",
		scopeErr: context.DeadlineExceeded,
	}
	o := New(adapter, false)
	_, err := o.Run(context.Background(), "/tmp/test")
	if err == nil {
		t.Fatal("expected error from scope failure")
	}
}

func TestRunEmptyScanners(t *testing.T) {
	adapter := &mockAdapter{
		name:     "local",
		scopeRes: []string{},
		runRes:   []Finding{},
	}
	o := New(adapter, false)
	result, err := o.Run(context.Background(), "/tmp/test")
	if err != nil {
		t.Fatalf("Run() unexpected error: %v", err)
	}
	if result.Status != "completed" {
		t.Errorf("expected status 'completed', got %q", result.Status)
	}
}

func TestRunScanFailure(t *testing.T) {
	adapter := &mockAdapter{
		name:     "local",
		scopeRes: []string{"gitleaks"},
		runErr:   context.DeadlineExceeded,
	}
	o := New(adapter, false)
	_, err := o.Run(context.Background(), "/tmp/test")
	if err == nil {
		t.Fatal("expected error from scan failure")
	}
}

func TestReportPhaseNonBlocking(t *testing.T) {
	// Buffer=0 means send blocks unless someone receives. The non-blocking
	// send should drop events rather than block the pipeline.
	ch := make(chan PhaseEvent, 0)
	o := NewWithEvents(&mockAdapter{name: "local", scopeRes: []string{"gitleaks"}}, false, ch)

	// Consume events in background to avoid deadlock on the few that succeed
	done := make(chan struct{})
	go func() {
		for range ch {
		}
		close(done)
	}()

	_, err := o.Run(context.Background(), "/tmp/test")
	if err != nil {
		t.Fatalf("Run() should complete without deadlock, got: %v", err)
	}
	close(ch)
	<-done
}

func TestReportPhaseOutputIdentical(t *testing.T) {
	// New (no eventsCh) and NewWithEvents (with consumed events) must
	// produce identical verbose text output.
	adapter := &mockAdapter{
		name:     "local",
		scopeRes: []string{"gitleaks"},
	}
	o1 := New(adapter, true)
	eventsCh := make(chan PhaseEvent, 64)
	o2 := NewWithEvents(adapter, true, eventsCh)

	// Consume o2 events from the bidirectional channel
	go func() {
		for range eventsCh {
		}
	}()

	result1, err1 := o1.Run(context.Background(), "/tmp/test")
	result2, err2 := o2.Run(context.Background(), "/tmp/test")

	if err1 != nil {
		t.Fatalf("o1 Run() error: %v", err1)
	}
	if err2 != nil {
		t.Fatalf("o2 Run() error: %v", err2)
	}
	if result1.Status != result2.Status {
		t.Errorf("status mismatch: %q vs %q", result1.Status, result2.Status)
	}
	if result1.Decision != result2.Decision {
		t.Errorf("decision mismatch: %q vs %q", result1.Decision, result2.Decision)
	}
	if len(result1.Findings) != len(result2.Findings) {
		t.Errorf("findings count mismatch: %d vs %d", len(result1.Findings), len(result2.Findings))
	}
}
