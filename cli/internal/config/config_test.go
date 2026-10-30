package config

import (
	"path/filepath"
	"testing"
)

func TestConfig_DefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.APIURL != "http://localhost:8080" {
		t.Fatalf("unexpected default API URL: %s", cfg.APIURL)
	}
	if cfg.DefaultBranch != "main" {
		t.Fatalf("unexpected default branch: %s", cfg.DefaultBranch)
	}
	if cfg.APIKey != "" {
		t.Fatalf("expected empty default API key, got: %s", cfg.APIKey)
	}
}

func TestConfig_SaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	customPath := filepath.Join(tmpDir, "kuro-test", "config.json")
	t.Setenv("KURO_CONFIG_PATH", customPath)

	loaded, err := Load()
	if err != nil {
		t.Fatalf("load non-existent config failed: %v", err)
	}
	if loaded.APIKey != "" {
		t.Fatalf("expected empty APIKey from non-existent config, got %s", loaded.APIKey)
	}

	loaded.APIKey = "kuro-test-key-123"
	loaded.APIURL = "https://kuro.internal:8443"
	if err := Save(loaded); err != nil {
		t.Fatalf("save config failed: %v", err)
	}

	reloaded, err := Load()
	if err != nil {
		t.Fatalf("re-load config failed: %v", err)
	}
	if reloaded.APIKey != "kuro-test-key-123" || reloaded.APIURL != "https://kuro.internal:8443" {
		t.Fatalf("unexpected reloaded config: %+v", reloaded)
	}

	resolved, err := ResolvePath()
	if err != nil {
		t.Fatalf("resolve path failed: %v", err)
	}
	if resolved != customPath {
		t.Fatalf("expected %s, got %s", customPath, resolved)
	}
}

func TestConfig_XDGPathResolution(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("KURO_CONFIG_PATH", "")
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	path, err := configPath()
	if err != nil {
		t.Fatalf("configPath failed: %v", err)
	}

	expected := filepath.Join(tmpDir, "kuro", "config.json")
	if path != expected {
		t.Fatalf("expected %s, got %s", expected, path)
	}
}
