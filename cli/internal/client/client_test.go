package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_HealthCheck(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("X-API-Key") != "test-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	c := New(server.URL, "test-key")
	if !c.HealthCheck() {
		t.Fatal("expected HealthCheck to return true")
	}

	badClient := New("https://invalid.localhost:99999", "bad")
	if badClient.HealthCheck() {
		t.Fatal("expected HealthCheck on invalid endpoint to return false")
	}
}

func TestClient_TriggerScanAndGetStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "POST" && r.URL.Path == "/scans/trigger":
			var req TriggerScanRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"scan_id":"scan-123","status":"running"}`))

		case r.Method == "GET" && r.URL.Path == "/scans/scan-123":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"scan_id":"scan-123",
				"status":"completed",
				"decision":"pass",
				"findings_by_severity":{"CRITICAL":0,"HIGH":0}
			}`))

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	c := New(server.URL, "test-key")

	trig, err := c.TriggerScan("https://github.com/org/repo", "main")
	if err != nil {
		t.Fatalf("TriggerScan failed: %v", err)
	}
	if trig.ScanID != "scan-123" || trig.Status != "running" {
		t.Fatalf("unexpected trigger response: %+v", trig)
	}

	status, err := c.GetScanStatus("scan-123")
	if err != nil {
		t.Fatalf("GetScanStatus failed: %v", err)
	}
	if status.ScanID != "scan-123" || status.Decision != "pass" {
		t.Fatalf("unexpected scan status: %+v", status)
	}
}

func TestClient_WebhooksCRUD(t *testing.T) {
	webhooks := []Webhook{
		{ID: "wh-1", Name: "slack-prod", URL: "https://hooks.slack.com/1", Type: "slack", Active: true},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.Method == "GET" && r.URL.Path == "/admin/webhooks":
			_ = json.NewEncoder(w).Encode(webhooks)

		case r.Method == "POST" && r.URL.Path == "/admin/webhooks":
			var in WebhookInput
			if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			wh := Webhook{
				ID:     "wh-2",
				Name:   in.Name,
				URL:    in.URL,
				Type:   in.Type,
				Active: true,
			}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(wh)

		case r.Method == "PATCH" && r.URL.Path == "/admin/webhooks/wh-1/toggle":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "wh-1", "active": false})

		case r.Method == "DELETE" && r.URL.Path == "/admin/webhooks/wh-1":
			w.WriteHeader(http.StatusNoContent)

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	c := New(server.URL, "test-key")

	list, err := c.ListWebhooks()
	if err != nil {
		t.Fatalf("ListWebhooks failed: %v", err)
	}
	if len(list) != 1 || list[0].ID != "wh-1" {
		t.Fatalf("unexpected list: %+v", list)
	}

	created, err := c.CreateWebhook(WebhookInput{
		Name: "teams-alert",
		URL:  "https://webhook.office.com/2",
		Type: "teams",
	})
	if err != nil {
		t.Fatalf("CreateWebhook failed: %v", err)
	}
	if created.ID != "wh-2" || created.Name != "teams-alert" {
		t.Fatalf("unexpected created webhook: %+v", created)
	}

	active, err := c.ToggleWebhook("wh-1")
	if err != nil {
		t.Fatalf("ToggleWebhook failed: %v", err)
	}
	if active != false {
		t.Fatalf("expected active to be false after toggle, got %v", active)
	}

	if err := c.DeleteWebhook("wh-1"); err != nil {
		t.Fatalf("DeleteWebhook failed: %v", err)
	}
}

func TestClient_StatusErrorHandling(t *testing.T) {
	tests := []struct {
		statusCode int
		body       string
		errSubstr  string
	}{
		{http.StatusUnauthorized, "{}", "invalid API key"},
		{http.StatusForbidden, "{}", "insufficient permissions"},
		{http.StatusNotFound, "{}", "resource not found"},
		{http.StatusBadRequest, `{"error":"missing field foo"}`, "missing field foo"},
		{http.StatusBadRequest, `invalid-json`, "bad request"},
		{http.StatusInternalServerError, "internal server crash", "API returned 500: internal server crash"},
	}

	for _, tc := range tests {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.statusCode)
			_, _ = w.Write([]byte(tc.body))
		}))

		c := New(server.URL, "test-key")
		_, err := c.GetScanStatus("any")
		server.Close()

		if err == nil {
			t.Fatalf("expected error for code %d, got nil", tc.statusCode)
		}
	}
}
