package formats


import (
	"testing"

	"kuro/cli/internal/client"
)

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		seconds  int
		expected string
	}{
		{0, "0s"},
		{1, "1s"},
		{59, "59s"},
		{60, "1m 0s"},
		{61, "1m 1s"},
		{120, "2m 0s"},
		{3600, "1h 0s"},       // 0m omitted when hours present
		{3661, "1h 1m 1s"},
		{86400, "24h 0s"},     // 0m omitted when hours present
		{90061, "25h 1m 1s"},
	}

	for _, tc := range tests {
		t.Run(tc.expected, func(t *testing.T) {
			got := formatDuration(tc.seconds)
			if got != tc.expected {
				t.Errorf("formatDuration(%d) = %q, want %q", tc.seconds, got, tc.expected)
			}
		})
	}
}

func TestSeverityEmoji(t *testing.T) {
	tests := []struct {
		severity string
		expected string
	}{
		{"CRITICAL", "\U0001f534"}, // 🔴
		{"HIGH", "\U0001f7e0"},     // 🟠
		{"MEDIUM", "\U0001f7e1"},   // 🟡
		{"LOW", "\u26aa"},          // ⚪
		{"UNKNOWN", "\u26aa"},      // ⚪ default
		{"", "\u26aa"},             // ⚪ default
	}

	for _, tc := range tests {
		t.Run(tc.severity, func(t *testing.T) {
			got := severityEmoji(tc.severity)
			if got != tc.expected {
				t.Errorf("severityEmoji(%q) = %q, want %q", tc.severity, got, tc.expected)
			}
		})
	}
}

func TestSeverityOrder(t *testing.T) {
	// Verify severity ordering: CRITICAL < HIGH < MEDIUM < LOW
	if severityOrder["CRITICAL"] >= severityOrder["HIGH"] {
		t.Error("CRITICAL should have lower (more severe) order than HIGH")
	}
	if severityOrder["HIGH"] >= severityOrder["MEDIUM"] {
		t.Error("HIGH should have lower order than MEDIUM")
	}
	if severityOrder["MEDIUM"] >= severityOrder["LOW"] {
		t.Error("MEDIUM should have lower order than LOW")
	}
}

func TestSortFindings(t *testing.T) {
	items := []client.TopFindingItem{
		{Severity: "LOW", Title: "low issue"},
		{Severity: "CRITICAL", Title: "critical issue"},
		{Severity: "HIGH", Title: "high issue"},
		{Severity: "MEDIUM", Title: "medium issue"},
	}

	sortFindings(items)

	expectedOrder := []string{"CRITICAL", "HIGH", "MEDIUM", "LOW"}
	for i, item := range items {
		if item.Severity != expectedOrder[i] {
			t.Errorf("position %d: expected %s, got %s", i, expectedOrder[i], item.Severity)
		}
	}
}

func TestSortFindingsStable(t *testing.T) {
	// Items with same severity should preserve relative order
	items := []client.TopFindingItem{
		{Severity: "HIGH", Title: "first high"},
		{Severity: "LOW", Title: "first low"},
		{Severity: "HIGH", Title: "second high"},
		{Severity: "LOW", Title: "second low"},
	}

	sortFindings(items)

	if items[0].Title != "first high" {
		t.Errorf("expected 'first high' first, got %s", items[0].Title)
	}
	if items[1].Title != "second high" {
		t.Errorf("expected 'second high' second, got %s", items[1].Title)
	}
	if items[2].Title != "first low" {
		t.Errorf("expected 'first low' third, got %s", items[2].Title)
	}
	if items[3].Title != "second low" {
		t.Errorf("expected 'second low' fourth, got %s", items[3].Title)
	}
}

func TestSortFindingsEmpty(t *testing.T) {
	// Should not panic on empty or single-element slices
	sortFindings(nil)
	sortFindings([]client.TopFindingItem{})
	sortFindings([]client.TopFindingItem{{Severity: "HIGH", Title: "only"}})
}
