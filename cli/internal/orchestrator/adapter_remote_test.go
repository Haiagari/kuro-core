package orchestrator


import (
	"testing"

	"kuro/cli/internal/client"
)

// ── convertFindings tests ──────────────────────────────────

func TestConvertFindings(t *testing.T) {
	t.Run("converts scan result with findings", func(t *testing.T) {
		r := &client.ScanResult{
			ScanID:   "scan-123",
			Status:   "completed",
			Decision: "block",
			TopFindings: []client.TopFindingItem{
				{
					Scanner:  "gitleaks",
					Severity: "CRITICAL",
					Title:    "hardcoded-secret",
					File:     "src/config.go",
					Line:     42,
				},
				{
					Scanner:  "semgrep",
					Severity: "HIGH",
					Title:    "xss-detected",
					File:     "src/app.js",
					Line:     55,
				},
				{
					Scanner:  "trivy",
					Severity: "MEDIUM",
					Title:    "CVE-2024-1234",
					File:     "go.mod",
					Line:     0,
				},
			},
		}

		findings := convertFindings(r)
		if len(findings) != 3 {
			t.Fatalf("expected 3 findings, got %d", len(findings))
		}

		// Finding 0
		f0 := findings[0]
		if f0.Scanner != "gitleaks" {
			t.Errorf("expected scanner 'gitleaks', got %q", f0.Scanner)
		}
		if f0.Severity != "CRITICAL" {
			t.Errorf("expected severity 'CRITICAL', got %q", f0.Severity)
		}
		if f0.Title != "hardcoded-secret" {
			t.Errorf("expected title 'hardcoded-secret', got %q", f0.Title)
		}
		if f0.FilePath != "src/config.go" {
			t.Errorf("expected FilePath 'src/config.go', got %q", f0.FilePath)
		}
		if f0.LineNumber != 42 {
			t.Errorf("expected LineNumber 42, got %d", f0.LineNumber)
		}

		// Finding 1
		f1 := findings[1]
		if f1.Scanner != "semgrep" {
			t.Errorf("expected scanner 'semgrep', got %q", f1.Scanner)
		}
		if f1.Severity != "HIGH" {
			t.Errorf("expected severity 'HIGH', got %q", f1.Severity)
		}

		// Finding 2
		f2 := findings[2]
		if f2.Scanner != "trivy" {
			t.Errorf("expected scanner 'trivy', got %q", f2.Scanner)
		}
		if f2.Severity != "MEDIUM" {
			t.Errorf("expected severity 'MEDIUM', got %q", f2.Severity)
		}
		if f2.LineNumber != 0 {
			t.Errorf("expected LineNumber 0, got %d", f2.LineNumber)
		}
	})

	t.Run("empty top findings returns empty slice", func(t *testing.T) {
		r := &client.ScanResult{
			ScanID:      "scan-456",
			Status:      "completed",
			TopFindings: []client.TopFindingItem{},
		}

		findings := convertFindings(r)
		if len(findings) != 0 {
			t.Errorf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("nil top findings returns empty slice", func(t *testing.T) {
		r := &client.ScanResult{
			ScanID:      "scan-789",
			Status:      "completed",
			TopFindings: nil,
		}

		findings := convertFindings(r)
		if len(findings) != 0 {
			t.Errorf("expected 0 findings for nil, got %d", len(findings))
		}
	})

	t.Run("single finding is converted", func(t *testing.T) {
		r := &client.ScanResult{
			TopFindings: []client.TopFindingItem{
				{
					Scanner:  "checkov",
					Severity: "HIGH",
					Title:    "CKV_DOCKER_1",
					File:     "Dockerfile",
					Line:     3,
				},
			},
		}

		findings := convertFindings(r)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if findings[0].Scanner != "checkov" {
			t.Errorf("expected scanner 'checkov', got %q", findings[0].Scanner)
		}
		if findings[0].Description != "" {
			t.Errorf("expected empty Description, got %q", findings[0].Description)
		}
	})

	t.Run("mixed severities are preserved", func(t *testing.T) {
		severities := []string{"CRITICAL", "HIGH", "MEDIUM", "LOW", "UNKNOWN"}
		var items []client.TopFindingItem
		for _, s := range severities {
			items = append(items, client.TopFindingItem{
				Scanner:  "test",
				Severity: s,
				Title:    "finding-" + s,
				File:     "file.txt",
				Line:     1,
			})
		}

		r := &client.ScanResult{TopFindings: items}
		findings := convertFindings(r)
		if len(findings) != len(severities) {
			t.Fatalf("expected %d findings, got %d", len(severities), len(findings))
		}
		for i, f := range findings {
			if f.Severity != severities[i] {
				t.Errorf("finding %d: expected severity %q, got %q", i, severities[i], f.Severity)
			}
		}
	})
}
