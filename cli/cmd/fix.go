package cmd

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
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

type secretDetector struct {
	name    string
	pattern *regexp.Regexp
	subexp  int
}

var knownSecretDetectors = []secretDetector{
	{
		name:    "AWS Access Key",
		pattern: regexp.MustCompile(`\b(AKIA|ASIA)[0-9A-Z]{16}\b`),
	},
	{
		name:    "GitHub Personal Access Token",
		pattern: regexp.MustCompile(`\b(ghp_[a-zA-Z0-9]{20,82}|gho_[a-zA-Z0-9]{20,82}|github_pat_[a-zA-Z0-9_]{20,82})\b`),
	},
	{
		name:    "Slack Token",
		pattern: regexp.MustCompile(`\bxox[baprs]-[0-9a-zA-Z-]{10,72}\b`),
	},
	{
		name:    "Stripe Secret Key",
		pattern: regexp.MustCompile(`\b([rs]k_live_[0-9a-zA-Z]{20,40})\b`),
	},
	{
		name:    "Hardcoded API Secret",
		pattern: regexp.MustCompile(`(?i)(?:api_key|apikey|secret_key|secret|auth_token|client_secret)\s*[:=]\s*["']([A-Za-z0-9_\-\.]{12,})["']`),
		subexp:  1,
	},
}

// RunFix executes the 'kuro fix' interactive threat remediation workflow.
func RunFix(args []string) {
	_ = RunFixWithStatus(args)
}

// RunFixWithStatus runs remediation and returns a nonzero status on input or apply failures.
func RunFixWithStatus(args []string) int {
	dryRun := false
	autoApply := false
	reportPath := ""
	targetPath := "."

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--dry-run", "-d":
			dryRun = true
		case "--auto", "-a", "--yes", "-y":
			autoApply = true
		case "--report", "-r":
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
				fmt.Fprintln(os.Stderr, "❌ --report requires a JSON file path")
				return 1
			}
			reportPath = args[i+1]
			i++
		case "--help", "-h":
			printFixUsage()
			return 0
		default:
			if strings.HasPrefix(args[i], "--report=") {
				reportPath = strings.TrimPrefix(args[i], "--report=")
				if reportPath == "" {
					fmt.Fprintln(os.Stderr, "❌ --report requires a JSON file path")
					return 1
				}
			} else if !strings.HasPrefix(args[i], "-") {
				targetPath = args[i]
			}
		}
	}

	root, rootPath, err := openTargetRoot(targetPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Unable to open target directory: %v\n", err)
		return 1
	}
	defer root.Close()
	findings, err := scanLocalFindingsRoot(root, rootPath, reportPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Unable to load remediation findings: %v\n", err)
		return 1
	}
	if len(findings) == 0 {
		fmt.Println("✨ No active vulnerabilities or hardcoded secrets found to remediate.")
		return 0
	}

	fmt.Printf("\n╭────────────────────────────────────────────────────────────╮\n")
	fmt.Printf("│        KURO Interactive Threat Remediation Assistant       │\n")
	fmt.Printf("│                   Found %2d Findings                       │\n", len(findings))
	fmt.Printf("╰────────────────────────────────────────────────────────────╯\n\n")

	failedFixes := 0
	for idx, f := range findings {
		fmt.Printf("─── [%d/%d] %s: %s ──────────────────────────────────────────\n", idx+1, len(findings), f.Severity, f.Title)
		fmt.Printf("File: %s (line %d)\n", f.FilePath, f.LineNumber)
		if f.CodeSnippet != "" {
			fmt.Printf("Snippet:  \033[31m%s\033[0m\n", strings.TrimSpace(f.CodeSnippet))
		}
		if f.FixSuggestion != "" {
			fmt.Printf("Proposed: \033[32m%s\033[0m\n", strings.TrimSpace(f.FixSuggestion))
		}

		fileRel, relErr := filepath.Rel(rootPath, f.FilePath)
		if relErr != nil || fileRel == ".." || strings.HasPrefix(fileRel, ".."+string(filepath.Separator)) || filepath.IsAbs(fileRel) {
			fmt.Fprintf(os.Stderr, "❌ Finding path is outside target directory: %s\n", f.FilePath)
			failedFixes++
			continue
		}
		if autoApply {
			if !applyFixToFile(root, fileRel, f, dryRun) {
				failedFixes++
			}
			continue
		}

		fmt.Printf("\nSelect Action: [f] Apply Auto-Fix | [p] Mark False Positive | [i] Ignore/Suppress | [s] Skip | [q] Quit > ")
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		action := strings.TrimSpace(strings.ToLower(input))

		switch action {
		case "f", "fix", "1":
			if !applyFixToFile(root, fileRel, f, dryRun) {
				failedFixes++
			}
		case "p", "fp", "2":
			if err := markFalsePositive(f, root); err != nil {
				fmt.Fprintf(os.Stderr, "❌ Failed to save suppression: %v\n", err)
				failedFixes++
			}
		case "i", "ignore", "3":
			if err := suppressFinding(f, root); err != nil {
				fmt.Fprintf(os.Stderr, "❌ Failed to save suppression: %v\n", err)
				failedFixes++
			}
		case "q", "quit":
			fmt.Println("Remediation session exited.")
			return 0
		default:
			fmt.Println("Skipped.")
		}
		fmt.Println()
	}

	if failedFixes > 0 {
		fmt.Printf("❌ Remediation failed for %d finding(s); review the output and apply manually.\n", failedFixes)
		return 1
	}
	fmt.Println("🎉 Remediation completed successfully.")
	return 0
}

