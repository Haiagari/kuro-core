package cmd


import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-isatty"

	"kuro/cli/internal/client"
	"kuro/cli/internal/config"
	"kuro/cli/internal/orchestrator"
	"kuro/cli/internal/tui"
)

// ── Colors (terminal) ──────────────────────────────────────

const (
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
	colorReset  = "\033[0m"
)

// RunScan handles the `kuro scan <target> [--remote] [--json] [--tui] [--history]` command.
func RunScan(args []string) {
	fs := flag.NewFlagSet("scan", flag.ContinueOnError)
	remoteFlag := fs.Bool("remote", false, "Force remote mode (send to server)")
	jsonFlag := fs.Bool("json", false, "Output in JSON format")
	historyFlag := fs.Bool("history", false, "Scan full git history (local mode only)")
	fs.Bool("tui", false, "Enable TUI mode (auto-detected on TTY)")

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	target := fs.Arg(0)
	if target == "" {
		fmt.Fprintln(os.Stderr, "Usage: kuro scan <path>|<url> [--remote] [--json] [--tui] [--history]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "  Local:  kuro scan ./my-project")
		fmt.Fprintln(os.Stderr, "  Remote: kuro scan --remote ./my-project")
		fmt.Fprintln(os.Stderr, "  URL:    kuro scan https://github.com/org/repo")
		fmt.Fprintln(os.Stderr, "  History: kuro scan --history ./my-project  (scan full git log)")
		os.Exit(1)
	}

	// ── Display mode priority: json > tui > auto-tty > text ──
	// Detect if --tui was explicitly provided (Go's flag package lacks Changed)
	tuiChanged := false
	for _, arg := range args {
		if arg == "--tui" || arg == "-tui" {
			tuiChanged = true
			break
		}
		if strings.HasPrefix(arg, "--tui=") || strings.HasPrefix(arg, "-tui=") {
			tuiChanged = true
			break
		}
	}
	tuiValue := fs.Lookup("tui").Value.String() == "true"
	isTTY := isatty.IsTerminal(os.Stdout.Fd())

	useTUI := false
	if *jsonFlag {
		// JSON mode — no TUI, no text progress
	} else if tuiChanged && tuiValue {
		useTUI = true
	} else if !tuiChanged && isTTY {
		// Auto-detect: stdout is a TTY and --tui was not explicitly set
		useTUI = true
	}

	// ── Choose adapter ────────────────────────────────────
	var adapter orchestrator.Adapter
	mode := ""

	if *remoteFlag {
		// Force remote mode
		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			os.Exit(1)
		}
		if cfg.APIKey == "" {
			fmt.Fprintln(os.Stderr, "Remote mode requires API key. Run 'kuro auth <key>' first.")
			os.Exit(1)
		}
		cl := client.New(cfg.APIURL, cfg.APIKey)
		adapter = orchestrator.NewRemoteAdapter(cl)
		mode = "remote"
	} else if isLocalPath(target) {
		// Local path → local mode
		history := *historyFlag
		adapter = orchestrator.NewLocalAdapter(history)
		if history {
			mode = fmt.Sprintf("local history (%s)", detectRuntime())
		} else {
			mode = fmt.Sprintf("local (%s)", detectRuntime())
		}
	} else {
		// URL → modo remoto
		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			os.Exit(1)
		}
		if cfg.APIKey == "" {
			fmt.Fprintln(os.Stderr, "No API key configured. Run 'kuro auth <key>' first.")
			os.Exit(1)
		}
		cl := client.New(cfg.APIURL, cfg.APIKey)
		adapter = orchestrator.NewRemoteAdapter(cl)
		mode = "remote"
	}

	// ── Header (text mode only) ───────────────────────────
	if !*jsonFlag && !useTUI {
		fmt.Println()
		fmt.Printf("%s🔍 Kuro — Security Gate%s\n", colorBold, colorReset)
		fmt.Printf("  Target: %s\n", target)
		fmt.Printf("  Mode:   %s\n", mode)
		fmt.Println()
	}

	// ── Run orchestrator ──────────────────────────────────
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Minute)
	defer cancel()

	var result *orchestrator.ScanResult
	var err error

	if useTUI {
		// ── TUI mode ──────────────────────────────────────
		eventsCh := make(chan orchestrator.PhaseEvent, 64)
		m := tui.NewModel(target, mode, eventsCh, ctx, cancel)
		prog := tea.NewProgram(m)

		tuiDone := make(chan struct{})
		go func() {
			defer close(tuiDone)
			if _, runErr := prog.Run(); runErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: TUI error: %v\n", runErr)
			}
		}()

		orch := orchestrator.NewWithEvents(adapter, false, eventsCh)
		result, err = orch.Run(ctx, target)
		close(eventsCh)

		// Wait for TUI to complete
		<-tuiDone
	} else {
		orch := orchestrator.New(adapter, !*jsonFlag)
		result, err = orch.Run(ctx, target)
	}

	// ── Output ────────────────────────────────────────────
	if code := scanOutput(result, err, *jsonFlag, useTUI); code != 0 {
		os.Exit(code)
	}
}

