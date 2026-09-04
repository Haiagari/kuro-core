package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
)

// ── Scanners ───────────────────────────────────────────────

func (a *LocalAdapter) runGitleaks(ctx context.Context, path string) ([]Finding, error) {
	output, err := a.runContainer(ctx,
		gitleaksImage,
		[]string{"dir", "--report-format=json", "--report-path=/proc/self/fd/1", "/src"},
		map[string]string{path: "/src:ro,Z"},
	)
	if err != nil {
		// Gitleaks exits 1 when findings exist — that's normal
		if len(output) == 0 {
			return nil, fmt.Errorf("no output: %w", err)
		}
	}

	return parseGitleaksOutput(output)
}

// runGitleaksHistory scans the full git history using gitleaks detect (no --no-git).
// Unlike runGitleaks (which scans the working tree with `dir`), this walks every commit.
func (a *LocalAdapter) runGitleaksHistory(ctx context.Context, path string) ([]Finding, error) {
	output, err := a.runContainer(ctx,
		gitleaksImage,
		[]string{"detect", "--source=/src", "--report-format=json", "--report-path=/proc/self/fd/1"},
		map[string]string{path: "/src:ro,Z"},
	)
	if err != nil {
		if len(output) == 0 {
			return nil, fmt.Errorf("no output: %w", err)
		}
	}

	return parseGitleaksOutput(output)
}

// runTrufflehogHistory scans the full git history using trufflehog filesystem scanner.
// TruffleHog walks files recursively, detecting secrets in all files including .git objects.
func (a *LocalAdapter) runTrufflehogHistory(ctx context.Context, path string) ([]Finding, error) {
	// TruffleHog needs write access for its temp cache, use :Z (not :ro,Z)
	output, err := a.runContainer(ctx,
		trufflehogImage,
		[]string{"filesystem", "/src", "--json", "--no-update", "--concurrency", "1"},
		map[string]string{path: "/src:Z"},
	)
	if err != nil {
		if len(output) == 0 {
			return nil, fmt.Errorf("no output: %w", err)
		}
	}

	return parseTrufflehogOutput(output)
}

func (a *LocalAdapter) runTrivy(ctx context.Context, path string) ([]Finding, error) {
	output, err := a.runContainer(ctx,
		trivyImage,
		[]string{"trivy", "fs", "--format=json", "--skip-db-update", "--offline-scan", "/src"},
		map[string]string{path: "/src:ro,Z"},
	)
	if err != nil && len(output) == 0 {
		// DB not cached locally — retry without --offline-scan to allow download
		output, err = a.runContainer(ctx,
			trivyImage,
			[]string{"trivy", "fs", "--format=json", "--skip-db-update", "/src"},
			map[string]string{path: "/src:ro,Z"},
		)
		if err != nil && len(output) == 0 {
			// Trivy is non-critical; return empty findings
			return nil, nil
		}
	}

	return parseTrivyOutput(output)
}

func (a *LocalAdapter) runCheckov(ctx context.Context, path string) ([]Finding, error) {
	output, err := a.runContainer(ctx,
		checkovImage,
		[]string{"checkov", "--directory=/src", "--output=json", "--soft-fail"},
		map[string]string{path: "/src:ro,Z"},
	)
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 2 {
			// Exit code 2 = no IaC files found; not an error
			return nil, nil
		}
		if len(output) == 0 {
			return nil, fmt.Errorf("checkov produced no output: %w", err)
		}
	}

	return parseCheckovOutput(output)
}
