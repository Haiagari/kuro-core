package doctor


import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

// ── Terminal colors ─────────────────────────────────────────

const (
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
	colorReset  = "\033[0m"
)

// ── Status constants ─────────────────────────────────────────

const (
	StatusPass = "pass"
	StatusWarn = "warn"
	StatusFail = "fail"
	StatusSkip = "skip"
)

// ── Priority constants ───────────────────────────────────────

const (
	PriorityCritical = "critical"
	PriorityWarning  = "warning"
	PriorityInfo     = "info"
)

// ── Overall status constants ─────────────────────────────────

const (
	OverallPass     = "ALL SYSTEMS OPERATIONAL"
	OverallDegraded = "SERVICES DEGRADED"
)

// CheckResult holds the result of a single diagnostic check.
type CheckResult struct {
	Name   string `json:"name"`
	Status string `json:"status"` // pass | warn | fail | skip
	Detail string `json:"detail"`
}

// Check defines a single diagnostic check.
type Check struct {
	Name     string
	Priority string
	DependsOn string       // name of the check this depends on (empty if none)
	Run      func() CheckResult
}

// DoctorOutput is the JSON serialization structure.
type DoctorOutput struct {
	Checks   []CheckResult `json:"checks"`
	Overall  string        `json:"overall"`
	ExitCode int           `json:"exit_code"`
}

// RunDoctor is the entry point — parses args, runs checks, prints output.
func RunDoctor(args []string) {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	jsonFlag := fs.Bool("json", false, "Output in JSON format")

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	results, exitCode := runChecks()

	if *jsonFlag {
		printJSONTo(os.Stdout, results, overallFromCode(exitCode), exitCode)
	} else {
		printTableTo(os.Stdout, results, overallFromCode(exitCode))
	}

	os.Exit(exitCode)
}

// overallFromCode returns the overall status string from exit code.
func overallFromCode(code int) string {
	if code == 0 {
		return OverallPass
	}
	return OverallDegraded
}

// runChecks executes all checks sequentially with dependency resolution.
func runChecks() ([]CheckResult, int) {
	checks := allChecks()
	priorityMap := buildPriorityMap(checks)
	var results []CheckResult
	resultMap := make(map[string]CheckResult)

	for _, c := range checks {
		// Check dependency
		if c.DependsOn != "" {
			if dep, ok := resultMap[c.DependsOn]; ok && dep.Status == StatusFail {
				results = append(results, CheckResult{
					Name:   c.Name,
					Status: StatusSkip,
					Detail: "Skipped: " + c.DependsOn + " is unavailable",
				})
				continue
			}
		}

		result := c.Run()
		result.Name = c.Name
		results = append(results, result)
		resultMap[c.Name] = result
	}

	_, code := aggregate(results, priorityMap)
	return results, code
}

// allChecks returns the list of all diagnostic checks in order.
func allChecks() []Check {
	return []Check{
		{Name: "PostgreSQL", Priority: PriorityCritical, DependsOn: "", Run: CheckPostgres},
		{Name: "NATS JetStream", Priority: PriorityCritical, DependsOn: "", Run: CheckNATS},
		{Name: "MinIO Storage", Priority: PriorityCritical, DependsOn: "", Run: CheckMinIO},
		{Name: "Docker Proxy", Priority: PriorityCritical, DependsOn: "", Run: CheckDockerProxy},
		{Name: "Scanner Images", Priority: PriorityWarning, DependsOn: "Docker Proxy", Run: CheckScannerImages},
		{Name: "Component Versions", Priority: PriorityWarning, DependsOn: "", Run: CheckVersions},
		{Name: "Disk Space", Priority: PriorityWarning, DependsOn: "Docker Proxy", Run: CheckDiskSpace},
		{Name: "Backup Status", Priority: PriorityInfo, DependsOn: "MinIO Storage", Run: CheckBackupStatus},
	}
}

