package orchestrator

import (
	"context"
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

	output, err := a.runContainer(ctx,
		semgrepImage,
		[]string{
			"semgrep", "scan",
			"--config=/kuro-rules/semgrep-core.yml",
			"--metrics=off",
			"--max-memory", "350",
			"--json",
			"/src",
		},
		map[string]string{
			path:     "/src:ro,Z",
			rulesDir: "/kuro-rules:ro,Z",
		},
	)
	if err != nil {
		if len(output) == 0 {
			return nil, fmt.Errorf("semgrep produced no output: %w", err)
		}
	}

	return parseSemgrepOutput(output)
}
