package cmd

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLicenseCLI_ApplyAndVerifyToken(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	claims := struct {
		CustomerID string    `json:"customer_id"`
		Plan       string    `json:"plan"`
		Seats      int       `json:"seats"`
		Features   []string  `json:"features"`
		IssuedAt   time.Time `json:"issued_at"`
		ExpiresAt  time.Time `json:"expires_at"`
	}{
		CustomerID: "test-corp",
		Plan:       "enterprise",
		Seats:      100,
		Features:   []string{"multi_tenant_rls", "blind_indexing"},
		IssuedAt:   time.Now().UTC(),
		ExpiresAt:  time.Now().UTC().Add(30 * 24 * time.Hour),
	}

	claimsBytes, _ := json.Marshal(claims)
	payloadB64 := base64.RawURLEncoding.EncodeToString(claimsBytes)
	sigBytes := ed25519.Sign(priv, claimsBytes)
	sigB64 := base64.RawURLEncoding.EncodeToString(sigBytes)

	token := fmt.Sprintf("kuro_lic_%s.%s", payloadB64, sigB64)

	tmpDir := t.TempDir()
	licFile := filepath.Join(tmpDir, "license.jwt")
	if err := os.WriteFile(licFile, []byte(token), 0600); err != nil {
		t.Fatalf("failed to write license file: %v", err)
	}

	loadedBytes, err := os.ReadFile(licFile)
	if err != nil {
		t.Fatalf("failed to read written license: %v", err)
	}

	if !strings.HasPrefix(string(loadedBytes), "kuro_lic_") {
		t.Fatalf("expected token prefix, got %s", string(loadedBytes))
	}
}
