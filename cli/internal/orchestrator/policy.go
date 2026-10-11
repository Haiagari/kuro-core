package orchestrator

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

//go:embed rules/default-policy.json
var defaultPolicyJSON []byte

type PolicyRule struct {
	Scanner      string   `json:"scanner,omitempty"`
	Severity     string   `json:"severity,omitempty"`
	Threshold    int      `json:"threshold,omitempty"`
	Action       string   `json:"action"`
	FilePatterns []string `json:"file_patterns,omitempty"`
	Reason       string   `json:"reason,omitempty"`
}
type Policy struct {
	Name    string       `json:"name"`
	Version string       `json:"version"`
	Rules   []PolicyRule `json:"rules"`
}

func parsePolicy(data []byte) (*Policy, error) {
	var p Policy
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return nil, fmt.Errorf("parse policy: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("policy must contain exactly one JSON object")
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return &p, nil
}
func LoadDefaultPolicy() (*Policy, error) { return parsePolicy(defaultPolicyJSON) }
func LoadPolicy(path string) (*Policy, error) {
	if path == "" {
		path = os.Getenv("KURO_POLICY_PATH")
	}
	if path == "" {
		return LoadDefaultPolicy()
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read policy: %w", err)
	}
	return parsePolicy(data)
}
func (p *Policy) Validate() error {
	if p == nil || p.Name == "" || p.Version == "" || len(p.Rules) == 0 {
		return fmt.Errorf("policy requires name, version and rules")
	}
	for i, r := range p.Rules {
		if r.Scanner == "" && r.Severity == "" && len(r.FilePatterns) == 0 {
			return fmt.Errorf("rule %d requires a filter", i)
		}
		if r.Threshold < 0 {
			return fmt.Errorf("rule %d has a negative threshold", i)
		}
		switch strings.ToUpper(r.Action) {
		case "BLOCK", "REVIEW", "MANUAL_REVIEW", "PASS":
		default:
			return fmt.Errorf("rule %d has an unknown action", i)
		}
		if r.Scanner != "" {
			switch strings.ToLower(r.Scanner) {
			case "gitleaks", "semgrep", "trivy", "checkov", "trufflehog":
			default:
				return fmt.Errorf("rule %d has an unsupported scanner", i)
			}
		}
		if r.Severity != "" {
			switch strings.ToUpper(r.Severity) {
			case "CRITICAL", "HIGH", "MEDIUM", "LOW", "INFO", "UNKNOWN":
			default:
				return fmt.Errorf("rule %d has an unknown severity", i)
			}
		}
		for _, pattern := range r.FilePatterns {
			if _, err := compilePolicyGlob(pattern); err != nil {
				return fmt.Errorf("rule %d: %w", i, err)
			}
		}
	}
	return nil
}

// Globs are relative to the scan root: * stays within a path segment; ** crosses it.
// Character classes and brace expansion are deliberately unsupported.
func compilePolicyGlob(pattern string) (*regexp.Regexp, error) {
	if pattern == "" || strings.HasPrefix(pattern, "/") || strings.Contains(pattern, "\\") || strings.ContainsAny(pattern, "[]{}") {
		return nil, fmt.Errorf("invalid relative policy glob %q", pattern)
	}
	for _, part := range strings.Split(pattern, "/") {
		if part == ".." {
			return nil, fmt.Errorf("parent traversal in policy glob")
		}
	}
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				i++
				if i+1 < len(pattern) && pattern[i+1] == '/' {
					i++
					b.WriteString("(?:.*/)?")
				} else {
					b.WriteString(".*")
				}
			} else {
				b.WriteString("[^/]*")
			}
		case '?':
			b.WriteString("[^/]")
		default:
			b.WriteString(regexp.QuoteMeta(string(pattern[i])))
		}
	}
	b.WriteString("$")
	return regexp.Compile(b.String())
}
func ruleMatches(r PolicyRule, f Finding) bool {
	if r.Scanner != "" && !strings.EqualFold(r.Scanner, f.Scanner) {
		return false
	}
	if r.Severity != "" && !strings.EqualFold(r.Severity, f.Severity) {
		return false
	}
	if len(r.FilePatterns) == 0 {
		return true
	}
	file := strings.TrimPrefix(strings.TrimPrefix(f.FilePath, "/src/"), "./")
	for _, pattern := range r.FilePatterns {
		re, err := compilePolicyGlob(pattern)
		if err == nil && re.MatchString(file) {
			return true
		}
	}
	return false
}
func (p *Policy) Evaluate(findings []Finding) string {
	if p.Validate() != nil {
		return "block"
	}
	blocked, review := false, false
	for _, rule := range p.Rules {
		count := 0
		for _, f := range findings {
			if ruleMatches(rule, f) {
				count++
			}
		}
		threshold := rule.Threshold
		if threshold == 0 {
			threshold = 1
		}
		if count >= threshold {
			switch strings.ToUpper(rule.Action) {
			case "BLOCK":
				blocked = true
			case "MANUAL_REVIEW", "REVIEW":
				review = true
			}
		}
	}
	// Mandatory safety floors cannot be relaxed by custom rules or PASS actions.
	for _, f := range findings {
		switch strings.ToUpper(f.Severity) {
		case "CRITICAL":
			blocked = true
		case "HIGH":
			review = true
		}
		if strings.EqualFold(f.Scanner, "gitleaks") || strings.EqualFold(f.Scanner, "trufflehog") {
			blocked = true
		}
	}
	if blocked {
		return "block"
	}
	if review {
		return "review"
	}
	return "pass"
}
