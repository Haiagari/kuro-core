package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const Version = "0.1.0"
const updateURL = "https://api.github.com/repos/Haiagari/kuro/releases/latest"

// GitHubRelease represents a GitHub release API response.
type GitHubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func RunUpdate(args []string) {
	if len(args) > 0 && args[0] == "--check" {
		checkUpdate()
		return
	}
	doUpdate()
}

func checkUpdate() {
	fmt.Printf("kuro version %s\n", Version)
	fmt.Printf("OS: %s/%s\n", runtime.GOOS, runtime.GOARCH)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(updateURL)
	if err != nil {
		fmt.Println("Could not check for updates (no connection to GitHub)")
		return
	}
	defer resp.Body.Close()

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		fmt.Println("Could not parse GitHub response")
		return
	}

	if compareVersions(release.TagName, Version) > 0 {
		fmt.Printf("New version available: %s (current: %s)\n", release.TagName, Version)
		fmt.Println("Run 'kuro update' to upgrade.")
	} else {
		fmt.Println("You already have the latest version.")
	}
}

func doUpdate() {
	fmt.Println("Updating Kuro...")

	// Determine the operating system
	goos := runtime.GOOS
	arch := runtime.GOARCH
	ext := ""
	if goos == "windows" {
		ext = ".exe"
	}

	assetName := fmt.Sprintf("kuro-%s-%s%s", goos, arch, ext)

	// Find asset in GitHub release
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(updateURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not connect to GitHub: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not read the release: %v\n", err)
		os.Exit(1)
	}

	var downloadURL string
	for _, asset := range release.Assets {
		if asset.Name == assetName {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}

	if downloadURL == "" {
		// No release found, show manual instructions
		fmt.Println("Automatic update not available for this platform.")
		fmt.Println("Run: git pull && cd cli && go build -o kuro .")
		return
	}

	// Downloading new version
	fmt.Printf("Downloading %s...\n", assetName)
	out, err := os.CreateTemp("", "kuro-*"+ext)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer os.Remove(out.Name())

	dlResp, err := client.Get(downloadURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer dlResp.Body.Close()

	if _, err := out.ReadFrom(dlResp.Body); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	out.Close()

	// Replace current binary
	selfPath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := os.Rename(out.Name(), selfPath); err != nil {
		// Fallback: try cp
		if err := exec.Command("cp", out.Name(), selfPath).Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			fmt.Println("Run with sudo or move the binary manually:")
			fmt.Printf("  sudo mv %s %s\n", out.Name(), selfPath)
			os.Exit(1)
		}
	}

	if err := os.Chmod(selfPath, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	}

	fmt.Printf("Updated to %s ✅\n", release.TagName)
}

// compareVersions compares two semver strings (with or without "v" prefix).
// Returns -1, 0, or 1.
func compareVersions(a, b string) int {
	pa := parseVersion(a)
	pb := parseVersion(b)
	for i := 0; i < 3; i++ {
		if pa[i] < pb[i] {
			return -1
		}
		if pa[i] > pb[i] {
			return 1
		}
	}
	return 0
}

func parseVersion(v string) [3]int {
	v = strings.TrimPrefix(v, "v")
	var parts [3]int
	for i, s := range strings.SplitN(v, ".", 3) {
		if i >= 3 {
			break
		}
		parts[i], _ = strconv.Atoi(s)
	}
	return parts
}
