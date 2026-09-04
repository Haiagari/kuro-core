package cmd

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateCanaryToken_AWS(t *testing.T) {
	token, secret, meta := GenerateCanaryToken("aws", "CI test AWS key")

	if !strings.HasPrefix(token, "AKIA") {
		t.Fatalf("expected AWS key to start with 'AKIA', got %q", token)
	}
	if len(token) != 20 {
		t.Fatalf("expected AWS key length to be 20, got %d (%q)", len(token), token)
	}
	if !strings.HasPrefix(secret, "wJalrXUtnFEMI/K7MDENG/bPxRfiCY") {
		t.Fatalf("expected AWS secret to have standard mock prefix, got %q", secret)
	}
	if meta.Type != "aws" {
		t.Fatalf("expected meta.Type = 'aws', got %q", meta.Type)
	}
	if meta.Memo != "CI test AWS key" {
		t.Fatalf("expected memo to match, got %q", meta.Memo)
	}
	if len(meta.Signature) != 16 {
		t.Fatalf("expected 16-char HMAC signature, got %d", len(meta.Signature))
	}
}

func TestGenerateCanaryToken_GitHub(t *testing.T) {
	token, _, meta := GenerateCanaryToken("github", "GitHub canary")

	if !strings.HasPrefix(token, "ghp_kuro") {
		t.Fatalf("expected GitHub token to start with 'ghp_kuro', got %q", token)
	}
	if meta.Type != "github" {
		t.Fatalf("expected meta.Type = 'github', got %q", meta.Type)
	}
}

func TestGenerateCanaryToken_Slack(t *testing.T) {
	token, _, meta := GenerateCanaryToken("slack", "Slack canary")

	if !strings.HasPrefix(token, "https://hooks.slack.com/services/") {
		t.Fatalf("expected Slack webhook prefix, got %q", token)
	}
	if meta.Type != "slack" {
		t.Fatalf("expected meta.Type = 'slack', got %q", meta.Type)
	}
}

func TestGenerateCanaryToken_JWT(t *testing.T) {
	token, _, meta := GenerateCanaryToken("jwt", "JWT canary")

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("expected 3-part JWT, got %d parts (%q)", len(parts), token)
	}
	if meta.Type != "jwt" {
		t.Fatalf("expected meta.Type = 'jwt', got %q", meta.Type)
	}
}

func TestFormatCanaryOutput(t *testing.T) {
	token, secret, meta := GenerateCanaryToken("aws", "Test memo")

	// 1. JSON
	jsonOut := formatCanaryOutput("aws", token, secret, meta, "json")
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(jsonOut), &parsed); err != nil {
		t.Fatalf("failed to unmarshal JSON output: %v", err)
	}

	// 2. YAML
	yamlOut := formatCanaryOutput("aws", token, secret, meta, "yaml")
	if !strings.Contains(yamlOut, "canary:") || !strings.Contains(yamlOut, token) {
		t.Fatalf("YAML output missing expected keys: %s", yamlOut)
	}

	// 3. Terraform
	tfOut := formatCanaryOutput("aws", token, secret, meta, "tf")
	if !strings.Contains(tfOut, "variable \"canary_api_key\"") {
		t.Fatalf("Terraform output missing variable block: %s", tfOut)
	}

	// 4. Env
	envOut := formatCanaryOutput("aws", token, secret, meta, "env")
	if !strings.Contains(envOut, "AWS_ACCESS_KEY_ID="+token) {
		t.Fatalf("ENV output missing AWS_ACCESS_KEY_ID: %s", envOut)
	}
}

func TestCanaryManifest_SaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	manifestFile := filepath.Join(tmpDir, ".canary-manifest.json")

	_, _, meta1 := GenerateCanaryToken("aws", "Key 1")
	meta1.TargetFile = filepath.Join(tmpDir, "canary_secrets.env")

	_, _, meta2 := GenerateCanaryToken("github", "Key 2")
	meta2.TargetFile = filepath.Join(tmpDir, "canary_github.env")

	var initialManifest CanaryManifest
	initialManifest.Version = "v1.0"
	initialManifest.Tokens = []CanaryTokenMetadata{meta1, meta2}

	saveCanaryManifest(manifestFile, initialManifest)

	loaded := loadCanaryManifest(manifestFile)
	if len(loaded.Tokens) != 2 {
		t.Fatalf("expected 2 loaded tokens, got %d", len(loaded.Tokens))
	}
	if loaded.Tokens[0].ID != meta1.ID || loaded.Tokens[1].ID != meta2.ID {
		t.Fatalf("loaded token IDs mismatch: %+v vs %+v", loaded.Tokens, initialManifest.Tokens)
	}
}

func TestIsCanarySignature(t *testing.T) {
	tokenAWS, _, _ := GenerateCanaryToken("aws", "aws")
	tokenGH, _, _ := GenerateCanaryToken("github", "gh")

	if !isCanarySignature(tokenAWS) {
		t.Errorf("expected isCanarySignature(%q) = true", tokenAWS)
	}
	if !isCanarySignature(tokenGH) {
		t.Errorf("expected isCanarySignature(%q) = true", tokenGH)
	}
	if isCanarySignature("random_unrelated_string_123") {
		t.Errorf("expected isCanarySignature for random string to be false")
	}
}
