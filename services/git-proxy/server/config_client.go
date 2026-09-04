package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
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
