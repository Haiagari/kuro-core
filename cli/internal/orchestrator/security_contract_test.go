package orchestrator

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestContainerFailureWithJSONIsNotSuccess(t *testing.T) {
	for _, code := range []string{"1", "2", "137"} {
		t.Run(code, func(t *testing.T) {
			runtime := filepath.Join(t.TempDir(), "runtime")
			os.WriteFile(runtime, []byte("#!/bin/sh\nprintf '[]'\nexit "+code+"\n"), 0755)
			a := &LocalAdapter{runtime: runtime}
			out, err := a.runContainer(context.Background(), "scanner", nil, nil)
			if err == nil || string(out) != "[]" {
				t.Fatalf("expected preserved output and process error: %q, %v", out, err)
			}
			_, err = a.runGitleaks(context.Background(), t.TempDir())
			if err == nil {
				t.Fatal("nonzero process with zero findings must fail")
			}
		})
	}
}
func TestGitleaksOnlyAcceptsDocumentedFindingExit(t *testing.T) {
	for _, tc := range []struct {
		code    string
		wantErr bool
	}{{"1", false}, {"2", true}, {"137", true}} {
		t.Run(tc.code, func(t *testing.T) {
			runtime := filepath.Join(t.TempDir(), "runtime")
			os.WriteFile(runtime, []byte("#!/bin/sh\nprintf '[{\"RuleID\":\"token\",\"File\":\"a.go\"}]'\nexit "+tc.code+"\n"), 0755)
			a := &LocalAdapter{runtime: runtime}
			findings, err := a.runGitleaks(context.Background(), t.TempDir())
			if (err != nil) != tc.wantErr {
				t.Fatalf("findings=%v error=%v", findings, err)
			}
		})
	}
}
func TestIncompleteScannerReportsBlock(t *testing.T) {
	for _, tc := range []struct {
		name  string
		parse func([]byte) ([]Finding, error)
		data  string
	}{
		{"semgrep errors", parseSemgrepOutput, `{"results":[],"errors":[{"type":"ParseError"}]}`},
		{"semgrep missing results", parseSemgrepOutput, `{}`},
		{"trivy missing results", parseTrivyOutput, `{}`},
		{"trufflehog malformed line", parseTrufflehogOutput, "{bad}\n"},
		{"trufflehog missing detector", parseTrufflehogOutput, "{}\n"},
		{"checkov parse errors", parseCheckovOutput, `{"check_type":"terraform","results":{"failed_checks":[],"parsing_errors":["main.tf"]}}`},
		{"checkov wrong shape", parseCheckovOutput, `[{"check_id":"CKV_1"}]`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := tc.parse([]byte(tc.data)); err == nil {
				t.Fatal("incomplete report accepted")
			}
		})
	}
}
func TestCheckovNativeMultipleFrameworks(t *testing.T) {
	data := `[{"check_type":"terraform","results":{"failed_checks":[{"check_id":"CKV_1","file_path":"/src/main.tf","check_name":"unsafe","file_line_range":[4,5]}]}},{"check_type":"dockerfile","results":{"failed_checks":[]}}]`
	findings, err := parseCheckovOutput([]byte(data))
	if err != nil || len(findings) != 1 || findings[0].LineNumber != 4 {
		t.Fatalf("%v %v", findings, err)
	}
}
func TestTrivyMissingOrExpiredDBFailsClosed(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("KURO_TRIVY_CACHE_DIR", dir)
	a := &LocalAdapter{runtime: "nonexistent-runtime"}
	if _, err := a.runTrivy(context.Background(), t.TempDir()); err == nil {
		t.Fatal("missing DB must fail")
	}
	os.MkdirAll(filepath.Join(dir, "db"), 0755)
	os.WriteFile(filepath.Join(dir, "db", "trivy.db"), []byte("fixture"), 0644)
	now := time.Now().UTC()
	for _, tc := range []struct {
		name          string
		updated, next time.Time
		version       int
		wantErr       bool
	}{
		{"fresh", now.Add(-time.Hour), now.Add(time.Hour), 2, false},
		{"expired", now.Add(-time.Hour), now.Add(-time.Minute), 2, true},
		{"too old", now.Add(-25 * time.Hour), now.Add(time.Hour), 2, true},
		{"future", now.Add(time.Hour), now.Add(2 * time.Hour), 2, true},
		{"wrong schema", now.Add(-time.Hour), now.Add(time.Hour), 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, _ := json.Marshal(map[string]any{"Version": tc.version, "UpdatedAt": tc.updated, "NextUpdate": tc.next})
			os.WriteFile(filepath.Join(dir, "db", "metadata.json"), data, 0644)
			if err := validateTrivyDB(dir, now); (err != nil) != tc.wantErr {
				t.Fatalf("%v", err)
			}
		})
	}
}
func TestPolicyCombinesFiltersAndPatterns(t *testing.T) {
	p := &Policy{Name: "test", Version: "1", Rules: []PolicyRule{{Scanner: "checkov", Severity: "MEDIUM", FilePatterns: []string{"k8s/**/*.yaml"}, Threshold: 2, Action: "BLOCK"}}}
	cases := []struct {
		findings []Finding
		want     string
	}{
		{[]Finding{{Scanner: "semgrep", Severity: "MEDIUM", FilePath: "k8s/app.yaml"}, {Scanner: "checkov", Severity: "LOW", FilePath: "k8s/app.yaml"}}, "pass"},
		{[]Finding{{Scanner: "checkov", Severity: "MEDIUM", FilePath: "/src/k8s/app.yaml"}}, "pass"},
		{[]Finding{{Scanner: "checkov", Severity: "MEDIUM", FilePath: "/src/k8s/app.yaml"}, {Scanner: "checkov", Severity: "MEDIUM", FilePath: "k8s/sub/app.yaml"}}, "block"},
		{[]Finding{{Scanner: "checkov", Severity: "MEDIUM", FilePath: "other/app.yaml"}, {Scanner: "checkov", Severity: "MEDIUM", FilePath: "other/app.yaml"}}, "pass"},
	}
	for _, tc := range cases {
		if got := p.Evaluate(tc.findings); got != tc.want {
			t.Fatalf("got %s want %s for %+v", got, tc.want, tc.findings)
		}
	}
}
func TestInvalidPolicyCannotFallbackToPass(t *testing.T) {
	for _, content := range []string{`{"name":"p","version":"1","rules":[{"severity":"MEDIUM","action":"BLOKC"}]}`, `{"name":"p","version":"1","rules":[],"typo":1}`, `null`, `{bad}`} {
		path := filepath.Join(t.TempDir(), "policy.json")
		os.WriteFile(path, []byte(content), 0644)
		t.Setenv("KURO_POLICY_PATH", path)
		o := New(&mockAdapter{name: "local"}, false)
		result, err := o.Run(context.Background(), t.TempDir())
		if err == nil || result.Decision != "block" {
			t.Fatalf("invalid policy accepted: %s (%v)", content, err)
		}
	}
}
func TestPolicySafetyFloorsCannotBeRelaxed(t *testing.T) {
	p := &Policy{Name: "test", Version: "1", Rules: []PolicyRule{{Severity: "HIGH", Action: "PASS"}}}
	if p.Evaluate([]Finding{{Severity: "HIGH"}}) != "review" || p.Evaluate([]Finding{{Severity: "CRITICAL"}}) != "block" || p.Evaluate([]Finding{{Scanner: "gitleaks", Severity: "LOW"}}) != "block" {
		t.Fatal("mandatory safety floor bypassed")
	}
}
func TestCacheDetectsSameSizePreservedMtimeChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.go")
	os.WriteFile(path, []byte("safe"), 0644)
	info, _ := os.Stat(path)
	cache, _ := NewFileCache(t.TempDir())
	cache.MarkScanned(path)
	os.WriteFile(path, []byte("evil"), 0644)
	os.Chtimes(path, info.ModTime(), info.ModTime())
	changed, _, err := cache.IsChanged(path)
	if err != nil || !changed {
		t.Fatalf("same-size change hidden: %v", err)
	}
}
func TestLocalScopeNeverReusesApproval(t *testing.T) {
	t.Setenv("KURO_CACHE_DIR", t.TempDir())
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.go"), []byte("package main"), 0644)
	a := &LocalAdapter{}
	a.CommitCache(dir)
	// Scanner/policy/DB changes cannot be bypassed by cached source fingerprints.
	scanners, err := a.Scope(context.Background(), dir)
	if err != nil || len(scanners) != 2 {
		t.Fatalf("current scanners skipped: %v %v", scanners, err)
	}
}

