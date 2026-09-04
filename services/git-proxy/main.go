package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

var (
	upstreamURL  = getEnv("UPSTREAM_URL", "https://github.com")
	listenAddr   = getEnv("LISTEN_ADDR", ":8000")
	tmpBase      = getEnv("TMP_DIR", "/tmp/kuro-proxy")
	scanHostBase = getEnv("SCAN_WORKDIR_HOST", "/tmp/kuro-scans")
	gitHubUser   = os.Getenv("GITHUB_USER")
	gitHubToken  = os.Getenv("GITHUB_TOKEN")
	proxyClient  = &http.Client{Timeout: 30 * time.Second}
)

type clientRate struct {
	timestamps []time.Time
}

type RateLimiter interface {
	Allow(ip string, maxRequests int, window time.Duration) bool
}

type rateLimiter struct {
	mu      sync.Mutex
	clients map[string]*clientRate
}

func (rl *rateLimiter) Allow(ip string, maxRequests int, window time.Duration) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	cr, ok := rl.clients[ip]
	if !ok {
		rl.clients[ip] = &clientRate{timestamps: []time.Time{now}}
		return true
	}
	cutoff := now.Add(-window)
	var valid []time.Time
	for _, t := range cr.timestamps {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	if len(valid) >= maxRequests {
		rl.clients[ip] = &clientRate{timestamps: valid}
		return false
	}
	valid = append(valid, now)
	rl.clients[ip] = &clientRate{timestamps: valid}
	return true
}

func newRateLimiter() *rateLimiter {
	rl := &rateLimiter{clients: make(map[string]*clientRate)}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("panic in rate limiter sweep: %v", r)
			}
		}()
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			rl.sweep()
		}
	}()
	return rl
}

func (rl *rateLimiter) sweep() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	// Cleanup timestamps older than 1 hour to prevent map growth
	cutoff := now.Add(-time.Hour)
	for ip, cr := range rl.clients {
		var valid []time.Time
		for _, t := range cr.timestamps {
			if t.After(cutoff) {
				valid = append(valid, t)
			}
		}
		if len(valid) == 0 {
			delete(rl.clients, ip)
		} else {
			cr.timestamps = valid
		}
	}
}

type proxyScanResponse struct {
	ScanID   string             `json:"scan_id"`
	Decision string             `json:"decision"`
	Findings []proxyScanFinding `json:"findings"`
}

type proxyScanFinding struct {
	File        string `json:"file"`
	Line        int    `json:"line"`
	Rule        string `json:"rule"`
	Scanner     string `json:"scanner"`
	Description string `json:"description"`
	Severity    string `json:"severity"`
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

type KuroAPIClient interface {
	// Scan returns the proxy scan decision. err is non-nil on any failure
	// (network error, non-2xx/403 status, decode failure) and distinguishes a
	// fail-open result from a genuine approval.
	Scan(dir, repo, commit, branch string) (blocked bool, findings []proxyScanFinding, err error)
}

type defaultKuroClient struct{}

func (c *defaultKuroClient) Scan(dir, repo, commit, branch string) (bool, []proxyScanFinding, error) {
	body, _ := json.Marshal(map[string]string{
		"path":   dir,
		"repo":   repo,
		"commit": commit,
		"branch": branch,
	})

	kuroURL := getEnv("KURO_URL", "http://api:8080")
	req, _ := http.NewRequest("POST", kuroURL+"/api/v1/scans/proxy", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token := os.Getenv("KURO_API_KEY"); token != "" {
		req.Header.Set("X-API-Key", token)
	}

	client := &http.Client{Timeout: 180 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, nil, fmt.Errorf("kuro scan request failed: %w", err)
	}
	defer resp.Body.Close()

	var result proxyScanResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, nil, fmt.Errorf("kuro scan: invalid response (status %d): %w", resp.StatusCode, err)
	}

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return result.Decision == "BLOCKED", result.Findings, nil
	case resp.StatusCode == http.StatusForbidden:
		return true, result.Findings, nil
	default:
		return false, nil, fmt.Errorf("kuro scan returned unexpected status %d", resp.StatusCode)
	}
}

type cacheEntry struct {
	timestamp time.Time
	blocked   bool
	findings  []proxyScanFinding
}

