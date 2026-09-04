package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// mockFaultyServer creates an HTTP test server that simulates various network & server faults.
func mockFaultyServer(faultType string, delay time.Duration) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if delay > 0 {
			time.Sleep(delay)
		}

		switch faultType {
		case "timeout":
			time.Sleep(2 * time.Second)
			w.WriteHeader(http.StatusGatewayTimeout)
		case "500_internal":
			http.Error(w, "Internal Server Error: scanner worker panic", http.StatusInternalServerError)
		case "502_bad_gateway":
			http.Error(w, "Bad Gateway: NATS JetStream disconnected", http.StatusBadGateway)
		case "503_service_unavailable":
			http.Error(w, "Service Unavailable: all scanner workers busy", http.StatusServiceUnavailable)
		case "504_gateway_timeout":
			http.Error(w, "Gateway Timeout: Semgrep scan exceeded 30s limit", http.StatusGatewayTimeout)
		case "malformed_json":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"decision": "BLOCKED", "findings": [{"file": "main.go", `)) // Truncated JSON
		case "corrupt_data_types":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"decision": 12345, "findings": "not_an_array"}`))
		case "empty_200":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(``))
		case "blocked_findings":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			resp := proxyScanResponse{
				Decision: "BLOCKED",
				Findings: []proxyScanFinding{
					{
						File:        "config.env",
						Line:        14,
						Rule:        "AWS_SECRET_KEY",
						Scanner:     "gitleaks",
						Description: "Hardcoded AWS Secret Access Key detected",
						Severity:    "CRITICAL",
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
		case "clean_pass":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			resp := proxyScanResponse{
				Decision: "PASSED",
				Findings: []proxyScanFinding{},
			}
			json.NewEncoder(w).Encode(resp)
		default:
			http.Error(w, "Unknown fault", http.StatusInternalServerError)
		}
	}))
}

// TestChaos_APIConnectionRefused verifies that when Kuro API is unreachable,
// the proxy strictly fails CLOSED and returns a CRITICAL finding.
func TestChaos_APIConnectionRefused(t *testing.T) {
	os.Setenv("PROXY_FAIL_MODE", "closed")
	os.Setenv("KURO_URL", "http://127.0.0.1:59999") // Unreachable port
	defer os.Unsetenv("KURO_URL")
	defer os.Unsetenv("PROXY_FAIL_MODE")

	client := newCachedKuroClient(&defaultKuroClient{})
	blocked, findings, err := client.Scan("/tmp/test", "owner/repo", "sha123", "main")

	if err == nil {
		t.Fatal("expected connection error, got nil")
	}
	if !blocked {
		t.Fatal("expected push to be BLOCKED under fail-closed policy, got allowed")
	}
	if len(findings) == 0 {
		t.Fatal("expected at least 1 fail-closed finding, got none")
	}
	if findings[0].Rule != "SECURITY_GATE_OFFLINE" {
		t.Fatalf("expected rule SECURITY_GATE_OFFLINE, got %q", findings[0].Rule)
	}
	if findings[0].Severity != "CRITICAL" {
		t.Fatalf("expected severity CRITICAL, got %q", findings[0].Severity)
	}
}

// TestChaos_APITimeout verifies that when Kuro API hangs, the client timeout
// triggers and fails CLOSED.
func TestChaos_APITimeout(t *testing.T) {
	os.Setenv("PROXY_FAIL_MODE", "closed")
	defer os.Unsetenv("PROXY_FAIL_MODE")

	server := mockFaultyServer("timeout", 0)
	defer server.Close()

	os.Setenv("KURO_URL", server.URL)
	defer os.Unsetenv("KURO_URL")

	// Use custom client with short timeout for chaos test
	faultClient := &chaosKuroClient{
		baseURL: server.URL,
		timeout: 50 * time.Millisecond,
	}
	cached := newCachedKuroClient(faultClient)

	start := time.Now()
	blocked, findings, err := cached.Scan("/tmp/test", "owner/repo", "sha-timeout", "main")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !blocked {
		t.Fatal("expected push to be BLOCKED on timeout, got allowed")
	}
	if len(findings) == 0 {
		t.Fatal("expected fail-closed finding on timeout")
	}
	if elapsed > 1*time.Second {
		t.Fatalf("test took %v, expected fast timeout failure", elapsed)
	}
}

// TestChaos_APIServerErrors verifies all 5xx HTTP status codes trigger Fail-Closed.
func TestChaos_APIServerErrors(t *testing.T) {
	faults := []string{"500_internal", "502_bad_gateway", "503_service_unavailable", "504_gateway_timeout"}

	for _, fault := range faults {
		t.Run(fault, func(t *testing.T) {
			os.Setenv("PROXY_FAIL_MODE", "closed")
			defer os.Unsetenv("PROXY_FAIL_MODE")

			server := mockFaultyServer(fault, 0)
			defer server.Close()

			os.Setenv("KURO_URL", server.URL)
			defer os.Unsetenv("KURO_URL")

			cached := newCachedKuroClient(&defaultKuroClient{})
			blocked, findings, err := cached.Scan("/tmp/test", "owner/repo", "sha-"+fault, "main")

			if err == nil {
				t.Fatalf("[%s] expected error, got nil", fault)
			}
			if !blocked {
				t.Fatalf("[%s] expected push to be BLOCKED, got allowed", fault)
			}
			if len(findings) == 0 {
				t.Fatalf("[%s] expected fail-closed finding", fault)
			}
			if findings[0].Rule != "SECURITY_GATE_OFFLINE" {
				t.Fatalf("[%s] expected rule SECURITY_GATE_OFFLINE, got %s", fault, findings[0].Rule)
			}
		})
	}
}

