package cmd

import (
	"errors"
	"testing"

	"kuro/cli/internal/orchestrator"
)

func TestDecisionExitCode(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"pass", 0},
		{"PASS", 0},
		{"approved", 0},
		{"ok", 0},
		{"review", 2},
		{"REVIEW", 2},
		{"block", 1},
		{"blocked", 1},
		{"", 1},
		{"weird", 1},
	}
	for _, tc := range cases {
		if got := decisionExitCode(tc.in); got != tc.want {
			t.Errorf("decisionExitCode(%q)=%d want %d", tc.in, got, tc.want)
		}
	}
}

func TestScanOutputDecisionExitCodes(t *testing.T) {
	mk := func(d string) *orchestrator.ScanResult {
		return &orchestrator.ScanResult{Target: "/t", Mode: "local", Status: "completed", Decision: d}
	}
	modes := []struct {
		name string
		json bool
		tui  bool
	}{
		{"json", true, false},
		{"tui", false, true},
		{"text", false, false},
	}
	for _, m := range modes {
		t.Run(m.name+"/pass", func(t *testing.T) {
			if code := scanOutput(mk("pass"), nil, m.json, m.tui); code != 0 {
				t.Fatalf("got %d want 0", code)
			}
		})
		t.Run(m.name+"/review", func(t *testing.T) {
			if code := scanOutput(mk("review"), nil, m.json, m.tui); code != 2 {
				t.Fatalf("got %d want 2", code)
			}
		})
		t.Run(m.name+"/block", func(t *testing.T) {
			if code := scanOutput(mk("block"), nil, m.json, m.tui); code != 1 {
				t.Fatalf("got %d want 1", code)
			}
		})
	}
	t.Run("nil result with error json", func(t *testing.T) {
		if code := scanOutput(nil, errors.New("boom"), true, false); code != 1 {
			t.Fatalf("got %d want 1", code)
		}
	})
}