type cachedKuroClient struct {
	underlying KuroAPIClient
	cache      sync.Map
}

func newCachedKuroClient(underlying KuroAPIClient) *cachedKuroClient {
	return &cachedKuroClient{
		underlying: underlying,
	}
}

func (c *cachedKuroClient) Scan(dir, repo, commit, branch string) (bool, []proxyScanFinding, error) {
	key := fmt.Sprintf("%s:%s", repo, commit)
	if val, ok := c.cache.Load(key); ok {
		if entry, ok := val.(cacheEntry); ok {
			if time.Since(entry.timestamp) < time.Hour && !entry.blocked {
				log.Printf("Kuro scan cache hit for %s", key)
				return entry.blocked, entry.findings, nil
			}
		}
	}

	blocked, findings, err := c.underlying.Scan(dir, repo, commit, branch)
	if err != nil {
		failMode := strings.ToLower(getEnv("PROXY_FAIL_MODE", "closed"))
		if failMode == "open" {
			log.Printf("kuro scan failed, allowing push under fail-open policy (result not cached): %v", err)
			return false, nil, err
		}
		log.Printf("kuro scan failed, blocking push under fail-closed zero-trust policy: %v", err)
		failClosedFindings := []proxyScanFinding{
			{
				File:        "git-proxy",
				Line:        1,
				Rule:        "SECURITY_GATE_OFFLINE",
				Scanner:     "kuro-proxy",
				Description: fmt.Sprintf("Kuro security pipeline is unreachable (%v) - push blocked under fail-closed zero-trust policy", err),
				Severity:    "CRITICAL",
			},
		}
		return true, failClosedFindings, err
	}
	if !blocked {
		c.cache.Store(key, cacheEntry{
			timestamp: time.Now(),
			blocked:   blocked,
			findings:  findings,
		})
	}
	return blocked, findings, nil
}

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

type ProxyHandler struct {
	Limiter RateLimiter
	Git     GitService
	Kuro    KuroAPIClient
}

func (h *ProxyHandler) handleProxy(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Handle info/refs (discovery phase)
	if strings.HasSuffix(path, "/info/refs") {
		proxyRequest(w, r)
		return
	}

	// Handle git-receive-pack (push phase)
	if strings.HasSuffix(path, "/git-receive-pack") && r.Method == "POST" {
		h.handleReceivePack(w, r)
		return
	}

	proxyRequest(w, r)
}

