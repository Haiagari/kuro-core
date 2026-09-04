package server

import (
	"io"
	"path/filepath"
	"testing"
)

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
