package cmd

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	CanarySecretSalt = "kuro-deception-engine-salt-v1"
	CanarySignaturePrefix = "KURO_CANARY"
)

// CanaryTokenMetadata holds the metadata embedded within or associated with a canary honeypot token.
type CanaryTokenMetadata struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`      // aws, github, slack, jwt, generic
	Memo      string    `json:"memo"`      // context or repo location
	CreatedAt time.Time `json:"created_at"`
	Signature string    `json:"signature"` // HMAC-SHA256 signature
	TargetFile string   `json:"target_file,omitempty"`
}

// CanaryManifest stores all injected canary tokens within a repository.
type CanaryManifest struct {
	Version   string                `json:"version"`
	UpdatedAt time.Time             `json:"updated_at"`
	Tokens    []CanaryTokenMetadata `json:"tokens"`
}

// RunCanary handles the 'kuro canary' subcommand family.
func RunCanary(args []string) {
	if len(args) == 0 {
		printCanaryUsage()
		return
	}

	sub := args[0]
	switch sub {
	case "generate":
		runCanaryGenerate(args[1:])
	case "inject":
		runCanaryInject(args[1:])
	case "verify":
		runCanaryVerify(args[1:])
	case "list":
		runCanaryList(args[1:])
	case "help", "--help", "-h":
		printCanaryUsage()
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown canary subcommand %q\n", sub)
		printCanaryUsage()
		os.Exit(1)
	}
}

func printCanaryUsage() {
	fmt.Print(`
╭────────────────────────────────────────────────────────────╮
│                  KURO Cyber Deception Engine               │
│                  Honeypot & Canary Tokens                  │
╰────────────────────────────────────────────────────────────╯

USAGE:
  kuro canary generate [flags]     Generate a standalone honeypot credential
  kuro canary inject <dir> [flags] Injected honeypot token into test fixtures
  kuro canary verify <token|file>  Check and decode a canary token signature
  kuro canary list [dir]           List all tracked canary tokens in workspace

FLAGS (generate / inject):
  --type <type>        Token type: aws (default), github, slack, jwt, generic
  --memo <text>        Description / attribution tag
  --format <fmt>       Output format: env (default), json, yaml, tf
  --output <file>      File path to write the generated token
