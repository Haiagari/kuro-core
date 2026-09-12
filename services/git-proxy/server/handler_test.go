package server

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type allowAllRateLimiter struct{}

func (allowAllRateLimiter) Allow(string, int, time.Duration) bool {
	return true
}

type handlerScanClient struct {
	blocked  bool
	findings []proxyScanFinding
	err      error
}

func (c *handlerScanClient) Scan(string, string, string, string) (bool, []proxyScanFinding, error) {
	return c.blocked, c.findings, c.err
}

func TestHandleReceivePackScanOutcomes(t *testing.T) {
	oldTmpBase, oldScanHostBase, oldUpstreamURL := tmpBase, scanHostBase, upstreamURL
	tmpBase = t.TempDir()
	scanHostBase = t.TempDir()
	defer func() {
		tmpBase, scanHostBase, upstreamURL = oldTmpBase, oldScanHostBase, oldUpstreamURL
	}()

	upstreamCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("upstream accepted"))
	}))
	defer upstream.Close()
	upstreamURL = upstream.URL

	tests := []struct {
		name          string
		client        handlerScanClient
		wantStatus    int
		wantForwarded bool
		wantResponse  string
	}{
		{
			name:          "scan error blocks without forwarding",
			client:        handlerScanClient{err: errors.New("scanner unavailable")},
			wantStatus:    http.StatusForbidden,
			wantForwarded: false,
			wantResponse:  "Unable to verify pushed refs",
		},
		{
			name: "blocked scan preserves blocked response",
			client: handlerScanClient{
				blocked:  true,
				findings: []proxyScanFinding{{Rule: "secret-found", File: "config.env", Line: 4}},
			},
			wantStatus:    http.StatusForbidden,
			wantForwarded: false,
			wantResponse:  "secret-found",
		},
		{
			name:          "passing scan forwards upstream",
			client:        handlerScanClient{},
			wantStatus:    http.StatusCreated,
			wantForwarded: true,
			wantResponse:  "upstream accepted",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstreamCalls = 0
			h := &ProxyHandler{
				Limiter: allowAllRateLimiter{},
				Git:     &fakeGitService{},
				Kuro:    &tt.client,
			}
			req := httptest.NewRequest(http.MethodPost, "/owner/repo/git-receive-pack", strings.NewReader("0000"))
			response := httptest.NewRecorder()

			h.handleReceivePack(response, req)

			if response.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body = %q", response.Code, tt.wantStatus, response.Body.String())
			}
			if forwarded := upstreamCalls > 0; forwarded != tt.wantForwarded {
				t.Fatalf("forwarded = %v, want %v", forwarded, tt.wantForwarded)
			}
			if !strings.Contains(response.Body.String(), tt.wantResponse) {
				t.Fatalf("response body = %q, want it to contain %q", response.Body.String(), tt.wantResponse)
			}
		})
	}
}
