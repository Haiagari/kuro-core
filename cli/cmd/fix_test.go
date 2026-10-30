package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Helper builders so GitHub Push Protection scanner does not false-positive on unit test strings.
func mockAWSKey() string {
	return "AKIA" + "IOSFODNN7EXAMPLE"
}

func mockGHToken() string {
	return "ghp_" + "1234567890abcdef1234567890abcdef"
}

func mockSlackToken() string {
	return "xoxb-" + "123456789012" + "-1234567890123" + "-456789abcdef0123456789ab"
}

func mockStripeKey() string {
	return "sk_live_" + "1234567890abcdef12345678"
}

func TestAutoFix_GoSecretReplacement(t *testing.T) {
	key := mockAWSKey()
	initialGoCode := fmt.Sprintf(`package main

func getClient() {
	awsKey := "%s"
	println(awsKey)
}
`, key)

	updated, changed := ApplyFixToContent("main.go", initialGoCode, key)
	if !changed {
		t.Fatalf("expected content to be modified by AutoFix")
	}

	if !strings.Contains(updated, `os.Getenv("AWS_ACCESS_KEY_ID")`) {
		t.Fatalf("expected os.Getenv replacement, got:\n%s", updated)
	}

	if !strings.Contains(updated, `import "os"`) {
		t.Fatalf("expected auto-injected os import, got:\n%s", updated)
	}
}

func TestAutoFix_PythonSecretReplacement(t *testing.T) {
	key := mockAWSKey()
	initialPyCode := fmt.Sprintf(`def connect():
    api_key = "%s"
    return api_key
`, key)

	updated, changed := ApplyFixToContent("client.py", initialPyCode, key)
	if !changed {
		t.Fatalf("expected Python content to be modified")
	}

	if !strings.Contains(updated, `os.environ.get("AWS_ACCESS_KEY_ID")`) {
		t.Fatalf("expected os.environ.get replacement, got:\n%s", updated)
	}

	if !strings.Contains(updated, "import os") {
		t.Fatalf("expected import os, got:\n%s", updated)
	}
}

func TestAutoFix_JavaScriptSecretReplacement(t *testing.T) {
	token := mockGHToken()
	initialJSCode := fmt.Sprintf(`const config = {
  githubToken: "%s"
};
`, token)

	updated, changed := ApplyFixToContent("config.ts", initialJSCode, token)
	if !changed {
		t.Fatalf("expected JS content to be modified")
	}

	if !strings.Contains(updated, "process.env.GITHUB_TOKEN") {
		t.Fatalf("expected process.env.GITHUB_TOKEN replacement, got:\n%s", updated)
	}
}

func TestAutoFix_SlackAndStripeReplacements(t *testing.T) {
	slackTok := mockSlackToken()
	slackCode := fmt.Sprintf(`token := "%s"`, slackTok)
	updatedSlack, ok := ApplyFixToContent("slack.go", slackCode, slackTok)
	if !ok || !strings.Contains(updatedSlack, `os.Getenv("SLACK_API_TOKEN")`) {
		t.Fatalf("failed slack replacement, got: %s", updatedSlack)
	}

	stripeTok := mockStripeKey()
	stripeCode := fmt.Sprintf(`key = "%s"`, stripeTok)
	updatedStripe, ok := ApplyFixToContent("billing.py", stripeCode, stripeTok)
	if !ok || !strings.Contains(updatedStripe, `os.environ.get("STRIPE_SECRET_KEY")`) {
		t.Fatalf("failed stripe replacement, got: %s", updatedStripe)
	}
}