func TestScopeTraversalFailureBlocks(t *testing.T) {
	a := &LocalAdapter{noCache: true}
	if _, err := a.Scope(context.Background(), filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("unreadable scope treated as clean")
	}
}
func TestTrivyOfflineArgsAndProcessFailure(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("KURO_TRIVY_CACHE_DIR", dir)
	os.MkdirAll(filepath.Join(dir, "db"), 0755)
	os.WriteFile(filepath.Join(dir, "db", "trivy.db"), []byte("fixture"), 0644)
	now := time.Now().UTC()
	meta, _ := json.Marshal(map[string]any{"Version": 2, "UpdatedAt": now.Add(-time.Hour), "NextUpdate": now.Add(time.Hour)})
	os.WriteFile(filepath.Join(dir, "db", "metadata.json"), meta, 0644)
	log := filepath.Join(t.TempDir(), "args")
	t.Setenv("KURO_TEST_ARGS", log)
	runtime := filepath.Join(t.TempDir(), "runtime")
	os.WriteFile(runtime, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$KURO_TEST_ARGS\"\nprintf '{\"Results\":[]}'\nexit 2\n"), 0755)
	a := &LocalAdapter{runtime: runtime}
	if _, err := a.runTrivy(context.Background(), t.TempDir()); err == nil {
		t.Fatal("Trivy failure with JSON accepted")
	}
	args, _ := os.ReadFile(log)
	for _, flag := range []string{"--network=none", "--cap-drop=ALL", "--security-opt=no-new-privileges", "--skip-db-update", "--skip-java-db-update", "--offline-scan", "--cache-dir=/trivy-cache", "HOME=/tmp", "--user\n" + strconv.Itoa(os.Getuid()) + ":" + strconv.Itoa(os.Getgid()), "GIT_CONFIG_VALUE_0=/src"} {
		if !strings.Contains(string(args), flag) {
			t.Fatalf("missing %s: %s", flag, args)
		}
	}
}

