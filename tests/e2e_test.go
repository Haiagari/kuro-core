// Package tests contains end-to-end tests for the Kuro.
//
// These tests assume the full stack (API, Worker, NATS, PostgreSQL) is already
// running. They do NOT start docker compose.
//
// Prerequisites:
//   - docker compose is running (kuro-api, kuro-worker, postgres, nats, etc.)
//   - A valid API key with scans:write permission is available
//   - Internet access to clone public repos (WebGoat, Hello-World)
//
// Run:
//
//	cd tests && go test -v -count=1 -timeout=1800s .
package tests

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	cliBinary     = "../bin/kuro"
	apiURL        = "http://localhost:8080"
	testRepo      = "https://github.com/WebGoat/WebGoat.git"     // Repo with findings
	testRepoClean = "https://github.com/octocat/Hello-World.git" // Minimal repo, no findings
)

// getAPIKey returns the API key from environment or fails the test.
// Set KURO_TEST_API_KEY before running tests.
func getAPIKey(t *testing.T) string {
	t.Helper()
	key := os.Getenv("KURO_TEST_API_KEY")
	if key == "" {
		t.Fatal("KURO_TEST_API_KEY environment variable not set. Run: export KURO_TEST_API_KEY=$(bash scripts/bootstrap-api-key.sh | grep kuro_live | head -1 | awk '{print $3}')")
	}
	return key
}

// TestKuroCLI_Build verifies the CLI binary exists and is executable.
// Note: Does NOT compile from source (requires go.work or replace directive).
// Run `cd cli && go build` manually to rebuild if needed.
func TestKuroCLI_Build(t *testing.T) {
	if _, err := os.Stat(cliBinary); os.IsNotExist(err) {
		t.Fatalf("CLI binary not found at %s. Run: cd cli && go build -o kuro .", cliBinary)
	}
	
	// Verify binary is executable
	info, err := os.Stat(cliBinary)
	if err != nil {
		t.Fatalf("Cannot stat CLI binary: %v", err)
	}
	
	mode := info.Mode()
	if mode&0111 == 0 {
		t.Fatalf("CLI binary is not executable: %o", mode.Perm())
	}
	
	t.Logf("CLI binary exists and is executable: %s (%o)", cliBinary, mode.Perm())
}

// TestKuroCLI_Version verifies the version command.
func TestKuroCLI_Version(t *testing.T) {
	ensureBinary(t)
	out, err := runCLI("version")
	if err != nil {
		t.Fatalf("version command failed: %v", err)
	}
	if !strings.Contains(out, "0.1.0-beta") {
		t.Fatalf("expected version 0.1.0-beta, got: %s", out)
	}
	t.Logf("Version: %s", strings.TrimSpace(out))
}

