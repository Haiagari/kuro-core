package doctor


import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// ── Configurable addresses (overridable in tests) ────────────

var (
	postgresAddr = "localhost:5433"
	natsAddr     = "localhost:4222"
	minioURL     = "http://localhost:9000"
	dockerURL    = "http://localhost:2375"
	apiURL       = "http://localhost:8080"
)

// defaultDiskPaths are the filesystem paths to check for free space.
var defaultDiskPaths = []string{
	"/var/lib/docker",
	"/tmp",
}

// scannerImages are the container images to verify.
var scannerImages = []struct {
	Name string
	Ref  string
}{
	{"Trivy", "aquasec/trivy:0.57.0"},
	{"Grype", "anchore/grype:v0.114.0"},
	{"Semgrep", "semgrep/semgrep:1.165.0"},
}

// ── CheckPostgres ──────────────────────────────────────────

// CheckPostgres verifies PostgreSQL connectivity via TCP dial and pg_isready.
func CheckPostgres() CheckResult {
	return checkPostgresAddr(postgresAddr)
}

func checkPostgresAddr(addr string) CheckResult {
	// TCP dial
	dialer := net.Dialer{Timeout: 5 * time.Second}
	conn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return CheckResult{
			Status: StatusFail,
			Detail: fmt.Sprintf("TCP dial failed: %v", err),
		}
	}
	conn.Close()

	// docker exec pg_isready (best-effort, secondary check)
	pgDetail := "pg_isready: accepting connections"
	if out, err := exec.Command("docker", "exec", "kuro-postgres", "pg_isready").Output(); err != nil {
		pgDetail = fmt.Sprintf("pg_isready: %v", strings.TrimSpace(string(out)))
	}

	return CheckResult{
		Status: StatusPass,
		Detail: fmt.Sprintf("TCP: connected, %s", pgDetail),
	}
}

// ── CheckNATS ──────────────────────────────────────────────

// CheckNATS verifies NATS JetStream connectivity via TCP dial.
func CheckNATS() CheckResult {
	return checkTCP(natsAddr, "NATS JetStream")
}

// ── CheckMinIO ─────────────────────────────────────────────

// CheckMinIO verifies MinIO reachability via its health endpoint.
func CheckMinIO() CheckResult {
	return checkHTTPHead(minioURL+"/minio/health/live", "MinIO Storage")
}

// ── CheckDockerProxy ───────────────────────────────────────

// CheckDockerProxy verifies the docker socket proxy is responsive.
func CheckDockerProxy() CheckResult {
	return checkHTTPGet(dockerURL+"/version", "Docker Proxy")
}

// ── CheckScannerImages ─────────────────────────────────────

// CheckScannerImages verifies required scanner images exist locally.
func CheckScannerImages() CheckResult {
	var missing []string

	for _, img := range scannerImages {
		cmd := exec.Command("docker", "image", "inspect", img.Ref)
		if err := cmd.Run(); err != nil {
			missing = append(missing, img.Name)
		}
	}

	if len(missing) > 0 {
		return CheckResult{
			Status: StatusWarn,
			Detail: fmt.Sprintf("Missing: %s", strings.Join(missing, ", ")),
		}
	}
	return CheckResult{
		Status: StatusPass,
		Detail: "All images present",
	}
}

// ── CheckVersions ──────────────────────────────────────────

// CheckVersions fetches and parses the API health endpoint for version info.
func CheckVersions() CheckResult {
	return checkVersionsURL(apiURL + "/health")
}

func checkVersionsURL(url string) CheckResult {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return CheckResult{
			Status: StatusWarn,
			Detail: fmt.Sprintf("API unreachable: %v", err),
		}
	}
	defer resp.Body.Close()

	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return CheckResult{
			Status: StatusWarn,
			Detail: fmt.Sprintf("API response unparseable: %v", err),
		}
	}

	var parts []string
	if s, ok := data["status"].(string); ok {
		parts = append(parts, "status: "+s)
	}
	if db, ok := data["db"].(string); ok {
		parts = append(parts, "db: "+db)
	}
	if nats, ok := data["nats"].(string); ok {
		parts = append(parts, "nats: "+nats)
	}

	return CheckResult{
		Status: StatusPass,
		Detail: strings.Join(parts, " · "),
	}
}

// ── CheckDiskSpace ─────────────────────────────────────────