// buildPriorityMap creates a name→priority lookup from a check list.
func buildPriorityMap(checks []Check) map[string]string {
	m := make(map[string]string, len(checks))
	for _, c := range checks {
		m[c.Name] = c.Priority
	}
	return m
}

// aggregate computes overall status and exit code from results.
// Any CRITICAL check with FAIL status → exit 1.
// WARNING and INFO failures never affect exit code.
func aggregate(results []CheckResult, priorityMap map[string]string) (string, int) {
	for _, r := range results {
		if r.Status == StatusFail {
			if pri, ok := priorityMap[r.Name]; ok && pri == PriorityCritical {
				return OverallDegraded, 1
			}
		}
	}
	return OverallPass, 0
}

// resolveDependency checks if a dependent check should be skipped.
// Returns a skip CheckResult if the dependency failed, nil to proceed.
func resolveDependency(priorResults []CheckResult, check Check) *CheckResult {
	if check.DependsOn == "" {
		return nil
	}
	for _, r := range priorResults {
		if r.Name == check.DependsOn && r.Status == StatusFail {
			return &CheckResult{
				Name:   check.Name,
				Status: StatusSkip,
				Detail: "Skipped: " + check.DependsOn + " is unavailable",
			}
		}
	}
	return nil
}

// statusIcon returns the icon for a status.
func statusIcon(status string) string {
	switch status {
	case StatusPass:
		return "✅"
	case StatusWarn:
		return "⚠️ "
	case StatusFail:
		return "❌"
	case StatusSkip:
		return "⬜"
	default:
		return "❓"
	}
}

// statusColor returns the ANSI color for a status.
func statusColor(status string) string {
	switch status {
	case StatusPass:
		return colorGreen
	case StatusWarn:
		return colorYellow
	case StatusFail:
		return colorRed
	case StatusSkip:
		return colorCyan
	default:
		return colorReset
	}
}

// statusLabel returns the display label for a status.
func statusLabel(status string) string {
	switch status {
	case StatusPass:
		return "PASS"
	case StatusWarn:
		return "WARN"
	case StatusFail:
		return "FAIL"
	case StatusSkip:
		return "SKIP"
	default:
		return strings.ToUpper(status)
	}
}

// printTableTo writes the formatted table output to w.
func printTableTo(w io.Writer, results []CheckResult, overall string) {
	// Header
	fmt.Fprintln(w)
	fmt.Fprintf(w, "%s━━━ Kuro — System Diagnostics%s\n\n", colorBold, colorReset)
	fmt.Fprintf(w, "  %-22s %-10s %s\n", "CHECK", "STATUS", "DETAIL")
	fmt.Fprintf(w, "  %s\n", strings.Repeat("─", 70))

	// Each check
	for _, r := range results {
		icon := statusIcon(r.Status)
		label := statusLabel(r.Status)
		sc := statusColor(r.Status)
		fmt.Fprintf(w, "  %-22s %s %s%s%s %s\n",
			r.Name,
			icon, sc, label, colorReset,
			r.Detail,
		)
	}

	// Overall
	fmt.Fprintln(w)
	if overall == OverallPass {
		fmt.Fprintf(w, "  %sOverall: ✅ %s%s\n", colorBold, overall, colorReset)
	} else {
		fmt.Fprintf(w, "  %sOverall: ❌ %s%s\n", colorBold, overall, colorReset)
	}
	fmt.Fprintln(w)
}

// printJSONTo writes structured JSON output to w.
func printJSONTo(w io.Writer, results []CheckResult, overall string, exitCode int) {
	output := DoctorOutput{
		Checks:   results,
		Overall:  overall,
		ExitCode: exitCode,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(output)
}

// stdoutWriter is the default output writer (overridden in tests).
var stdoutWriter io.Writer = os.Stdout

// printTable writes the table to stdout.
func printTable(results []CheckResult, overall string) {
	printTableTo(stdoutWriter, results, overall)
}

// printJSON writes the JSON to stdout.
func printJSON(results []CheckResult, overall string, exitCode int) {
	printJSONTo(stdoutWriter, results, overall, exitCode)
}
