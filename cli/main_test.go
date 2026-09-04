package main


import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var binPath string

func TestMain(m *testing.M) {
	tmpDir, err := os.MkdirTemp("", "kuro-cli-test-*")
	if err != nil {
		os.Exit(1)
	}

	binPath = filepath.Join(tmpDir, "kuro")

	cmd := exec.Command("go", "build", "-o", binPath, ".")
	if err := cmd.Run(); err != nil {
		// If build fails, clean up and exit
		os.RemoveAll(tmpDir)
		os.Exit(1)
	}

	code := m.Run()
	os.RemoveAll(tmpDir)
	os.Exit(code)
}

func TestHelpFlag(t *testing.T) {
	out, err := exec.Command(binPath, "--help").CombinedOutput()
	if err != nil {
		t.Fatalf("--help should not error: %v\nOutput: %s", err, out)
	}

	output := string(out)
	if !strings.Contains(output, "KURO Pipeline") {
		t.Error("--help should contain 'KURO Pipeline'")
	}
	if !strings.Contains(output, "scan") {
		t.Error("--help should mention 'scan' command")
	}
	if !strings.Contains(output, "deploy") {
		t.Error("--help should mention 'deploy' command")
	}
	if !strings.Contains(output, "help") {
		t.Error("--help should mention 'help' command")
	}
}

func TestHelpShortFlag(t *testing.T) {
	out, err := exec.Command(binPath, "-h").CombinedOutput()
	if err != nil {
		t.Fatalf("-h should not error: %v\nOutput: %s", err, out)
	}

	output := string(out)
	if !strings.Contains(output, "KURO Pipeline") {
		t.Error("-h should show help with 'KURO Pipeline'")
	}
}

func TestHelpSubcommand(t *testing.T) {
	out, err := exec.Command(binPath, "help").CombinedOutput()
	if err != nil {
		t.Fatalf("help subcommand should not error: %v\nOutput: %s", err, out)
	}

	if !strings.Contains(string(out), "KURO Pipeline") {
		t.Error("help subcommand should show 'KURO Pipeline'")
	}
}

func TestVersion(t *testing.T) {
	out, err := exec.Command(binPath, "version").CombinedOutput()
	if err != nil {
		t.Fatalf("version should not error: %v\nOutput: %s", err, out)
	}

	output := strings.TrimSpace(string(out))
	if !strings.Contains(output, "kuro version") {
		t.Errorf("version output should contain 'kuro version', got: %q", output)
	}
	if !strings.Contains(output, Version) {
		t.Errorf("version output should contain %q, got: %q", Version, output)
	}
}

func TestNoArgs(t *testing.T) {
	cmd := exec.Command(binPath)
	out, err := cmd.CombinedOutput()

	// With no args, the CLI should exit 1
	if err == nil {
		t.Error("kuro with no args should exit non-zero")
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitErr.ExitCode() != 1 {
			t.Errorf("expected exit code 1, got %d", exitErr.ExitCode())
		}
	} else {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}

	if !strings.Contains(string(out), "KURO Pipeline") {
		t.Error("no-args output should show help with 'KURO Pipeline'")
	}
}

func TestUnknownCommand(t *testing.T) {
	cmd := exec.Command(binPath, "nonexistent")
	out, err := cmd.CombinedOutput()

	if err == nil {
		t.Error("unknown command should exit non-zero")
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitErr.ExitCode() != 1 {
			t.Errorf("expected exit code 1, got %d", exitErr.ExitCode())
		}
	}

	stderr := string(out)
	if !strings.Contains(stderr, "unknown command") {
		t.Errorf("unknown command should print error, got: %s", stderr)
	}
}

func TestScanWithNoArgs(t *testing.T) {
	cmd := exec.Command(binPath, "scan")
	out, err := cmd.CombinedOutput()

	if err == nil {
		t.Error("scan with no args should exit non-zero")
	}

	output := string(out)
	if !strings.Contains(output, "Usage: kuro scan") {
		t.Errorf("scan with no args should show usage, got: %s", output)
	}
}

func TestAuthWithNoArgs(t *testing.T) {
	cmd := exec.Command(binPath, "auth")
	out, err := cmd.CombinedOutput()

	if err == nil {
		t.Error("auth with no args should exit non-zero")
	}

	output := string(out)
	if !strings.Contains(output, "Usage: kuro auth") {
		t.Errorf("auth with no args should show usage, got: %s", output)
	}
}

func TestWebhookUnknownSubcommand(t *testing.T) {
	cmd := exec.Command(binPath, "webhook", "badsub")
	out, err := cmd.CombinedOutput()

	if err == nil {
		t.Error("unknown webhook subcommand should exit non-zero")
	}

	output := string(out)
	if !strings.Contains(output, "unknown webhook subcommand") {
		t.Errorf("should report unknown webhook subcommand, got: %s", output)
	}
}

func TestSetupWithNoArgs(t *testing.T) {
	cmd := exec.Command(binPath, "setup")
	out, err := cmd.CombinedOutput()

	if err == nil {
		t.Error("setup with no args should exit non-zero")
	}

	output := string(out)
	if !strings.Contains(output, "Usage: kuro setup") {
		t.Errorf("setup with no args should show usage, got: %s", output)
	}
}

func TestSetupUnknownComponent(t *testing.T) {
	cmd := exec.Command(binPath, "setup", "bogus")
	out, err := cmd.CombinedOutput()

	if err == nil {
		t.Error("unknown setup component should exit non-zero")
	}

	output := string(out)
	if !strings.Contains(output, "Unknown component") {
		t.Errorf("should report unknown component, got: %s", output)
	}
}

func TestWebhookHelp(t *testing.T) {
	out, err := exec.Command(binPath, "webhook", "help").CombinedOutput()
	if err != nil {
		t.Fatalf("webhook help should not error: %v\nOutput: %s", err, out)
	}

	output := string(out)
	if !strings.Contains(output, "kuro webhook") && !strings.Contains(output, "Manage notification") {
		t.Errorf("webhook help should show usage, got: %s", output)
	}
}

func TestBackupInvalidSubcommand(t *testing.T) {
	cmd := exec.Command(binPath, "backup", "badsub")
	out, err := cmd.CombinedOutput()

	if err == nil {
		t.Error("unknown backup subcommand should exit non-zero")
	}

	output := string(out)
	if !strings.Contains(output, "unknown command") {
		t.Errorf("should report unknown command, got: %s", output)
	}
}
