package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Config represents the CLI configuration stored in ~/.kuro/config.json.
type Config struct {
	APIURL        string `json:"api_url"`
	APIKey        string `json:"api_key"`
	DefaultBranch string `json:"default_branch"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		APIURL:        "http://localhost:8080",
		APIKey:        "",
		DefaultBranch: "main",
	}
}

// configPath resolves the path to the config file using XDG conventions.
// Priority:
// 1. $KURO_CONFIG_PATH env var
// 2. $XDG_CONFIG_HOME/kuro/config.json
// 3. $HOME/.kuro/config.json
func configPath() (string, error) {
	if p := os.Getenv("KURO_CONFIG_PATH"); p != "" {
		return p, nil
	}

	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "kuro", "config.json"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".kuro", "config.json"), nil
}

// configDir returns the parent directory of the config file.
func configDir() (string, error) {
	path, err := configPath()
	if err != nil {
		return "", err
	}
	return filepath.Dir(path), nil
}

// Load reads the config file from disk. If the file does not exist, it returns
// a Config with defaults (api_key="").
func Load() (Config, error) {
	cfg := DefaultConfig()

	path, err := configPath()
	if err != nil {
		return cfg, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("cannot read config file %s: %w", path, err)
	}

	if len(data) == 0 {
		return cfg, nil
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("invalid config file at %s: %w", path, err)
	}

	return cfg, nil
}

// Save writes the config to disk with 0600 permissions.
// The config directory is created with 0700 if it does not exist.
func Save(cfg Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("cannot create config directory %s: %w", dir, err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("cannot write config file %s: %w", path, err)
	}

	return nil
}

// ResolvePath returns the resolved config file path (useful for display).
func ResolvePath() (string, error) {
	return configPath()
}
