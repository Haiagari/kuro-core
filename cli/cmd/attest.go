package cmd

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const GitNotesRef = "refs/notes/kuro-attestation"

// RunAttest handles the 'kuro attest' subcommand family.
func RunAttest(args []string) {
	if len(args) == 0 {
		printAttestUsage()
		return
	}

	sub := args[0]
	switch sub {
	case "verify":
		runAttestVerify(args[1:])
	case "keygen":
		runAttestKeygen(args[1:])
	case "inspect":
		runAttestInspect(args[1:])
	case "help", "--help", "-h":
		printAttestUsage()
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown attest subcommand %q\n", sub)
		printAttestUsage()
		os.Exit(1)
	}
}

func printAttestUsage() {
	fmt.Print(`
╭────────────────────────────────────────────────────────────╮
│             KURO Cryptographic Attestation Gate            │
│                 Supply Chain Security & SLSA               │
╰────────────────────────────────────────────────────────────╯

USAGE:
  kuro attest verify [flags]       Verify in-toto attestation signature on commit
  kuro attest keygen [flags]       Generate new Ed25519 attestation signing keypair
  kuro attest inspect <file>       Inspect and decode attestation JSON envelope

FLAGS (verify):
  --commit <sha>       Target commit SHA (default: HEAD)
  --repo <path>        Path to git repository (default: current directory)
  --pubkey <key|file>  Public key (hex string or file path)
  --file <path>        Verify standalone attestation JSON file instead of git note
`)
}

func runAttestKeygen(args []string) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating keypair: %v\n", err)
		os.Exit(1)
	}

	pubHex := hex.EncodeToString(pub)
	privHex := hex.EncodeToString(priv)
	pubHash := sha256.Sum256(pub)
	keyID := hex.EncodeToString(pubHash[:8])

	fmt.Printf("╭────────────────────────────────────────────────────────────╮\n")
	fmt.Printf("│ Generated Kuro Ed25519 Attestation Keypair                 │\n")
	fmt.Printf("╰────────────────────────────────────────────────────────────╯\n")
	fmt.Printf("Key ID (Fingerprint): %s\n\n", keyID)
	fmt.Printf("PUBLIC KEY (Safe to distribute to K8s / CI):\n%s\n\n", pubHex)
	fmt.Printf("PRIVATE KEY (Keep secret in KURO_ATTESTATION_PRIVATE_KEY):\n%s\n\n", privHex)
}

func runAttestInspect(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Error: file path required to inspect")
		os.Exit(1)
	}

	fileBytes, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		os.Exit(1)
	}

	var env struct {
		PayloadType string `json:"payloadType"`
		Payload     string `json:"payload"`
		Signatures  []struct {
			KeyID string `json:"keyid"`
			Sig   string `json:"sig"`
		} `json:"signatures"`
	}

	if err := json.Unmarshal(fileBytes, &env); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing attestation envelope: %v\n", err)
		os.Exit(1)
	}

	rawStmt, err := base64.StdEncoding.DecodeString(env.Payload)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error decoding statement payload: %v\n", err)
		os.Exit(1)
	}

	var prettyJSON map[string]interface{}
	json.Unmarshal(rawStmt, &prettyJSON)
	formatted, _ := json.MarshalIndent(prettyJSON, "", "  ")

	fmt.Printf("╭────────────────────────────────────────────────────────────╮\n")
	fmt.Printf("│ in-toto Statement Content (%s)             │\n", env.PayloadType)
	fmt.Printf("╰────────────────────────────────────────────────────────────╯\n")
	fmt.Println(string(formatted))
}

