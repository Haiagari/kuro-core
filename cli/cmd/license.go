package cmd

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// RunLicense handles the 'kuro license' command family.
func RunLicense(args []string) {
	if len(args) == 0 {
		printLicenseUsage()
		return
	}

	sub := args[0]
	switch sub {
	case "status":
		runLicenseStatus(args[1:])
	case "apply":
		runLicenseApply(args[1:])
	case "generate":
		runLicenseGenerate(args[1:])
	case "help", "--help", "-h":
		printLicenseUsage()
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown license command %q\n", sub)
		printLicenseUsage()
		os.Exit(1)
	}
}

func printLicenseUsage() {
	fmt.Print(`
╭────────────────────────────────────────────────────────────╮
│               KURO License & Tier Entitlements             │
│                 Open-Core & Commercial Engine              │
╰────────────────────────────────────────────────────────────╯

USAGE:
  kuro license status              Display active tier and enterprise capabilities
  kuro license apply <token|file>  Install commercial Kuro Enterprise license
  kuro license generate [flags]    Generate signed offline license token (Admin only)
`)
}

func runLicenseStatus(args []string) {
	token := getActiveLicenseToken()
	if token == "" {
		fmt.Printf("╭────────────────────────────────────────────────────────────╮\n")
		fmt.Printf("│ Active Tier: KURO COMMUNITY (Open-Source Core)             │\n")
		fmt.Printf("╰────────────────────────────────────────────────────────────╯\n")
		fmt.Printf("Status:       Active (Free / No Expiration)\n")
		fmt.Printf("Core Modules: Gitleaks, Semgrep, Trivy, CLI, SQLite/PG\n\n")
		fmt.Printf("Enterprise Features Locked (Upgrade to Kuro Enterprise):\n")
		fmt.Printf("  • Multi-tenant Row-Level Security (PostgreSQL RLS)\n")
		fmt.Printf("  • Zero-Knowledge Blind Index Finding Vault (AES-256-GCM)\n")
		fmt.Printf("  • in-toto Cryptographic Supply Chain Attestation\n")
		fmt.Printf("  • WebAssembly & OPA Rego Dynamic Policy Engine\n")
		fmt.Printf("  • eBPF Linux Kernel Container Auditing\n")
		fmt.Printf("  • Monorepo Smart Diff Delta Scanning (<50ms)\n\n")
		fmt.Printf("Apply license via: 'kuro license apply <token>'\n")
		return
	}

	// Parse token
	token = strings.TrimPrefix(token, "kuro_lic_")
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		fmt.Fprintf(os.Stderr, "Error: Malformed license token\n")
		return
	}

	claimsBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error decoding license payload: %v\n", err)
		return
	}

	var claims struct {
		CustomerID string    `json:"customer_id"`
		Plan       string    `json:"plan"`
		Seats      int       `json:"seats"`
		Features   []string  `json:"features"`
		IssuedAt   time.Time `json:"issued_at"`
		ExpiresAt  time.Time `json:"expires_at"`
	}

	if err := json.Unmarshal(claimsBytes, &claims); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing claims: %v\n", err)
		return
	}

	fmt.Printf("╭────────────────────────────────────────────────────────────╮\n")
	fmt.Printf("│ Active Tier: KURO ENTERPRISE                               │\n")
	fmt.Printf("╰────────────────────────────────────────────────────────────╯\n")
	fmt.Printf("Customer ID:   %s\n", claims.CustomerID)
	fmt.Printf("Plan:          %s\n", strings.ToUpper(claims.Plan))
	fmt.Printf("Seats:         %d Developer Licenses\n", claims.Seats)
	fmt.Printf("Issued At:     %s\n", claims.IssuedAt.Format("2006-01-02"))
	if !claims.ExpiresAt.IsZero() {
		daysLeft := int(time.Until(claims.ExpiresAt).Hours() / 24)
		fmt.Printf("Expires At:    %s (%d days remaining)\n\n", claims.ExpiresAt.Format("2006-01-02"), daysLeft)
	}

	fmt.Printf("Unlocked Enterprise Capabilities:\n")
	for _, f := range claims.Features {
		fmt.Printf("  ✔ %s\n", f)
	}
}

func runLicenseApply(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Error: license token or file path required")
		os.Exit(1)
	}

	raw := args[0]
	if fileBytes, err := os.ReadFile(raw); err == nil {
		raw = strings.TrimSpace(string(fileBytes))
	}

	home, _ := os.UserHomeDir()
	configDir := filepath.Join(home, ".kuro")
	_ = os.MkdirAll(configDir, 0755)

	licensePath := filepath.Join(configDir, "license.jwt")
	if err := os.WriteFile(licensePath, []byte(raw), 0600); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing license file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Kuro Enterprise license applied successfully!\n")
	fmt.Printf("Stored in: %s\n", licensePath)
	fmt.Println("Run 'kuro license status' to verify unlocked capabilities.")
}

func runLicenseGenerate(args []string) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating key: %v\n", err)
		os.Exit(1)
	}

	customer := "demo-enterprise"
	seats := 100
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--customer", "-c":
			if i+1 < len(args) { customer = args[i+1]; i++ }
		}
	}

	claims := struct {
		CustomerID string    `json:"customer_id"`
		Plan       string    `json:"plan"`
		Seats      int       `json:"seats"`
		Features   []string  `json:"features"`
		IssuedAt   time.Time `json:"issued_at"`
		ExpiresAt  time.Time `json:"expires_at"`
	}{
		CustomerID: customer,
		Plan:       "enterprise",
		Seats:      seats,
		Features: []string{
			"multi_tenant_rls",
			"blind_indexing",
			"supply_chain_attestation",
			"wasm_policy_engine",
			"ebpf_kernel_monitor",
			"monorepo_delta_scan",
			"distributed_locks",
		},
		IssuedAt:  time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(365 * 24 * time.Hour),
	}

	claimsBytes, _ := json.Marshal(claims)
	payloadB64 := base64.RawURLEncoding.EncodeToString(claimsBytes)
	sigBytes := ed25519.Sign(priv, claimsBytes)
	sigB64 := base64.RawURLEncoding.EncodeToString(sigBytes)

	token := fmt.Sprintf("kuro_lic_%s.%s", payloadB64, sigB64)

	fmt.Printf("╭────────────────────────────────────────────────────────────╮\n")
	fmt.Printf("│ Generated Kuro Enterprise Signed License Token             │\n")
	fmt.Printf("╰────────────────────────────────────────────────────────────╯\n")
	fmt.Printf("Customer:    %s (%d seats)\n", customer, seats)
	fmt.Printf("Token:\n%s\n\n", token)
	_ = pub
}

func getActiveLicenseToken() string {
	if token := os.Getenv("KURO_LICENSE_KEY"); token != "" {
		return token
	}
	home, _ := os.UserHomeDir()
	licensePath := filepath.Join(home, ".kuro", "license.jwt")
	if data, err := os.ReadFile(licensePath); err == nil {
		return strings.TrimSpace(string(data))
	}
	return ""
}
