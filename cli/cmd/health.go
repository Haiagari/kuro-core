package cmd

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"time"
)

// RunHealth handles the `kuro health [--watch]` command.
func RunHealth(args []string) {
	fs := flag.NewFlagSet("health", flag.ContinueOnError)
	watch := fs.Bool("watch", false, "Watch health every 10s")

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	for {
		checkAll()
		if !*watch {
			break
		}
		fmt.Println("--- Watching (Ctrl+C to stop) ---")
		time.Sleep(10 * time.Second)
	}
}

func checkAll() {
	fmt.Println("🔍 Health Check — Kuro")
	fmt.Println("")

	services := []struct {
		name string
		url  string
	}{
		{"API", "http://localhost:8080/health"},
		{"Worker", "http://localhost:9090/metrics"},
	}

	for _, svc := range services {
		checkHTTP(svc.name, svc.url)
	}

	// Docker container status
	checkDocker()
}

func checkHTTP(name, url string) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		fmt.Printf("  ❌ %s: unreachable (%v)\n", name, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		fmt.Printf("  ✅ %s: healthy\n", name)
	} else {
		fmt.Printf("  ⚠️  %s: status %d\n", name, resp.StatusCode)
	}
}

func checkDocker() {
	fmt.Print("  Containers: ")
	cmd := exec.Command("docker", "compose", "ps", "--format", "table {{.Name}}\t{{.Status}}")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}
