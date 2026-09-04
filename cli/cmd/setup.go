package cmd

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// RunSetup handles the `kuro setup <component>` command.
// Components: tls, ollama, runner, nats, firewall
func RunSetup(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: kuro setup <component>")
		fmt.Fprintln(os.Stderr, "Components: tls, ollama, runner, nats, firewall, hooks")
		os.Exit(1)
	}

	component := args[0]
	rest := args[1:]

	switch component {
	case "tls":
		setupTLS(rest)
	case "ollama":
		setupOllama(rest)
	case "runner":
		setupRunner(rest)
	case "nats":
		setupNATS(rest)
	case "firewall":
		setupFirewall(rest)
	case "hooks":
		setupHooks(rest)
	case "images":
		setupImages(rest)
	default:
		fmt.Fprintf(os.Stderr, "Unknown component: %q\n", component)
		fmt.Fprintln(os.Stderr, "Components: tls, ollama, runner, nats, firewall, hooks, images")
		os.Exit(1)
	}
}

func setupTLS(args []string) {
	fs := flag.NewFlagSet("setup tls", flag.ContinueOnError)
	disable := fs.Bool("disable", false, "Disable TLS")
	status := fs.Bool("status", false, "Show TLS status")
	_ = fs.Parse(args)

	script := "scripts/setup-tls.sh"
	if *disable {
		runScript(script, "--disable")
	} else if *status {
		runScript(script, "--status")
	} else {
		runScript(script)
	}
}

func setupOllama(args []string) {
	fs := flag.NewFlagSet("setup ollama", flag.ContinueOnError)
	force := fs.Bool("f", false, "Force reinstall")
	_ = fs.Parse(args)

	script := "scripts/setup-ollama.sh"
	if *force {
		runScript(script, "-f")
	} else {
		runScript(script)
	}
}

func setupRunner(args []string) {
	runScript("scripts/setup-runner.sh")
}

func setupNATS(args []string) {
	runScript("scripts/setup-nats-streams.sh")
}

func setupFirewall(args []string) {
	runScript("scripts/setup-firewall.sh")
}

func setupHooks(args []string) {
	runScript("scripts/setup-hooks.sh")
}

func setupImages(args []string) {
	_ = strings.Join(args, " ") // silence unused warning

	// Scanner images (same pinned versions as the local orchestrator)
	images := []struct{
		Name string
		Tag  string
		Full string
	}{
		{"Gitleaks",   "zricethezav/gitleaks",   "docker.io/zricethezav/gitleaks:v8.30.1"},
		{"Semgrep",    "semgrep/semgrep",        "docker.io/semgrep/semgrep:1.165.0"},
		{"Trivy",      "aquasec/trivy",          "docker.io/aquasec/trivy:0.57.0"},
		{"Checkov",    "bridgecrew/checkov",     "docker.io/bridgecrew/checkov:3.2.400"},
	}

	// Detect runtime
	runtime := detectRuntimeStr()
	fmt.Printf("\n🔍 Kuro — Setup Scanner Images\n")
	fmt.Printf("  Runtime: %s\n\n", runtime)

	// Verify the runtime is available
	if _, err := exec.LookPath(runtime); err != nil {
		fmt.Fprintf(os.Stderr, "❌ %s is not installed. Install it first.\n", runtime)
		os.Exit(1)
	}

	// Pull each image
	success := 0
	failed := 0

	for _, img := range images {
		fmt.Printf("  ▶ %-12s %s ", img.Name, img.Full)

		cmd := exec.Command(runtime, "pull", img.Full)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Env = append(os.Environ(), "REGISTRY_AUTH_FILE="+os.Getenv("HOME")+"/.docker/config.json")

		if err := cmd.Run(); err != nil {
			fmt.Printf("  %s\n", colorRed+"❌"+colorReset)
			failed++
			continue
		}
		fmt.Printf("  %s\n", colorGreen+"✅"+colorReset)
		success++
	}

	// Summary
	fmt.Printf("\n  %sResult:%s %d downloaded, %d failed", colorBold, colorReset, success, failed)
	if failed > 0 {
		fmt.Printf(" — check your internet connection or run the command manually:\n")
		for _, img := range images {
			fmt.Printf("    %s pull %s\n", runtime, img.Full)
		}
	}
	fmt.Println()

	if failed > 0 {
		os.Exit(1)
	}
}

func detectRuntimeStr() string {
	if _, err := exec.LookPath("podman"); err == nil {
		return "podman"
	}
	if _, err := exec.LookPath("docker"); err == nil {
		return "docker"
	}
	return "docker"
}

func runScript(script string, extraArgs ...string) {
	cmd := exec.Command("bash", append([]string{script}, extraArgs...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  Script %s failed: %v\n", script, err)
	}
}
