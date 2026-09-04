package main

import (
	"strings"
	"testing"
)

func TestParseScanJSON_PassNotBlocked(t *testing.T) {
	out := []byte(`{"decision":"pass","findings":[]}`)
	blocked, findings, err := parseScanJSON(out)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if blocked {
		t.Fatalf("pass should not be blocked")
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestParseScanJSON_ApprovedAndOk(t *testing.T) {
	for _, d := range []string{"approved", "ok", "PASS", " Approved "} {
		out := []byte(`{"decision":"` + d + `","findings":[]}`)
		blocked, _, err := parseScanJSON(out)
		if err != nil {
			t.Fatalf("decision %q: %v", d, err)
		}
		if blocked {
			t.Fatalf("decision %q should not be blocked", d)
		}
	}
}

func TestParseScanJSON_BlockAndReviewBlocked(t *testing.T) {
	cases := []string{"block", "blocked", "review", "deny", "BLOCK", "Review"}
	for _, d := range cases {
		out := []byte(`{
			"decision":"` + d + `",
			"findings":[{"scanner":"gitleaks","severity":"CRITICAL","title":"aws-key","file":"a.go","line":3}]
		}`)
		blocked, findings, err := parseScanJSON(out)
		if err != nil {
			t.Fatalf("decision %q: %v", d, err)
		}
		if !blocked {
			t.Fatalf("decision %q should be blocked", d)
		}
		if len(findings) != 1 {
			t.Fatalf("decision %q: want 1 finding, got %d", d, len(findings))
		}
		if findings[0].Rule != "aws-key" || findings[0].File != "a.go" || findings[0].Line != 3 {
			t.Fatalf("decision %q: unexpected finding %#v", d, findings[0])
		}
	}
}

func TestParseScanJSON_InvalidJSON(t *testing.T) {
	_, _, err := parseScanJSON([]byte(`not-json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseScanJSON_ErrorField(t *testing.T) {
	out := []byte(`{"decision":"","error":"scanner boom"}`)
	blocked, findings, err := parseScanJSON(out)
	if err == nil {
		t.Fatal("expected error when error field set")
	}
	if blocked {
		t.Fatal("should not report blocked on parse/error path")
	}
	if findings != nil {
		t.Fatalf("expected nil findings, got %#v", findings)
	}
	if !strings.Contains(err.Error(), "scanner boom") {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestParseScanJSON_UnknownWithFindingsBlocks(t *testing.T) {
	out := []byte(`{"decision":"maybe","findings":[{"scanner":"x","severity":"HIGH","title":"t","file":"f","line":1}]}`)
	blocked, findings, err := parseScanJSON(out)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !blocked {
		t.Fatal("unknown decision with findings should fail-closed block")
	}
	if len(findings) != 1 {
		t.Fatalf("want 1 finding, got %d", len(findings))
	}
}

func TestParseScanJSON_UnknownWithoutFindingsErrors(t *testing.T) {
	out := []byte(`{"decision":"maybe","findings":[]}`)
	blocked, _, err := parseScanJSON(out)
	if err == nil {
		t.Fatal("expected error for unknown decision with no findings")
	}
	if blocked {
		t.Fatal("should not be blocked when returning error")
	}
}