func (h *ProxyHandler) handleReceivePack(w http.ResponseWriter, r *http.Request) {
	repoPath := extractRepoPath(r.URL.Path)
	log.Printf("Receiving push to %s", repoPath)

	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	if !h.Limiter.Allow(ip, 5, time.Minute) {
		http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
		return
	}

	// Create temporary directory to extract the packfile
	id := fmt.Sprintf("push-%d", time.Now().UnixNano())
	tmpDir := filepath.Join(tmpBase, id)
	scanDir := filepath.Join(tmpDir, "scan")
	gitDir := filepath.Join(tmpDir, "repo.git")

	// Shared directory with the Worker (via SCAN_WORKDIR_HOST bind mount)
	scanHostDir := filepath.Join(scanHostBase, "proxy", id, "files")

	os.MkdirAll(scanDir, 0755)
	os.MkdirAll(scanHostDir, 0755)
	defer os.RemoveAll(tmpDir)

	// Initialize temporary bare repo
	if err := h.Git.InitBare(gitDir); err != nil {
		log.Printf("git init failed: %v", err)
		http.Error(w, "failed to initialize repository", http.StatusBadRequest)
		return
	}

	// Save full body to forward to GitHub if approved
	r.Body = http.MaxBytesReader(w, r.Body, 500<<20)
	fullBodyFile := filepath.Join(tmpDir, "full_body.bin")
	fullF, _ := os.Create(fullBodyFile)
	teeReader := io.TeeReader(r.Body, fullF)

	pktReader := &pktLineReader{r: teeReader}
	var refCmds []string

	// Read ref commands until flush packet (0000)
	for {
		pkt, err := pktReader.ReadPacket()
		if err != nil {
			log.Printf("Error reading pkt-line: %v", err)
			break
		}
		if pkt == nil {
			break // Flush packet
		}
		refCmds = append(refCmds, string(pkt))
	}

	// Collect every non-deletion ref. A single push may update several refs,
	// and each one must be scanned. Deletion refs (sha starts with 0000000)
	// are skipped but must not stop the loop.
	refs := parsePushRefs(refCmds)

	var commitSHA, refName string
	if len(refs) > 0 {
		commitSHA = refs[0].sha
		refName = refs[0].ref
	}

	packFile := filepath.Join(tmpDir, "push.pack")
	f, err := os.Create(packFile)
	if err != nil {
		http.Error(w, "failed to create packfile", http.StatusInternalServerError)
		return
	}
	_, err = io.Copy(f, r.Body)
	f.Close()
	// Append packfile to full body (teeReader only captured pkt-lines)
	packData, _ := os.ReadFile(packFile)
	fullF.Write(packData)
	fullF.Close()
	if err != nil {
		http.Error(w, "failed to read packfile", http.StatusBadRequest)
		return
	}

	// Unpack objects
	packF, err := os.Open(packFile)
	if err != nil {
		log.Printf("Failed to open packfile %s: %v", packFile, err)
		http.Error(w, "failed to open packfile", http.StatusInternalServerError)
		return
	}
	repoURL := strings.TrimRight(upstreamURL, "/") + "/" + repoPath
	if err := h.Git.UnpackObjects(gitDir, packF, repoURL); err != nil {
		packF.Close()
		log.Printf("git unpack-objects failed: %v", err)
		http.Error(w, "failed to unpack objects", http.StatusBadRequest)
		return
	}
	packF.Close()

	// Determine branch
	branch := ""
	if strings.HasPrefix(refName, "refs/heads/") {
		branch = strings.TrimPrefix(refName, "refs/heads/")
	}

	// Fall back to defaults if parsing failed
	if commitSHA == "" {
		commitSHA = h.Git.ResolveCommitSHA(gitDir)
	}
	if branch == "" {
		branch = h.Git.ResolveBranch(gitDir)
	}

	// Extract files to temporary scanDir using the resolved commitSHA
	if err := extractRefsForScan(h.Git, gitDir, scanDir, refs, commitSHA); err != nil {
		log.Printf("BLOCKED: %s - %v", repoPath, err)
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(w, "remote: KURO CORE - PUSH BLOCKED\n")
		fmt.Fprintf(w, "remote:   %v\n", err)
		fmt.Fprintf(w, "remote:\nremote: Unable to verify pushed refs. Fix and push again.\n")
		return
	}

	// Copy extracted files to the shared Worker directory
	if err := copyExtractedFiles(scanDir, scanHostDir); err != nil {
		log.Printf("failed to copy extracted files: %v", err)
		http.Error(w, "failed to copy extracted files", http.StatusInternalServerError)
		return
	}

	log.Printf("Files extracted to %s for push %s", scanHostDir, repoPath)

	// Multi-ref pushes are scanned as a single combined tree; use a combined
	// identity so the scan cache cannot approve a push whose secondary refs
	// changed but whose first ref is unchanged.
	if len(refs) > 1 {
		shas := make([]string, 0, len(refs))
		for _, r := range refs {
			shas = append(shas, r.sha)
		}
		commitSHA = strings.Join(shas, ",")
	}

	// Scan via Core local CLI (SCAN_MODE=local) or Enterprise API (SCAN_MODE=api).
	blocked, findings, _ := h.Kuro.Scan(scanHostDir, repoPath, commitSHA, branch)

	if blocked {
		log.Printf("BLOCKED: %s - %d findings", repoPath, len(findings))
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(w, "remote: KURO CORE - PUSH BLOCKED\n")
		for _, f := range findings {
			if f.Description != "" {
				fmt.Fprintf(w, "remote:   %s | %s | %s:%d - %s\n", f.Severity, f.Rule, f.File, f.Line, f.Description)
			} else {
				fmt.Fprintf(w, "remote:   %s | %s | %s:%d\n", f.Severity, f.Rule, f.File, f.Line)
			}
		}
		fmt.Fprintf(w, "remote:\nremote: Fix the issues above or run `kuro doctor` and push again.\n")
		return
	}

	log.Printf("APPROVED: %s", repoPath)

	// Forward full body (pkt-lines + packfile) to GitHub
	bodyStat, _ := os.Stat(fullBodyFile)
	f, err = os.Open(fullBodyFile)
	if err != nil {
		log.Printf("Failed to open full body %s: %v", fullBodyFile, err)
		http.Error(w, "failed to read body for forwarding", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	upstream := upstreamURL + "/" + repoPath + ".git/git-receive-pack"
	req, _ := http.NewRequestWithContext(r.Context(), "POST", upstream, f)
	for _, h := range []string{"Content-Type", "Transfer-Encoding"} {
		if v := r.Header.Values(h); len(v) > 0 {
			req.Header[h] = v
		}
	}
	req.ContentLength = bodyStat.Size()
	addAuth(req)

	resp, err := proxyClient.Do(req)
	if err != nil {
		http.Error(w, "upstream error: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for k, v := range resp.Header {
		w.Header()[k] = v
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func main() {
	os.MkdirAll(tmpBase, 0755)
	os.MkdirAll(scanHostBase, 0755)

	handler := &ProxyHandler{
		Limiter: newRateLimiter(),
		Git:     &defaultGitService{},
		Kuro:    selectScanClient(),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", handler.handleProxy)

	srv := &http.Server{
		Addr:    listenAddr,
		Handler: mux,
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("panic in shutdown handler: %v", r)
			}
		}()
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Printf("Shutting down Git Proxy...")
		srv.Shutdown(context.Background())
	}()

	log.Printf("Kuro Core Git Proxy on %s -> %s (scan=%s)", listenAddr, upstreamURL, getEnv("SCAN_MODE", "local"))
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
	log.Printf("Server stopped")
}

// copyExtractedFiles copies the extracted files to the shared directory
// with the Worker (bind mount via SCAN_WORKDIR_HOST).
func copyExtractedFiles(srcDir, dstDir string) error {
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dstDir, relPath)
		if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
			return err
		}

		src, err := os.Open(path)
		if err != nil {
			log.Printf("copyExtractedFiles: failed to open %s: %v", path, err)
			return err
		}

		dst, err := os.Create(dstPath)
		if err != nil {
			src.Close()
			log.Printf("copyExtractedFiles: failed to create %s: %v", dstPath, err)
			return err
		}

		_, err = io.Copy(dst, src)
		src.Close()
		dst.Close()
		if err != nil {
			log.Printf("copyExtractedFiles: failed to copy %s: %v", path, err)
			return err
		}
		return nil
	})
}

