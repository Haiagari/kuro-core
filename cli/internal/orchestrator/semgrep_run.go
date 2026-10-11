package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func (a *LocalAdapter) runSemgrep(ctx context.Context, path string) ([]Finding, error) {
	// Bundle rules locally so Semgrep works under --network=none (no --config=auto).
	rulesDir, err := os.MkdirTemp("", "kuro-semgrep-rules-*")
	if err != nil {
		return nil, fmt.Errorf("create semgrep rules dir: %w", err)
	}
	defer os.RemoveAll(rulesDir)

	rulesFile := filepath.Join(rulesDir, "semgrep-core.yml")
	if err := os.WriteFile(rulesFile, semgrepCoreRules, 0o444); err != nil {
		return nil, fmt.Errorf("write semgrep rules: %w", err)
	}

	args := []string{
		"semgrep", "scan",
		"--config=/kuro-rules/semgrep-core.yml",
		"--metrics=off",
		"--disable-version-check",
		"--jobs=1",
		"--strict",
		"--max-memory", "350",
		"--json",
		"/src",
	}
	if a.ciMode {
		args = append(args, "--disable-nosem", "--no-git-ignore", "--x-ignore-semgrepignore-files")
	}
	output, err := a.runContainer(ctx, semgrepImage, args,
		map[string]string{
			path:     "/src:ro,Z",
			rulesDir: "/kuro-rules:ro,Z",
		},
	)
	if err != nil {
		// Semgrep's JSON mode reports configuration errors on stdout, not stderr.
		var report struct {
			Errors []struct {
				Message string `json:"message"`
			} `json:"errors"`
		}
		if json.Unmarshal(output, &report) == nil && len(report.Errors) > 0 {
			return nil, fmt.Errorf("%w: %s", err, report.Errors[0].Message)
		}
		return nil, err
	}

	return parseSemgrepOutput(output)
}
