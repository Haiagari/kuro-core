package doctor


import (
	"encoding/json"
	"strings"
	"testing"
)

// ── Aggregator tests ─────────────────────────────────────────

func TestAggregate_AllPass(t *testing.T) {
	results := []CheckResult{
		{Name: "PostgreSQL", Status: StatusPass, Detail: "connected"},
		{Name: "NATS JetStream", Status: StatusPass, Detail: "connected"},
	}
	priorityMap := map[string]string{
		"PostgreSQL":     PriorityCritical,
		"NATS JetStream": PriorityCritical,
	}

	overall, code := aggregate(results, priorityMap)
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
	if overall != OverallPass {
		t.Errorf("expected %q, got %q", OverallPass, overall)
	}
}

func TestAggregate_CriticalFail(t *testing.T) {
	results := []CheckResult{
		{Name: "PostgreSQL", Status: StatusFail, Detail: "connection refused"},
		{Name: "NATS JetStream", Status: StatusPass, Detail: "connected"},
	}
	priorityMap := map[string]string{
		"PostgreSQL":     PriorityCritical,
		"NATS JetStream": PriorityCritical,
	}

	overall, code := aggregate(results, priorityMap)
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if overall != OverallDegraded {
		t.Errorf("expected %q, got %q", OverallDegraded, overall)
	}
}

func TestAggregate_WarningOnlyFail(t *testing.T) {
	results := []CheckResult{
		{Name: "PostgreSQL", Status: StatusPass, Detail: "connected"},
		{Name: "Scanner Images", Status: StatusWarn, Detail: "missing: semgrep"},
	}
	priorityMap := map[string]string{
		"PostgreSQL":     PriorityCritical,
		"Scanner Images": PriorityWarning,
	}

	overall, code := aggregate(results, priorityMap)
	if code != 0 {
		t.Errorf("expected exit code 0 (warning only), got %d", code)
	}
	if overall != OverallPass {
		t.Errorf("expected %q, got %q", OverallPass, overall)
	}
}

func TestAggregate_InfoOnlyFail(t *testing.T) {
	results := []CheckResult{
		{Name: "PostgreSQL", Status: StatusPass, Detail: "connected"},
		{Name: "Backup Status", Status: StatusWarn, Detail: "stale"},
	}
	priorityMap := map[string]string{
		"PostgreSQL":    PriorityCritical,
		"Backup Status": PriorityInfo,
	}

	overall, code := aggregate(results, priorityMap)
	if code != 0 {
		t.Errorf("expected exit code 0 (info only), got %d", code)
	}
	if overall != OverallPass {
		t.Errorf("expected %q, got %q", OverallPass, overall)
	}
}

func TestAggregate_MixedFail(t *testing.T) {
	results := []CheckResult{
		{Name: "PostgreSQL", Status: StatusFail, Detail: "down"},
		{Name: "NATS JetStream", Status: StatusFail, Detail: "down"},
		{Name: "Scanner Images", Status: StatusWarn, Detail: "missing"},
		{Name: "Backup Status", Status: StatusSkip, Detail: "skipped"},
	}
	priorityMap := map[string]string{
		"PostgreSQL":     PriorityCritical,
		"NATS JetStream": PriorityCritical,
		"Scanner Images": PriorityWarning,
		"Backup Status":  PriorityInfo,
	}

	overall, code := aggregate(results, priorityMap)
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if overall != OverallDegraded {
		t.Errorf("expected %q, got %q", OverallDegraded, overall)
	}
}

// ── Output formatting tests ───────────────────────────────────

func TestPrintTable_AllPass(t *testing.T) {
	results := []CheckResult{
		{Name: "PostgreSQL", Status: StatusPass, Detail: "TCP: connected"},
		{Name: "Scanner Images", Status: StatusWarn, Detail: "Missing: semgrep"},
		{Name: "Backup Status", Status: StatusSkip, Detail: "MinIO unavailable"},
	}
	var buf strings.Builder
	printTableTo(&buf, results, OverallPass)

	out := buf.String()
	if !strings.Contains(out, "PostgreSQL") {
		t.Error("table missing PostgreSQL")
	}
	if !strings.Contains(out, OverallPass) {
		t.Error("table missing overall status")
	}
	if !strings.Contains(out, "PASS") {
		t.Error("table missing PASS status")
	}
	if !strings.Contains(out, "WARN") {
		t.Error("table missing WARN status")
	}
	if !strings.Contains(out, "SKIP") {
		t.Error("table missing SKIP status")
	}
}

func TestPrintTable_Degraded(t *testing.T) {
	results := []CheckResult{
		{Name: "PostgreSQL", Status: StatusFail, Detail: "connection refused"},
	}
	var buf strings.Builder
	printTableTo(&buf, results, OverallDegraded)

	out := buf.String()
	if !strings.Contains(out, "FAIL") {
		t.Error("table missing FAIL status")
	}
	if !strings.Contains(out, OverallDegraded) {
		t.Error("table missing degraded status")
	}
}

