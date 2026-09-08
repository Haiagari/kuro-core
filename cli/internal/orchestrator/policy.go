package orchestrator

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

//go:embed rules/default-policy.json
var defaultPolicyJSON []byte

// PolicyRule defines a single rule in the policy engine.
type PolicyRule struct {
	Scanner      string   `json:"scanner,omitempty"`
	Severity     string   `json:"severity,omitempty"`
	Threshold    int      `json:"threshold,omitempty"`
	Action       string   `json:"action"` // "BLOCK", "MANUAL_REVIEW", "REVIEW", "PASS"
	FilePatterns []string `json:"file_patterns,omitempty"`
	Reason       string   `json:"reason,omitempty"`
}

// Policy is the parsed representation of a security policy JSON.
type Policy struct {
	Name    string       `json:"name"`
	Version string       `json:"version"`
	Rules   []PolicyRule `json:"rules"`
}

// LoadDefaultPolicy parses the embedded default policy.
func LoadDefaultPolicy() (*Policy, error) {
	var p Policy
	if err := json.Unmarshal(defaultPolicyJSON, &p); err != nil {
		return nil, fmt.Errorf("unmarshal embedded policy: %w", err)
	}
	return &p, nil
}

// LoadPolicy loads a policy from a file path, or falls back to embedded default.
func LoadPolicy(path string) (*Policy, error) {
	if path == "" {
		path = os.Getenv("KURO_POLICY_PATH")
	}
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read policy file %s: %w", path, err)
		}
		var p Policy
		if err := json.Unmarshal(data, &p); err != nil {
			return nil, fmt.Errorf("parse policy JSON %s: %w", path, err)
		}
		return &p, nil
	}
	return LoadDefaultPolicy()
}

// Evaluate applies the policy rules to a list of findings and returns "block", "review", or "pass".
func (p *Policy) Evaluate(findings []Finding) string {
	if len(findings) == 0 {
		return "pass"
	}

	// Count findings by severity and by scanner
	countsBySeverity := make(map[string]int)
	countsByScanner := make(map[string]int)
	for _, f := range findings {
		countsBySeverity[strings.ToUpper(f.Severity)]++
		countsByScanner[strings.ToLower(f.Scanner)]++
	}

	isBlocked := false
	isReview := false

	if p != nil {
		for _, rule := range p.Rules {
			threshold := rule.Threshold
			if threshold <= 0 {
				threshold = 1
			}

			matchedCount := 0
			if rule.Severity != "" {
				matchedCount = countsBySeverity[strings.ToUpper(rule.Severity)]
			} else if rule.Scanner != "" {
				matchedCount = countsByScanner[strings.ToLower(rule.Scanner)]
			}

			if matchedCount >= threshold {
				action := strings.ToUpper(rule.Action)
				switch action {
				case "BLOCK":
					isBlocked = true
				case "MANUAL_REVIEW", "REVIEW":
					isReview = true
				}
			}
		}
	}

	// Fail-closed fallbacks: any CRITICAL always blocks, any HIGH always reviews
	if countsBySeverity["CRITICAL"] > 0 {
		isBlocked = true
	}
	if countsBySeverity["HIGH"] > 0 {
		isReview = true
	}

	switch {
	case isBlocked:
		return "block"
	case isReview:
		return "review"
	default:
		return "pass"
	}
}
