package cmd

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
)

// RunDeploy handles the `kuro deploy [--tls] [--ollama]` command.
func RunDeploy(args []string) {
	fs := flag.NewFlagSet("deploy", flag.ContinueOnError)
	tlsFlag := fs.Bool("tls", false, "Enable TLS")
	ollamaFlag := fs.Bool("ollama", false, "Include Ollama")

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	fmt.Println(" Deploying Kuro...")

	// Step 1: docker compose up
	fmt.Print("  Starting services...")
	cmd := exec.Command("docker", "compose", "up", "-d",
		"postgres", "nats", "garage", "api", "worker", "notifier", "backup")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "\n❌ Error starting services: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(" ✅")

	// Step 2: Init DB + buckets
	fmt.Print("  Initializing DB + buckets...")
	initCmd := exec.Command("bash", "scripts/init-docker.sh")
	initCmd.Stdout = os.Stdout
	initCmd.Stderr = os.Stderr
	if err := initCmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "\n⚠️  Init failed: %v\n", err)
	}
	fmt.Println(" ✅")

	// Step 3: Generate API key
	fmt.Print("  Generating API key...")
	keyCmd := exec.Command("bash", "scripts/bootstrap-api-key.sh")
	output, err := keyCmd.Output()
	if err == nil {
		fmt.Printf(" ✅\n%s", string(output))
	} else {
		fmt.Println(" ⚠️  Could not generate key automatically")
	}

	// Step 4: Verify
	fmt.Print("  Verifying services...")
	psCmd := exec.Command("docker", "compose", "ps")
	psCmd.Stdout = os.Stdout
	psCmd.Stderr = os.Stderr
	_ = psCmd.Run()

	// Optional TLS
	if *tlsFlag {
		fmt.Println("  Configuring TLS...")
		tlsCmd := exec.Command("bash", "scripts/setup-tls.sh")
		tlsCmd.Stdout = os.Stdout
		tlsCmd.Stderr = os.Stderr
		_ = tlsCmd.Run()
	}

	// Optional Ollama
	if *ollamaFlag {
		fmt.Println("  Configuring Ollama...")
		ollamaCmd := exec.Command("bash", "scripts/setup-ollama.sh", "-f")
		ollamaCmd.Stdout = os.Stdout
		ollamaCmd.Stderr = os.Stderr
		_ = ollamaCmd.Run()
	}

	fmt.Println("\n✅ Deploy complete")
}