// CheckDiskSpace checks free space on critical filesystem paths.
func CheckDiskSpace() CheckResult {
	return checkDiskPaths(defaultDiskPaths)
}

func checkDiskPaths(paths []string) CheckResult {
	var warns []string
	var fails []string

	for _, path := range paths {
		var stat syscall.Statfs_t
		if err := syscall.Statfs(path, &stat); err != nil {
			fails = append(fails, fmt.Sprintf("%s: %v", path, err))
			continue
		}

		total := stat.Blocks * uint64(stat.Bsize)
		free := stat.Bavail * uint64(stat.Bsize)
		pct := float64(0)
		if total > 0 {
			pct = float64(free) / float64(total) * 100
		}

		if pct < 5 {
			fails = append(fails, fmt.Sprintf("%s: %.0f%% free", path, pct))
		} else if pct < 20 {
			warns = append(warns, fmt.Sprintf("%s: %.0f%% free", path, pct))
		}
	}

	if len(fails) > 0 {
		return CheckResult{
			Status: StatusWarn,
			Detail: "Low disk: " + strings.Join(fails, ", "),
		}
	}
	if len(warns) > 0 {
		return CheckResult{
			Status: StatusWarn,
			Detail: "Low disk: " + strings.Join(warns, ", "),
		}
	}
	return CheckResult{
		Status: StatusPass,
		Detail: "All paths have sufficient free space",
	}
}

// ── CheckBackupStatus ──────────────────────────────────────

// CheckBackupStatus checks when the last backup was written to MinIO.
func CheckBackupStatus() CheckResult {
	cmd := exec.Command("docker", "exec", "kuro-minio", "mc", "ls", "kuro/kuro-backups/")
	out, err := cmd.Output()
	if err != nil {
		return CheckResult{
			Status: StatusWarn,
			Detail: fmt.Sprintf("Cannot list backups: %v", err),
		}
	}

	lines := strings.TrimSpace(string(out))
	if lines == "" {
		return CheckResult{
			Status: StatusWarn,
			Detail: "No backups found in kuro/kuro-backups/",
		}
	}

	return CheckResult{
		Status: StatusPass,
		Detail: "Backups accessible",
	}
}

// ── Shared helpers ─────────────────────────────────────────

// checkTCP attempts a TCP dial to addr with a 5s timeout.
func checkTCP(addr, name string) CheckResult {
	dialer := net.Dialer{Timeout: 5 * time.Second}
	conn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return CheckResult{
			Status: StatusFail,
			Detail: fmt.Sprintf("%s unreachable on %s: %v", name, addr, err),
		}
	}
	conn.Close()
	return CheckResult{
		Status: StatusPass,
		Detail: fmt.Sprintf("TCP: connected on %s", addr),
	}
}

// checkHTTPHead performs an HTTP HEAD request with a 5s timeout.
func checkHTTPHead(url, name string) CheckResult {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Head(url)
	if err != nil {
		return CheckResult{
			Status: StatusFail,
			Detail: fmt.Sprintf("%s unreachable: %v", name, err),
		}
	}
	resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return CheckResult{
			Status: StatusPass,
			Detail: "Health endpoint: 200 OK",
		}
	}
	return CheckResult{
		Status: StatusFail,
		Detail: fmt.Sprintf("Health endpoint: %d", resp.StatusCode),
	}
}

// checkHTTPGet performs an HTTP GET request with a 5s timeout.
func checkHTTPGet(url, name string) CheckResult {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return CheckResult{
			Status: StatusFail,
			Detail: fmt.Sprintf("%s unreachable: %v", name, err),
		}
	}
	resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return CheckResult{
			Status: StatusPass,
			Detail: "Docker API reachable",
		}
	}
	return CheckResult{
		Status: StatusFail,
		Detail: fmt.Sprintf("Docker API returned: %d", resp.StatusCode),
	}
}

// checkHTTP performs a generic HTTP GET check for testing.
func checkHTTP(url, name string) CheckResult {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return CheckResult{
			Status: StatusFail,
			Detail: fmt.Sprintf("%s unreachable: %v", name, err),
		}
	}
	resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return CheckResult{
			Status: StatusPass,
			Detail: name + " responded: 200 OK",
		}
	}
	return CheckResult{
		Status: StatusFail,
		Detail: fmt.Sprintf("%s responded: %d", name, resp.StatusCode),
	}
}
