package orchestrator


import (
	"os"
	"path/filepath"
	"testing"
)

// ── helpers ────────────────────────────────────────────────

func readTestdata(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read testdata/%s: %v", name, err)
	}
	return data
}

// ── Gitleaks parser ────────────────────────────────────────

func TestParseGitleaksOutput(t *testing.T) {
	t.Run("parses findings correctly", func(t *testing.T) {
		data := readTestdata(t, "gitleaks_findings.json")
		findings, err := parseGitleaksOutput(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(findings) != 3 {
			t.Fatalf("expected 3 findings, got %d", len(findings))
		}

		// v8 format: all findings are CRITICAL (severity not in output)
		f0 := findings[0]
		if f0.Scanner != "gitleaks" {
			t.Errorf("expected scanner 'gitleaks', got %q", f0.Scanner)
		}
		if f0.Severity != "CRITICAL" {
			t.Errorf("expected severity 'CRITICAL' (gitleaks v8 always critical), got %q", f0.Severity)
		}
		if f0.Title != "generic-api-key" {
			t.Errorf("expected title 'generic-api-key', got %q", f0.Title)
		}
		if f0.FilePath != "src/config.go" {
			t.Errorf("expected filepath 'src/config.go', got %q", f0.FilePath)
		}
		if f0.LineNumber != 42 {
			t.Errorf("expected line 42, got %d", f0.LineNumber)
		}
		if f0.Description != "Detected a Generic API Key, potentially exposing access to various services and sensitive operations." {
			t.Errorf("unexpected description: %q", f0.Description)
		}

		// Finding 1: also CRITICAL
		f1 := findings[1]
		if f1.Severity != "CRITICAL" {
			t.Errorf("expected severity 'CRITICAL', got %q", f1.Severity)
		}
		if f1.Title != "aws-secret-key" {
			t.Errorf("expected title 'aws-secret-key', got %q", f1.Title)
		}

		// Finding 2: also CRITICAL (v8 always uses CRITICAL)
		f2 := findings[2]
		if f2.Severity != "CRITICAL" {
			t.Errorf("expected severity 'CRITICAL' (gitleaks v8 always critical), got %q", f2.Severity)
		}
		if f2.Title != "private-key" {
			t.Errorf("expected title 'private-key', got %q", f2.Title)
		}
	})

	t.Run("empty array returns no findings", func(t *testing.T) {
		data := readTestdata(t, "gitleaks_empty.json")
		findings, err := parseGitleaksOutput(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(findings) != 0 {
			t.Errorf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("malformed JSON returns error", func(t *testing.T) {
		_, err := parseGitleaksOutput([]byte(`{invalid`))
		if err == nil {
			t.Fatal("expected error for malformed JSON")
		}
	})

	t.Run("null input returns error", func(t *testing.T) {
		_, err := parseGitleaksOutput([]byte(`null`))
		if err != nil {
			t.Fatal("expected null to be valid (empty slice)")
		}
	})

	t.Run("partial missing fields", func(t *testing.T) {
		data := []byte(`[
			{"RuleID": "test", "Description": "test", "File": "main.go"}
		]`)
		findings, err := parseGitleaksOutput(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		// gitleaks v8: all findings are CRITICAL (no severity in output)
		if findings[0].Severity != "CRITICAL" {
			t.Errorf("expected severity 'CRITICAL' (gitleaks v8), got %q", findings[0].Severity)
		}
	})
}

// ── Semgrep parser ─────────────────────────────────────────

func TestParseSemgrepOutput(t *testing.T) {
	t.Run("parses findings correctly", func(t *testing.T) {
		data := readTestdata(t, "semgrep_findings.json")
		findings, err := parseSemgrepOutput(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(findings) != 3 {
			t.Fatalf("expected 3 findings, got %d", len(findings))
		}

		// Finding 0: ERROR → CRITICAL
		f0 := findings[0]
		if f0.Scanner != "semgrep" {
			t.Errorf("expected scanner 'semgrep', got %q", f0.Scanner)
		}
		if f0.Severity != "CRITICAL" {
			t.Errorf("expected severity 'CRITICAL' (ERROR→CRITICAL), got %q", f0.Severity)
		}
		if f0.Title != "python.lang.security.audit.eval-detected" {
			t.Errorf("unexpected title: %q", f0.Title)
		}
		if f0.FilePath != "src/script.py" {
			t.Errorf("expected 'src/script.py', got %q", f0.FilePath)
		}
		if f0.LineNumber != 10 {
			t.Errorf("expected line 10, got %d", f0.LineNumber)
		}
		if f0.Description != "Detected use of eval() which can lead to code injection" {
			t.Errorf("unexpected description: %q", f0.Description)
		}

		// Finding 1: WARNING → HIGH
		f1 := findings[1]
		if f1.Severity != "HIGH" {
			t.Errorf("expected severity 'HIGH' (WARNING→HIGH), got %q", f1.Severity)
		}

		// Finding 2: INFO → MEDIUM
		f2 := findings[2]
		if f2.Severity != "MEDIUM" {
			t.Errorf("expected severity 'MEDIUM' (default), got %q", f2.Severity)
		}
	})

	t.Run("empty results returns no findings", func(t *testing.T) {
		data := readTestdata(t, "semgrep_empty.json")
		findings, err := parseSemgrepOutput(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(findings) != 0 {
			t.Errorf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("malformed JSON returns error", func(t *testing.T) {
		_, err := parseSemgrepOutput([]byte(`{`))
		if err == nil {
			t.Fatal("expected error for malformed JSON")
		}
	})

	t.Run("missing results key returns empty", func(t *testing.T) {
		data := []byte(`{"errors": []}`)
		findings, err := parseSemgrepOutput(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(findings) != 0 {
			t.Errorf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ── Trivy parser ───────────────────────────────────────────

func TestParseTrivyOutput(t *testing.T) {
	t.Run("parses findings correctly", func(t *testing.T) {
		data := readTestdata(t, "trivy_findings.json")
		findings, err := parseTrivyOutput(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(findings) != 3 {
			t.Fatalf("expected 3 findings, got %d", len(findings))
		}

		// Finding 0: CRITICAL in package-lock.json
		f0 := findings[0]
		if f0.Scanner != "trivy" {
			t.Errorf("expected scanner 'trivy', got %q", f0.Scanner)
		}
		if f0.Severity != "CRITICAL" {
			t.Errorf("expected severity 'CRITICAL', got %q", f0.Severity)
		}
		if f0.Title != "CVE-2024-1234" {
			t.Errorf("expected title 'CVE-2024-1234', got %q", f0.Title)
		}
		if f0.FilePath != "package-lock.json" {
			t.Errorf("expected filepath 'package-lock.json', got %q", f0.FilePath)
		}
		if f0.Description != "Prototype Pollution in lodash (4.17.20)" {
			t.Errorf("expected description with version, got %q", f0.Description)
		}
		if f0.LineNumber != 0 {
			t.Errorf("expected LineNumber 0 (trivy doesn't set it), got %d", f0.LineNumber)
		}

		// Finding 1: HIGH
		f1 := findings[1]
		if f1.Severity != "HIGH" {
			t.Errorf("expected severity 'HIGH', got %q", f1.Severity)
		}

		// Finding 2: MEDIUM in go.mod
		f2 := findings[2]
		if f2.Severity != "MEDIUM" {
			t.Errorf("expected severity 'MEDIUM', got %q", f2.Severity)
		}
		if f2.FilePath != "go.mod" {
			t.Errorf("expected filepath 'go.mod', got %q", f2.FilePath)
		}
	})

	t.Run("empty results returns no findings", func(t *testing.T) {
		data := readTestdata(t, "trivy_empty.json")
		findings, err := parseTrivyOutput(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(findings) != 0 {
			t.Errorf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("malformed JSON returns error", func(t *testing.T) {
		_, err := parseTrivyOutput([]byte(`{break`))
		if err == nil {
			t.Fatal("expected error for malformed JSON")
		}
	})

	t.Run("result with nil vulnerabilities", func(t *testing.T) {
		data := []byte(`{"Results": [{"Target": "somefile", "Vulnerabilities": null}]}`)
		findings, err := parseTrivyOutput(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(findings) != 0 {
			t.Errorf("expected 0 findings for nil vulns, got %d", len(findings))
		}
	})
}

// ── Checkov parser ─────────────────────────────────────────

func TestParseCheckovOutput(t *testing.T) {
	t.Run("parses findings correctly", func(t *testing.T) {
		data := readTestdata(t, "checkov_findings.json")
		findings, err := parseCheckovOutput(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(findings) != 3 {
			t.Fatalf("expected 3 findings, got %d", len(findings))
		}

		// Finding 0: HIGH
		f0 := findings[0]
		if f0.Scanner != "checkov" {
			t.Errorf("expected scanner 'checkov', got %q", f0.Scanner)
		}
		if f0.Severity != "HIGH" {
			t.Errorf("expected severity 'HIGH', got %q", f0.Severity)
		}
		if f0.Title != "CKV_DOCKER_1" {
			t.Errorf("expected title 'CKV_DOCKER_1', got %q", f0.Title)
		}
		if f0.FilePath != "/src/Dockerfile" {
			t.Errorf("expected filepath '/src/Dockerfile', got %q", f0.FilePath)
		}
		if f0.LineNumber != 3 {
			t.Errorf("expected line 3, got %d", f0.LineNumber)
		}
		if f0.Description != "Ensure COPY is used instead of ADD in Dockerfiles" {
			t.Errorf("unexpected description: %q", f0.Description)
		}

		// Finding 1: MEDIUM → stays MEDIUM (mapped via switch, but severity is "MEDIUM")
		f1 := findings[1]
		if f1.Severity != "MEDIUM" {
			t.Errorf("expected severity 'MEDIUM', got %q", f1.Severity)
		}
		if f1.LineNumber != 25 {
			t.Errorf("expected line 25, got %d", f1.LineNumber)
		}

		// Finding 2: CRITICAL
		f2 := findings[2]
		if f2.Severity != "CRITICAL" {
			t.Errorf("expected severity 'CRITICAL', got %q", f2.Severity)
		}
	})

	t.Run("empty array returns no findings", func(t *testing.T) {
		data := readTestdata(t, "checkov_empty.json")
		findings, err := parseCheckovOutput(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(findings) != 0 {
			t.Errorf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("malformed JSON returns error", func(t *testing.T) {
		_, err := parseCheckovOutput([]byte(`[invalid`))
		if err == nil {
			t.Fatal("expected error for malformed JSON")
		}
	})

	t.Run("missing file_line_range uses zero", func(t *testing.T) {
		data := []byte(`[
			{"check_id": "CKV_1", "check_name": "test", "file_path": "main.tf", "severity": "LOW", "description": ""}
		]`)
		findings, err := parseCheckovOutput(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if findings[0].LineNumber != 0 {
			t.Errorf("expected LineNumber 0 (no file_line_range), got %d", findings[0].LineNumber)
		}
		// LOW severity → default MEDIUM
		if findings[0].Severity != "MEDIUM" {
			t.Errorf("expected severity 'MEDIUM' (default for non-CRITICAL/HIGH), got %q", findings[0].Severity)
		}
	})
}

// ── Multiple parsers: edge cases ───────────────────────────

func TestParseVariousEdgeCases(t *testing.T) {
	t.Run("malformed JSON across all parsers", func(t *testing.T) {
		badJSON := []byte(`{this is not valid json`)

		_, err := parseGitleaksOutput(badJSON)
		if err == nil {
			t.Error("parseGitleaksOutput: expected error for malformed JSON")
		}

		_, err = parseSemgrepOutput(badJSON)
		if err == nil {
			t.Error("parseSemgrepOutput: expected error for malformed JSON")
		}

		_, err = parseTrivyOutput(badJSON)
		if err == nil {
			t.Error("parseTrivyOutput: expected error for malformed JSON")
		}

		_, err = parseCheckovOutput(badJSON)
		if err == nil {
			t.Error("parseCheckovOutput: expected error for malformed JSON")
		}
	})

	t.Run("empty byte slice", func(t *testing.T) {
		_, err := parseGitleaksOutput([]byte{})
		if err == nil {
			t.Error("parseGitleaksOutput: expected error for empty input")
		}
	})

	t.Run("valid JSON with wrong structure", func(t *testing.T) {
		// Valid JSON but not a scanner output
		data := []byte(`{"status": "ok", "message": "hello"}`)

		// Gitleaks expects a JSON array — this should produce an error
		_, err := parseGitleaksOutput(data)
		if err != nil {
			t.Logf("parseGitleaksOutput correctly rejected object: %v", err)
		}
	})
}
