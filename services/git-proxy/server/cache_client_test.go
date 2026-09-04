package server

import (
	"errors"
	"os"
	"testing"
	"time"
)

type mockKuroClient struct {
	calls    int
	blocked  bool
	findings []proxyScanFinding
	err      error
}

func (m *mockKuroClient) Scan(dir, repo, commit, branch string) (bool, []proxyScanFinding, error) {
	m.calls++
	return m.blocked, m.findings, m.err
}

func TestCachedKuroClient(t *testing.T) {
	underlying := &mockKuroClient{
		blocked:  false,
		findings: nil,
	}
	cached := newCachedKuroClient(underlying)

	// First scan: should call underlying
	blocked, findings, err := cached.Scan("dir", "repo", "commit1", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if blocked {
		t.Fatal("expected approved")
	}
	if len(findings) != 0 {
		t.Fatal("expected no findings")
	}
	if underlying.calls != 1 {
		t.Fatalf("expected 1 call, got %d", underlying.calls)
	}

	// Second scan (within 1 hour, approved): should hit cache and NOT call underlying
	blocked, findings, err = cached.Scan("dir", "repo", "commit1", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if blocked {
		t.Fatal("expected approved")
	}
	if underlying.calls != 1 {
		t.Fatalf("expected call count to remain 1, got %d", underlying.calls)
	}

	// Third scan with a different commit: should call underlying
	blocked, findings, err = cached.Scan("dir", "repo", "commit2", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if underlying.calls != 2 {
		t.Fatalf("expected 2 calls, got %d", underlying.calls)
	}

	// Mocking a blocked commit
	underlyingBlocked := &mockKuroClient{
		blocked:  true,
		findings: []proxyScanFinding{{File: "test", Line: 1}},
	}
	cachedBlocked := newCachedKuroClient(underlyingBlocked)

	// First scan for blocked: should call underlying
	blocked, findings, err = cachedBlocked.Scan("dir", "repo", "commit3", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !blocked {
		t.Fatal("expected blocked")
	}
	if underlyingBlocked.calls != 1 {
		t.Fatalf("expected 1 call, got %d", underlyingBlocked.calls)
	}

	// Second scan for blocked (not approved): should NOT hit cache (not cached because not approved) and should call underlying again
	blocked, findings, err = cachedBlocked.Scan("dir", "repo", "commit3", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !blocked {
		t.Fatal("expected blocked")
	}
	if underlyingBlocked.calls != 2 {
		t.Fatalf("expected 2 calls, got %d", underlyingBlocked.calls)
	}

	// Cache entry expiry test (we can construct an entry manually and check if we bypass it if older than 1h)
	cachedExpired := newCachedKuroClient(underlying)
	cachedExpired.cache.Store("repo:commitExpired", cacheEntry{
		timestamp: time.Now().Add(-2 * time.Hour), // Expired
		blocked:   false,
		findings:  nil,
	})
	underlying.calls = 0
	_, _, err = cachedExpired.Scan("dir", "repo", "commitExpired", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if underlying.calls != 1 {
		t.Fatalf("expected underlying call for expired entry, got %d", underlying.calls)
	}
}

func TestCachedKuroClient_FailClosedDefault(t *testing.T) {
	os.Unsetenv("PROXY_FAIL_MODE")
	underlying := &mockKuroClient{err: errors.New("network error")}
	cached := newCachedKuroClient(underlying)

	// First scan fails: fail-closed (blocked) with synthetic finding and err surfaced.
	blocked, findings, err := cached.Scan("dir", "repo", "commit1", "main")
	if !blocked {
		t.Fatal("expected fail-closed (blocked) on scan failure by default")
	}
	if len(findings) == 0 || findings[0].Rule != "SECURITY_GATE_OFFLINE" {
		t.Fatalf("expected SECURITY_GATE_OFFLINE finding on scan failure, got %+v", findings)
	}
	if err == nil {
		t.Fatal("expected error on scan failure")
	}
	if underlying.calls != 1 {
		t.Fatalf("expected 1 call, got %d", underlying.calls)
	}

	// Second identical scan: must re-run the scan, not hit the cache.
	underlying.err = nil
	underlying.blocked = false
	blocked, findings, err = cached.Scan("dir", "repo", "commit1", "main")
	if err != nil {
		t.Fatalf("unexpected error on recovered scan: %v", err)
	}
	if blocked {
		t.Fatal("expected approved on recovered scan")
	}
	if underlying.calls != 2 {
		t.Fatalf("expected failure NOT to be cached (2 calls), got %d", underlying.calls)
	}
}

func TestCachedKuroClient_FailOpenWhenConfigured(t *testing.T) {
	os.Setenv("PROXY_FAIL_MODE", "open")
	defer os.Unsetenv("PROXY_FAIL_MODE")

	underlying := &mockKuroClient{err: errors.New("network error")}
	cached := newCachedKuroClient(underlying)

	blocked, findings, err := cached.Scan("dir", "repo", "commit1", "main")
	if blocked {
		t.Fatal("expected fail-open (approved) when PROXY_FAIL_MODE=open")
	}
	if len(findings) != 0 {
		t.Fatal("expected no findings on fail-open scan failure")
	}
	if err == nil {
		t.Fatal("expected error to still be returned on scan failure")
	}
}
