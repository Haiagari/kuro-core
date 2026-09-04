package cmd


import (
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"kuro/cli/internal/orchestrator"
)

func TestScanTerminal(t *testing.T) {
	tests := []struct {
		status   string
		expected bool
	}{
		{"completed", true},
		{"failed", true},
		{"blocked", true},
		{"", false},
		{"pending", false},
		{"running", false},
		{"queued", false},
		{"in_progress", false},
		{"COMPLETED", false}, // case-sensitive
	}

	for _, tc := range tests {
		t.Run(tc.status, func(t *testing.T) {
			got := scanTerminal(tc.status)
			if got != tc.expected {
				t.Errorf("scanTerminal(%q) = %v, want %v", tc.status, got, tc.expected)
			}
		})
	}
}

// tuiMode represents the detected display mode for testing.
type tuiMode int

const (
	modeText tuiMode = iota
	modeTUI
	modeJSON
)

// detectMode simulates the priority routing logic from scan.go.
func detectMode(jsonSet, tuiSet, tuiValue bool, isTTY bool) tuiMode {
	if jsonSet {
		return modeJSON
	}
	if tuiSet && tuiValue {
		return modeTUI
	}
	if !tuiSet && isTTY {
		return modeTUI
	}
	return modeText
}

func TestTuiFlagPriority(t *testing.T) {
	tests := []struct {
		name     string
		jsonSet  bool
		tuiSet   bool
		tuiValue bool
		isTTY    bool
		want     tuiMode
	}{
		{"json alone", true, false, false, false, modeJSON},
		{"json with tui", true, true, true, false, modeJSON},
		{"json on tty", true, false, false, true, modeJSON},
		{"json and tui on tty", true, true, true, true, modeJSON},
		{"tui explicit", false, true, true, false, modeTUI},
		{"tui on tty", false, true, true, true, modeTUI},
		{"tui=false on tty", false, true, false, true, modeText},
		{"no flags on tty (auto)", false, false, false, true, modeTUI},
		{"no flags non-tty", false, false, false, false, modeText},
		{"tui non-tty", false, true, true, false, modeTUI},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectMode(tt.jsonSet, tt.tuiSet, tt.tuiValue, tt.isTTY)
			if got != tt.want {
				t.Errorf("detectMode(json=%v, tuiSet=%v, tuiVal=%v, tty=%v) = %v, want %v",
					tt.jsonSet, tt.tuiSet, tt.tuiValue, tt.isTTY, got, tt.want)
			}
		})
	}
}

// failedScanResult is a ScanResult with Status "failed", as returned by
// orchestrator.Run on any phase failure.
func failedScanResult() *orchestrator.ScanResult {
	return &orchestrator.ScanResult{
		Target: "/tmp/foo",
		Mode:   "local",
		Status: "failed",
	}
}

// captureStdout runs fn with os.Stdout redirected and returns what it printed.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()

	fn()
	w.Close()
	out, _ := io.ReadAll(r)
	os.Stdout = old
	return string(out)
}

func TestScanOutputJSONFailure(t *testing.T) {
	out := captureStdout(t, func() {
		code := scanOutput(failedScanResult(), errors.New("fetch failed: no such file or directory"), true, false)
		if code != 1 {
			t.Errorf("scanOutput JSON failure exit code = %d, want 1", code)
		}
	})

	if !strings.Contains(out, `"error"`) {
		t.Errorf("expected JSON output to include an error field, got: %s", out)
	}
	if !strings.Contains(out, `"status": "failed"`) {
		t.Errorf("expected JSON output status failed, got: %s", out)
	}
}

func TestScanOutputTUIFailure(t *testing.T) {
	code := scanOutput(failedScanResult(), errors.New("scan failed: boom"), false, true)
	if code != 1 {
		t.Errorf("scanOutput TUI failure exit code = %d, want 1", code)
	}
}

func TestScanOutputTextFailure(t *testing.T) {
	code := scanOutput(failedScanResult(), errors.New("scan failed: boom"), false, false)
	if code != 1 {
		t.Errorf("scanOutput text failure exit code = %d, want 1", code)
	}
}

func TestScanOutputSuccessExitCodeZero(t *testing.T) {
	result := &orchestrator.ScanResult{
		Target:   "/tmp/foo",
		Mode:     "local",
		Status:   "completed",
		Decision: "pass",
	}

	modes := []struct {
		name     string
		jsonMode bool
		tuiMode  bool
	}{
		{"json", true, false},
		{"tui", false, true},
		{"text", false, false},
	}
	for _, tt := range modes {
		t.Run(tt.name, func(t *testing.T) {
			if code := scanOutput(result, nil, tt.jsonMode, tt.tuiMode); code != 0 {
				t.Errorf("scanOutput %s success exit code = %d, want 0", tt.name, code)
			}
		})
	}
}

func TestReorderFlagsBeforeArgs(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{"flags first unchanged", []string{"--json", "./repo"}, []string{"--json", "./repo"}},
		{"flag after path", []string{"./repo", "--json"}, []string{"--json", "./repo"}},
		{"mixed", []string{"./repo", "--json", "--history"}, []string{"--json", "--history", "./repo"}},
		{"double dash stops", []string{"./repo", "--", "--json"}, []string{"./repo", "--json"}},
		{"empty", nil, []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := reorderFlagsBeforeArgs(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("len=%d want %d (%v vs %v)", len(got), len(tt.want), got, tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Fatalf("got %v want %v", got, tt.want)
				}
			}
		})
	}
}
