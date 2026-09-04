package server

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

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
