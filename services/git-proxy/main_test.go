package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGetEnv_UsesEnvVar(t *testing.T) {
	os.Setenv("TEST_GETENV_KEY", "from-env")
	defer os.Unsetenv("TEST_GETENV_KEY")

	got := getEnv("TEST_GETENV_KEY", "fallback")
	if got != "from-env" {
		t.Fatalf("expected %q, got %q", "from-env", got)
	}
}

func TestGetEnv_UsesFallback(t *testing.T) {
	os.Unsetenv("TEST_GETENV_MISSING")

	got := getEnv("TEST_GETENV_MISSING", "fallback")
	if got != "fallback" {
		t.Fatalf("expected %q, got %q", "fallback", got)
	}
}

func TestGetEnv_EmptyEnvUsesFallback(t *testing.T) {
	os.Setenv("TEST_GETENV_EMPTY", "")
	defer os.Unsetenv("TEST_GETENV_EMPTY")

	got := getEnv("TEST_GETENV_EMPTY", "fallback")
	if got != "fallback" {
		t.Fatalf("expected %q, got %q", "fallback", got)
	}
}

func TestExtractRepoPath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/owner/repo.git/git-receive-pack", "owner/repo"},
		{"/owner/repo/git-receive-pack", "owner/repo"},
		{"/owner/repo.git", "owner/repo"},
		{"/owner/repo", "owner/repo"},
		{"/a/b/c.git/git-receive-pack", "a/b/c"},
		{"/git-receive-pack", ""},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := extractRepoPath(tc.input)
			if got != tc.expected {
				t.Fatalf("extractRepoPath(%q) = %q, want %q", tc.input, got, tc.expected)
			}
		})
	}
}

func TestAddAuth_NoToken(t *testing.T) {
	oldToken := gitHubToken
	oldUser := gitHubUser
	gitHubToken = ""
	gitHubUser = ""
	defer func() {
		gitHubToken = oldToken
		gitHubUser = oldUser
	}()

	req, _ := http.NewRequest("GET", "https://example.com/test", nil)
	addAuth(req)

	if auth := req.Header.Get("Authorization"); auth != "" {
		t.Fatalf("expected no Authorization header, got %q", auth)
	}
}

func TestAddAuth_BearerOnly(t *testing.T) {
	oldToken := gitHubToken
	oldUser := gitHubUser
	gitHubToken = "ghp_testtoken"
	gitHubUser = ""
	defer func() {
		gitHubToken = oldToken
		gitHubUser = oldUser
	}()

	req, _ := http.NewRequest("GET", "https://example.com/test", nil)
	addAuth(req)

	if got := req.Header.Get("Authorization"); got != "Bearer ghp_testtoken" {
		t.Fatalf("expected Bearer token, got %q", got)
	}
}

func TestAddAuth_BasicAuth(t *testing.T) {
	oldToken := gitHubToken
	oldUser := gitHubUser
	gitHubToken = "ghp_testtoken"
	gitHubUser = "myuser"
	defer func() {
		gitHubToken = oldToken
		gitHubUser = oldUser
	}()

	req, _ := http.NewRequest("GET", "https://example.com/test", nil)
	addAuth(req)

	user, pass, ok := req.BasicAuth()
	if !ok {
		t.Fatal("expected BasicAuth to be set")
	}
	if user != "myuser" {
		t.Fatalf("expected user %q, got %q", "myuser", user)
	}
	if pass != "ghp_testtoken" {
		t.Fatalf("expected pass %q, got %q", "ghp_testtoken", pass)
	}
}

func TestProxyScanFindingJSON(t *testing.T) {
	f := proxyScanFinding{
		File:        "src/main.go",
		Line:        42,
		Rule:        "G101",
		Scanner:     "gitleaks",
		Description: "Hardcoded credential",
		Severity:    "high",
	}

	got, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	expected := `{"file":"src/main.go","line":42,"rule":"G101","scanner":"gitleaks","description":"Hardcoded credential","severity":"high"}`
	if string(got) != expected {
		t.Fatalf("unexpected JSON:\n  got:  %s\n  want: %s", string(got), expected)
	}
}

func TestProxyScanResponseJSON(t *testing.T) {
	r := proxyScanResponse{
		ScanID:   "scan-123",
		Decision: "BLOCKED",
		Findings: []proxyScanFinding{
			{File: "test.txt", Line: 1, Rule: "G101", Scanner: "gitleaks", Description: "secret", Severity: "high"},
		},
	}

	got, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	expected := `{"scan_id":"scan-123","decision":"BLOCKED","findings":[{"file":"test.txt","line":1,"rule":"G101","scanner":"gitleaks","description":"secret","severity":"high"}]}`
	if string(got) != expected {
		t.Fatalf("unexpected JSON:\n  got:  %s\n  want: %s", string(got), expected)
	}
}

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

func TestParsePushRefs_CollectsAllNonDeletionRefs(t *testing.T) {
	cmds := []string{
		"0000000000000000000000000000000000000000 1111111111111111111111111111111111111111 refs/heads/feature-clean",
		"1111111111111111111111111111111111111111 2222222222222222222222222222222222222222 refs/heads/feature-malicious\x00report-status",
		"2222222222222222222222222222222222222222 0000000000000000000000000000000000000000 refs/heads/deleted",
		"malformed-line",
	}

	refs := parsePushRefs(cmds)

	if len(refs) != 2 {
		t.Fatalf("expected 2 non-deletion refs, got %d: %+v", len(refs), refs)
	}
	if refs[0].sha != "1111111111111111111111111111111111111111" || refs[0].ref != "refs/heads/feature-clean" {
		t.Fatalf("unexpected first ref: %+v", refs[0])
	}
	if refs[1].sha != "2222222222222222222222222222222222222222" || refs[1].ref != "refs/heads/feature-malicious" {
		t.Fatalf("unexpected second ref: %+v", refs[1])
	}
}

