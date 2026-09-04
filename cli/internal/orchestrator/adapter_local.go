package orchestrator


import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

func (a *LocalAdapter) runSemgrep(ctx context.Context, path string) ([]Finding, error) {
	output, err := a.runContainer(ctx,
		semgrepImage,
		[]string{"semgrep", "scan", "--config=auto", "--max-memory", "350", "--json", "/src"},
		map[string]string{path: "/src:ro,Z"},
	)
	if err != nil {
		if len(output) == 0 {
			return nil, fmt.Errorf("semgrep produced no output: %w", err)
		}
	}

	return parseSemgrepOutput(output)
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

// ── Container runtime ──────────────────────────────────────

func (a *LocalAdapter) runContainer(ctx context.Context, image string, args []string, volumes map[string]string) ([]byte, error) {
	cmdArgs := []string{"run", "--rm", "--memory=512m", "--cpus=1.0"}
	for host, container := range volumes {
		cmdArgs = append(cmdArgs, "-v", host+":"+container)
	}
	cmdArgs = append(cmdArgs, image)
	cmdArgs = append(cmdArgs, args...)

	cmd := exec.CommandContext(ctx, a.runtime, cmdArgs...)
	stdout, err := cmd.Output()
	if err != nil {
		// Most scanners exit 1 when findings exist. Return stdout anyway.
		if stdout != nil && len(stdout) > 0 {
			return stdout, nil
		}
		return nil, fmt.Errorf("container exited: %w", err)
	}
	return stdout, nil
}

// ── Parsers ─────────────────────────────────────────────────

func parseGitleaksOutput(data []byte) ([]Finding, error) {
	// Extract JSON array from mixed output (logo, logs, JSON)
	start := bytes.Index(data, []byte("["))
	end := bytes.LastIndex(data, []byte("]"))
	if start >= 0 && end > start {
		data = data[start : end+1]
	}

	var results []struct {
		RuleID      string `json:"RuleID"`
		Description string `json:"Description"`
		File        string `json:"File"`
		StartLine   int    `json:"StartLine"`
		Secret      string `json:"Secret"`
	}

	if err := json.Unmarshal(data, &results); err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}

	var findings []Finding
	for _, r := range results {
		findings = append(findings, Finding{
			Scanner:    "gitleaks",
			Severity:   "CRITICAL",
			Title:      r.RuleID,
			FilePath:   r.File,
			LineNumber: r.StartLine,
			Description: r.Description,
		})
	}
	return findings, nil
}

func parseSemgrepOutput(data []byte) ([]Finding, error) {
	var result struct {
		Results []struct {
			CheckID string `json:"check_id"`
			Path    string `json:"path"`
			Start   struct {
				Line int `json:"line"`
			} `json:"start"`
			Extra struct {
				Severity string `json:"severity"`
				Message  string `json:"message"`
				Lines    string `json:"lines"`
			} `json:"extra"`
		} `json:"results"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse semgrep: %w", err)
	}

	var findings []Finding
	for _, r := range result.Results {
		sev := "MEDIUM"
		switch r.Extra.Severity {
		case "ERROR":
			sev = "CRITICAL"
		case "WARNING":
			sev = "HIGH"
		}
		findings = append(findings, Finding{
			Scanner:    "semgrep",
			Severity:   sev,
			Title:      r.CheckID,
			FilePath:   r.Path,
			LineNumber: r.Start.Line,
			Description: r.Extra.Message,
		})
	}
	return findings, nil
}

func parseTrivyOutput(data []byte) ([]Finding, error) {
	var result struct {
		Results []struct {
			Target       string `json:"Target"`
			Vulnerabilities []struct {
				VulnerabilityID string `json:"VulnerabilityID"`
				PkgName    string `json:"PkgName"`
				Severity   string `json:"Severity"`
				Title      string `json:"Title"`
				InstalledVersion string `json:"InstalledVersion"`
			} `json:"Vulnerabilities"`
		} `json:"Results"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse trivy: %w", err)
	}

	var findings []Finding
	for _, r := range result.Results {
		for _, v := range r.Vulnerabilities {
			findings = append(findings, Finding{
				Scanner:    "trivy",
				Severity:   v.Severity,
				Title:      v.VulnerabilityID,
				FilePath:   r.Target,
				Description: fmt.Sprintf("%s (%s)", v.Title, v.InstalledVersion),
			})
		}
	}
	return findings, nil
}

func parseTrufflehogOutput(data []byte) ([]Finding, error) {
	// TruffleHog outputs NDJSON (one JSON object per line)
	var findings []Finding
	for _, line := range bytes.Split(data, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		var result struct {
			DetectorName string `json:"DetectorName"`
			SourceMetadata struct {
				Data struct {
					File   string `json:"file"`
					Line   int    `json:"line"`
					Commit string `json:"commit"`
					Email  string `json:"email"`
				} `json:"Data"`
			} `json:"SourceMetadata"`
			Raw      string `json:"Raw"`
			Verified bool   `json:"Verified"`
		}

		if err := json.Unmarshal(line, &result); err != nil {
			continue // skip malformed lines
		}

		snippet := result.Raw
		if len(snippet) > 120 {
			snippet = snippet[:120]
		}

		desc := "Secret detected"
		if result.SourceMetadata.Data.Commit != "" {
			desc = fmt.Sprintf("Secret detected in commit %s", result.SourceMetadata.Data.Commit)
		}

		findings = append(findings, Finding{
			Scanner:    "trufflehog",
			Severity:   "CRITICAL",
			Title:      result.DetectorName,
			FilePath:   result.SourceMetadata.Data.File,
			LineNumber: result.SourceMetadata.Data.Line,
			Description: desc,
			Verified:   result.Verified,
		})
	}

	return findings, nil
}

func parseCheckovOutput(data []byte) ([]Finding, error) {
	var results []struct {
		CheckID    string `json:"check_id"`
		CheckName  string `json:"check_name"`
		FilePath   string `json:"file_path"`
		FileLineRange []int `json:"file_line_range"`
		Severity   string `json:"severity"`
		Description string `json:"description"`
	}

	if err := json.Unmarshal(data, &results); err != nil {
		return nil, fmt.Errorf("parse checkov: %w", err)
	}

	var findings []Finding
	for _, r := range results {
		line := 0
		if len(r.FileLineRange) > 0 {
			line = r.FileLineRange[0]
		}
		sev := "MEDIUM"
		switch r.Severity {
		case "CRITICAL", "HIGH":
			sev = r.Severity
		}
		findings = append(findings, Finding{
			Scanner:    "checkov",
			Severity:   sev,
			Title:      r.CheckID,
			FilePath:   r.FilePath,
			LineNumber: line,
			Description: r.CheckName,
		})
	}
	return findings, nil
}

// ── Runtime detection ──────────────────────────────────────

func detectRuntime() string {
	// Prioritize docker over podman — podman mixes stdout/stderr
	if _, err := exec.LookPath("docker"); err == nil {
		return "docker"
	}
	if _, err := exec.LookPath("podman"); err == nil {
		return "podman"
	}
	return "docker" // fallback, will give a clearer error later
}
