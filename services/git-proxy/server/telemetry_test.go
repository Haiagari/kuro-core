package server

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestTelemetry_FormatSidebandPacket(t *testing.T) {
	msg := "remote: Scanning commit...\n"
	pkt := FormatSidebandPacket(2, msg)

	// Format: 4-byte hex length + 1 byte channel + message
	expectedLen := len(msg) + 5
	if len(pkt) != expectedLen {
		t.Fatalf("expected packet length %d, got %d", expectedLen, len(pkt))
	}

	if pkt[4] != 2 {
		t.Fatalf("expected channel byte 2 (sideband progress), got %d", pkt[4])
	}
}

func TestTelemetry_RenderScannerProgress(t *testing.T) {
	buf := &bytes.Buffer{}
	reporter := NewLiveTelemetryReporter(buf, false)

	reporter.EmitBanner("my-org/backend-api", "d3b07384d113", "main")
	reporter.UpdateStage("Gitleaks", "passed", 150*time.Millisecond, 0)
	reporter.UpdateStage("Semgrep", "passed", 850*time.Millisecond, 0)
	reporter.UpdateStage("Trivy", "blocked", 600*time.Millisecond, 2)
	reporter.EmitVerdict(false, 2, 1600*time.Millisecond)

	output := buf.String()

	if !strings.Contains(output, "KURO Zero-Trust Security Pipeline") {
		t.Errorf("expected banner in telemetry output")
	}
	if !strings.Contains(output, "Gitleaks") || !strings.Contains(output, "Semgrep") {
		t.Errorf("expected scanner names in telemetry output")
	}
	if !strings.Contains(output, "[KURO VERDICT] BLOCKED") {
		t.Errorf("expected BLOCKED verdict in telemetry output")
	}
}

func TestTelemetry_ApprovedVerdictFlow(t *testing.T) {
	buf := &bytes.Buffer{}
	reporter := NewLiveTelemetryReporter(buf, true)

	reporter.EmitBanner("my-org/clean-service", "999888777", "main")
	reporter.UpdateStage("Gitleaks", "passed", 100*time.Millisecond, 0)
	reporter.UpdateStage("Semgrep", "passed", 400*time.Millisecond, 0)
	reporter.UpdateStage("Trivy", "passed", 300*time.Millisecond, 0)
	reporter.EmitVerdict(true, 0, 800*time.Millisecond)

	output := buf.String()

	if !strings.Contains(output, "[KURO VERDICT] APPROVED") {
		t.Fatalf("expected APPROVED verdict, got: %s", output)
	}
	if !strings.Contains(output, "Cryptographically signed") {
		t.Fatalf("expected attestation notice in verdict, got: %s", output)
	}
}
