package cmd

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const GitNotesRef = "refs/notes/kuro-attestation"
const attestationPayloadType = "application/vnd.in-toto+json"
const attestationPredicateType = "https://kuro.dev/attestation/security-gate/v1"

type attestationEnvelope struct {
	PayloadType string `json:"payloadType"`
	Payload     string `json:"payload"`
	Signatures  []struct {
		KeyID string `json:"keyid"`
		Sig   string `json:"sig"`
	} `json:"signatures"`
}
type attestationStatement struct {
	Type          string `json:"_type"`
	PredicateType string `json:"predicateType"`
	Subject       []struct {
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

func RunAttest(args []string) {
	if len(args) == 0 {
		printAttestUsage()
		return
	}
	var err error
	switch args[0] {
	case "verify":
		var code int
		code, err = verifyAttestationCommand(args[1:])
		if err == nil && code != 0 {
			os.Exit(code)
		}
	case "keygen":
		runAttestKeygen(args[1:])
	case "inspect":
		err = inspectAttestationCommand(args[1:])
	case "help", "--help", "-h":
		printAttestUsage()
	default:
		err = fmt.Errorf("unknown attest subcommand %q", args[0])
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Attestation error: %v\n", err)
		os.Exit(1)
	}
}
func printAttestUsage() {
	fmt.Print(`USAGE:
  kuro attest verify --pubkey <hex|file> [--commit SHA|ref] [--repo path] [--file envelope.json] [--legacy-raw]
  kuro attest inspect <envelope.json>  Decode only; no proof of authenticity
  kuro attest keygen                  Generate a local Ed25519 keypair

verify requires a trusted public key and binds the signed subject to --commit
(default HEAD in --repo, default .). DSSE PAE is required unless --legacy-raw
explicitly selects the historical raw-payload format. No SLSA level is asserted.
`)
}
func runAttestKeygen(args []string) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Key generation failed: %v\n", err)
		os.Exit(1)
	}
	fingerprint := sha256.Sum256(pub)
	fmt.Printf("Key ID: %s\nPUBLIC KEY:\n%s\nPRIVATE KEY (keep secret):\n%s\n", hex.EncodeToString(fingerprint[:8]), hex.EncodeToString(pub), hex.EncodeToString(priv))
}
func decodeAttestationBase64(s string) ([]byte, error) {
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.URLEncoding, base64.RawStdEncoding, base64.RawURLEncoding} {
		if b, err := enc.DecodeString(s); err == nil {
			return b, nil
		}
	}
	return nil, fmt.Errorf("invalid base64 encoding")
}
func decodeAttestation(data []byte) (attestationEnvelope, []byte, error) {
	var env attestationEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return env, nil, err
	}
	if env.PayloadType != attestationPayloadType || env.Payload == "" {
		return env, nil, fmt.Errorf("unsupported payloadType or missing payload")
	}
	payload, err := decodeAttestationBase64(env.Payload)
	if err != nil {
		return env, nil, err
	}
	if !json.Valid(payload) {
		return env, nil, fmt.Errorf("payload is not valid JSON")
	}
	return env, payload, nil
}
func inspectAttestationCommand(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("inspect requires one file path")
	}
	data, err := os.ReadFile(args[0])
	if err != nil {
		return err
	}
	_, payload, err := decodeAttestation(data)
	if err != nil {
		return err
	}
	var value any
	if err := json.Unmarshal(payload, &value); err != nil {
		return err
	}
	pretty, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println("UNVERIFIED CONTENT — signature and commit binding were not checked")
	fmt.Println(string(pretty))
	return nil
}
func dssePAE(payloadType string, payload []byte) []byte {
	prefix := fmt.Sprintf("DSSEv1 %d %s %d ", len([]byte(payloadType)), payloadType, len(payload))
	return append([]byte(prefix), payload...)
}
func isCommitDigest(s string) bool {
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}
func verifyAttestation(data []byte, pub ed25519.PublicKey, expectedCommit string, legacyRaw bool) (*attestationStatement, error) {
	if len(pub) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("a trusted 32-byte public key is required")
	}
	if !isCommitDigest(expectedCommit) {
		return nil, fmt.Errorf("expected commit must be a full SHA-1 or SHA-256 digest")
	}
	env, payload, err := decodeAttestation(data)
	if err != nil {
		return nil, err
	}
	signed := dssePAE(env.PayloadType, payload)
	if legacyRaw {
		signed = payload
	}
	verified := false
	for _, sig := range env.Signatures {
		b, err := decodeAttestationBase64(sig.Sig)
		if err == nil && len(b) == ed25519.SignatureSize && ed25519.Verify(pub, signed, b) {
			verified = true
			break
		}
	}
	if !verified {
		return nil, fmt.Errorf("no signature matches the trusted key and selected signature format")
	}
	var stmt attestationStatement
	if err := json.Unmarshal(payload, &stmt); err != nil {
		return nil, err
	}
	if stmt.Type != "https://in-toto.io/Statement/v1" || stmt.PredicateType != attestationPredicateType {
		return nil, fmt.Errorf("unsupported statement or predicate type")
	}
	if len(stmt.Subject) == 0 {
		return nil, fmt.Errorf("statement has no subjects")
	}
	bound := false
	for _, subject := range stmt.Subject {
		commit := subject.Digest["commit"]
		if subject.Name == "" || !isCommitDigest(commit) {
			return nil, fmt.Errorf("subject requires a name and full commit digest")
		}
		if strings.EqualFold(commit, expectedCommit) {
			bound = true
		}
	}
	if !bound {
		return nil, fmt.Errorf("signed subject does not match expected commit %s", expectedCommit)
	}
	p := stmt.Predicate
	hash, err := hex.DecodeString(p.PolicyHash)
	if p.Decision == "" || p.ScanID == "" || len(p.Scanners) == 0 || p.Issuer == "" || p.IssuedAt.IsZero() || err != nil || len(hash) != sha256.Size {
		return nil, fmt.Errorf("incomplete security-gate predicate")
	}
	switch strings.ToUpper(p.Decision) {
	case "PASS", "APPROVED", "REVIEW", "BLOCK", "BLOCKED", "DENY":
	default:
		return nil, fmt.Errorf("unknown gate decision")
	}
	return &stmt, nil
}
func resolveAttestationCommit(repo, ref string) (string, error) {
	cmd := exec.Command("git", "-C", repo, "rev-parse", "--verify", "--end-of-options", ref+"^{commit}")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("resolve commit %q: %w", ref, err)
	}
	digest := strings.TrimSpace(string(out))
	if !isCommitDigest(digest) {
		return "", fmt.Errorf("invalid resolved commit")
	}
	return digest, nil
}
func verifyAttestationCommand(args []string) (int, error) {
	fs := flag.NewFlagSet("attest verify", flag.ContinueOnError)
	commit := fs.String("commit", "HEAD", "expected commit")
	repo := fs.String("repo", ".", "repository path")
	pubkey := fs.String("pubkey", os.Getenv("KURO_ATTESTATION_PUBLIC_KEY"), "trusted public key")
	file := fs.String("file", "", "standalone envelope")
	legacy := fs.Bool("legacy-raw", false, "explicit historical raw-payload format")
	// Preserve documented short aliases.
	fs.StringVar(commit, "c", "HEAD", "expected commit")
	fs.StringVar(repo, "r", ".", "repository path")
	fs.StringVar(pubkey, "p", os.Getenv("KURO_ATTESTATION_PUBLIC_KEY"), "trusted key")
	fs.StringVar(file, "f", "", "standalone envelope")
	if err := fs.Parse(args); err != nil {
		return 1, err
	}
	if fs.NArg() != 0 {
		return 1, fmt.Errorf("unexpected positional arguments")
	}
	if *pubkey == "" {
		return 1, fmt.Errorf("verify requires --pubkey or KURO_ATTESTATION_PUBLIC_KEY; use inspect for decoding")
	}
	key := *pubkey
	if b, err := os.ReadFile(key); err == nil {
		key = strings.TrimSpace(string(b))
	}
	keyBytes, err := hex.DecodeString(strings.TrimSpace(key))
	if err != nil || len(keyBytes) != ed25519.PublicKeySize {
		return 1, fmt.Errorf("public key must be 32-byte hex or a file containing it")
	}
	expected := *commit
	if *file == "" || !isCommitDigest(expected) {
		expected, err = resolveAttestationCommit(*repo, *commit)
		if err != nil {
			return 1, err
		}
	}
	var data []byte
	if *file != "" {
		data, err = os.ReadFile(*file)
	} else {
		data, err = exec.Command("git", "-C", *repo, "notes", "--ref="+GitNotesRef, "show", expected).Output()
	}
	if err != nil {
		return 1, fmt.Errorf("read attestation: %w", err)
	}
	stmt, err := verifyAttestation(data, ed25519.PublicKey(keyBytes), expected, *legacy)
	if err != nil {
		return 1, err
	}
	format := "DSSE PAE"
	if *legacy {
		format = "LEGACY raw-payload (not DSSE)"
	}
	fmt.Printf("VERIFIED SIGNATURE AND COMMIT BINDING (%s)\nCommit: %s\nDecision: %s\nIssuer: %s\nIssued at: %s\n", format, expected, stmt.Predicate.Decision, stmt.Predicate.Issuer, stmt.Predicate.IssuedAt.Format(time.RFC3339))
	if !strings.EqualFold(stmt.Predicate.Decision, "PASS") && !strings.EqualFold(stmt.Predicate.Decision, "APPROVED") {
		return 2, nil
	}
	return 0, nil
}