`)
}

// GenerateCanaryToken creates a realistic canary credential with embedded HMAC signature.
func GenerateCanaryToken(tokenType, memo string) (token string, secret string, meta CanaryTokenMetadata) {
	randBytes := make([]byte, 16)
	rand.Read(randBytes)
	tokenID := hex.EncodeToString(randBytes)[:12]
	now := time.Now().UTC()

	// Compute HMAC signature over ID + Type + Timestamp
	mac := hmac.New(sha256.New, []byte(CanarySecretSalt))
	mac.Write([]byte(fmt.Sprintf("%s:%s:%d", tokenID, tokenType, now.Unix())))
	sig := hex.EncodeToString(mac.Sum(nil))[:16]

	meta = CanaryTokenMetadata{
		ID:        tokenID,
		Type:      strings.ToLower(tokenType),
		Memo:      memo,
		CreatedAt: now,
		Signature: sig,
	}

	switch strings.ToLower(tokenType) {
	case "github":
		// Realistic GitHub personal access token (40 chars)
		token = fmt.Sprintf("ghp_kuro%s%s", tokenID, sig)
		secret = token
	case "slack":
		// Realistic Slack incoming webhook
		token = fmt.Sprintf("https://hooks.slack.com/services/T00000000/B00000000/kuro%s%s", tokenID, sig)
		secret = token
	case "jwt":
		// Mock signed JWT with canary claims
		header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
		claims := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(`{"iss":"kuro-canary","sub":"%s","sig":"%s","iat":%d}`, tokenID, sig, now.Unix())))
		signature := base64.RawURLEncoding.EncodeToString([]byte(sig))
		token = fmt.Sprintf("%s.%s.%s", header, claims, signature)
		secret = token
	case "generic":
		token = fmt.Sprintf("kuro_live_canary_%s_%s", tokenID, sig)
		secret = token
	case "aws":
		fallthrough
	default:
		// Realistic AWS Access Key ID (20 chars starting with AKIA)
		token = fmt.Sprintf("AKIA%s%s", strings.ToUpper(tokenID)[:8], strings.ToUpper(sig)[:8])
		// Realistic AWS Secret Access Key (40 chars base64-like)
		secret = fmt.Sprintf("wJalrXUtnFEMI/K7MDENG/bPxRfiCY%s", tokenID)
	}

	return token, secret, meta
}

func runCanaryGenerate(args []string) {
	tokenType := "aws"
	memo := "Standalone honeypot token"
	format := "env"
	outputFile := ""

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--type":
			if i+1 < len(args) { tokenType = args[i+1]; i++ }
		case "--memo":
			if i+1 < len(args) { memo = args[i+1]; i++ }
		case "--format":
			if i+1 < len(args) { format = args[i+1]; i++ }
		case "--output", "-o":
			if i+1 < len(args) { outputFile = args[i+1]; i++ }
		}
	}

	token, secret, meta := GenerateCanaryToken(tokenType, memo)
	content := formatCanaryOutput(tokenType, token, secret, meta, format)

	if outputFile != "" {
		if err := os.WriteFile(outputFile, []byte(content), 0600); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing output file: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✅ Canary token [%s] written to %s\n", meta.ID, outputFile)
	} else {
		fmt.Printf("╭────────────────────────────────────────────────────────────╮\n")
		fmt.Printf("│ Generated Kuro Canary Token [%s] (%s)\n", meta.ID, strings.ToUpper(tokenType))
		fmt.Printf("╰────────────────────────────────────────────────────────────╯\n")
		fmt.Println(content)
	}
}

func formatCanaryOutput(tokenType, token, secret string, meta CanaryTokenMetadata, format string) string {
	switch strings.ToLower(format) {
	case "json":
		data := map[string]interface{}{
			"canary_metadata": meta,
			"credentials": map[string]string{
				"key":    token,
				"secret": secret,
			},
		}
		out, _ := json.MarshalIndent(data, "", "  ")
		return string(out)
	case "yaml", "yml":
		return fmt.Sprintf("# Kuro Canary Token [%s] — %s\ncanary:\n  id: %q\n  type: %q\n  key: %q\n  secret: %q\n",
			meta.ID, meta.CreatedAt.Format(time.RFC3339), meta.ID, meta.Type, token, secret)
	case "tf", "terraform":
		return fmt.Sprintf("# Kuro Canary Honeypot Variable [%s]\nvariable %q {\n  default = %q\n  description = \"Canary token: %s\"\n}\n",
			meta.ID, "canary_api_key", token, meta.Memo)
	case "env":
		fallthrough
	default:
		switch strings.ToLower(tokenType) {
		case "aws":
			return fmt.Sprintf("# Kuro Canary Honeypot Token [%s]\nAWS_ACCESS_KEY_ID=%s\nAWS_SECRET_ACCESS_KEY=%s\nAWS_DEFAULT_REGION=us-east-1\n",
				meta.ID, token, secret)
		case "github":
			return fmt.Sprintf("# Kuro Canary Honeypot Token [%s]\nGITHUB_TOKEN=%s\n", meta.ID, token)
		case "slack":
			return fmt.Sprintf("# Kuro Canary Honeypot Token [%s]\nSLACK_WEBHOOK_URL=%s\n", meta.ID, token)
		default:
			return fmt.Sprintf("# Kuro Canary Honeypot Token [%s]\nAPI_KEY=%s\nSECRET_KEY=%s\n", meta.ID, token, secret)
		}
	}
}

func runCanaryInject(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Error: directory path required for inject. Example: kuro canary inject ./tests/fixtures")
		os.Exit(1)
	}

	targetDir := args[0]
	tokenType := "aws"
	memo := "Injected test fixture canary"
	format := "env"

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--type":
			if i+1 < len(args) { tokenType = args[i+1]; i++ }
		case "--memo":
			if i+1 < len(args) { memo = args[i+1]; i++ }
		case "--format":
			if i+1 < len(args) { format = args[i+1]; i++ }
		}
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating target directory: %v\n", err)
		os.Exit(1)
	}

	token, secret, meta := GenerateCanaryToken(tokenType, memo)
	fileName := "canary_secrets.env"
	if format == "json" {
		fileName = "canary_credentials.json"
	}
	targetFilePath := filepath.Join(targetDir, fileName)
	meta.TargetFile = targetFilePath

	content := formatCanaryOutput(tokenType, token, secret, meta, format)
	if err := os.WriteFile(targetFilePath, []byte(content), 0600); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing canary file: %v\n", err)
		os.Exit(1)
	}

	// Update local .canary-manifest.json in current or target directory
	manifestPath := ".canary-manifest.json"
	manifest := loadCanaryManifest(manifestPath)
	manifest.UpdatedAt = time.Now().UTC()
	manifest.Tokens = append(manifest.Tokens, meta)
	saveCanaryManifest(manifestPath, manifest)

	fmt.Printf("🎯 Injected canary token [%s] into %s\n", meta.ID, targetFilePath)
	fmt.Printf("📝 Recorded in %s (Active tokens: %d)\n", manifestPath, len(manifest.Tokens))
}

func runCanaryVerify(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Error: token string or file path required for verification")
		os.Exit(1)
	}

	input := args[0]
	// Check if input is a file
	if fileBytes, err := os.ReadFile(input); err == nil {
		input = string(fileBytes)
	}

	foundCount := 0
	lines := strings.Split(input, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Search for canary patterns
		if isCanarySignature(line) {
			foundCount++
			fmt.Printf("✅ Kuro Canary Detected in: %s\n", line)
		}
	}

	if foundCount == 0 {
		if isCanarySignature(input) {
			fmt.Printf("✅ Valid Kuro Canary Token Signature Detected\n")
		} else {
			fmt.Println("ℹ️  No Kuro Canary Token signatures detected.")
		}
	}
}

func isCanarySignature(token string) bool {
	return strings.Contains(token, "kuro") || strings.Contains(token, "KURO") ||
		strings.Contains(token, "ghp_kuro") || strings.Contains(token, "kuro_live_canary") ||
		strings.HasPrefix(token, "AKIA")
}

func runCanaryList(args []string) {
	manifestPath := ".canary-manifest.json"
	if len(args) > 0 {
		manifestPath = filepath.Join(args[0], ".canary-manifest.json")
	}

	manifest := loadCanaryManifest(manifestPath)
	if len(manifest.Tokens) == 0 {
		fmt.Println("No active canary tokens found in manifest.")
		return
	}

	fmt.Printf("╭────────────────────────────────────────────────────────────╮\n")
	fmt.Printf("│ Tracked Canary Tokens (%d active)                          │\n", len(manifest.Tokens))
	fmt.Printf("╰────────────────────────────────────────────────────────────╯\n")
	for i, t := range manifest.Tokens {
		fmt.Printf("[%d] ID: %s | Type: %-8s | Created: %s | File: %s\n    Memo: %s\n",
			i+1, t.ID, t.Type, t.CreatedAt.Format("2006-01-02 15:04"), t.TargetFile, t.Memo)
	}
}

func loadCanaryManifest(path string) CanaryManifest {
	var m CanaryManifest
	m.Version = "v1.0"
	bytes, err := os.ReadFile(path)
	if err != nil {
		return m
	}
	json.Unmarshal(bytes, &m)
	return m
}

func saveCanaryManifest(path string, m CanaryManifest) {
	bytes, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(path, bytes, 0644)
}
