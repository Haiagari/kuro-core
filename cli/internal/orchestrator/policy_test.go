package orchestrator

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPolicy_LoadDefaultPolicy(t *testing.T) {
	p, err := LoadDefaultPolicy()
	if err != nil {
		t.Fatalf("LoadDefaultPolicy failed: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil policy")
	}
	if len(p.Rules) == 0 {
		t.Fatal("expected embedded policy to have rules, got 0")
	}
}

func TestPolicy_LoadCustomFile(t *testing.T) {
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "custom.json")
	customContent := `{
		"name": "custom-strict",
		"version": "1.0",
		"rules": [
			{"severity": "MEDIUM", "action": "BLOCK"}
		]
	}`
	if err := os.WriteFile(policyPath, []byte(customContent), 0644); err != nil {
		t.Fatal(err)
	}

	p, err := LoadPolicy(policyPath)
	if err != nil {
		t.Fatalf("LoadPolicy failed: %v", err)
	}
	if p.Name != "custom-strict" {
		t.Errorf("expected name 'custom-strict', got %q", p.Name)
	}

	// Test that medium blocks under this custom policy
	findings := []Finding{
		{Severity: "MEDIUM", Title: "Style issue"},
	}
	decision := p.Evaluate(findings)
	if decision != "block" {
		t.Errorf("expected decision 'block' from custom policy, got %q", decision)
	}
}

func TestPolicy_Evaluate(t *testing.T) {
	p, err := LoadDefaultPolicy()
	if err != nil {
		t.Fatalf("failed to load default policy: %v", err)
	}

	t.Run("clean findings returns pass", func(t *testing.T) {
		if got := p.Evaluate(nil); got != "pass" {
			t.Errorf("expected 'pass', got %q", got)
		}
		if got := p.Evaluate([]Finding{}); got != "pass" {
			t.Errorf("expected 'pass', got %q", got)
		}
	})

	t.Run("gitleaks findings blocks immediately", func(t *testing.T) {
		findings := []Finding{
			{Scanner: "gitleaks", Severity: "LOW", Title: "token"},
		}
		if got := p.Evaluate(findings); got != "block" {
			t.Errorf("expected 'block', got %q", got)
		}
	})

	t.Run("critical severity blocks", func(t *testing.T) {
		findings := []Finding{
			{Scanner: "semgrep", Severity: "CRITICAL", Title: "RCE"},
		}
		if got := p.Evaluate(findings); got != "block" {
			t.Errorf("expected 'block', got %q", got)
		}
	})

	t.Run("high severity returns review", func(t *testing.T) {
		findings := []Finding{
			{Scanner: "semgrep", Severity: "HIGH", Title: "SQLi"},
		}
		if got := p.Evaluate(findings); got != "review" {
			t.Errorf("expected 'review', got %q", got)
		}
	})

	t.Run("medium and low return pass under default policy", func(t *testing.T) {
		findings := []Finding{
			{Scanner: "semgrep", Severity: "MEDIUM", Title: "info"},
			{Scanner: "semgrep", Severity: "LOW", Title: "debug"},
		}
		if got := p.Evaluate(findings); got != "pass" {
			t.Errorf("expected 'pass', got %q", got)
		}
	})
}