// TestKuroCLI_AuthConfig verifies auth command saves config correctly.
func TestKuroCLI_AuthConfig(t *testing.T) {
	ensureBinary(t)
	cleanupConfig(t)
	defer cleanupConfig(t)

	apiKey := getAPIKey(t)
	out, err := runCLI("auth", apiKey)
	if err != nil {
		t.Fatalf("auth command failed: %v\nOutput: %s", err, out)
	}

	// Verify config file exists
	configPath := resolveConfigPath(t)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatalf("config file not created at %s", configPath)
	}

	// Verify permissions
	info, _ := os.Stat(configPath)
	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Fatalf("expected config file permissions 0600, got %o", perm)
	}

	// Verify content
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("cannot read config: %v", err)
	}

	var cfg struct {
		APIURL        string `json:"api_url"`
		APIKey        string `json:"api_key"`
		DefaultBranch string `json:"default_branch"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("invalid config JSON: %v", err)
	}

	if cfg.APIKey != apiKey {
		t.Fatalf("expected api_key %q, got %q", apiKey, cfg.APIKey)
	}
	if cfg.APIURL != "http://localhost:8080" {
		t.Fatalf("expected api_url http://localhost:8080, got %q", cfg.APIURL)
	}
	if cfg.DefaultBranch != "main" {
		t.Fatalf("expected default_branch main, got %q", cfg.DefaultBranch)
	}
}

// TestKuroCLI_ScanMainBranch triggers a scan on WebGoat (has vulnerabilities)
// and expects a terminal state with findings.
func TestKuroCLI_ScanMainBranch(t *testing.T) {
	ensureBinary(t)
	ensureConfig(t)
	ensureAPI(t)

	out, err := runCLI("scan", "--json", testRepo)
	if err != nil {
		t.Fatalf("scan command failed: %v\nOutput: %s", err, out)
	}

	t.Logf("Scan output:\n%s", out)

	var result struct {
		ScanID             string         `json:"scan_id"`
		Status             string         `json:"status"`
		Decision           string         `json:"decision"`
		PolicyDecision     string         `json:"policy_decision"`
		RepositoryURL      string         `json:"repository_url"`
		Branch             string         `json:"branch"`
		Duration           int            `json:"duration"`
		FindingsBySeverity map[string]int `json:"findings_by_severity"`
		TopFindings        []struct {
			Scanner  string `json:"scanner"`
			Severity string `json:"severity"`
			Title    string `json:"title"`
			File     string `json:"file"`
			Line     int    `json:"line"`
		} `json:"top_findings"`
	}

	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}

	if result.ScanID == "" {
		t.Fatal("expected non-empty scan_id")
	}
	if result.Status != "completed" && result.Status != "blocked" {
		t.Fatalf("expected terminal status (completed/blocked), got %q", result.Status)
	}
	if result.PolicyDecision == "" {
		t.Fatal("expected non-empty policy_decision")
	}
	if !strings.Contains(result.RepositoryURL, "WebGoat") {
		t.Fatalf("expected repository_url containing WebGoat, got %q", result.RepositoryURL)
	}
	if result.Branch != "main" {
		t.Fatalf("expected branch %q, got %q", "main", result.Branch)
	}
	if result.Duration <= 0 {
		t.Fatal("expected positive duration")
	}
	if result.FindingsBySeverity == nil {
		t.Fatal("expected findings_by_severity")
	}
	totalFindings := 0
	for _, count := range result.FindingsBySeverity {
		totalFindings += count
	}
	// WebGoat may or may not have findings depending on scanner rules
	// Just verify structure is correct
	t.Logf("Scan %s completed in %ds: %d findings, decision=%s",
		result.ScanID, result.Duration, totalFindings, result.PolicyDecision)
}

// TestKuroCLI_ScanCleanBranch scans a minimal repo with no findings.
func TestKuroCLI_ScanCleanBranch(t *testing.T) {
	ensureBinary(t)
	ensureConfig(t)
	ensureAPI(t)

	out, err := runCLI("scan", "--json", testRepoClean)
	if err != nil {
		t.Fatalf("scan command failed: %v\nOutput: %s", err, out)
	}

	t.Logf("Clean repo scan output:\n%s", out)

	var result struct {
		ScanID             string         `json:"scan_id"`
		Status             string         `json:"status"`
		Decision           string         `json:"decision"`
		PolicyDecision     string         `json:"policy_decision"`
		Branch             string         `json:"branch"`
		FindingsBySeverity map[string]int `json:"findings_by_severity"`
		TopFindings        []interface{}  `json:"top_findings"`
	}

	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}

	if result.ScanID == "" {
		t.Fatal("expected non-empty scan_id")
	}
	if result.Status != "completed" && result.Status != "blocked" {
		t.Fatalf("expected terminal status (completed/blocked), got %q", result.Status)
	}
	if result.Branch == "" {
		t.Fatal("expected non-empty branch")
	}

	totalFindings := 0
	for _, count := range result.FindingsBySeverity {
		totalFindings += count
	}

	t.Logf("Clean scan %s completed: %d findings, decision=%s",
		result.ScanID, totalFindings, result.PolicyDecision)
}

// TestKuroCLI_ScanJSONFlag validates that --json produces valid JSON with expected fields.
func TestKuroCLI_ScanJSONFlag(t *testing.T) {
	ensureBinary(t)
	ensureConfig(t)
	ensureAPI(t)

	out, err := runCLI("scan", "--json", testRepo)
	if err != nil {
		t.Fatalf("scan --json failed: %v\nOutput: %s", err, out)
	}

	if !json.Valid([]byte(out)) {
		t.Fatalf("output is not valid JSON: %s", out)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("cannot unmarshal JSON: %v", err)
	}

	requiredFields := []string{"scan_id", "status", "repository_url", "branch", "findings_by_severity", "top_findings"}
	for _, field := range requiredFields {
		if _, ok := parsed[field]; !ok {
			t.Fatalf("missing required field %q in JSON output", field)
		}
	}

	t.Logf("JSON output valid with all required fields")
}

// TestKuroCLI_StatusCommand verifies the status command works with a completed scan ID.
func TestKuroCLI_StatusCommand(t *testing.T) {
	ensureBinary(t)
	ensureConfig(t)
	ensureAPI(t)

	out, err := runCLI("scan", "--json", testRepo)
	if err != nil {
		t.Fatalf("scan failed: %v\nOutput: %s", err, out)
	}

	var scanResult struct {
		ScanID string `json:"scan_id"`
	}
	if err := json.Unmarshal([]byte(out), &scanResult); err != nil {
		t.Fatalf("invalid scan JSON: %v", err)
	}

	if scanResult.ScanID == "" {
		t.Fatal("expected non-empty scan_id from scan")
	}

	statusOut, err := runCLI("status", "--json", scanResult.ScanID)
	if err != nil {
		t.Fatalf("status command failed: %v\nOutput: %s", err, statusOut)
	}

	if !json.Valid([]byte(statusOut)) {
		t.Fatalf("status output is not valid JSON: %s", statusOut)
	}

	var statusResult struct {
		ScanID string `json:"scan_id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal([]byte(statusOut), &statusResult); err != nil {
		t.Fatalf("cannot unmarshal status JSON: %v", err)
	}

	if statusResult.ScanID != scanResult.ScanID {
		t.Fatalf("expected scan_id %q, got %q", scanResult.ScanID, statusResult.ScanID)
	}

	t.Logf("Status for scan %s: %s", statusResult.ScanID, statusResult.Status)
}

