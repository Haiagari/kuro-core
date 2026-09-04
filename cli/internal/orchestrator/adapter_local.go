package orchestrator


import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ── Pinned scanner image versions (mirrors worker/scanners/) ──

const (
	// docker.io/ prefix ensures Podman resolves from Docker Hub, not localhost
	gitleaksImage        = "docker.io/zricethezav/gitleaks:v8.30.1"
	semgrepImage         = "docker.io/semgrep/semgrep:1.165.0"
	trivyImage           = "docker.io/aquasec/trivy:0.57.0"
	checkovImage         = "docker.io/bridgecrew/checkov:3.2.400"
	trufflehogImage      = "docker.io/trufflesecurity/trufflehog:3.81.0"
)

// ── LocalAdapter ───────────────────────────────────────────

// LocalAdapter runs scanners directly via Docker/Podman.
type LocalAdapter struct {
	runtime     string // "docker" or "podman"
	historyScan bool   // scan full git history instead of working tree
}

// NewLocalAdapter creates a local adapter by detecting the available runtime.
// Pass historyScan=true for full git history scan mode.
func NewLocalAdapter(historyScan bool) *LocalAdapter {
	r := detectRuntime()
	return &LocalAdapter{runtime: r, historyScan: historyScan}
}

func (a *LocalAdapter) Name() string { return "local" }

// Fetch verifies the path exists and is a directory.
// Then launches an async pull of scanner images so they
// are cached when executed.
func (a *LocalAdapter) Fetch(ctx context.Context, target string) (string, error) {
	abs, err := filepath.Abs(target)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("access %s: %w", abs, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory", abs)
	}

	// Lanzar pull asíncrono de imágenes — no bloquea el scan.
	// Usamos context.Background() para que el pull continúe incluso si
	// cancelan el scan. stdout/stderr van a io.Discard para no ensuciar.
	go func() {
		images := []string{gitleaksImage, semgrepImage, trivyImage, checkovImage}
		if a.historyScan {
			images = append(images, trufflehogImage)
		}
		for _, img := range images {
			cmd := exec.CommandContext(context.Background(), a.runtime, "pull", img)
			cmd.Stdout = io.Discard
			cmd.Stderr = io.Discard
			cmd.Run() //nolint:errcheck
		}
	}()

	return abs, nil
}

// Scope determines which scanners to apply based on the repo files.
func (a *LocalAdapter) Scope(ctx context.Context, target string) ([]string, error) {
	// History mode: only run git history scanners, skip file-walking scope
	if a.historyScan {
		return []string{"gitleaks-history", "trufflehog-history"}, nil
	}

	scanners := []string{"gitleaks", "semgrep"}

	// ── File change cache ──────────────────────────────────
	var totalFiles, cachedFiles, newFiles int
	cache := openFileCache()
	if cache != nil {
		filepath.Walk(target, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			totalFiles++
			changed, _, err := cache.IsChanged(path)
			if err != nil {
				// Can't stat — count as new to be safe
				newFiles++
				return nil
			}
			if changed {
				newFiles++
			} else {
				cachedFiles++
			}
			return nil
		})
	}

	// ── Scanner detection ──────────────────────────────────
	// Detect if there are dependencies for Trivy/Syft/Grype
	hasLockFile := false
	filepath.Walk(target, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		name := info.Name()
		if name == "go.mod" || name == "package-lock.json" || name == "yarn.lock" ||
			name == "requirements.txt" || name == "Gemfile.lock" || name == "Cargo.lock" {
			hasLockFile = true
		}
		return nil
	})

	if hasLockFile {
		scanners = append(scanners, "trivy")
	}

	// Detect if there is a Dockerfile for Checkov
	hasDockerfile := false
	filepath.Walk(target, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if info.Name() == "Dockerfile" || strings.HasSuffix(info.Name(), ".tf") {
			hasDockerfile = true
		}
		return nil
	})

	if hasDockerfile {
		scanners = append(scanners, "checkov")
	}

	// ── Print file stats ───────────────────────────────────
	if cache != nil && totalFiles > 0 {
		fmt.Fprintf(os.Stderr, "  %d files · %d cached · %d new\n", totalFiles, cachedFiles, newFiles)

		// Mark all files as scanned and persist
		filepath.Walk(target, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			cache.MarkScanned(path) // best-effort
			return nil
		})
		cache.Save() // best-effort
	}

	return scanners, nil
}

// openFileCache initializes a file cache at $HOME/.kuro/cache.
// Returns nil if the cache cannot be set up (e.g. no home dir).
func openFileCache() *FileCache {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	cache, err := NewFileCache(filepath.Join(home, ".kuro", "cache"))
	if err != nil {
		return nil
	}
	return cache
}

// scanResult carries a single scanner's findings, error, and elapsed time back to the
// collector goroutine so progress can be printed as each scanner finishes.
type scanResult struct {
	name     string
	findings []Finding
	err      error
	elapsed  time.Duration
}

// Run executes the scanners in Docker/Podman containers in parallel.
func (a *LocalAdapter) Run(ctx context.Context, target string, scanners []string) (RunResult, error) {
	results := make(chan scanResult, len(scanners))

	for _, sc := range scanners {
		go func(scanner string) {
			// Create per-scanner timeout context.
			// History scans need more time (full git log vs working tree).
			timeout := 5 * time.Minute
			if a.historyScan {
				timeout = 15 * time.Minute
			}
			scanCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			start := time.Now()

			var findings []Finding
			var err error

			switch scanner {
			case "gitleaks":
				findings, err = a.runGitleaks(scanCtx, target)
			case "gitleaks-history":
				findings, err = a.runGitleaksHistory(scanCtx, target)
			case "trufflehog-history":
				findings, err = a.runTrufflehogHistory(scanCtx, target)
			case "semgrep":
				findings, err = a.runSemgrep(scanCtx, target)
			case "trivy":
				findings, err = a.runTrivy(scanCtx, target)
			case "checkov":
				findings, err = a.runCheckov(scanCtx, target)
			}

			results <- scanResult{
				name:     scanner,
				findings: findings,
				err:      err,
				elapsed:  time.Since(start),
			}
		}(sc)
	}

	var allFindings []Finding
	var timings []ScannerTiming
	for i := 0; i < len(scanners); i++ {
		r := <-results

		prefix := "  ├─"
		if i == len(scanners)-1 {
			prefix = "  └─"
		}

		if r.err != nil {
			if !isSkipError(r.err) {
				fmt.Fprintf(os.Stderr, "%s %s... %s (%.1fs)\n", prefix, r.name, r.err, r.elapsed.Seconds())
			}
			timings = append(timings, ScannerTiming{Name: r.name, Findings: 0, Duration: r.elapsed})
			continue
		}

		fmt.Fprintf(os.Stderr, "%s %s... %d findings (%.1fs)\n", prefix, r.name, len(r.findings), r.elapsed.Seconds())
		timings = append(timings, ScannerTiming{Name: r.name, Findings: len(r.findings), Duration: r.elapsed})
		allFindings = append(allFindings, r.findings...)
	}

	return RunResult{Findings: allFindings, Timings: timings}, nil
}

// isSkipError returns true for expected scanner errors that should not be
// printed as warnings (e.g. "no output", "no applicable files").
func isSkipError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "no output") || strings.Contains(msg, "produced no output")
}
