package server

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func loadConfig() {
	upstreamURL = getEnv("UPSTREAM_URL", "https://github.com")
	listenAddr = getEnv("LISTEN_ADDR", ":8000")
	tmpBase = getEnv("TMP_DIR", "/tmp/kuro-proxy")
	scanHostBase = getEnv("SCAN_WORKDIR_HOST", "/tmp/kuro-scans")
	gitHubUser = os.Getenv("GITHUB_USER")
	gitHubToken = os.Getenv("GITHUB_TOKEN")
}

// Run starts the fail-closed Git proxy HTTP server and blocks until ctx is
// cancelled or ListenAndServe fails. It respects LISTEN_ADDR, UPSTREAM_URL,
// SCAN_MODE, KURO_BIN, PROXY_FAIL_MODE, TMP_DIR, SCAN_WORKDIR_HOST, and
// related environment variables.
func Run(ctx context.Context) error {
	loadConfig()

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

	errCh := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				errCh <- fmt.Errorf("panic in ListenAndServe: %v", r)
			}
		}()
		log.Printf("Kuro Core Git Proxy on %s -> %s (scan=%s)", listenAddr, upstreamURL, getEnv("SCAN_MODE", "local"))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		log.Printf("Shutting down Git Proxy...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		<-errCh
		log.Printf("Server stopped")
		return nil
	case err := <-errCh:
		return err
	}
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