func TestPrintJSON(t *testing.T) {
	results := []CheckResult{
		{Name: "PostgreSQL", Status: StatusPass, Detail: "connected"},
		{Name: "NATS", Status: StatusFail, Detail: "timeout"},
	}
	var buf strings.Builder
	printJSONTo(&buf, results, OverallDegraded, 1)

	var output DoctorOutput
	if err := json.Unmarshal([]byte(buf.String()), &output); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if len(output.Checks) != 2 {
		t.Errorf("expected 2 checks, got %d", len(output.Checks))
	}
	if output.Overall != OverallDegraded {
		t.Errorf("expected %q, got %q", OverallDegraded, output.Overall)
	}
	if output.ExitCode != 1 {
		t.Errorf("expected exit_code 1, got %d", output.ExitCode)
	}
	if output.Checks[0].Name != "PostgreSQL" || output.Checks[0].Status != StatusPass {
		t.Errorf("first check mismatch: %+v", output.Checks[0])
	}
}

func TestPrintJSON_AllPass(t *testing.T) {
	results := []CheckResult{
		{Name: "PostgreSQL", Status: StatusPass, Detail: "connected"},
	}
	var buf strings.Builder
	printJSONTo(&buf, results, OverallPass, 0)

	var output DoctorOutput
	if err := json.Unmarshal([]byte(buf.String()), &output); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if output.Overall != OverallPass {
		t.Errorf("expected %q, got %q", OverallPass, output.Overall)
	}
	if output.ExitCode != 0 {
		t.Errorf("expected exit_code 0, got %d", output.ExitCode)
	}
}

// ── Dependency resolution tests ───────────────────────────────

func TestResolveDependency_SkipsOnDepFailure(t *testing.T) {
	skip := resolveDependency(
		[]CheckResult{
			{Name: "Docker Proxy", Status: StatusFail, Detail: "unreachable"},
		},
		Check{Name: "Scanner Images", Priority: PriorityWarning, DependsOn: "Docker Proxy"},
	)
	if skip == nil {
		t.Fatal("expected skip result, got nil")
	}
	if skip.Status != StatusSkip {
		t.Errorf("expected skip status, got %s", skip.Status)
	}
	if !strings.Contains(skip.Detail, "Docker Proxy") {
		t.Errorf("expected detail referencing Docker Proxy, got: %s", skip.Detail)
	}
}

func TestResolveDependency_ProceedsOnDepPass(t *testing.T) {
	skip := resolveDependency(
		[]CheckResult{
			{Name: "Docker Proxy", Status: StatusPass, Detail: "ok"},
		},
		Check{Name: "Scanner Images", Priority: PriorityWarning, DependsOn: "Docker Proxy"},
	)
	if skip != nil {
		t.Errorf("expected nil (proceed), got skip: %+v", skip)
	}
}

func TestResolveDependency_MinIOFailSkipsBackup(t *testing.T) {
	skip := resolveDependency(
		[]CheckResult{
			{Name: "MinIO Storage", Status: StatusFail, Detail: "unreachable"},
		},
		Check{Name: "Backup Status", Priority: PriorityInfo, DependsOn: "MinIO Storage"},
	)
	if skip == nil {
		t.Fatal("expected skip result, got nil")
	}
	if skip.Status != StatusSkip {
		t.Errorf("expected skip status, got %s", skip.Status)
	}
}

func TestResolveDependency_NoDependency(t *testing.T) {
	skip := resolveDependency(
		[]CheckResult{},
		Check{Name: "PostgreSQL", Priority: PriorityCritical, DependsOn: ""},
	)
	if skip != nil {
		t.Errorf("expected nil for no dependency, got: %+v", skip)
	}
}

// ── Priority map builder ─────────────────────────────────

func TestPriorityMap(t *testing.T) {
	checks := []Check{
		{Name: "PostgreSQL", Priority: PriorityCritical},
		{Name: "Scanner Images", Priority: PriorityWarning},
		{Name: "Backup Status", Priority: PriorityInfo},
	}
	m := buildPriorityMap(checks)

	if m["PostgreSQL"] != PriorityCritical {
		t.Errorf("expected critical, got %s", m["PostgreSQL"])
	}
	if m["Scanner Images"] != PriorityWarning {
		t.Errorf("expected warning, got %s", m["Scanner Images"])
	}
	if m["Backup Status"] != PriorityInfo {
		t.Errorf("expected info, got %s", m["Backup Status"])
	}
	if _, ok := m["Nonexistent"]; ok {
		t.Error("unexpected key in map")
	}
}
