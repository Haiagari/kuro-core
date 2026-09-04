package orchestrator

import (
	"strings"
	"testing"
)

func TestSemgrepCoreRulesEmbedded(t *testing.T) {
	if len(semgrepCoreRules) == 0 {
		t.Fatal("semgrepCoreRules embed is empty")
	}
	body := string(semgrepCoreRules)
	if !strings.Contains(body, "rules:") {
		t.Fatal("embedded rules missing top-level rules: key")
	}
	if !strings.Contains(body, "kuro.generic.hardcoded-credential") {
		t.Fatal("embedded rules missing hardcoded-credential rule id")
	}
	// Ensure we did not accidentally ship a network-dependent auto config stub
	if strings.Contains(body, "config=auto") {
		t.Fatal("embedded rules must not reference config=auto")
	}
}
