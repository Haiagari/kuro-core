package doctor


import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAggregate_AllPass(t *testing.T) {
	results := []CheckResult{
		{Name: "Container Runtime", Status: StatusPass, Detail: "docker ready"},
		{Name: "Git", Status: StatusPass, Detail: "git 2.40"},
	}
	priorityMap := map[string]string{
		"Container Runtime": PriorityCritical,
		"Git":               PriorityCritical,
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
		{Name: "Container Runtime", Status: StatusFail, Detail: "missing"},
		{Name: "Git", Status: StatusPass, Detail: "ok"},
	}
	priorityMap := map[string]string{
		"Container Runtime": PriorityCritical,
		"Git":               PriorityCritical,
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
		{Name: "Container Runtime", Status: StatusPass, Detail: "ok"},
		{Name: "Scanner Images", Status: StatusWarn, Detail: "missing: semgrep"},
	}
	priorityMap := map[string]string{
		"Container Runtime": PriorityCritical,
		"Scanner Images":    PriorityWarning,
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
		{Name: "Git", Status: StatusPass, Detail: "ok"},
		{Name: "Kuro Binary", Status: StatusWarn, Detail: "not on PATH"},
	}
	priorityMap := map[string]string{
		"Git":         PriorityCritical,
		"Kuro Binary": PriorityInfo,
	}

	overall, code := aggregate(results, priorityMap)
	if code != 0 {
		t.Errorf("expected exit code 0 (info only), got %d", code)
	}
	if overall != OverallPass {
		t.Errorf("expected %q, got %q", OverallPass, overall)
	}
}

func TestPrintTable_AllPass(t *testing.T) {
	results := []CheckResult{
		{Name: "Git", Status: StatusPass, Detail: "git 2.40"},
		{Name: "Scanner Images", Status: StatusWarn, Detail: "Missing: semgrep"},
		{Name: "Kuro Binary", Status: StatusSkip, Detail: "skipped"},
	}
	var buf strings.Builder
	printTableTo(&buf, results, OverallPass)

	out := buf.String()
	if !strings.Contains(out, "Kuro Core") {
		t.Error("table missing Kuro Core header")
	}
	if !strings.Contains(out, "Git") {
		t.Error("table missing Git")
	}
	if !strings.Contains(out, OverallPass) {
		t.Error("table missing overall status")
	}
}

func TestPrintJSON(t *testing.T) {
	results := []CheckResult{
		{Name: "Git", Status: StatusPass, Detail: "ok"},
		{Name: "Container Runtime", Status: StatusFail, Detail: "missing"},
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
	if output.ExitCode != 1 {
		t.Errorf("expected exit_code 1, got %d", output.ExitCode)
	}
}

func TestResolveDependency_SkipsOnDepFailure(t *testing.T) {
	skip := resolveDependency(
		[]CheckResult{
			{Name: "Container Runtime", Status: StatusFail, Detail: "unreachable"},
		},
		Check{Name: "Scanner Images", Priority: PriorityWarning, DependsOn: "Container Runtime"},
	)
	if skip == nil {
		t.Fatal("expected skip result, got nil")
	}
	if skip.Status != StatusSkip {
		t.Errorf("expected skip status, got %s", skip.Status)
	}
}

func TestResolveDependency_ProceedsOnDepPass(t *testing.T) {
	skip := resolveDependency(
		[]CheckResult{
			{Name: "Container Runtime", Status: StatusPass, Detail: "ok"},
		},
		Check{Name: "Scanner Images", Priority: PriorityWarning, DependsOn: "Container Runtime"},
	)
	if skip != nil {
		t.Errorf("expected nil (proceed), got skip: %+v", skip)
	}
}

func TestPriorityMap(t *testing.T) {
	checks := []Check{
		{Name: "Container Runtime", Priority: PriorityCritical},
		{Name: "Scanner Images", Priority: PriorityWarning},
		{Name: "Kuro Binary", Priority: PriorityInfo},
	}
	m := buildPriorityMap(checks)
	if m["Container Runtime"] != PriorityCritical {
		t.Errorf("expected critical, got %s", m["Container Runtime"])
	}
}

func TestAllChecks_CoreOnly(t *testing.T) {
	checks := allChecks()
	names := make(map[string]bool)
	for _, c := range checks {
		names[c.Name] = true
	}
	for _, forbidden := range []string{"PostgreSQL", "NATS JetStream", "MinIO Storage", "Docker Proxy", "Backup Status"} {
		if names[forbidden] {
			t.Errorf("Core doctor must not include Enterprise check %q", forbidden)
		}
	}
	for _, required := range []string{"Container Runtime", "Git", "Scanner Images", "Disk Space", "Kuro Binary"} {
		if !names[required] {
			t.Errorf("Core doctor missing check %q", required)
		}
	}
}
