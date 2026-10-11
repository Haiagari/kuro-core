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

	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return nil, fmt.Errorf("null Gitleaks report")
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
			Scanner:     "gitleaks",
			Severity:    "CRITICAL",
			Title:       r.RuleID,
			FilePath:    r.File,
			LineNumber:  r.StartLine,
			Description: r.Description,
		})
	}
	return findings, nil
}

func parseSemgrepOutput(data []byte) ([]Finding, error) {
	var shape map[string]json.RawMessage
	if err := json.Unmarshal(data, &shape); err != nil {
		return nil, err
	}
	if _, ok := shape["results"]; !ok {
		return nil, fmt.Errorf("scanner report missing results")
	}

	var result struct {
		Errors  []json.RawMessage `json:"errors"`
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

	if len(result.Errors) > 0 {
		return nil, fmt.Errorf("Semgrep reported %d analysis errors", len(result.Errors))
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
			Scanner:     "semgrep",
			Severity:    sev,
			Title:       r.CheckID,
			FilePath:    r.Path,
			LineNumber:  r.Start.Line,
			Description: r.Extra.Message,
		})
	}
	return findings, nil
}

func parseTrivyOutput(data []byte) ([]Finding, error) {
	var shape map[string]json.RawMessage
	if err := json.Unmarshal(data, &shape); err != nil {
		return nil, err
	}
	if _, ok := shape["Results"]; !ok {
		return nil, fmt.Errorf("scanner report missing Results")
	}

	var result struct {
		Results []struct {
			Target          string `json:"Target"`
			Vulnerabilities []struct {
				VulnerabilityID  string `json:"VulnerabilityID"`
				PkgName          string `json:"PkgName"`
				Severity         string `json:"Severity"`
				Title            string `json:"Title"`
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
				Scanner:     "trivy",
				Severity:    v.Severity,
				Title:       v.VulnerabilityID,
				FilePath:    r.Target,
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
			DetectorName   string `json:"DetectorName"`
			SourceMetadata struct {
				Data struct {
					Git *struct {
						File   string `json:"file"`
						Line   int    `json:"line"`
						Commit string `json:"commit"`
					} `json:"Git"`
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
			return nil, fmt.Errorf("parse trufflehog: %w", err)
		}

		if result.DetectorName == "" {
			return nil, fmt.Errorf("TruffleHog result missing DetectorName")
		}

		if git := result.SourceMetadata.Data.Git; git != nil {
			result.SourceMetadata.Data.File = git.File
			result.SourceMetadata.Data.Line = git.Line
			result.SourceMetadata.Data.Commit = git.Commit
		}
		desc := "Secret detected"
		if result.SourceMetadata.Data.Commit != "" {
			desc = fmt.Sprintf("Secret detected in commit %s", result.SourceMetadata.Data.Commit)
		}

		findings = append(findings, Finding{
			Scanner:     "trufflehog",
			Severity:    "CRITICAL",
			Title:       result.DetectorName,
			FilePath:    result.SourceMetadata.Data.File,
			LineNumber:  result.SourceMetadata.Data.Line,
			Description: desc,
			Verified:    result.Verified,
		})
	}

	return findings, nil
}

// Checkov emits a report object (one framework) or an array of report objects.
func parseCheckovOutput(data []byte) ([]Finding, error) {
	type check struct {
		CheckID       string `json:"check_id"`
		CheckName     string `json:"check_name"`
		FilePath      string `json:"file_path"`
		FileLineRange []int  `json:"file_line_range"`
		Severity      string `json:"severity"`
	}
	type report struct {
		CheckType string `json:"check_type"`
		Results   *struct {
			Failed        []check           `json:"failed_checks"`
			ParsingErrors []json.RawMessage `json:"parsing_errors"`
		} `json:"results"`
		Summary struct {
			ParsingErrors int `json:"parsing_errors"`
		} `json:"summary"`
	}
	data = bytes.TrimSpace(data)
	var reports []report
	if len(data) == 0 {
		return nil, fmt.Errorf("empty Checkov output")
	}
	if data[0] == '{' {
		var r report
		if err := json.Unmarshal(data, &r); err != nil {
			return nil, err
		}
		reports = []report{r}
	} else if data[0] == '[' {
		if err := json.Unmarshal(data, &reports); err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("invalid Checkov report shape")
	}
	var findings []Finding
	for _, r := range reports {
		if r.CheckType == "" || r.Results == nil {
			return nil, fmt.Errorf("Checkov report missing check_type or results")
		}
		if r.Summary.ParsingErrors > 0 || len(r.Results.ParsingErrors) > 0 {
			return nil, fmt.Errorf("Checkov reported parsing errors")
		}
		for _, c := range r.Results.Failed {
			if c.CheckID == "" || c.FilePath == "" {
				return nil, fmt.Errorf("invalid Checkov finding")
			}
			line := 0
			if len(c.FileLineRange) > 0 {
				line = c.FileLineRange[0]
			}
			sev := "MEDIUM"
			if c.Severity == "CRITICAL" || c.Severity == "HIGH" {
				sev = c.Severity
			}
			findings = append(findings, Finding{Scanner: "checkov", Severity: sev, Title: c.CheckID, FilePath: c.FilePath, LineNumber: line, Description: c.CheckName})
		}
	}
	return findings, nil
}
