package doctor


import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// Core scanner images (must match cli/internal/orchestrator/adapter_local.go pins).
var coreScannerImages = []struct {
	Name string
	Ref  string
}{
	{"Gitleaks", "docker.io/zricethezav/gitleaks:v8.30.1"},
	{"Semgrep", "docker.io/semgrep/semgrep:1.165.0"},
	{"Trivy", "docker.io/aquasec/trivy:0.57.0"},
	{"Checkov", "docker.io/bridgecrew/checkov:3.2.400"},
}

// defaultDiskPaths are the filesystem paths to check for free space.
var defaultDiskPaths = []string{
	os.TempDir(),
	"/tmp",
}

// detectedRuntime is set by CheckContainerRuntime for dependent checks.
var detectedRuntime string

// ── CheckContainerRuntime ─────────────────────────────────

// CheckContainerRuntime verifies Docker or Podman is usable for local scans.
func CheckContainerRuntime() CheckResult {
	if _, err := exec.LookPath("docker"); err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := exec.CommandContext(ctx, "docker", "info").Run(); err == nil {
			detectedRuntime = "docker"
			return CheckResult{Status: StatusPass, Detail: "docker daemon ready"}
		}
	}
	if _, err := exec.LookPath("podman"); err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := exec.CommandContext(ctx, "podman", "info").Run(); err == nil {
			detectedRuntime = "podman"
			return CheckResult{Status: StatusPass, Detail: "podman ready"}
		}
		detectedRuntime = "podman"
		return CheckResult{Status: StatusWarn, Detail: "podman binary found; daemon check timed out or failed"}
	}
	detectedRuntime = ""
	return CheckResult{
		Status: StatusFail,
		Detail: "neither docker nor podman is available — required for local scans",
	}
}

// ── CheckGit ───────────────────────────────────────────────

// CheckGit verifies git is on PATH (needed by history scans and git-proxy).
func CheckGit() CheckResult {
	path, err := exec.LookPath("git")
	if err != nil {
		return CheckResult{Status: StatusFail, Detail: "git not found on PATH"}
	}
	out, err := exec.Command("git", "--version").Output()
	if err != nil {
		return CheckResult{Status: StatusFail, Detail: fmt.Sprintf("git --version failed: %v", err)}
	}
	return CheckResult{
		Status: StatusPass,
		Detail: fmt.Sprintf("%s (%s)", strings.TrimSpace(string(out)), path),
	}
}

// ── CheckScannerImages ─────────────────────────────────────

// CheckScannerImages verifies Core scanner images exist locally.
func CheckScannerImages() CheckResult {
	runtime := detectedRuntime
	if runtime == "" {
		runtime = "docker"
	}
	var missing []string
	for _, img := range coreScannerImages {
		cmd := exec.Command(runtime, "image", "inspect", img.Ref)
		if err := cmd.Run(); err != nil {
			// Also try without docker.io/ prefix
			alt := strings.TrimPrefix(img.Ref, "docker.io/")
			if exec.Command(runtime, "image", "inspect", alt).Run() != nil {
				missing = append(missing, img.Name)
			}
		}
	}
	if len(missing) > 0 {
		return CheckResult{
			Status: StatusWarn,
			Detail: fmt.Sprintf("Missing (will pull on first scan): %s", strings.Join(missing, ", ")),
		}
	}
	return CheckResult{Status: StatusPass, Detail: "Gitleaks, Semgrep, Trivy, Checkov present"}
}

// ── CheckDiskSpace ─────────────────────────────────────────

// CheckDiskSpace checks free space on critical filesystem paths.
func CheckDiskSpace() CheckResult {
	paths := defaultDiskPaths
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, home)
	}
	return checkDiskPaths(paths)
}

func checkDiskPaths(paths []string) CheckResult {
	var warns []string
	var fails []string

	for _, path := range paths {
		var stat syscall.Statfs_t
		if err := syscall.Statfs(path, &stat); err != nil {
			fails = append(fails, fmt.Sprintf("%s: %v", path, err))
			continue
		}

		total := stat.Blocks * uint64(stat.Bsize)
		free := stat.Bavail * uint64(stat.Bsize)
		pct := float64(0)
		if total > 0 {
			pct = float64(free) / float64(total) * 100
		}

		if pct < 5 {
			fails = append(fails, fmt.Sprintf("%s: %.0f%% free", path, pct))
		} else if pct < 20 {
			warns = append(warns, fmt.Sprintf("%s: %.0f%% free", path, pct))
		}
	}

	if len(fails) > 0 {
		return CheckResult{
			Status: StatusWarn,
			Detail: "Low disk: " + strings.Join(fails, ", "),
		}
	}
	if len(warns) > 0 {
		return CheckResult{
			Status: StatusWarn,
			Detail: "Low disk: " + strings.Join(warns, ", "),
		}
	}
	return CheckResult{
		Status: StatusPass,
		Detail: "All paths have sufficient free space",
	}
}

// ── CheckKuroBinary ────────────────────────────────────────

// CheckKuroBinary looks for the kuro CLI on PATH or ./bin/kuro.
func CheckKuroBinary() CheckResult {
	if path, err := exec.LookPath("kuro"); err == nil {
		return CheckResult{Status: StatusPass, Detail: path}
	}
	candidates := []string{
		filepath.Join(".", "bin", "kuro"),
		filepath.Join("bin", "kuro"),
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			abs, _ := filepath.Abs(c)
			return CheckResult{Status: StatusPass, Detail: abs + " (local build)"}
		}
	}
	return CheckResult{
		Status: StatusWarn,
		Detail: "kuro not on PATH — run `make build` then `./bin/kuro`",
	}
}
