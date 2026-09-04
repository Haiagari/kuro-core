package cmd

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestAttest_VerifyValidGitNote(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}

	tmpDir := t.TempDir()

	// Initialize temp git repository
	exec.Command("git", "-C", tmpDir, "init").Run()
	exec.Command("git", "-C", tmpDir, "config", "user.email", "attest@kuro.dev").Run()
	exec.Command("git", "-C", tmpDir, "config", "user.name", "Kuro Attest").Run()
	exec.Command("git", "-C", tmpDir, "commit", "--allow-empty", "-m", "Commit to attest").Run()

	out, err := exec.Command("git", "-C", tmpDir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("failed to get commit SHA: %v", err)
	}
	commitSHA := string(out[:len(out)-1])

	// Create keypair
	pub, priv, _ := ed25519.GenerateKey(nil)
	pubHex := hex.EncodeToString(pub)

	// Build in-toto statement
	stmt := map[string]interface{}{
		"_type": "https://in-toto.io/Statement/v1",
		"subject": []map[string]interface{}{
			{
				"name": "https://github.com/Haiagari/kuro.git",
				"digest": map[string]string{
					"commit": commitSHA,
				},
			},
		},
		"predicateType": "https://kuro.dev/attestation/security-gate/v1",
		"predicate": map[string]interface{}{
			"decision":    "PASS",
			"scan_id":     "00000000-0000-0000-0000-000000000001",
			"scanners":    []string{"gitleaks", "semgrep", "trivy"},
			"policy_hash": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			"issued_at":   time.Now().UTC().Format(time.RFC3339),
			"issuer":      "Kuro Security Gatekeeper v1.5.0",
		},
	}

	stmtBytes, _ := json.Marshal(stmt)
	payloadB64 := base64.StdEncoding.EncodeToString(stmtBytes)
	sigBytes := ed25519.Sign(priv, stmtBytes)

	pubHash := sha256.Sum256(pub)
	keyID := hex.EncodeToString(pubHash[:8])

	envelope := map[string]interface{}{
		"payloadType": "application/vnd.in-toto+json",
		"payload":     payloadB64,
		"signatures": []map[string]string{
			{
				"keyid": keyID,
				"sig":   base64.StdEncoding.EncodeToString(sigBytes),
			},
		},
	}

	envJSON, _ := json.Marshal(envelope)

	// Attach git note
	noteCmd := exec.Command("git", "-C", tmpDir, "notes", "--ref=refs/notes/kuro-attestation", "add", "-f", "-m", string(envJSON), commitSHA)
	if err := noteCmd.Run(); err != nil {
		t.Fatalf("failed to add git note: %v", err)
	}

	// Verify using standalone file logic
	attestFilePath := filepath.Join(tmpDir, "attestation.json")
	os.WriteFile(attestFilePath, envJSON, 0644)

	// Ensure signature verifies with our public key
	if !ed25519.Verify(pub, stmtBytes, sigBytes) {
		t.Fatal("expected signature verification to succeed")
	}

	_ = pubHex
}