func TestAutoFix_SuppressionManifest(t *testing.T) {
	tmpDir := t.TempDir()
	root, err := os.OpenRoot(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	record := SuppressionRecord{
		FindingID: "f-123", RuleID: "HARDCODED_KEY", FilePath: "services/api/main.go",
		Reason: "Approved mock key in test suite", CreatedAt: time.Now().UTC(), Author: "lead-architect",
	}
	if err := saveSuppression(root, record); err != nil {
		t.Fatalf("save suppression: %v", err)
	}
	data, err := root.ReadFile(".kuro-suppressions.json")
	if err != nil {
		t.Fatalf("failed to read created suppression manifest: %v", err)
	}
	if !strings.Contains(string(data), "Approved mock key in test suite") {
		t.Fatalf("manifest missing reason text: %s", string(data))
	}
}

func TestAutoFix_SuppressionManifestStaysInOpenedRoot(t *testing.T) {
	parent := t.TempDir()
	targetDir := filepath.Join(parent, "target")
	movedDir := filepath.Join(parent, "target-moved")
	outsideDir := filepath.Join(parent, "outside")
	for _, dir := range []string{targetDir, outsideDir} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	root, err := os.OpenRoot(targetDir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := os.Rename(targetDir, movedDir); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsideDir, targetDir); err != nil {
		t.Fatal(err)
	}
	record := SuppressionRecord{FindingID: "f-escape", RuleID: "RULE", Reason: "test", CreatedAt: time.Now().UTC(), Author: "test"}
	if err := saveSuppression(root, record); err != nil {
		t.Fatalf("save in opened root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outsideDir, ".kuro-suppressions.json")); !os.IsNotExist(err) {
		t.Fatalf("suppression escaped opened root: err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(movedDir, ".kuro-suppressions.json")); err != nil {
		t.Fatalf("manifest missing from opened root: %v", err)
	}
}

func TestScanLocalFindings_MultiSecretDetection(t *testing.T) {
	tmpDir := t.TempDir()

	fixtureCode := fmt.Sprintf(`package test
const AwsKey = "%s"
const GhToken = "%s"
const SlackToken = "%s"
const StripeKey = "%s"
const SecretVal = "api_key = \"supersecret1234567890\""
`, mockAWSKey(), mockGHToken(), mockSlackToken(), mockStripeKey())

	fixtureFile := filepath.Join(tmpDir, "secrets.go")
	if err := os.WriteFile(fixtureFile, []byte(fixtureCode), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	findings, err := scanLocalFindings(tmpDir, "")
	if err != nil {
		t.Fatalf("scan local findings: %v", err)
	}
	if len(findings) < 4 {
		t.Fatalf("expected at least 4 detected secrets, got %d", len(findings))
	}

	foundAWS := false
	foundGH := false
	foundSlack := false
	foundStripe := false
	for _, f := range findings {
		switch f.Title {
		case "AWS Access Key":
			foundAWS = true
		case "GitHub Personal Access Token":
			foundGH = true
		case "Slack Token":
			foundSlack = true
		case "Stripe Secret Key":
			foundStripe = true
		}
	}

	if !foundAWS || !foundGH || !foundSlack || !foundStripe {
		t.Fatalf("did not detect all secret types: AWS=%v, GH=%v, Slack=%v, Stripe=%v",
			foundAWS, foundGH, foundSlack, foundStripe)
	}
}

func TestScanLocalFindings_FromReport(t *testing.T) {
	tmpDir := t.TempDir()

	token := mockGHToken()
	targetFile := filepath.Join(tmpDir, "app.py")
	if err := os.WriteFile(targetFile, []byte(fmt.Sprintf("token = \"%s\"\n", token)), 0640); err != nil {
		t.Fatal(err)
	}

	reportPayload := map[string]any{
		"findings": []map[string]any{{
			"scanner": "gitleaks", "severity": "CRITICAL", "title": "GitHub Personal Access Token",
			"file_path": "app.py", "line_number": 1, "description": "Detected hardcoded token",
		}},
	}
	reportData, err := json.Marshal(reportPayload)
	if err != nil {
		t.Fatal(err)
	}
	reportPath := filepath.Join(tmpDir, "kuro-report.json")
	if err := os.WriteFile(reportPath, reportData, 0600); err != nil {
		t.Fatal(err)
	}

	findings, err := scanLocalFindings(tmpDir, reportPath)
	if err != nil {
		t.Fatalf("load findings: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding loaded from report, got %d", len(findings))
	}
	if findings[0].SecretValue != token {
		t.Fatalf("expected secretValue extracted from reported file, got %q", findings[0].SecretValue)
	}
	if findings[0].FilePath != targetFile {
		t.Fatalf("expected normalized absolute file path %q, got %q", targetFile, findings[0].FilePath)
	}
}

func TestScanLocalFindings_ReportErrorsFailClosed(t *testing.T) {
	root := t.TempDir()
	for _, tc := range []struct{ name, payload string }{
		{"missing-file", ""}, {"malformed-json", "{"}, {"missing-findings-array", `{}`}, {"null-findings-array", `{"findings":null}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(root, tc.name+".json")
			if tc.payload != "" {
				if err := os.WriteFile(path, []byte(tc.payload), 0600); err != nil {
					t.Fatal(err)
				}
			}
			_, err := scanLocalFindings(root, path)
			if err == nil {
				t.Fatal("expected report error; report input must not silently fall back to filesystem scanning")
			}
		})
	}
}

func TestScanLocalFindings_EmptyReportDoesNotFallback(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "local.go"), []byte(fmt.Sprintf("key = %q\\n", mockAWSKey())), 0600); err != nil {
		t.Fatal(err)
	}
	report := filepath.Join(root, "report.json")
	if err := os.WriteFile(report, []byte(`{"findings":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	findings, err := scanLocalFindings(root, report)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("empty report must not fall back to local heuristic scanning: %#v", findings)
	}
}

func TestScanLocalFindings_ReportRejectsOutOfRootAndUnverifiable(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.go")
	if err := os.WriteFile(outside, []byte(fmt.Sprintf("x = %q\n", mockAWSKey())), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "escape.go")
	if err := os.Symlink(outside, link); err != nil && !os.IsPermission(err) {
		t.Fatal(err)
	}
	cases := []struct {
		name, file string
		line       int
	}{
		{"outside", outside, 1}, {"invalid-line", "inside.go", 0}, {"missing-file", "missing.go", 1}, {"symlink-escape", "escape.go", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload, err := json.Marshal(map[string]any{"findings": []map[string]any{{"scanner": "gitleaks", "severity": "CRITICAL", "title": "secret", "file_path": tc.file, "line_number": tc.line}}})
			if err != nil {
				t.Fatal(err)
			}
			report := filepath.Join(root, "report.json")
			if err := os.WriteFile(report, payload, 0600); err != nil {
				t.Fatal(err)
			}
			findings, err := scanLocalFindings(root, report)
			if err == nil || len(findings) != 0 {
				t.Fatalf("expected rejected report finding, got %#v, err=%v", findings, err)
			}
		})
	}
}

func TestApplyFixToContent_UnsupportedFileIsNotRewritten(t *testing.T) {
	secret := mockAWSKey()
	original := `value = "` + secret + `"`
	updated, changed := ApplyFixToContent("config.yaml", original, secret)
	if changed || updated != original {
		t.Fatalf("unsupported syntax must remain unchanged, changed=%v content=%q", changed, updated)
	}
}

func TestApplyFixToContent_UnquotedSecretIsNotRewritten(t *testing.T) {
	secret := mockAWSKey()
	original := "// documentation mentions " + secret + " in prose\n"
	updated, changed := ApplyFixToContent("main.go", original, secret)
	if changed || updated != original {
		t.Fatalf("unquoted secret must remain unchanged, changed=%v content=%q", changed, updated)
	}
}

func TestFixHelpDocumentsReportLimits(t *testing.T) {
	stdout, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	previous := os.Stdout
	os.Stdout = writer
	status := RunFixWithStatus([]string{"--help"})
	_ = writer.Close()
	os.Stdout = previous
	output, err := io.ReadAll(stdout)
	_ = stdout.Close()
	if err != nil {
		t.Fatal(err)
	}
	if status != 0 {
		t.Fatalf("help returned status %d", status)
	}
	for _, want := range []string{"--report, -r <file>", "Go, Python, or JS/TS", "inside [path]", "exit status 1"} {
		if !strings.Contains(string(output), want) {
			t.Errorf("help missing %q: %s", want, output)
		}
	}
}

func TestApplyFixToFile_PreservesPermissions(t *testing.T) {
	rootDir := t.TempDir()
	path := filepath.Join(rootDir, "app.go")
	secret := mockAWSKey()
	original := fmt.Sprintf("package main\\nconst key = %q\\n", secret)
	if err := os.WriteFile(path, []byte(original), 0640); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(rootDir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if ok := applyFixToFile(root, "app.go", RemediationFinding{FilePath: path, SecretValue: secret}, false); !ok {
		t.Fatal("expected replacement")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0640 {
		t.Fatalf("permissions changed: got %04o, want 0640", got)
	}
	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(updated), secret) {
		t.Fatal("secret remains in rewritten file")
	}
}

func TestApplyFixToFile_DirectorySwapCannotEscapeRoot(t *testing.T) {
	parent := t.TempDir()
	targetDir := filepath.Join(parent, "target")
	outsideDir := filepath.Join(parent, "outside")
	movedDir := filepath.Join(parent, "target-moved")
	if err := os.Mkdir(targetDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(outsideDir, 0700); err != nil {
		t.Fatal(err)
	}
	secret := mockAWSKey()
	targetFile := filepath.Join(targetDir, "app.go")
	outsideFile := filepath.Join(outsideDir, "app.go")
	original := fmt.Sprintf("package main\\nconst key = %q\\n", secret)
	if err := os.WriteFile(targetFile, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outsideFile, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(targetDir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := os.Rename(targetDir, movedDir); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsideDir, targetDir); err != nil {
		t.Fatal(err)
	}

	if !applyFixToFile(root, "app.go", RemediationFinding{FilePath: targetFile, SecretValue: secret}, false) {
		t.Fatal("expected rooted replacement")
	}
	outsideAfter, err := os.ReadFile(outsideFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(outsideAfter) != original {
		t.Fatal("replacement escaped the opened target root")
	}
	insideAfter, err := os.ReadFile(filepath.Join(movedDir, "app.go"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(insideAfter), secret) {
		t.Fatal("opened target root was not updated")
	}
}

func TestRunFixWithStatus_AutoUnremediableFindingFails(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "app.py")
	original := "print('no recognized secret')\\n"
	if err := os.WriteFile(source, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	report := filepath.Join(root, "report.json")
	payload, err := json.Marshal(map[string]any{"findings": []map[string]any{{"scanner": "semgrep", "severity": "HIGH", "title": "SQL injection", "file_path": "app.py", "line_number": 1}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(report, payload, 0600); err != nil {
		t.Fatal(err)
	}

	stdout, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	previous := os.Stdout
	os.Stdout = writer
	status := RunFixWithStatus([]string{root, "--report", report, "--auto"})
	_ = writer.Close()
	os.Stdout = previous
	output, err := io.ReadAll(stdout)
	_ = stdout.Close()
	if err != nil {
		t.Fatal(err)
	}
	if status == 0 {
		t.Fatal("expected nonzero status when auto mode cannot remediate a finding")
	}
	if strings.Contains(string(output), "completed successfully") {
		t.Fatalf("must not claim success: %s", output)
	}
	after, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != original {
		t.Fatalf("unremediable finding modified source: %q", after)
	}
}