func TestParsePushRefs_DeletionDoesNotStopCollection(t *testing.T) {
	cmds := []string{
		"aaaa 0000000000000000000000000000000000000000 refs/heads/deleted",
		"aaaa 1111111111111111111111111111111111111111 refs/heads/live",
	}
	refs := parsePushRefs(cmds)
	if len(refs) != 1 || refs[0].ref != "refs/heads/live" {
		t.Fatalf("deletion ref must be skipped without stopping collection, got %+v", refs)
	}
}

type fakeGitService struct {
	extracted []struct{ scanDir, rev string }
	commitish map[string]bool
}

func (f *fakeGitService) InitBare(dir string) error                                   { return nil }
func (f *fakeGitService) UnpackObjects(dir string, r io.Reader, repoURL string) error { return nil }
func (f *fakeGitService) ResolveCommitSHA(dir string) string                          { return "" }
func (f *fakeGitService) ResolveBranch(dir string) string                             { return "" }
func (f *fakeGitService) ExtractFiles(gitDir, scanDir, rev string) {
	f.extracted = append(f.extracted, struct{ scanDir, rev string }{scanDir, rev})
}
func (f *fakeGitService) IsCommitish(dir, rev string) bool {
	if f.commitish == nil {
		return true
	}
	return f.commitish[rev]
}

func TestExtractRefsForScan_MultiRefCoversAllRefs(t *testing.T) {
	git := &fakeGitService{}
	refs := []pushRef{
		{sha: "aaaa", ref: "refs/heads/feature-clean"},
		{sha: "bbbb", ref: "refs/heads/feature-malicious"},
	}
	scanDir := "/tmp/scan"

	if err := extractRefsForScan(git, "/tmp/repo.git", scanDir, refs, "aaaa"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(git.extracted) != 2 {
		t.Fatalf("expected 2 extractions (one per ref), got %d", len(git.extracted))
	}
	if git.extracted[0].scanDir != filepath.Join(scanDir, "refs_heads_feature-clean") {
		t.Fatalf("unexpected first scan dir: %s", git.extracted[0].scanDir)
	}
	if git.extracted[1].scanDir != filepath.Join(scanDir, "refs_heads_feature-malicious") {
		t.Fatalf("unexpected second scan dir: %s", git.extracted[1].scanDir)
	}
	if git.extracted[0].rev != "aaaa" || git.extracted[1].rev != "bbbb" {
		t.Fatalf("unexpected revs: %+v", git.extracted)
	}
}

func TestExtractRefsForScan_SingleRefKeepsRootLayout(t *testing.T) {
	git := &fakeGitService{}
	refs := []pushRef{{sha: "aaaa", ref: "refs/heads/main"}}

	if err := extractRefsForScan(git, "/tmp/repo.git", "/tmp/scan", refs, "aaaa"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(git.extracted) != 1 || git.extracted[0].scanDir != "/tmp/scan" {
		t.Fatalf("single ref must extract at the scan dir root, got %+v", git.extracted)
	}
}

func TestExtractRefsForScan_BlocksWhenRefObjectMissing(t *testing.T) {
	git := &fakeGitService{commitish: map[string]bool{"aaaa": true, "bbbb": false}}
	refs := []pushRef{
		{sha: "aaaa", ref: "refs/heads/a"},
		{sha: "bbbb", ref: "refs/heads/b"},
	}

	if err := extractRefsForScan(git, "/tmp/repo.git", "/tmp/scan", refs, "aaaa"); err == nil {
		t.Fatal("expected error when a ref object is missing from the pack")
	}
	if len(git.extracted) != 1 {
		t.Fatalf("extraction must stop on the unverifiable ref, got %d extractions", len(git.extracted))
	}
}

func TestSanitizeRefDir(t *testing.T) {
	cases := map[string]string{
		"refs/heads/feature":   "refs_heads_feature",
		"refs/heads/../../etc": "refs_heads_.._.._etc",
		"../escape":            ".._escape",
		"../../etc/passwd":     ".._.._etc_passwd",
		"":                     "ref",
		".":                    "ref",
		"..":                   "ref",
	}
	for in, want := range cases {
		if got := sanitizeRefDir(in); got != want {
			t.Errorf("sanitizeRefDir(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCachedKuroClient_MultiRefCombinedIdentity(t *testing.T) {
	underlying := &mockKuroClient{blocked: false}
	cached := newCachedKuroClient(underlying)

	// Push A: refs (sha1, sha2) -> combined identity key
	blocked, _, err := cached.Scan("dir", "repo", "sha1,sha2", "feature-clean")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if blocked {
		t.Fatal("expected approved")
	}
	// Push B: same first ref, different second ref -> must NOT reuse A's cache
	blocked, _, err = cached.Scan("dir", "repo", "sha1,sha3", "feature-clean")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if blocked {
		t.Fatal("expected approved")
	}
	if underlying.calls != 2 {
		t.Fatalf("expected 2 underlying calls (distinct ref sets), got %d", underlying.calls)
	}
	// Push C: identical ref set -> cache hit
	blocked, _, err = cached.Scan("dir", "repo", "sha1,sha2", "feature-clean")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if blocked {
		t.Fatal("expected approved")
	}
	if underlying.calls != 2 {
		t.Fatalf("expected cache hit for identical ref set, got %d calls", underlying.calls)
	}
}