func printFixUsage() {
	fmt.Print(`
USAGE:
  kuro fix [path] [flags]     Interactive or report-driven secret remediation

FLAGS:
  --dry-run, -d        Preview code transformations without writing to disk
  --auto, -a, -y       Automatically apply safe replacements without prompts
  --report, -r <file>  Ingest findings from a Kuro scan JSON report
  --help, -h           Show this help message

Automatic replacements require a recognized quoted secret in Go, Python, or JS/TS.
Report paths must resolve to files inside [path]. Unsupported findings require manual review.
A report or apply failure returns exit status 1; success returns 0.
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
		return content, false
	}

	quotedSecret := fmt.Sprintf(`"%s"`, secretValue)
	singleQuoted := fmt.Sprintf(`'%s'`, secretValue)

	if strings.Contains(content, quotedSecret) {
		return strings.Replace(content, quotedSecret, replacement, 1), true
	}
	if strings.Contains(content, singleQuoted) {
		return strings.Replace(content, singleQuoted, replacement, 1), true
	}

	return content, false
}

func applyFixToFile(root *os.Root, relPath string, f RemediationFinding, dryRun bool) bool {
	relPath = filepath.Clean(relPath)
	if filepath.IsAbs(relPath) || relPath == "." || relPath == ".." || strings.HasPrefix(relPath, ".."+string(filepath.Separator)) {
		fmt.Fprintf(os.Stderr, "❌ Refusing path outside target directory: %s\n", f.FilePath)
		return false
	}
	info, err := root.Lstat(relPath)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		fmt.Fprintf(os.Stderr, "❌ Refusing non-regular or symlink target: %s\n", f.FilePath)
		return false
	}
	data, err := root.ReadFile(relPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to read %s: %v\n", f.FilePath, err)
		return false
	}

	updated, changed := ApplyFixToContent(f.FilePath, string(data), f.SecretValue)
	if !changed {
		fmt.Printf("⚠️  No safe automatic replacement is available for %s\n", f.FilePath)
		return false
	}
	if dryRun {
		fmt.Printf("🔍 [Dry Run] Diff for %s:\n\n%s\n", f.FilePath, updated)
		return true
	}

	if err := writeRootFileAtomic(root, relPath, []byte(updated), info.Mode().Perm()); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to safely replace %s: %v\n", f.FilePath, err)
		return false
	}
	fmt.Printf("✅ Fixed %s: Extracted secret to environment variable.\n", f.FilePath)
	return true
}

func createRootedTempFile(root *os.Root, relPath string, mode fs.FileMode) (string, *os.File, error) {
	dir, base := filepath.Dir(relPath), filepath.Base(relPath)
	for attempt := 0; attempt < 10; attempt++ {
		suffix := make([]byte, 12)
		if _, err := rand.Read(suffix); err != nil {
			return "", nil, err
		}
		tempRel := filepath.Join(dir, "."+base+".kuro-"+hex.EncodeToString(suffix))
		file, err := root.OpenFile(tempRel, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
		if err == nil {
			return tempRel, file, nil
		}
		if !os.IsExist(err) {
			return "", nil, err
		}
	}
	return "", nil, fmt.Errorf("unable to allocate a unique temporary file")
}

func markFalsePositive(f RemediationFinding, root *os.Root) error {
	record := SuppressionRecord{
		FindingID: f.ID,
		RuleID:    f.Title,
		FilePath:  f.FilePath,
		Reason:    "Marked as False Positive via Kuro CLI interactive triage",
		CreatedAt: time.Now().UTC(),
		Author:    "developer",
	}

	if err := saveSuppression(root, record); err != nil {
		return err
	}
	fmt.Printf("✅ Finding marked as False Positive. Stored in .kuro-suppressions.json\n")
	return nil
}

func suppressFinding(f RemediationFinding, root *os.Root) error {
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

	if err := saveSuppression(root, record); err != nil {
		return err
	}
	fmt.Printf("✅ Rule suppressed: %s\n", f.Title)
	return nil
}

func saveSuppression(root *os.Root, record SuppressionRecord) error {
	const manifestPath = ".kuro-suppressions.json"
	var records []SuppressionRecord
	mode := fs.FileMode(0644)
	if info, err := root.Lstat(manifestPath); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("suppression manifest is not a regular file")
		}
		mode = info.Mode().Perm()
		data, err := root.ReadFile(manifestPath)
		if err != nil {
			return fmt.Errorf("read suppression manifest: %w", err)
		}
		if err := json.Unmarshal(data, &records); err != nil {
			return fmt.Errorf("parse suppression manifest: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat suppression manifest: %w", err)
	}
	records = append(records, record)
	out, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return fmt.Errorf("encode suppression manifest: %w", err)
	}
	return writeRootFileAtomic(root, manifestPath, out, mode)
}

func writeRootFileAtomic(root *os.Root, relPath string, data []byte, mode fs.FileMode) (err error) {
	tempRel, tempFile, err := createRootedTempFile(root, relPath, mode)
	if err != nil {
		return err
	}
	keepTemp := true
	defer func() {
		if keepTemp {
			_ = root.Remove(tempRel)
		}
	}()
	if _, err := tempFile.Write(data); err != nil {
		_ = tempFile.Close()
		return err
	}
	if err := tempFile.Chmod(mode); err != nil {
		_ = tempFile.Close()
		return err
	}
	if err := tempFile.Sync(); err != nil {
		_ = tempFile.Close()
		return err
	}
	if err := tempFile.Close(); err != nil {
		return err
	}
	if err := root.Rename(tempRel, relPath); err != nil {
		return err
	}
	keepTemp = false
	return nil
}

func deriveEnvVarName(filePath, secretValue string) string {
	base := strings.ToUpper(filepath.Base(filePath))
	base = regexp.MustCompile(`[^A-Z0-9]`).ReplaceAllString(base, "_")
	if strings.HasPrefix(secretValue, "AKIA") || strings.HasPrefix(secretValue, "ASIA") {
		return "AWS_ACCESS_KEY_ID"
	}
	if strings.HasPrefix(secretValue, "ghp_") || strings.HasPrefix(secretValue, "gho_") || strings.HasPrefix(secretValue, "github_pat_") {
		return "GITHUB_TOKEN"
	}
	if strings.HasPrefix(secretValue, "xox") {
		return "SLACK_API_TOKEN"
	}
	if strings.HasPrefix(secretValue, "sk_live_") || strings.HasPrefix(secretValue, "rk_live_") || strings.Contains(strings.ToLower(filePath), "stripe") {
		return "STRIPE_SECRET_KEY"
	}
	if strings.Contains(secretValue, "PRIVATE KEY") {
		return "PRIVATE_KEY"
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

func extractSecretMatch(line string) (name string, value string) {
	for _, det := range knownSecretDetectors {
		if det.subexp > 0 {
			matches := det.pattern.FindStringSubmatch(line)
			if len(matches) > det.subexp && matches[det.subexp] != "" {
				return det.name, matches[det.subexp]
			}
		} else {
			match := det.pattern.FindString(line)
			if match != "" {
				return det.name, match
			}
		}
	}
	return "", ""
}

func openTargetRoot(targetPath string) (*os.Root, string, error) {
	absPath, err := filepath.Abs(targetPath)
	if err != nil {
		return nil, "", fmt.Errorf("resolve target path: %w", err)
	}
	rootPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return nil, "", fmt.Errorf("resolve target path: %w", err)
	}
	before, err := os.Stat(rootPath)
	if err != nil {
		return nil, "", fmt.Errorf("stat target path: %w", err)
	}
	if !before.IsDir() {
		return nil, "", fmt.Errorf("target path is not a directory")
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return nil, "", fmt.Errorf("open target root: %w", err)
	}
	after, err := root.Stat(".")
	if err != nil || !os.SameFile(before, after) {
		_ = root.Close()
		if err != nil {
			return nil, "", fmt.Errorf("stat opened target root: %w", err)
		}
		return nil, "", fmt.Errorf("target directory changed while opening")
	}
	return root, rootPath, nil
}

func loadFindingsFromReport(root *os.Root, rootPath, reportPath string) ([]RemediationFinding, error) {
	data, err := os.ReadFile(reportPath)
	if err != nil {
		return nil, fmt.Errorf("read report %s: %w", reportPath, err)
	}
	var rep struct {
		Findings *[]struct {
			Scanner     string `json:"scanner"`
			Severity    string `json:"severity"`
			Title       string `json:"title"`
			FilePath    string `json:"file_path"`
			LineNumber  int    `json:"line_number"`
			Description string `json:"description"`
		} `json:"findings"`
	}
	if err := json.Unmarshal(data, &rep); err != nil {
		return nil, fmt.Errorf("parse report JSON %s: %w", reportPath, err)
	}

	if rep.Findings == nil {
		return nil, fmt.Errorf("report JSON is missing a findings array")
	}
	if len(*rep.Findings) == 0 {
		return []RemediationFinding{}, nil
	}
	var results []RemediationFinding
	for i, f := range *rep.Findings {
		if f.FilePath == "" || f.LineNumber < 1 {
			return nil, fmt.Errorf("finding %d has no valid file path or line number", i+1)
		}
		filePath := f.FilePath
		if !filepath.IsAbs(filePath) {
			filePath = filepath.Join(rootPath, filePath)
		}
		filePath, err = filepath.Abs(filePath)
		if err != nil {
			return nil, fmt.Errorf("resolve finding %d path: %w", i+1, err)
		}
		rel, err := filepath.Rel(rootPath, filePath)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
			return nil, fmt.Errorf("finding %d file is outside target directory", i+1)
		}
		info, err := root.Lstat(rel)
		if err != nil {
			return nil, fmt.Errorf("stat finding %d file: %w", i+1, err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return nil, fmt.Errorf("finding %d file is not a regular non-symlink file", i+1)
		}
		content, err := root.ReadFile(rel)
		if err != nil {
			return nil, fmt.Errorf("read finding %d file: %w", i+1, err)
		}
		filePath = filepath.Join(rootPath, rel)
		lines := strings.Split(string(content), "\n")
		if f.LineNumber > len(lines) {
			return nil, fmt.Errorf("finding %d line number is outside file", i+1)
		}
		snippet := lines[f.LineNumber-1]
		_, secretVal := extractSecretMatch(snippet)
		fixSuggestion := "Manual review required; no automatic replacement is available."
		if secretVal != "" {
			fixSuggestion = "Extract recognized secret to an environment variable."
		}
		results = append(results, RemediationFinding{
			ID:            fmt.Sprintf("report-%d", i+1),
			Scanner:       f.Scanner,
			Severity:      f.Severity,
			Title:         f.Title,
			FilePath:      filePath,
			LineNumber:    f.LineNumber,
			CodeSnippet:   snippet,
			SecretValue:   secretVal,
			FixSuggestion: fixSuggestion,
		})
	}
	return results, nil
}

func scanLocalFindings(targetPath, reportPath string) ([]RemediationFinding, error) {
	root, rootPath, err := openTargetRoot(targetPath)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return scanLocalFindingsRoot(root, rootPath, reportPath)
}

func scanLocalFindingsRoot(root *os.Root, rootPath, reportPath string) ([]RemediationFinding, error) {
	if reportPath != "" {
		return loadFindingsFromReport(root, rootPath, reportPath)
	}
	var findings []RemediationFinding
	findingIdx := 0
	walkErr := fs.WalkDir(root.FS(), ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == "." {
			return nil
		}
		components := strings.Split(path, "/")
		for _, component := range components {
			if component == ".git" || component == "vendor" || component == "node_modules" {
				if entry.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
		}
		if entry.Type()&os.ModeSymlink != 0 || entry.IsDir() {
			return nil
		}
		info, err := root.Lstat(path)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		data, err := root.ReadFile(path)
		if err != nil {
			return err
		}
		content := string(data)
		for lineIdx, line := range strings.Split(content, "\n") {
			title, secretMatch := extractSecretMatch(line)
			if secretMatch == "" {
				continue
			}
			findingIdx++
			filePath := filepath.Join(rootPath, filepath.FromSlash(path))
			findings = append(findings, RemediationFinding{
				ID: fmt.Sprintf("secret-%d", findingIdx), Scanner: "kuro-security-rules", Severity: "CRITICAL",
				Title: title, FilePath: filePath, LineNumber: lineIdx + 1, CodeSnippet: line,
				SecretValue: secretMatch, FixSuggestion: fmt.Sprintf("Replace literal with %s", deriveEnvVarName(filePath, secretMatch)),
			})
		}
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("scan target directory: %w", walkErr)
	}
	return findings, nil
}