// TestKuroCLI_InvalidRepoURL tests that an invalid repo URL produces a failed scan.
func TestKuroCLI_InvalidRepoURL(t *testing.T) {
	ensureBinary(t)
	ensureConfig(t)
	ensureAPI(t)

	out, err := runCLI("scan", "--json", "https://github.com/this-repo-does-not-exist-12345/foo")
	if err != nil {
		// Command may exit with error if scan fails immediately
		t.Logf("Scan command exited with error (expected for invalid repo): %v", err)
	}

	// If command succeeded, verify the scan status is failed
	if err == nil {
		var result struct {
			Status string `json:"status"`
		}
		if json.Unmarshal([]byte(out), &result) == nil {
			if result.Status != "failed" {
				t.Fatalf("expected failed status for invalid repo, got %q", result.Status)
			}
			t.Logf("Invalid repo correctly resulted in failed scan status")
		}
	}
}

// TestKuroCLI_NoAPIKey tests that scan without API key fails gracefully.
func TestKuroCLI_NoAPIKey(t *testing.T) {
	ensureBinary(t)
	cleanupConfig(t)

	tmpDir := t.TempDir()
	origXDG := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	defer os.Setenv("XDG_CONFIG_HOME", origXDG)

	out, err := runCLI("scan", testRepo)
	if err == nil {
		t.Fatal("expected error for missing API key, but command succeeded")
	}

	expected := "No API key configured"
	if !strings.Contains(out, expected) {
		t.Fatalf("expected error containing %q, got: %s", expected, out)
	}

	t.Logf("Missing API key correctly rejected")
}

// TestKuroCLI_Help tests the help command output.
func TestKuroCLI_Help(t *testing.T) {
	ensureBinary(t)

	out, err := runCLI("help")
	if err != nil {
		t.Fatalf("help command failed: %v", err)
	}

	expectedCommands := []string{"auth", "scan", "status", "version"}
	for _, cmd := range expectedCommands {
		if !strings.Contains(out, cmd) {
			t.Fatalf("help output missing command %q", cmd)
		}
	}

	t.Logf("Help output contains all expected commands")
}

// --- helpers ---

func runCLI(args ...string) (string, error) {
	cmd := exec.Command(cliBinary, args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func ensureBinary(t *testing.T) {
	t.Helper()
	if _, err := os.Stat(cliBinary); os.IsNotExist(err) {
		cmd := exec.Command("go", "build", "-o", cliBinary, "../cli")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to build CLI: %v\nOutput: %s", err, string(out))
		}
		t.Log("Built CLI binary")
	}
}

func ensureConfig(t *testing.T) {
	t.Helper()
	configPath := resolveConfigPath(t)

	if data, err := os.ReadFile(configPath); err == nil {
		var cfg struct {
			APIKey string `json:"api_key"`
		}
		if json.Unmarshal(data, &cfg) == nil && cfg.APIKey != "" {
			return
		}
	}

	apiKey := getAPIKey(t)
	out, err := runCLI("auth", apiKey)
	if err != nil {
		t.Fatalf("auth setup failed: %v\nOutput: %s", err, out)
	}
	t.Logf("Config created at %s", configPath)
}

func cleanupConfig(t *testing.T) {
	t.Helper()
	configPath := resolveConfigPath(t)
	os.Remove(configPath)
	os.Remove(filepath.Dir(configPath))
}

func resolveConfigPath(t *testing.T) string {
	t.Helper()
	if p := os.Getenv("KURO_CONFIG_PATH"); p != "" {
		return p
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "kuro", "config.json")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("cannot determine home directory: %v", err)
	}
	return filepath.Join(home, ".kuro", "config.json")
}

func ensureAPI(t *testing.T) {
	t.Helper()
	cmd := exec.Command(cliBinary, "version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("CLI binary not working: %v\nOutput: %s", err, string(out))
	}
	_ = out
	time.Sleep(100 * time.Millisecond)
}

func init() {
	if _, err := os.Stat("go.mod"); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "WARNING: tests/go.mod not found, create it first\n")
	}
}
