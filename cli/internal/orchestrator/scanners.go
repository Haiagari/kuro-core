package orchestrator

import (
	"context"
	"errors"
	"os/exec"
	"time"
)

// ── Scanners ───────────────────────────────────────────────

func (a *LocalAdapter) runGitleaks(ctx context.Context, path string) ([]Finding, error) {
	args := []string{"dir", "--report-format=json", "--report-path=/proc/self/fd/1", "/src"}
	if a.ciMode {
		args = append(args, "--ignore-gitleaks-allow", "--gitleaks-ignore-path=/dev/null")
	}
	output, err := a.runContainer(ctx,
		gitleaksImage,
		args,
		map[string]string{path: "/src:ro,Z"},
	)
	findings, parseErr := parseGitleaksOutput(output)
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 || ctx.Err() != nil || parseErr != nil || len(findings) == 0 {
			return nil, err
		}
	}
	return findings, parseErr
}

// runGitleaksHistory scans the full git history using gitleaks detect (no --no-git).
// Unlike runGitleaks (which scans the working tree with `dir`), this walks every commit.
func (a *LocalAdapter) runGitleaksHistory(ctx context.Context, path string) ([]Finding, error) {
	output, err := a.runContainer(ctx,
		gitleaksImage,
		[]string{"detect", "--source=/src", "--report-format=json", "--report-path=/proc/self/fd/1"},
		map[string]string{path: "/src:ro,Z"},
	)
	findings, parseErr := parseGitleaksOutput(output)
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 || ctx.Err() != nil || parseErr != nil || len(findings) == 0 {
			return nil, err
		}
	}
	return findings, parseErr
}

// runTrufflehogHistory uses TruffleHog's git source, with network verification disabled.
func (a *LocalAdapter) runTrufflehogHistory(ctx context.Context, path string) ([]Finding, error) {
	output, err := a.runContainer(ctx, trufflehogImage,
		[]string{"git", "file:///src", "--json", "--no-update", "--no-verification", "--concurrency", "1"},
		map[string]string{path: "/src:ro,Z"})
	if err != nil {
		return nil, err
	}
	return parseTrufflehogOutput(output)
}

func (a *LocalAdapter) runTrivy(ctx context.Context, path string) ([]Finding, error) {
	cacheDir, err := trivyCacheDir()
	if err != nil {
		return nil, err
	}
	if err := validateTrivyDB(cacheDir, time.Now()); err != nil {
		return nil, err
	}
	args := []string{"fs", "--scanners=vuln", "--format=json", "--exit-code=0", "--cache-dir=/trivy-cache",
		"--skip-db-update", "--skip-java-db-update", "--offline-scan", "/src"}
	if a.ciMode {
		args = append(args, "--ignorefile=/dev/null")
	}
	output, err := a.runContainer(ctx, trivyImage, args,
		map[string]string{path: "/src:ro,Z", cacheDir: "/trivy-cache:Z"})
	if err != nil {
		return nil, err
	}
	return parseTrivyOutput(output)
}

func (a *LocalAdapter) runCheckov(ctx context.Context, path string) ([]Finding, error) {
	output, err := a.runContainer(ctx,
		checkovImage,
		[]string{"--directory=/src", "--output=json", "--soft-fail", "--skip-download", "--framework", "terraform", "dockerfile"},
		map[string]string{path: "/src:ro,Z"},
	)
	if err != nil {
		return nil, err
	}

	return parseCheckovOutput(output)
}
