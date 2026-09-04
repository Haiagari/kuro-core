package server

import (
	"encoding/json"
	"net/http"
	"os"
	"testing"
)

func TestGetEnv_UsesEnvVar(t *testing.T) {
	os.Setenv("TEST_GETENV_KEY", "from-env")
	defer os.Unsetenv("TEST_GETENV_KEY")

	got := getEnv("TEST_GETENV_KEY", "fallback")
	if got != "from-env" {
		t.Fatalf("expected %q, got %q", "from-env", got)
	}
}

func TestGetEnv_UsesFallback(t *testing.T) {
	os.Unsetenv("TEST_GETENV_MISSING")

	got := getEnv("TEST_GETENV_MISSING", "fallback")
	if got != "fallback" {
		t.Fatalf("expected %q, got %q", "fallback", got)
	}
}

func TestGetEnv_EmptyEnvUsesFallback(t *testing.T) {
	os.Setenv("TEST_GETENV_EMPTY", "")
	defer os.Unsetenv("TEST_GETENV_EMPTY")

	got := getEnv("TEST_GETENV_EMPTY", "fallback")
	if got != "fallback" {
		t.Fatalf("expected %q, got %q", "fallback", got)
	}
}

func TestExtractRepoPath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/owner/repo.git/git-receive-pack", "owner/repo"},
		{"/owner/repo/git-receive-pack", "owner/repo"},
		{"/owner/repo.git", "owner/repo"},
		{"/owner/repo", "owner/repo"},
		{"/a/b/c.git/git-receive-pack", "a/b/c"},
		{"/git-receive-pack", ""},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := extractRepoPath(tc.input)
			if got != tc.expected {
				t.Fatalf("extractRepoPath(%q) = %q, want %q", tc.input, got, tc.expected)
			}
		})
	}
}

func TestAddAuth_NoToken(t *testing.T) {
	oldToken := gitHubToken
	oldUser := gitHubUser
	gitHubToken = ""
	gitHubUser = ""
	defer func() {
		gitHubToken = oldToken
		gitHubUser = oldUser
	}()

	req, _ := http.NewRequest("GET", "https://example.com/test", nil)
	addAuth(req)

	if auth := req.Header.Get("Authorization"); auth != "" {
		t.Fatalf("expected no Authorization header, got %q", auth)
	}
}

func TestAddAuth_BearerOnly(t *testing.T) {
	oldToken := gitHubToken
	oldUser := gitHubUser
	gitHubToken = "ghp_testtoken"
	gitHubUser = ""
	defer func() {
		gitHubToken = oldToken
		gitHubUser = oldUser
	}()

	req, _ := http.NewRequest("GET", "https://example.com/test", nil)
	addAuth(req)

	if got := req.Header.Get("Authorization"); got != "Bearer ghp_testtoken" {
		t.Fatalf("expected Bearer token, got %q", got)
	}
}

func TestAddAuth_BasicAuth(t *testing.T) {
	oldToken := gitHubToken
	oldUser := gitHubUser
	gitHubToken = "ghp_testtoken"
	gitHubUser = "myuser"
	defer func() {
		gitHubToken = oldToken
		gitHubUser = oldUser
	}()

	req, _ := http.NewRequest("GET", "https://example.com/test", nil)
	addAuth(req)

	user, pass, ok := req.BasicAuth()
	if !ok {
		t.Fatal("expected BasicAuth to be set")
	}
	if user != "myuser" {
		t.Fatalf("expected user %q, got %q", "myuser", user)
	}
	if pass != "ghp_testtoken" {
		t.Fatalf("expected pass %q, got %q", "ghp_testtoken", pass)
	}
}

func TestProxyScanFindingJSON(t *testing.T) {
	f := proxyScanFinding{
		File:        "src/main.go",
		Line:        42,
		Rule:        "G101",
		Scanner:     "gitleaks",
		Description: "Hardcoded credential",
		Severity:    "high",
	}

	got, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	expected := `{"file":"src/main.go","line":42,"rule":"G101","scanner":"gitleaks","description":"Hardcoded credential","severity":"high"}`
	if string(got) != expected {
		t.Fatalf("unexpected JSON:\n  got:  %s\n  want: %s", string(got), expected)
	}
}

func TestProxyScanResponseJSON(t *testing.T) {
	r := proxyScanResponse{
		ScanID:   "scan-123",
		Decision: "BLOCKED",
		Findings: []proxyScanFinding{
			{File: "test.txt", Line: 1, Rule: "G101", Scanner: "gitleaks", Description: "secret", Severity: "high"},
		},
	}

	got, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	expected := `{"scan_id":"scan-123","decision":"BLOCKED","findings":[{"file":"test.txt","line":1,"rule":"G101","scanner":"gitleaks","description":"secret","severity":"high"}]}`
	if string(got) != expected {
		t.Fatalf("unexpected JSON:\n  got:  %s\n  want: %s", string(got), expected)
	}
}
