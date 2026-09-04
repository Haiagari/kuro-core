package orchestrator

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// ── Parsers ─────────────────────────────────────────────────

func parseGitleaksOutput(data []byte) ([]Finding, error) {
	// Extract JSON array from mixed output (logo, logs, JSON)
	start := bytes.Index(data, []byte("["))
	end := bytes.LastIndex(data, []byte("]"))
	if start >= 0 && end > start {
		data = data[start : end+1]
	}

	var results []struct {
		RuleID      string `json:"RuleID"`
		Description string `json:"Description"`
		File        string `json:"File"`
		StartLine   int    `json:"StartLine"`
		Secret      string `json:"Secret"`
	}

	if err := json.Unmarshal(data, &results); err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}

	var findings []Finding
	for _, r := range results {
		findings = append(findings, Finding{
			Scanner:    "gitleaks",
			Severity:   "CRITICAL",
			Title:      r.RuleID,
			FilePath:   r.File,
			LineNumber: r.StartLine,
			Description: r.Description,
		})
	}
	return findings, nil
}

func parseSemgrepOutput(data []byte) ([]Finding, error) {
	var result struct {
		Results []struct {
			CheckID string `json:"check_id"`
			Path    string `json:"path"`
			Start   struct {
				Line int `json:"line"`
			} `json:"start"`
			Extra struct {
				Severity string `json:"severity"`
				Message  string `json:"message"`
				Lines    string `json:"lines"`
			} `json:"extra"`
		} `json:"results"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse semgrep: %w", err)
	}

	var findings []Finding
	for _, r := range result.Results {
		sev := "MEDIUM"
		switch r.Extra.Severity {
		case "ERROR":
			sev = "CRITICAL"
		case "WARNING":
			sev = "HIGH"
		}
		findings = append(findings, Finding{
			Scanner:    "semgrep",
			Severity:   sev,
			Title:      r.CheckID,
			FilePath:   r.Path,
			LineNumber: r.Start.Line,
			Description: r.Extra.Message,
		})
	}
	return findings, nil
}

func parseTrivyOutput(data []byte) ([]Finding, error) {
	var result struct {
		Results []struct {
			Target       string `json:"Target"`
			Vulnerabilities []struct {
				VulnerabilityID string `json:"VulnerabilityID"`
				PkgName    string `json:"PkgName"`
				Severity   string `json:"Severity"`
				Title      string `json:"Title"`
				InstalledVersion string `json:"InstalledVersion"`
			} `json:"Vulnerabilities"`
		} `json:"Results"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse trivy: %w", err)
	}

	var findings []Finding
	for _, r := range result.Results {
		for _, v := range r.Vulnerabilities {
			findings = append(findings, Finding{
				Scanner:    "trivy",
				Severity:   v.Severity,
				Title:      v.VulnerabilityID,
				FilePath:   r.Target,
				Description: fmt.Sprintf("%s (%s)", v.Title, v.InstalledVersion),
			})
		}
	}
	return findings, nil
}

func parseTrufflehogOutput(data []byte) ([]Finding, error) {
	// TruffleHog outputs NDJSON (one JSON object per line)
	var findings []Finding
	for _, line := range bytes.Split(data, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		var result struct {
			DetectorName string `json:"DetectorName"`
			SourceMetadata struct {
				Data struct {
					File   string `json:"file"`
					Line   int    `json:"line"`
					Commit string `json:"commit"`
					Email  string `json:"email"`
				} `json:"Data"`
			} `json:"SourceMetadata"`
			Raw      string `json:"Raw"`
			Verified bool   `json:"Verified"`
		}

		if err := json.Unmarshal(line, &result); err != nil {
			continue // skip malformed lines
		}

		snippet := result.Raw
		if len(snippet) > 120 {
			snippet = snippet[:120]
		}

		desc := "Secret detected"
		if result.SourceMetadata.Data.Commit != "" {
			desc = fmt.Sprintf("Secret detected in commit %s", result.SourceMetadata.Data.Commit)
		}

		findings = append(findings, Finding{
			Scanner:    "trufflehog",
			Severity:   "CRITICAL",
			Title:      result.DetectorName,
			FilePath:   result.SourceMetadata.Data.File,
			LineNumber: result.SourceMetadata.Data.Line,
			Description: desc,
			Verified:   result.Verified,
		})
	}

	return findings, nil
}

func parseCheckovOutput(data []byte) ([]Finding, error) {
	var results []struct {
		CheckID    string `json:"check_id"`
		CheckName  string `json:"check_name"`
		FilePath   string `json:"file_path"`
		FileLineRange []int `json:"file_line_range"`
		Severity   string `json:"severity"`
		Description string `json:"description"`
	}

	if err := json.Unmarshal(data, &results); err != nil {
		return nil, fmt.Errorf("parse checkov: %w", err)
	}

	var findings []Finding
	for _, r := range results {
		line := 0
		if len(r.FileLineRange) > 0 {
			line = r.FileLineRange[0]
		}
		sev := "MEDIUM"
		switch r.Severity {
		case "CRITICAL", "HIGH":
			sev = r.Severity
		}
		findings = append(findings, Finding{
			Scanner:    "checkov",
			Severity:   sev,
			Title:      r.CheckID,
			FilePath:   r.FilePath,
			LineNumber: line,
			Description: r.CheckName,
		})
	}
	return findings, nil
}
