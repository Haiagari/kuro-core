package server

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type GitService interface {
	InitBare(dir string) error
	UnpackObjects(dir string, r io.Reader, repoURL string) error
	ResolveCommitSHA(dir string) string
	ResolveBranch(dir string) string
	ExtractFiles(gitDir, scanDir, rev string)
	IsCommitish(dir, rev string) bool
}

type defaultGitService struct{}

func (s *defaultGitService) InitBare(dir string) error {
	_, err := runCmd("git", "init", "--bare", dir)
	return err
}

func (s *defaultGitService) UnpackObjects(dir string, r io.Reader, repoURL string) error {
	packDir := filepath.Join(dir, "objects", "pack")
	os.MkdirAll(packDir, 0755)
	packPath := filepath.Join(packDir, "tmp.pack")
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("read pack: %w", err)
	}
	if err := os.WriteFile(packPath, data, 0644); err != nil {
		return fmt.Errorf("write pack: %w", err)
	}
	// Fetch upstream objects to resolve thin pack deltas (new repos send complete packs)
	if repoURL != "" {
		fetchURL := repoURL
		if gitHubToken != "" {
			if gitHubUser != "" {
				fetchURL = "https://" + gitHubUser + ":" + gitHubToken + "@" + strings.TrimPrefix(repoURL, "https://")
			} else {
				fetchURL = "https://git:" + gitHubToken + "@" + strings.TrimPrefix(repoURL, "https://")
			}
		}
		log.Printf("Fetching upstream objects for thin pack: %s", repoURL)
		if out, err := runCmdOutput("git", "-C", dir, "fetch", "--depth=1", fetchURL, "+refs/heads/*:refs/heads/*"); err != nil {
			sanitized := strings.ReplaceAll(string(out), gitHubToken, "[REDACTED]")
			log.Printf("Upstream fetch failed (new repo?): %s", sanitized)
		}
	}
	// Now unpack the thin pack against the fetched objects
	packF, _ := os.Open(packPath)
	if packF != nil {
		defer packF.Close()
		out, err := runCmdWithReader(packF, "git", "-C", dir, "unpack-objects")
		if err != nil {
			return fmt.Errorf("unpack: %s", string(out))
		}
		return nil
	}
	return fmt.Errorf("cannot open packfile")
}

func (s *defaultGitService) ResolveCommitSHA(gitDir string) string {
	out, err := runCmdOutput("git", "-C", gitDir, "rev-parse", "HEAD")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func (s *defaultGitService) ResolveBranch(gitDir string) string {
	out, err := runCmdOutput("git", "-C", gitDir, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		return ""
	}
	ref := strings.TrimSpace(string(out))
	if strings.HasPrefix(ref, "-") {
		return ""
	}
	return ref
}

func (s *defaultGitService) ExtractFiles(gitDir, scanDir, rev string) {
	if rev == "" {
		rev = "HEAD"
		out, err := runCmdOutput("git", "-C", gitDir, "ls-tree", rev)
		if err != nil || len(strings.TrimSpace(string(out))) == 0 {
			out, _ := runCmdOutput("git", "-C", gitDir, "rev-list", "--all", "--max-count=1")
			head := strings.TrimSpace(string(out))
			if head != "" {
				rev = head
			}
		}
	}

	os.MkdirAll(scanDir, 0755)

	gitCmd := exec.Command("git", "-C", gitDir, "archive", "--format=tar", rev)
	tarCmd := exec.Command("tar", "-x", "-C", scanDir)

	pipe, err := gitCmd.StdoutPipe()
	if err != nil {
		return
	}
	tarCmd.Stdin = pipe

	if err := gitCmd.Start(); err != nil {
		return
	}
	if err := tarCmd.Start(); err != nil {
		pipe.Close()
		if gitCmd.Process != nil {
			gitCmd.Process.Kill()
		}
		gitCmd.Wait()
		return
	}

	gitCmd.Wait()
	tarCmd.Wait()
}

// IsCommitish reports whether rev exists in the repo and points to a commit
// or tag (i.e. something git archive can export). Used to reject refs whose
// objects were omitted from the pack, so a ref can never be skipped silently.
func (s *defaultGitService) IsCommitish(dir, rev string) bool {
	out, err := runCmdOutput("git", "-C", dir, "cat-file", "-t", rev)
	if err != nil {
		return false
	}
	t := strings.TrimSpace(string(out))
	return t == "commit" || t == "tag"
}

// sanitizeRefDir converts a ref name into a safe single-path segment so a
// malicious ref cannot escape the scan directory via path traversal.
func sanitizeRefDir(ref string) string {
	var b strings.Builder
	for _, r := range ref {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	s := b.String()
	if s == "" || s == "." || s == ".." {
		s = "ref"
	}
	return s
}

// extractRefsForScan extracts every pushed ref into scanDir. A single ref
// keeps the original root layout; multiple refs each get their own subfolder
// so the combined scan covers all of them. Missing/non-commit objects are a
// hard error: the push must not proceed if a ref cannot be scanned.
func extractRefsForScan(git GitService, gitDir, scanDir string, refs []pushRef, commitSHA string) error {
	if len(refs) <= 1 {
		git.ExtractFiles(gitDir, scanDir, commitSHA)
		return nil
	}
	for _, r := range refs {
		if !git.IsCommitish(gitDir, r.sha) {
			return fmt.Errorf("ref %s: object %s not found in pack", r.ref, r.sha)
		}
		git.ExtractFiles(gitDir, filepath.Join(scanDir, sanitizeRefDir(r.ref)), r.sha)
	}
	return nil
}

type pktLineReader struct {
	r io.Reader
}

// pushRef is a single non-deletion ref command parsed from the receive-pack
// pkt-lines: the new object SHA and the ref being updated.
type pushRef struct {
	sha string
	ref string
}

// parsePushRefs extracts every non-deletion ref from the receive-pack
// pkt-lines. Deletion refs (sha starts with 0000000) are skipped but do not
// stop collection, so all pushed refs are returned.
func parsePushRefs(refCmds []string) []pushRef {
	var refs []pushRef
	for _, cmd := range refCmds {
		parts := strings.Split(cmd, " ")
		if len(parts) >= 3 {
			sha := parts[1]
			ref := parts[2]
			if idx := strings.Index(ref, "\x00"); idx != -1 {
				ref = ref[:idx]
			}
			ref = strings.TrimSpace(ref)
			if sha != "" && !strings.HasPrefix(sha, "0000000") {
				refs = append(refs, pushRef{sha: sha, ref: ref})
			}
		}
	}
	return refs
}

func (pr *pktLineReader) ReadPacket() ([]byte, error) {
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(pr.r, lenBuf); err != nil {
		return nil, err
	}
	length, err := strconv.ParseUint(string(lenBuf), 16, 16)
	if err != nil {
		return nil, fmt.Errorf("invalid pkt-line length: %w", err)
	}
	if length == 0 {
		return nil, nil // Flush packet
	}
	if length < 4 {
		return nil, fmt.Errorf("pkt-line length too short: %d", length)
	}
	dataBuf := make([]byte, length-4)
	if _, err := io.ReadFull(pr.r, dataBuf); err != nil {
		return nil, err
	}
	return dataBuf, nil
}
