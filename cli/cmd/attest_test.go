package cmd

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const testCommit = "0123456789012345678901234567890123456789"

func signedAttestation(t *testing.T, priv ed25519.PrivateKey, commit, decision string, legacy bool) []byte {
	t.Helper()
	stmt := map[string]any{
		"_type": "https://in-toto.io/Statement/v1", "predicateType": attestationPredicateType,
		"subject":   []any{map[string]any{"name": "https://github.com/Haiagari/kuro-core.git", "digest": map[string]string{"commit": commit}}},
		"predicate": map[string]any{"decision": decision, "scan_id": "test-scan", "scanners": []string{"gitleaks"}, "policy_hash": strings.Repeat("0", 64), "issued_at": time.Now().UTC(), "issuer": "test gate"},
	}
	payload, err := json.Marshal(stmt)
	if err != nil {
		t.Fatal(err)
	}
	signed := dssePAE(attestationPayloadType, payload)
	if legacy {
		signed = payload
	}
	env := map[string]any{"payloadType": attestationPayloadType, "payload": base64.StdEncoding.EncodeToString(payload), "signatures": []any{map[string]string{"sig": base64.StdEncoding.EncodeToString(ed25519.Sign(priv, signed))}}}
	data, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
func mutateEnvelope(t *testing.T, data []byte, mutate func(map[string]any)) []byte {
	t.Helper()
	var env map[string]any
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatal(err)
	}
	mutate(env)
	result, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	return result
}
func TestAttestationSignatureAndCommitBinding(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	otherPub, _, _ := ed25519.GenerateKey(nil)
	data := signedAttestation(t, priv, testCommit, "PASS", false)
	cases := []struct {
		name    string
		data    []byte
		key     ed25519.PublicKey
		commit  string
		legacy  bool
		wantErr bool
	}{
		{"valid", data, pub, testCommit, false, false},
		{"wrong key", data, otherPub, testCommit, false, true},
		{"missing key", data, nil, testCommit, false, true},
		{"wrong commit", data, pub, strings.Repeat("a", 40), false, true},
		{"abbreviated commit", data, pub, "0123456", false, true},
		{"missing signatures", mutateEnvelope(t, data, func(e map[string]any) { e["signatures"] = []any{} }), pub, testCommit, false, true},
		{"malformed signature", mutateEnvelope(t, data, func(e map[string]any) { e["signatures"] = []any{map[string]string{"sig": "!bad"}} }), pub, testCommit, false, true},
		{"wrong type", mutateEnvelope(t, data, func(e map[string]any) { e["payloadType"] = "application/json" }), pub, testCommit, false, true},
		{"changed payload", mutateEnvelope(t, data, func(e map[string]any) { e["payload"] = base64.StdEncoding.EncodeToString([]byte(`{}`)) }), pub, testCommit, false, true},
		{"legacy rejected by default", signedAttestation(t, priv, testCommit, "PASS", true), pub, testCommit, false, true},
		{"legacy explicit", signedAttestation(t, priv, testCommit, "PASS", true), pub, testCommit, true, false},
		{"DSSE rejected in legacy mode", data, pub, testCommit, true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := verifyAttestation(tc.data, tc.key, tc.commit, tc.legacy)
			if (err != nil) != tc.wantErr {
				t.Fatalf("%v", err)
			}
		})
	}
}
func TestAttestationAcceptsLaterValidSignatureAndURLBase64(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	data := signedAttestation(t, priv, testCommit, "PASS", false)
	data = mutateEnvelope(t, data, func(e map[string]any) {
		payload, _ := base64.StdEncoding.DecodeString(e["payload"].(string))
		e["payload"] = base64.RawURLEncoding.EncodeToString(payload)
		signatures := e["signatures"].([]any)
		sig := signatures[0].(map[string]any)
		bytes, _ := base64.StdEncoding.DecodeString(sig["sig"].(string))
		sig["sig"] = base64.RawURLEncoding.EncodeToString(bytes)
		e["signatures"] = append([]any{map[string]string{"sig": "invalid"}}, signatures...)
	})
	if _, err := verifyAttestation(data, pub, testCommit, false); err != nil {
		t.Fatal(err)
	}
}
func TestDSSEPAEReferenceVector(t *testing.T) {
	if got := string(dssePAE("http://example.com/HelloWorld", []byte("hello world"))); got != "DSSEv1 29 http://example.com/HelloWorld 11 hello world" {
		t.Fatal(got)
	}
}
func TestAttestCommandRequiresKeyAndReturnsDecisionCode(t *testing.T) {
	t.Setenv("KURO_ATTESTATION_PUBLIC_KEY", "")
	if _, err := verifyAttestationCommand([]string{"--file", "unused"}); err == nil {
		t.Fatal("verify without key accepted")
	}
	pub, priv, _ := ed25519.GenerateKey(nil)
	file := filepath.Join(t.TempDir(), "attestation.json")
	for _, tc := range []struct {
		decision string
		code     int
	}{{"PASS", 0}, {"review", 2}, {"BLOCK", 2}} {
		os.WriteFile(file, signedAttestation(t, priv, testCommit, tc.decision, false), 0644)
		code, err := verifyAttestationCommand([]string{"--file", file, "--commit", testCommit, "--pubkey", hex.EncodeToString(pub)})
		if err != nil || code != tc.code {
			t.Fatalf("%s code=%d err=%v", tc.decision, code, err)
		}
	}
}
func TestAttestVerifyRealGitNoteAndRejectReplay(t *testing.T) {
	dir := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git: %v %s", err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init")
	git("config", "user.name", "Kuro Test")
	git("config", "user.email", "test@example.invalid")
	git("commit", "--allow-empty", "-m", "first")
	first := git("rev-parse", "HEAD")
	pub, priv, _ := ed25519.GenerateKey(nil)
	data := signedAttestation(t, priv, first, "PASS", false)
	git("notes", "--ref="+GitNotesRef, "add", "-m", string(data), first)
	code, err := verifyAttestationCommand([]string{"--repo", dir, "--pubkey", hex.EncodeToString(pub)})
	if err != nil || code != 0 {
		t.Fatalf("valid note rejected: %v", err)
	}
	git("commit", "--allow-empty", "-m", "second")
	git("notes", "--ref="+GitNotesRef, "add", "-m", string(data), "HEAD")
	if _, err := verifyAttestationCommand([]string{"--repo", dir, "--pubkey", hex.EncodeToString(pub)}); err == nil {
		t.Fatal("attestation copied to another commit accepted")
	}
}
