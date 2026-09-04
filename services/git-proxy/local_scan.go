package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// localCLIClient runs Core's local `kuro scan --json` instead of the Enterprise API.
type localCLIClient struct {
	kuroBin string
}

func newLocalCLIClient() *localCLIClient {
	bin := getEnv("KURO_BIN", "kuro")
	return &localCLIClient{kuroBin: bin}
}

func (c *localCLIClient) Scan(dir, repo, commit, branch string) (bool, []proxyScanFinding, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.kuroBin, "scan", dir, "--json")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	out := stdout.Bytes()
	if len(bytes.TrimSpace(out)) == 0 {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" && err != nil {
			msg = err.Error()
		}
		return false, nil, fmt.Errorf("kuro scan produced no JSON output: %s", msg)
	}

	return parseScanJSON(out)
}

// parseScanJSON maps kuro scan --json output to a proxy block decision.
// pass/approved/ok → not blocked; block/blocked/review/deny → blocked.
func parseScanJSON(out []byte) (blocked bool, findings []proxyScanFinding, err error) {
	var result struct {
		Decision string `json:"decision"`
		Status   string `json:"status"`
		Error    string `json:"error"`
		Findings []struct {
			Scanner  string `json:"scanner"`
			Severity string `json:"severity"`
			Title    string `json:"title"`
			File     string `json:"file"`
			Line     int    `json:"line"`
		} `json:"findings"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return false, nil, fmt.Errorf("parse kuro scan JSON: %w (stdout=%q)", err, truncate(string(out), 200))
	}

	findings = make([]proxyScanFinding, 0, len(result.Findings))
	for _, f := range result.Findings {
		findings = append(findings, proxyScanFinding{
			File:        f.File,
			Line:        f.Line,
			Rule:        f.Title,
			Scanner:     f.Scanner,
			Description: f.Title,
			Severity:    f.Severity,
		})
	}

	decision := strings.ToLower(strings.TrimSpace(result.Decision))
	switch decision {
	case "pass", "approved", "ok":
		return false, findings, nil
	case "block", "blocked", "review", "deny":
		return true, findings, nil
	default:
		if result.Error != "" {
			return false, nil, fmt.Errorf("kuro scan error: %s", result.Error)
		}
		// Unknown decision with findings → fail-closed block
		if len(findings) > 0 {
			return true, findings, nil
		}
		return false, nil, fmt.Errorf("kuro scan returned unknown decision %q", result.Decision)
	}
}

func selectScanClient() KuroAPIClient {
	mode := strings.ToLower(getEnv("SCAN_MODE", "local"))
	switch mode {
	case "api", "remote", "enterprise":
		return newCachedKuroClient(&defaultKuroClient{})
	default:
		bin := getEnv("KURO_BIN", "kuro")
		if _, err := exec.LookPath(bin); err != nil {
			// Fall back to common local build path relative to CWD
			if _, err2 := os.Stat("bin/kuro"); err2 == nil {
				bin = "bin/kuro"
			}
		}
		return newCachedKuroClient(&localCLIClient{kuroBin: bin})
	}
}
