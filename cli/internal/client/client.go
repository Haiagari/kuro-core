package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// defaultTimeout is the HTTP client timeout for all API calls.
const defaultTimeout = 30 * time.Second

// Client is an HTTP client for the Kuro API.
type Client struct {
	BaseURL    string
	APIKey     string
	httpClient *http.Client
}

// New creates a new Client with the given base URL and API key.
func New(baseURL, apiKey string) *Client {
	return &Client{
		BaseURL: baseURL,
		APIKey:  apiKey,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

// ScanResult represents the full scan detail response from the API.
type ScanResult struct {
	ScanID             string            `json:"scan_id"`
	Status             string            `json:"status"`
	Decision           string            `json:"decision"`
	PolicyDecision     string            `json:"policy_decision"`
	RepositoryURL      string            `json:"repository_url"`
	Branch             string            `json:"branch"`
	CreatedAt          string            `json:"created_at"`
	FinishedAt         *string           `json:"finished_at,omitempty"`
	Duration           int               `json:"duration"`
	FindingsBySeverity map[string]int    `json:"findings_by_severity"`
	TopFindings        []TopFindingItem  `json:"top_findings"`
}

// ── Webhook Types ────────────────────────────────────────────────────────────

// Webhook represents a notification webhook channel.
type Webhook struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	URL       string          `json:"url"`
	Type      string          `json:"type"`
	Events    []string        `json:"events"`
	Active    bool            `json:"active"`
	CreatedAt string          `json:"created_at"`
	Metadata  json.RawMessage `json:"metadata,omitempty"`
}

// WebhookInput is the payload for creating a webhook.
type WebhookInput struct {
	Name     string          `json:"name"`
	URL      string          `json:"url"`
	Type     string          `json:"type"`
	Events   []string        `json:"events,omitempty"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

// ── Webhook Methods ──────────────────────────────────────────────────────────

// ListWebhooks returns all configured notification channels.
func (c *Client) ListWebhooks() ([]Webhook, error) {
	var webhooks []Webhook
	if err := c.doRequest("GET", "/admin/webhooks", nil, &webhooks); err != nil {
		return nil, err
	}
	return webhooks, nil
}

// CreateWebhook creates a new notification channel.
func (c *Client) CreateWebhook(input WebhookInput) (*Webhook, error) {
	var wh Webhook
	if err := c.doRequest("POST", "/admin/webhooks", &input, &wh); err != nil {
		return nil, err
	}
	return &wh, nil
}

// DeleteWebhook deletes a notification channel by ID.
func (c *Client) DeleteWebhook(id string) error {
	return c.doRequest("DELETE", "/admin/webhooks/"+id, nil, nil)
}

// ToggleWebhook activates/deactivates a webhook by ID.
func (c *Client) ToggleWebhook(id string) (bool, error) {
	var resp struct {
		ID     string `json:"id"`
		Active bool   `json:"active"`
	}
	if err := c.doRequest("PATCH", "/admin/webhooks/"+id+"/toggle", nil, &resp); err != nil {
		return false, err
	}
	return resp.Active, nil
}

// TopFindingItem represents a single finding in the top_findings list.
type TopFindingItem struct {
	Scanner  string `json:"scanner"`
	Severity string `json:"severity"`
	Title    string `json:"title"`
	File     string `json:"file"`
	Line     int    `json:"line"`
}

// TriggerScanRequest is sent to POST /api/scans/trigger.
type TriggerScanRequest struct {
	RepositoryURL string `json:"repository_url"`
	Branch        string `json:"branch"`
	Reference     string `json:"reference"`
}

// TriggerScanResponse is returned by POST /api/scans/trigger.
type TriggerScanResponse struct {
	ScanID string `json:"scan_id"`
	Status string `json:"status"`
}

// doRequest performs an HTTP request and decodes the JSON response.
func (c *Client) doRequest(method, path string, body, dst any) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.BaseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("X-API-Key", c.APIKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("cannot reach Kuro API at %s: %w", c.BaseURL, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return statusError(resp.StatusCode, string(respBody))
	}

	if dst != nil {
		if err := json.Unmarshal(respBody, dst); err != nil {
			return fmt.Errorf("invalid API response: %w", err)
		}
	}

	return nil
}

// statusError returns a human-readable error for non-2xx HTTP status codes.
func statusError(code int, body string) error {
	switch code {
	case http.StatusUnauthorized:
		return fmt.Errorf("invalid API key. Run 'kuro auth <key>' to update")
	case http.StatusForbidden:
		return fmt.Errorf("insufficient permissions for this action")
	case http.StatusNotFound:
		return fmt.Errorf("resource not found")
	case http.StatusBadRequest:
		// Try to extract error message from JSON body
		var errResp struct {
			Error string `json:"error"`
		}
		if json.Unmarshal([]byte(body), &errResp) == nil && errResp.Error != "" {
			return fmt.Errorf("%s", errResp.Error)
		}
		return fmt.Errorf("bad request")
	default:
		return fmt.Errorf("API returned %d: %s", code, body)
	}
}

// TriggerScan sends a POST /scans/trigger request.
func (c *Client) TriggerScan(repoURL, branch string) (*TriggerScanResponse, error) {
	req := TriggerScanRequest{
		RepositoryURL: repoURL,
		Branch:        branch,
		Reference:     "",
	}
	var resp TriggerScanResponse
	if err := c.doRequest("POST", "/scans/trigger", &req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetScanStatus fetches the current scan status from GET /scans/{id}.
func (c *Client) GetScanStatus(scanID string) (*ScanResult, error) {
	var resp ScanResult
	if err := c.doRequest("GET", "/scans/"+scanID, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// HealthCheck performs a GET /health request and returns true if the API is reachable.
func (c *Client) HealthCheck() bool {
	var resp interface{}
	err := c.doRequest("GET", "/health", nil, &resp)
	return err == nil
}
