package cmd

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// RunUp handles the `kuro up [flags]` command.
func RunUp(args []string) {
	fs := flag.NewFlagSet("up", flag.ContinueOnError)
	minimal := fs.Bool("minimal", false, "Only postgres, api, git-proxy")
	down := fs.Bool("down", false, "Stop all services")
	status := fs.Bool("status", false, "Show service status")
	noDash := fs.Bool("no-dash", false, "All except dashboard")
	noNats := fs.Bool("no-nats", false, "All except NATS")

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	// Validate mutual exclusivity of action flags
	actions := 0
	for _, active := range []bool{*down, *status, *minimal} {
		if active {
			actions++
		}
	}
	if actions > 1 {
		fmt.Fprintln(os.Stderr, "Error: --down, --status, and --minimal are mutually exclusive")
		os.Exit(1)
	}

	// Find the project root
	projectRoot, err := findProjectRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Verify docker-compose.yml exists
	composeFile := filepath.Join(projectRoot, "docker-compose.yml")
	if _, err := os.Stat(composeFile); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: docker-compose.yml not found at %s\n", composeFile)
		os.Exit(1)
	}

	// Check docker availability
	if _, err := exec.LookPath("docker"); err != nil {
		fmt.Fprintln(os.Stderr, "Error: docker is not installed or not in PATH")
		os.Exit(1)
	}

	// Switch to the project root so docker compose finds the compose file
	origDir, _ := os.Getwd()
	if err := os.Chdir(projectRoot); err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot cd to %s: %v\n", projectRoot, err)
		os.Exit(1)
	}
	defer func() { _ = os.Chdir(origDir) }()

	switch {
	case *down:
		runCompose("down")
	case *status:
		runCompose("ps")
	case *minimal:
		runCompose("up", "-d", "postgres", "api", "git-proxy")
	case *noDash:
		runCompose("up", "-d",
			"postgres", "migrator", "nats", "garage", "docker-proxy",
			"api", "worker", "notifier", "backup", "trivy-updater", "git-proxy")
	case *noNats:
		runCompose("up", "-d",
			"postgres", "migrator", "garage", "docker-proxy",
			"api", "worker", "dashboard", "notifier", "backup", "trivy-updater", "git-proxy")
	default:
		runCompose("up", "-d")
	}
}

// findProjectRoot returns the git top-level directory, falling back to cwd.
func findProjectRoot() (string, error) {
	if out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output(); err == nil {
		return strings.TrimSpace(string(out)), nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("cannot determine working directory: %w", err)
	}
	return cwd, nil
}

// runCompose executes a docker compose command and inherits stdout/stderr.
// On failure it calls os.Exit(1).
func runCompose(args ...string) {
	cmd := exec.Command("docker", append([]string{"compose"}, args...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.Exit(1)
	}
}