// TestChaos_CorruptAndMalformedResponses tests handling of broken payloads.
func TestChaos_CorruptAndMalformedResponses(t *testing.T) {
	payloadFaults := []string{"malformed_json", "corrupt_data_types", "empty_200"}

	for _, fault := range payloadFaults {
		t.Run(fault, func(t *testing.T) {
			os.Setenv("PROXY_FAIL_MODE", "closed")
			defer os.Unsetenv("PROXY_FAIL_MODE")

			server := mockFaultyServer(fault, 0)
			defer server.Close()

			os.Setenv("KURO_URL", server.URL)
			defer os.Unsetenv("KURO_URL")

			cached := newCachedKuroClient(&defaultKuroClient{})
			blocked, findings, err := cached.Scan("/tmp/test", "owner/repo", "sha-corrupt-"+fault, "main")

			if err == nil {
				t.Fatalf("[%s] expected decode error, got nil", fault)
			}
			if !blocked {
				t.Fatalf("[%s] expected push to be BLOCKED on corrupt payload, got allowed", fault)
			}
			if len(findings) == 0 {
				t.Fatalf("[%s] expected fail-closed finding", fault)
			}
		})
	}
}

// TestChaos_FlappingAPIRecovery ensures the cache is never polluted with fail-closed errors
// and recovers immediately once the API becomes healthy.
func TestChaos_FlappingAPIRecovery(t *testing.T) {
	os.Setenv("PROXY_FAIL_MODE", "closed")
	defer os.Unsetenv("PROXY_FAIL_MODE")

	var isHealthy atomic.Bool
	isHealthy.Store(false)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isHealthy.Load() {
			http.Error(w, "NATS down", http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(proxyScanResponse{
			Decision: "PASSED",
			Findings: []proxyScanFinding{},
		})
	}))
	defer server.Close()

	os.Setenv("KURO_URL", server.URL)
	defer os.Unsetenv("KURO_URL")

	cached := newCachedKuroClient(&defaultKuroClient{})
	repo := "org/flapping-repo"
	commit := "sha-flapping-001"

	// 1. First scan while unhealthy -> MUST FAIL CLOSED
	blocked1, findings1, err1 := cached.Scan("/tmp/test", repo, commit, "main")
	if err1 == nil || !blocked1 || len(findings1) == 0 {
		t.Fatalf("step 1 failed: expected blocked fail-closed, got blocked=%v err=%v", blocked1, err1)
	}

	// 2. Server recovers -> MUST PASS (proving errors were NOT cached)
	isHealthy.Store(true)

	blocked2, findings2, err2 := cached.Scan("/tmp/test", repo, commit, "main")
	if err2 != nil {
		t.Fatalf("step 2 failed: expected clean pass after recovery, got error: %v", err2)
	}
	if blocked2 {
		t.Fatalf("step 2 failed: expected approved pass after recovery, got blocked=true")
	}
	if len(findings2) != 0 {
		t.Fatalf("step 2 failed: expected 0 findings, got %d", len(findings2))
	}

	// 3. Third scan -> Cache Hit for the clean pass
	blocked3, findings3, err3 := cached.Scan("/tmp/test", repo, commit, "main")
	if err3 != nil || blocked3 || len(findings3) != 0 {
		t.Fatalf("step 3 failed: expected cache hit pass, got blocked=%v err=%v", blocked3, err3)
	}
}

// TestChaos_ConcurrentPushesUnderChaos simulates 50 concurrent pushes during active network flapping.
func TestChaos_ConcurrentPushesUnderChaos(t *testing.T) {
	os.Setenv("PROXY_FAIL_MODE", "closed")
	defer os.Unsetenv("PROXY_FAIL_MODE")

	var reqCount atomic.Int64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := reqCount.Add(1)
		// Alternate between healthy pass and 500 error
		if count%2 == 0 {
			http.Error(w, "Chaos crash", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(proxyScanResponse{
			Decision: "PASSED",
			Findings: []proxyScanFinding{},
		})
	}))
	defer server.Close()

	os.Setenv("KURO_URL", server.URL)
	defer os.Unsetenv("KURO_URL")

	cached := newCachedKuroClient(&defaultKuroClient{})

	var wg sync.WaitGroup
	numWorkers := 30

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			commit := fmt.Sprintf("commit-%d", workerID)
			blocked, findings, err := cached.Scan("/tmp/test", "owner/chaos-repo", commit, "main")

			// In all cases, verdict must be strictly consistent:
			// If err != nil -> blocked MUST be true (fail-closed)
			// If err == nil -> blocked is false (passed)
			if err != nil {
				if !blocked {
					t.Errorf("worker %d: had error %v but blocked was false!", workerID, err)
				}
				if len(findings) == 0 {
					t.Errorf("worker %d: had error but 0 findings returned", workerID)
				}
			} else {
				if blocked {
					t.Errorf("worker %d: clean response but blocked=true", workerID)
				}
			}
		}(i)
	}

	wg.Wait()
}

// chaosKuroClient implements KuroAPIClient with configurable timeout for chaos testing.
type chaosKuroClient struct {
	baseURL string
	timeout time.Duration
}

func (c *chaosKuroClient) Scan(dir, repo, commit, branch string) (bool, []proxyScanFinding, error) {
	client := &http.Client{Timeout: c.timeout}
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/v1/scans/proxy", nil)
	if err != nil {
		return false, nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return false, nil, fmt.Errorf("chaos request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	var result proxyScanResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, nil, err
	}

	return result.Decision == "BLOCKED", result.Findings, nil
}