func proxyRequest(w http.ResponseWriter, r *http.Request) {
	upstream := upstreamURL + r.URL.Path
	if r.URL.RawQuery != "" {
		upstream += "?" + r.URL.RawQuery
	}

	req, _ := http.NewRequestWithContext(r.Context(), r.Method, upstream, r.Body)
	for _, h := range []string{"Content-Type", "Content-Length", "Transfer-Encoding"} {
		if v := r.Header.Values(h); len(v) > 0 {
			req.Header[h] = v
		}
	}
	addAuth(req)

	resp, err := proxyClient.Do(req)
	if err != nil {
		http.Error(w, "proxy error", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for k, v := range resp.Header {
		w.Header()[k] = v
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func addAuth(req *http.Request) {
	if gitHubToken != "" {
		if gitHubUser != "" {
			req.SetBasicAuth(gitHubUser, gitHubToken)
		} else {
			req.Header.Set("Authorization", "Bearer "+gitHubToken)
		}
	}
}

func extractRepoPath(urlPath string) string {
	p := strings.TrimSuffix(urlPath, "/git-receive-pack")
	p = strings.TrimSuffix(p, ".git")
	p = strings.TrimPrefix(p, "/")
	if strings.HasPrefix(p, "-") {
		return ""
	}
	return p
}

func runCmd(name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func runCmdWithStdin(stdinPath string, name string, args ...string) (string, error) {
	data, err := os.ReadFile(stdinPath)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = bytes.NewReader(data)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func runCmdWithReader(r io.Reader, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = r
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func runCmdOutput(name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, name, args...).Output()
}