func TestDefaultPolicyBlocksNestedIaCAndDependencyFindings(t *testing.T) {
	p, err := LoadDefaultPolicy()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []Finding{{Scanner: "checkov", Severity: "MEDIUM", FilePath: "/src/nested/main.tf"}, {Scanner: "trivy", Severity: "MEDIUM", FilePath: "/src/nested/Cargo.lock"}} {
		if p.Evaluate([]Finding{f}) != "block" {
			t.Fatalf("default scanner rule skipped nested finding: %+v", f)
		}
	}
}
func TestTrufflehogNativeGitMetadata(t *testing.T) {
	data := `{"DetectorName":"GitHub","SourceMetadata":{"Data":{"Git":{"file":"config.go","line":12,"commit":"abc123"}}},"Verified":false}`
	findings, err := parseTrufflehogOutput([]byte(data))
	if err != nil || len(findings) != 1 || findings[0].FilePath != "config.go" || findings[0].LineNumber != 12 || findings[0].Verified {
		t.Fatalf("native metadata lost: %v %v", findings, err)
	}
}

func TestSemgrepConfigurationFailureIncludesDiagnostic(t *testing.T) {
	argsFile := filepath.Join(t.TempDir(), "args")
	t.Setenv("KURO_TEST_ARGS", argsFile)
	runtime := filepath.Join(t.TempDir(), "runtime")
	os.WriteFile(runtime, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$KURO_TEST_ARGS\"\nprintf '%s' '{\"errors\":[{\"message\":\"Invalid YAML rule\"}],\"results\":[]}'\nexit 7\n"), 0755)
	a := &LocalAdapter{runtime: runtime}
	if _, err := a.runSemgrep(context.Background(), t.TempDir()); err == nil || !strings.Contains(err.Error(), "Invalid YAML rule") {
		t.Fatalf("missing scanner configuration diagnostic: %v", err)
	}
	args, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	for _, flag := range []string{"--disable-version-check", "--jobs=1", "--metrics=off", "--strict"} {
		if !strings.Contains(string(args), flag) {
			t.Fatalf("missing offline scanner flag %s: %s", flag, args)
		}
	}
}

func TestCIModeIgnoresCandidateScannerSuppressions(t *testing.T) {
	argsFile := filepath.Join(t.TempDir(), "args")
	t.Setenv("KURO_TEST_ARGS", argsFile)
	runtime := filepath.Join(t.TempDir(), "runtime")
	os.WriteFile(runtime, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$KURO_TEST_ARGS\"\nprintf '%s' \"$KURO_TEST_REPORT\"\n"), 0755)
	adapter := &LocalAdapter{runtime: runtime}
	adapter.SetCIMode(true)
	t.Setenv("KURO_TEST_REPORT", "[]")
	if _, err := adapter.runGitleaks(context.Background(), t.TempDir()); err != nil {
		t.Fatal(err)
	}
	args, _ := os.ReadFile(argsFile)
	for _, expected := range []string{"GITLEAKS_CONFIG_TOML=[extend]\nuseDefault = true", "--ignore-gitleaks-allow", "--gitleaks-ignore-path=/dev/null"} {
		if !strings.Contains(string(args), expected) {
			t.Fatalf("missing trusted Gitleaks setting %s", expected)
		}
	}
	t.Setenv("KURO_TEST_REPORT", `{"results":[],"errors":[]}`)
	if _, err := adapter.runSemgrep(context.Background(), t.TempDir()); err != nil {
		t.Fatal(err)
	}
	args, _ = os.ReadFile(argsFile)
	for _, expected := range []string{"--disable-nosem", "--no-git-ignore", "--x-ignore-semgrepignore-files"} {
		if !strings.Contains(string(args), expected) {
			t.Fatalf("missing trusted Semgrep setting %s", expected)
		}
	}
}
