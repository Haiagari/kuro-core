package doctor


import (
	"os"
	"strings"
	"testing"
)

func TestCheckDiskSpace_Pass(t *testing.T) {
	result := CheckDiskSpace()
	if result.Status == StatusFail {
		t.Errorf("disk should not fail: %s", result.Detail)
	}
}

func TestCheckDiskSpace_NonexistentDir(t *testing.T) {
	result := checkDiskPaths([]string{"/nonexistent-path-xyz-123"})
	if result.Status != StatusWarn {
		t.Errorf("expected WARN for nonexistent dir, got %s: %s", result.Status, result.Detail)
	}
}

func TestCheckDiskSpace_RealTemp(t *testing.T) {
	result := checkDiskPaths([]string{os.TempDir()})
	if result.Status == StatusFail {
		t.Errorf("temp dir should not fail: %s", result.Detail)
	}
}

func TestCheckGit(t *testing.T) {
	result := CheckGit()
	// CI / workstations almost always have git; if missing, Fail is correct.
	if result.Status != StatusPass && result.Status != StatusFail {
		t.Errorf("unexpected status: %s (%s)", result.Status, result.Detail)
	}
}

func TestCheckContainerRuntime(t *testing.T) {
	result := CheckContainerRuntime()
	_ = result // environment-dependent; must not panic
}

func TestCheckScannerImages(t *testing.T) {
	_ = CheckContainerRuntime()
	result := CheckScannerImages()
	_ = result // must not panic without images
}

func TestCheckKuroBinary(t *testing.T) {
	result := CheckKuroBinary()
	if result.Status != StatusPass && result.Status != StatusWarn {
		t.Errorf("expected PASS or WARN, got %s: %s", result.Status, result.Detail)
	}
	if result.Status == StatusWarn && !strings.Contains(result.Detail, "make build") {
		t.Errorf("warn detail should hint make build, got: %s", result.Detail)
	}
}