// scanOutput prints the scan result in the selected display mode and returns
// the process exit code: 0 on success, 1 when the scan failed.
func scanOutput(result *orchestrator.ScanResult, err error, jsonMode, tuiMode bool) int {
	if jsonMode {
		printJSON(result, err)
		if err != nil {
			return 1
		}
		return 0
	}

	// In TUI mode the scan has already been displayed; print
	// a compact summary below so there's a persistent record.
	if tuiMode {
		fmt.Println()
		fmt.Printf("%s═══════════════════════════════════════%s\n", colorBold, colorReset)
		if err != nil {
			fmt.Printf("%s  ❌ SCAN FAILED%s\n", colorRed, colorReset)
			fmt.Printf("  Error: %v\n", err)
		} else {
			switch result.Decision {
			case "pass":
				fmt.Printf("%s  ✅ PASS%s\n", colorGreen, colorReset)
			case "review":
				fmt.Printf("%s  ⚠️  REVIEW%s\n", colorYellow, colorReset)
			default:
				fmt.Printf("%s  ❌ BLOCK%s\n", colorRed, colorReset)
			}
			fmt.Printf("  Duration: %s\n", result.Duration.Round(time.Millisecond))
		}
		fmt.Println()
		if err != nil {
			return 1
		}
		return 0
	}

	fmt.Println()
	fmt.Printf("%s═══════════════════════════════════════%s\n", colorBold, colorReset)

	if err != nil {
		fmt.Printf("%s  ❌ SCAN FAILED%s\n", colorRed, colorReset)
		fmt.Printf("  Error: %v\n", err)
		return 1
	}

	switch result.Decision {
	case "pass":
		fmt.Printf("%s  ✅ PASS — Clean code%s\n", colorGreen, colorReset)
	case "review":
		fmt.Printf("%s  ⚠️  REVIEW — Review required%s\n", colorYellow, colorReset)
	default:
		fmt.Printf("%s  ❌ BLOCK — Would block the push%s\n", colorRed, colorReset)
	}

	fmt.Printf("  Duration: %s\n", result.Duration.Round(time.Millisecond))
	fmt.Println()

	// ── Findings ─────────────────────────────────────────
	if len(result.Findings) > 0 {
		fmt.Printf("  %sFindings:%s\n", colorBold, colorReset)
		for _, f := range result.Findings {
			sevColor := colorYellow
			if f.Severity == "CRITICAL" || f.Severity == "HIGH" {
				sevColor = colorRed
			} else if f.Severity == "LOW" {
				sevColor = colorGreen
			}
			fmt.Printf("    %s%s%s │ %s │ %s:%d\n",
				sevColor, padSeverity(f.Severity), colorReset,
				f.Title,
				f.FilePath, f.LineNumber)
		}
		fmt.Println()
	}

	// Summary
	if result.FindingsBySeverity != nil {
		fmt.Printf("  %sSummary:%s\n", colorBold, colorReset)
		if c, ok := result.FindingsBySeverity["CRITICAL"]; ok && c > 0 {
			fmt.Printf("    %sCritical: %d%s\n", colorRed, c, colorReset)
		}
		if c, ok := result.FindingsBySeverity["HIGH"]; ok && c > 0 {
			fmt.Printf("    %sHigh:     %d%s\n", colorRed, c, colorReset)
		}
		if c, ok := result.FindingsBySeverity["MEDIUM"]; ok && c > 0 {
			fmt.Printf("    %sMedium:   %d%s\n", colorYellow, c, colorReset)
		}
		if c, ok := result.FindingsBySeverity["LOW"]; ok && c > 0 {
			fmt.Printf("    %sLow:      %d%s\n", colorGreen, c, colorReset)
		}
	}
	fmt.Println()
	return 0
}