func runAttestVerify(args []string) {
	commitSHA := "HEAD"
	repoPath := "."
	pubKeyStr := os.Getenv("KURO_ATTESTATION_PUBLIC_KEY")
	filePath := ""

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--commit", "-c":
			if i+1 < len(args) { commitSHA = args[i+1]; i++ }
		case "--repo", "-r":
			if i+1 < len(args) { repoPath = args[i+1]; i++ }
		case "--pubkey", "-p":
			if i+1 < len(args) { pubKeyStr = args[i+1]; i++ }
		case "--file", "-f":
			if i+1 < len(args) { filePath = args[i+1]; i++ }
		}
	}

	var envBytes []byte
	var err error

	if filePath != "" {
		envBytes, err = os.ReadFile(filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading attestation file: %v\n", err)
			os.Exit(1)
		}
	} else {
		// Read from Git Notes: git notes --ref=refs/notes/kuro-attestation show <commit>
		cmd := exec.Command("git", "-C", repoPath, "notes", "--ref="+GitNotesRef, "show", commitSHA)
		envBytes, err = cmd.CombinedOutput()
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ Verification Failed: No Kuro attestation note found on commit %s\n", commitSHA)
			os.Exit(1)
		}
	}

	var envelope struct {
		PayloadType string `json:"payloadType"`
		Payload     string `json:"payload"`
		Signatures  []struct {
			KeyID string `json:"keyid"`
			Sig   string `json:"sig"`
		} `json:"signatures"`
	}

	if err := json.Unmarshal(envBytes, &envelope); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Verification Failed: Malformed attestation envelope: %v\n", err)
		os.Exit(1)
	}

	stmtBytes, err := base64.StdEncoding.DecodeString(envelope.Payload)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Verification Failed: Invalid base64 payload\n")
		os.Exit(1)
	}

	var stmt struct {
		Subject []struct {
			Name   string            `json:"name"`
			Digest map[string]string `json:"digest"`
		} `json:"subject"`
		Predicate struct {
			Decision   string    `json:"decision"`
			ScanID     string    `json:"scan_id"`
			Scanners   []string  `json:"scanners"`
			PolicyHash string    `json:"policy_hash"`
			IssuedAt   time.Time `json:"issued_at"`
			Issuer     string    `json:"issuer"`
		} `json:"predicate"`
	}

	if err := json.Unmarshal(stmtBytes, &stmt); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Verification Failed: Invalid statement structure: %v\n", err)
		os.Exit(1)
	}

	// Verify cryptographic signature if public key is provided
	if pubKeyStr != "" {
		var pubKey ed25519.PublicKey
		if fileContent, err := os.ReadFile(pubKeyStr); err == nil {
			pubKeyStr = strings.TrimSpace(string(fileContent))
		}
		keyBytes, err := hex.DecodeString(strings.TrimSpace(pubKeyStr))
		if err != nil || len(keyBytes) != ed25519.PublicKeySize {
			fmt.Fprintf(os.Stderr, "Error: Invalid public key (must be 32-byte hex string)\n")
			os.Exit(1)
		}
		pubKey = ed25519.PublicKey(keyBytes)

		sigBytes, _ := base64.StdEncoding.DecodeString(envelope.Signatures[0].Sig)
		if !ed25519.Verify(pubKey, stmtBytes, sigBytes) {
			fmt.Fprintf(os.Stderr, "❌ SIGNATURE REJECTED: Attestation signature does NOT match public key!\n")
			os.Exit(1)
		}
	}

	fmt.Printf("✅ VERIFIED KURO CRYPTOGRAPHIC ATTESTATION\n")
	fmt.Printf("──────────────────────────────────────────\n")
	fmt.Printf("Decision:     %s\n", stmt.Predicate.Decision)
	fmt.Printf("Scan ID:      %s\n", stmt.Predicate.ScanID)
	fmt.Printf("Scanners:     %s\n", strings.Join(stmt.Predicate.Scanners, ", "))
	fmt.Printf("Issued By:    %s\n", stmt.Predicate.Issuer)
	fmt.Printf("Issued At:    %s\n", stmt.Predicate.IssuedAt.Format(time.RFC3339))
	if len(stmt.Subject) > 0 {
		fmt.Printf("Commit SHA:   %s\n", stmt.Subject[0].Digest["commit"])
	}

	if stmt.Predicate.Decision != "PASS" && stmt.Predicate.Decision != "APPROVED" {
		fmt.Printf("\n⚠️  WARNING: Gatekeeper decision was %s\n", stmt.Predicate.Decision)
		os.Exit(2)
	}
}
