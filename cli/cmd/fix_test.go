package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAutoFix_GoSecretReplacement(t *testing.T) {
	initialGoCode := `package main

func getClient() {
	awsKey := "AKIAIOSFODNN7EXAMPLE"
	println(awsKey)
}
`

	updated, changed := ApplyFixToContent("main.go", initialGoCode, "AKIAIOSFODNN7EXAMPLE")
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
	initialPyCode := `def connect():
    api_key = "AKIAIOSFODNN7EXAMPLE"
    return api_key
`

	updated, changed := ApplyFixToContent("client.py", initialPyCode, "AKIAIOSFODNN7EXAMPLE")
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
	initialJSCode := `const config = {
  githubToken: "ghp_1234567890abcdef1234567890abcdef"
};
`

	updated, changed := ApplyFixToContent("config.ts", initialJSCode, "ghp_1234567890abcdef1234567890abcdef")
	if !changed {
		t.Fatalf("expected JS content to be modified")
	}

	if !strings.Contains(updated, "process.env.GITHUB_TOKEN") {
		t.Fatalf("expected process.env.GITHUB_TOKEN replacement, got:\n%s", updated)
	}
}

func TestAutoFix_SuppressionManifest(t *testing.T) {
	tmpDir := t.TempDir()

	record := SuppressionRecord{
		FindingID: "f-123",
		RuleID:    "HARDCODED_KEY",
		FilePath:  "services/api/main.go",
		Reason:    "Approved mock key in test suite",
		CreatedAt: time.Now().UTC(),
		Author:    "lead-architect",
	}

	saveSuppression(tmpDir, record)

	manifestPath := filepath.Join(tmpDir, ".kuro-suppressions.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("failed to read created suppression manifest: %v", err)
	}

	if !strings.Contains(string(data), "Approved mock key in test suite") {
		t.Fatalf("manifest missing reason text: %s", string(data))
	}
}
