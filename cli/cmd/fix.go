package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// RemediationFinding represents a vulnerability or secret eligible for automated fix or triage.
type RemediationFinding struct {
	ID            string `json:"id"`
	Scanner       string `json:"scanner"`
	Severity      string `json:"severity"`
	Title         string `json:"title"`
	FilePath      string `json:"file_path"`
	LineNumber    int    `json:"line_number"`
	CodeSnippet   string `json:"code_snippet"`
	SecretValue   string `json:"secret_value,omitempty"`
	FixSuggestion string `json:"fix_suggestion"`
	Applied       bool   `json:"applied"`
}

// SuppressionRecord represents an acknowledged false positive or ignored rule.
type SuppressionRecord struct {
	FindingID string    `json:"finding_id"`
	RuleID    string    `json:"rule_id"`
	FilePath  string    `json:"file_path"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
	Author    string    `json:"author"`
}

// RunFix executes the 'kuro fix' interactive threat remediation workflow.
func RunFix(args []string) {
	dryRun := false
	autoApply := false
	targetPath := "."

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--dry-run", "-d":
			dryRun = true
		case "--auto", "-a", "--yes", "-y":
			autoApply = true
		case "--help", "-h":
			printFixUsage()
			return
		default:
			if !strings.HasPrefix(args[i], "-") {
				targetPath = args[i]
			}
		}
	}

	findings := scanLocalFindings(targetPath)
	if len(findings) == 0 {
		fmt.Println("✨ No active vulnerabilities or hardcoded secrets found to remediate.")
		return
	}

	fmt.Printf("\n╭────────────────────────────────────────────────────────────╮\n")
	fmt.Printf("│        KURO Interactive Threat Remediation Assistant       │\n")
	fmt.Printf("│               Found %2d Actionable Findings                 │\n", len(findings))
	fmt.Printf("╰────────────────────────────────────────────────────────────╯\n\n")

	for idx, f := range findings {
		fmt.Printf("─── [%d/%d] %s: %s ──────────────────────────────────────────\n", idx+1, len(findings), f.Severity, f.Title)
		fmt.Printf("File: %s (line %d)\n", f.FilePath, f.LineNumber)
		if f.CodeSnippet != "" {
			fmt.Printf("Snippet:  \033[31m%s\033[0m\n", strings.TrimSpace(f.CodeSnippet))
		}
		if f.FixSuggestion != "" {
			fmt.Printf("Proposed: \033[32m%s\033[0m\n", strings.TrimSpace(f.FixSuggestion))
		}

		if autoApply {
			applyFixToFile(f, dryRun)
			continue
		}

		fmt.Printf("\nSelect Action: [f] Apply Auto-Fix | [p] Mark False Positive | [i] Ignore/Suppress | [s] Skip | [q] Quit > ")
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		action := strings.TrimSpace(strings.ToLower(input))

		switch action {
		case "f", "fix", "1":
			applyFixToFile(f, dryRun)
		case "p", "fp", "2":
			markFalsePositive(f, targetPath)
		case "i", "ignore", "3":
			suppressFinding(f, targetPath)
		case "q", "quit":
			fmt.Println("Remediation session exited.")
			return
		default:
			fmt.Println("Skipped.")
		}
		fmt.Println()
	}

	fmt.Println("🎉 Remediation completed successfully.")
}

func printFixUsage() {
	fmt.Print(`
USAGE:
  kuro fix [path] [flags]     Interactive terminal threat remediation & auto-fix

FLAGS:
  --dry-run, -d      Preview code transformations without writing to disk
  --auto, -a, -y     Automatically apply recommended fixes to all detected secrets
  --help, -h         Show this help message
`)
}

// ApplyFixToContent executes string transformations to safely extract hardcoded secrets to env vars.
func ApplyFixToContent(filePath, content, secretValue string) (string, bool) {
	if secretValue == "" || !strings.Contains(content, secretValue) {
		return content, false
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	var replacement string
	envVarName := deriveEnvVarName(filePath, secretValue)

	switch ext {
	case ".go":
		replacement = fmt.Sprintf(`os.Getenv("%s")`, envVarName)
		// Ensure "os" import if not present
		if !strings.Contains(content, `"os"`) {
			content = injectGoImport(content, "os")
		}
	case ".py":
		replacement = fmt.Sprintf(`os.environ.get("%s")`, envVarName)
		if !strings.Contains(content, "import os") {
			content = "import os\n" + content
		}
	case ".js", ".ts", ".jsx", ".tsx":
		replacement = fmt.Sprintf(`process.env.%s`, envVarName)
	default:
		replacement = fmt.Sprintf(`"${%s}"`, envVarName)
	}

	// Replace literal quoted secret value with env var accessor
	quotedSecret := fmt.Sprintf(`"%s"`, secretValue)
	singleQuoted := fmt.Sprintf(`'%s'`, secretValue)

	if strings.Contains(content, quotedSecret) {
		return strings.Replace(content, quotedSecret, replacement, 1), true
	}
	if strings.Contains(content, singleQuoted) {
		return strings.Replace(content, singleQuoted, replacement, 1), true
	}

	return strings.Replace(content, secretValue, replacement, 1), true
}

func applyFixToFile(f RemediationFinding, dryRun bool) {
	data, err := os.ReadFile(f.FilePath)
	if err != nil {
		fmt.Printf("❌ Failed to read %s: %v\n", f.FilePath, err)
		return
	}

	updated, changed := ApplyFixToContent(f.FilePath, string(data), f.SecretValue)
	if !changed {
		fmt.Printf("⚠️  Could not automatically resolve syntax in %s\n", f.FilePath)
		return
	}

	if dryRun {
		fmt.Printf("🔍 [Dry Run] Diff for %s:\n\n%s\n", f.FilePath, updated)
		return
	}

	if err := os.WriteFile(f.FilePath, []byte(updated), 0644); err != nil {
		fmt.Printf("❌ Failed to write update to %s: %v\n", f.FilePath, err)
		return
	}

	fmt.Printf("✅ Fixed %s: Extracted secret to environment variable.\n", f.FilePath)
}

func markFalsePositive(f RemediationFinding, targetDir string) {
	record := SuppressionRecord{
		FindingID: f.ID,
		RuleID:    f.Title,
		FilePath:  f.FilePath,
		Reason:    "Marked as False Positive via Kuro CLI interactive triage",
		CreatedAt: time.Now().UTC(),
		Author:    "developer",
	}

	saveSuppression(targetDir, record)
	fmt.Printf("✅ Finding marked as False Positive. Stored in .kuro-suppressions.json\n")
}

func suppressFinding(f RemediationFinding, targetDir string) {
	fmt.Print("Enter justification reason for temporary suppression: ")
	reader := bufio.NewReader(os.Stdin)
	reason, _ := reader.ReadString('\n')
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "Developer accepted risk"
	}

	record := SuppressionRecord{
		FindingID: f.ID,
		RuleID:    f.Title,
		FilePath:  f.FilePath,
		Reason:    reason,
		CreatedAt: time.Now().UTC(),
		Author:    "developer",
	}

	saveSuppression(targetDir, record)
	fmt.Printf("✅ Rule suppressed: %s\n", f.Title)
}

func saveSuppression(targetDir string, record SuppressionRecord) {
	manifestPath := filepath.Join(targetDir, ".kuro-suppressions.json")
	var records []SuppressionRecord

	if data, err := os.ReadFile(manifestPath); err == nil {
		_ = json.Unmarshal(data, &records)
	}

	records = append(records, record)
	out, _ := json.MarshalIndent(records, "", "  ")
	_ = os.WriteFile(manifestPath, out, 0644)
}

func deriveEnvVarName(filePath, secretValue string) string {
	base := strings.ToUpper(filepath.Base(filePath))
	base = regexp.MustCompile(`[^A-Z0-9]`).ReplaceAllString(base, "_")
	if strings.HasPrefix(secretValue, "AKIA") {
		return "AWS_ACCESS_KEY_ID"
	}
	if strings.HasPrefix(secretValue, "ghp_") {
		return "GITHUB_TOKEN"
	}
	if strings.Contains(strings.ToLower(filePath), "stripe") {
		return "STRIPE_SECRET_KEY"
	}
	return fmt.Sprintf("APP_SECRET_%s", base)
}

func injectGoImport(content, pkg string) string {
	if strings.Contains(content, "import (") {
		return strings.Replace(content, "import (", fmt.Sprintf("import (\n\t\"%s\"", pkg), 1)
	}
	if strings.Contains(content, "package ") {
		lines := strings.Split(content, "\n")
		var updated []string
		injected := false
		for _, l := range lines {
			updated = append(updated, l)
			if strings.HasPrefix(l, "package ") && !injected {
				updated = append(updated, fmt.Sprintf("\nimport \"%s\"\n", pkg))
				injected = true
			}
		}
		return strings.Join(updated, "\n")
	}
	return content
}

func scanLocalFindings(targetPath string) []RemediationFinding {
	// Sample mock scanner parser or reading last scan output
	var findings []RemediationFinding

	_ = filepath.Walk(targetPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.Contains(path, ".git") || strings.Contains(path, "vendor") || strings.Contains(path, "node_modules") {
			return nil
		}

		// Quick heuristic scan for hardcoded credentials in files
		if data, err := os.ReadFile(path); err == nil {
			content := string(data)
			lines := strings.Split(content, "\n")
			for lineIdx, line := range lines {
				if strings.Contains(line, "AKIA") && (strings.Contains(line, "=") || strings.Contains(line, ":")) {
					r := regexp.MustCompile(`AKIA[0-9A-Z]{16}`)
					match := r.FindString(line)
					if match != "" {
						findings = append(findings, RemediationFinding{
							ID:            fmt.Sprintf("secret-aws-%d", lineIdx),
							Scanner:       "gitleaks",
							Severity:      "CRITICAL",
							Title:         "Hardcoded AWS Access Key",
							FilePath:      path,
							LineNumber:    lineIdx + 1,
							CodeSnippet:   line,
							SecretValue:   match,
							FixSuggestion: `Replace literal with os.Getenv("AWS_ACCESS_KEY_ID")`,
						})
					}
				}
			}
		}
		return nil
	})

	return findings
}
