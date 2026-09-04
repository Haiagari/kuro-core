package cmd

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const Version = "0.1.1"
const updateURL = "https://api.github.com/repos/Haiagari/kuro-core/releases/latest"
const installOneLiner = "curl -sSL https://raw.githubusercontent.com/Haiagari/kuro-core/main/scripts/install.sh | sh"

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

	goos := runtime.GOOS
	arch := runtime.GOARCH
	if goos == "windows" {
		fmt.Println("Automatic update is not supported on Windows.")
		fmt.Printf("Install with:\n  %s\n", installOneLiner)
		return
	}

	client := &http.Client{Timeout: 60 * time.Second}
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

	// GoReleaser archive: kuro_{{ .Version }}_{{ .Os }}_{{ .Arch }}.tar.gz
	// .Version matches the git tag (e.g. v0.1.1).
	assetName := fmt.Sprintf("kuro_%s_%s_%s.tar.gz", release.TagName, goos, arch)

	var downloadURL string
	for _, asset := range release.Assets {
		if asset.Name == assetName {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}

	if downloadURL == "" {
		fmt.Println("Automatic update asset not found for this platform.")
		fmt.Printf("Install or upgrade with:\n  %s\n", installOneLiner)
		fmt.Printf("# pin: %s -s -- %s\n", installOneLiner, release.TagName)
		return
	}

	fmt.Printf("Downloading %s...\n", assetName)
	tmpDir, err := os.MkdirTemp("", "kuro-update-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, assetName)
	out, err := os.Create(archivePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	dlResp, err := client.Get(downloadURL)
	if err != nil {
		out.Close()
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if _, err := io.Copy(out, dlResp.Body); err != nil {
		dlResp.Body.Close()
		out.Close()
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	dlResp.Body.Close()
	out.Close()

	binPath, err := extractKuroFromTarGz(archivePath, tmpDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: extracting archive: %v\n", err)
		fmt.Printf("Fall back to:\n  %s\n", installOneLiner)
		os.Exit(1)
	}

	selfPath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	selfPath, err = filepath.EvalSymlinks(selfPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := os.Rename(binPath, selfPath); err != nil {
		if err := exec.Command("cp", binPath, selfPath).Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			fmt.Println("Run with sudo or move the binary manually:")
			fmt.Printf("  sudo mv %s %s\n", binPath, selfPath)
			fmt.Printf("Or use the install script:\n  %s\n", installOneLiner)
			os.Exit(1)
		}
	}

	if err := os.Chmod(selfPath, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	}

	fmt.Printf("Updated to %s \u2705\n", release.TagName)
}

// extractKuroFromTarGz unpacks the GoReleaser archive and returns the path to the kuro binary.
func extractKuroFromTarGz(archivePath, destDir string) (string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		base := filepath.Base(hdr.Name)
		if hdr.Typeflag != tar.TypeReg || base != "kuro" {
			continue
		}
		outPath := filepath.Join(destDir, "kuro")
		out, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			return "", err
		}
		if _, err := io.Copy(out, tr); err != nil {
			out.Close()
			return "", err
		}
		if err := out.Close(); err != nil {
			return "", err
		}
		return outPath, nil
	}
	return "", fmt.Errorf("kuro binary not found in archive")
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