// ── Helpers ────────────────────────────────────────────────

func isLocalPath(target string) bool {
	// Starts with http, https, git@ → remote
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") ||
		strings.HasPrefix(target, "git@") || strings.HasPrefix(target, "ssh://") {
		return false
	}
	// Starts with ./ or / or ~ → local
	if strings.HasPrefix(target, "./") || strings.HasPrefix(target, "/") || strings.HasPrefix(target, "~/") {
		return true
	}
	// If existing directory → local scan
	if info, err := os.Stat(target); err == nil && info.IsDir() {
		return true
	}
	// Default to remote
	return false
}

func detectRuntime() string {
	if _, err := exec.LookPath("docker"); err == nil {
		return "docker"
	}
	if _, err := exec.LookPath("podman"); err == nil {
		return "podman"
	}
	return "docker"
}

func padSeverity(s string) string {
	switch s {
	case "CRITICAL":
		return "CRITICAL "
	case "HIGH":
		return "HIGH     "
	case "MEDIUM":
		return "MEDIUM   "
	case "LOW":
		return "LOW      "
	default:
		return fmt.Sprintf("%-8s", s)
	}
}

func printJSON(result *orchestrator.ScanResult, scanErr error) {
	// Simple JSON output
	fmt.Printf("{\n")
	fmt.Printf("  \"target\": %q,\n", result.Target)
	fmt.Printf("  \"mode\": %q,\n", result.Mode)
	fmt.Printf("  \"status\": %q,\n", result.Status)
	fmt.Printf("  \"decision\": %q,\n", result.Decision)
	fmt.Printf("  \"duration\": %q,\n", result.Duration.Round(time.Millisecond))
	if scanErr != nil {
		fmt.Printf("  \"error\": %q,\n", scanErr.Error())
	}
	fmt.Printf("  \"findings\": [\n")
	for i, f := range result.Findings {
		comma := ""
		if i < len(result.Findings)-1 {
			comma = ","
		}
		fmt.Printf("    {\"scanner\":%q,\"severity\":%q,\"title\":%q,\"file\":%q,\"line\":%d}%s\n",
			f.Scanner, f.Severity, f.Title, f.FilePath, f.LineNumber, comma)
	}
	fmt.Printf("  ]\n")
	fmt.Printf("}\n")
}

// ScanPhaseResult represents a single phase output for the terminal.
type ScanPhaseResult struct {
	Name   string
	Status string
	Detail string
}

// PrintPhaseResult prints a phase line.
func PrintPhaseResult(phase string, status string, detail string) {
	statusColor := colorGreen
	if status == "running" {
		statusColor = colorCyan
	} else if status == "fail" {
		statusColor = colorRed
	}

	fmt.Printf("  %s%-12s%s %s%s%s",
		colorBold, phase, colorReset,
		statusColor, status, colorReset)
	if detail != "" {
		fmt.Printf("  %s", detail)
	}
	fmt.Println()
}
