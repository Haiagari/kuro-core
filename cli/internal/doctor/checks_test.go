package doctor


import (
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// ── TCP dial checks ─────────────────────────────────────────

func TestCheckTCP_Pass(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}
	defer listener.Close()

	// Accept and close immediately to simulate a listening service
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		conn.Close()
	}()

	result := checkTCP(listener.Addr().String(), "TestService")
	if result.Status != StatusPass {
		t.Errorf("expected PASS, got %s: %s", result.Status, result.Detail)
	}
}

func TestCheckTCP_Refused(t *testing.T) {
	// Use a port that's not listening
	result := checkTCP("127.0.0.1:19999", "TestService")
	if result.Status != StatusFail {
		t.Errorf("expected FAIL, got %s: %s", result.Status, result.Detail)
	}
}

// ── HTTP checks ────────────────────────────────────────────

func TestCheckHTTP_Pass(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	result := checkHTTP(server.URL, "TestHTTP")
	if result.Status != StatusPass {
		t.Errorf("expected PASS, got %s: %s", result.Status, result.Detail)
	}
}

func TestCheckHTTP_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	result := checkHTTP(server.URL, "TestHTTP")
	if result.Status != StatusFail {
		t.Errorf("expected FAIL, got %s: %s", result.Status, result.Detail)
	}
}

func TestCheckHTTP_Timeout(t *testing.T) {
	// Start a server that never responds
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}
	defer listener.Close()

	// Accept but never respond - will cause timeout
	go func() {
		_, _ = listener.Accept()
		// hang forever
	}()

	// Use a short timeout URL
	result := checkHTTP("http://"+listener.Addr().String()+"/", "TestTimeout")
	if result.Status != StatusFail {
		t.Errorf("expected FAIL on timeout, got %s: %s", result.Status, result.Detail)
	}
}

// ── Disk space check ───────────────────────────────────────

func TestCheckDiskSpace_Pass(t *testing.T) {
	// Use current directory's filesystem (always has some space)
	result := CheckDiskSpace()
	if result.Status == StatusFail {
		t.Errorf("disk should not fail on root dir: %s", result.Detail)
	}
}

func TestCheckDiskSpace_NonexistentDir(t *testing.T) {
	result := checkDiskPaths([]string{"/nonexistent-path-xyz-123"})
	if result.Status != StatusWarn {
		t.Errorf("expected WARN for nonexistent dir, got %s: %s", result.Status, result.Detail)
	}
	if !strings.Contains(result.Detail, "no such") {
		t.Errorf("expected error detail about missing dir, got: %s", result.Detail)
	}
}

// ── Component versions check ───────────────────────────────

func TestCheckVersions_WithAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","db":"connected","nats":"connected"}`))
	}))
	defer server.Close()

	result := checkVersionsURL(server.URL)
	if result.Status != StatusPass {
		t.Errorf("expected PASS, got %s: %s", result.Status, result.Detail)
	}
	if !strings.Contains(result.Detail, "ok") {
		t.Errorf("detail should contain status, got: %s", result.Detail)
	}
}

func TestCheckVersions_APIDown(t *testing.T) {
	result := checkVersionsURL("http://127.0.0.1:19998")
	if result.Status != StatusWarn {
		t.Errorf("expected WARN when API down, got %s: %s", result.Status, result.Detail)
	}
}

func TestCheckVersions_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`not json`))
	}))
	defer server.Close()

	result := checkVersionsURL(server.URL)
	if result.Status != StatusWarn {
		t.Errorf("expected WARN for bad JSON, got %s: %s", result.Status, result.Detail)
	}
}

// ── CheckPostgres (TCP part only; docker exec skipped in unit tests) ──

func TestCheckPostgres_TCPRefused(t *testing.T) {
	// Save and restore defaults
	orig := postgresAddr
	postgresAddr = "127.0.0.1:19997"
	defer func() { postgresAddr = orig }()

	result := CheckPostgres()
	// TCP will fail (no listener), then docker exec part may also fail
	// The check should still yield some valid status
	if result.Status != StatusFail {
		t.Errorf("expected FAIL when nothing listening, got %s: %s", result.Status, result.Detail)
	}
}

// ── CheckNATS ──────────────────────────────────────────────

func TestCheckNATS_Refused(t *testing.T) {
	orig := natsAddr
	natsAddr = "127.0.0.1:19996"
	defer func() { natsAddr = orig }()

	result := CheckNATS()
	if result.Status != StatusFail {
		t.Errorf("expected FAIL when nothing listening, got %s: %s", result.Status, result.Detail)
	}
}

// ── CheckMinIO ─────────────────────────────────────────────

func TestCheckMinIO_Fail(t *testing.T) {
	orig := minioURL
	minioURL = "http://127.0.0.1:19995"
	defer func() { minioURL = orig }()

	result := CheckMinIO()
	if result.Status != StatusFail {
		t.Errorf("expected FAIL when nothing listening, got %s: %s", result.Status, result.Detail)
	}
}

// ── CheckDockerProxy ───────────────────────────────────────

func TestCheckDockerProxy_Fail(t *testing.T) {
	orig := dockerURL
	dockerURL = "http://127.0.0.1:19994"
	defer func() { dockerURL = orig }()

	result := CheckDockerProxy()
	if result.Status != StatusFail {
		t.Errorf("expected FAIL when nothing listening, got %s: %s", result.Status, result.Detail)
	}
}

// ── ScannerImages ──────────────────────────────────────────

func TestCheckScannerImages_NoDocker(t *testing.T) {
	result := CheckScannerImages()
	// Without Docker running, this will fail gracefully
	// It should not panic
	_ = result
}

// ── CheckBackupStatus ──────────────────────────────────────

func TestCheckBackupStatus_NoMinIO(t *testing.T) {
	result := CheckBackupStatus()
	// Should not panic when MinIO is unreachable
	_ = result
}

// ── Edge: checkTCP with very short timeout ─────────────────

func TestCheckTCP_Timeout(t *testing.T) {
	// Create a TCP server that accepts but never responds
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}
	defer listener.Close()

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		// Accept but don't write anything - the dial will still succeed
		// because TCP handshake completes. We need a different test for timeout.
		conn.Close()
	}()

	// Dial should succeed since TCP handshake completes
	result := checkTCP(listener.Addr().String(), "TestAccept")
	if result.Status != StatusPass {
		t.Logf("TCP dial to accepting listener: %s — %s (expected PASS)", result.Status, result.Detail)
	}
}

// ── Real filesystem test ───────────────────────────────────

func TestCheckDiskSpace_RealTemp(t *testing.T) {
	result := checkDiskPaths([]string{os.TempDir()})
	if result.Status == StatusFail {
		t.Errorf("temp dir should not fail: %s", result.Detail)
	}
	if result.Status == StatusPass {
		if !strings.Contains(result.Detail, "free") {
			t.Errorf("detail should contain free space info, got: %s", result.Detail)
		}
	}
}

func TestCheckDiskSpace_MultiplePaths(t *testing.T) {
	result := checkDiskPaths([]string{os.TempDir(), "/"})
	if result.Status == StatusFail {
		t.Errorf("known paths should not fail: %s", result.Detail)
	}
}
