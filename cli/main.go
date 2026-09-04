package main

import (
	"fmt"
	"os"
	"strings"

	"kuro/cli/cmd"
)

const Version = "v1.4.1"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	subcommand := os.Args[1]

	switch subcommand {
	case "auth":
		cmd.RunAuth(os.Args[2:])
	case "scan":
		cmd.RunScan(os.Args[2:])
	case "status":
		cmd.RunStatus(os.Args[2:])
	case "backup":
		cmd.RunBackup(os.Args[2:])
	case "webhook":
		cmd.RunWebhook(os.Args[2:])
	case "update":
		cmd.RunUpdate(os.Args[2:])
	case "deploy":
		cmd.RunDeploy(os.Args[2:])
	case "setup":
		cmd.RunSetup(os.Args[2:])
	case "doctor":
		cmd.RunDoctor(os.Args[2:])
	case "health":
		cmd.RunHealth(os.Args[2:])
	case "up":
		cmd.RunUp(os.Args[2:])
	case "canary", "honeypot":
		cmd.RunCanary(os.Args[2:])
	case "attest":
		cmd.RunAttest(os.Args[2:])
	case "fix", "triage":
		cmd.RunFix(os.Args[2:])
	case "license":
		cmd.RunLicense(os.Args[2:])
	case "version":
		fmt.Printf("kuro version %s\n", Version)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown command %q\n", subcommand)
		fmt.Fprintln(os.Stderr, "Run 'kuro help' for usage.")
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(strings.TrimSpace(`

╭────────────────────────────────────────────────────────────╮
│                                                            │
│                       KURO Pipeline                         │
│                    CLI v` + Version + `                    │
│                                                            │
├────────────────────────────────────────────────────────────┤
│                                                            │
│  ┌─ MANAGEMENT ───────────────────────────────────────────┐   │
│  │                                                     │   │
│  │  deploy [--tls] [--ollama]    Deploy Kuro         │   │
│  │  setup <component>           Configure component │   │
│  │    setup images               Download images   │   │
│  │  health [--watch]             Check health       │   │
│  │  up [--flags]                Manage the stack    │   │
│  │    --down                    Stop services     │   │
│  │    --status                  Service status   │   │
│  │    --minimal                 Postgres + API only   │   │
│  │    --no-dash                 No dashboard         │   │
│  │    --no-nats                 No NATS              │   │
│  │                                                     │   │
│  └─────────────────────────────────────────────────────┘   │
│                                                            │
│  ┌─ SCAN ────────────────────────────────────────────┐   │
│  │                                                     │   │
│  │  scan <target>     Scan code                  │   │
│  │    Local:  ./path    No server (Docker/Podman)   │   │
│  │    Remote: <url>     Sends to Kuro server          │   │
│  │    --remote          Force remote mode             │   │
│  │    --json            JSON output                    │   │
│  │  status <scan-id>   View scan result           │   │
│  │    --json            JSON output                    │   │
│  │                                                     │   │
│  └─────────────────────────────────────────────────────┘   │
│                                                            │
│  ┌─ CONFIG ─────────────────────────────────────┐   │
│  │                                                     │   │
│  │  auth <api-key>              Save API key        │   │
│  │  backup list                 List backups         │   │
│  │  backup restore <file>    Restore backup       │   │
│  │  webhook list                List webhooks        │   │
│  │  webhook add <flags>         Add webhook        │   │
│  │  webhook delete <id>         Delete webhook       │   │
│  │  webhook toggle <id>         Enable/disable     │   │
│  │                                                     │   │
│  └─────────────────────────────────────────────────────┘   │
│                                                            │
│  ┌─ DECEPTION & CANARY ────────────────────────────────┐   │
│  │                                                     │   │
│  │  canary generate [flags]   Create honeypot key      │   │
│  │  canary inject <dir>       Inject into fixtures     │   │
│  │  canary verify <token>     Verify canary signature  │   │
│  │  canary list               List active canaries     │   │
│  │                                                     │   │
│  └─────────────────────────────────────────────────────┘   │
│                                                            │
│  ┌─ ATTESTATION & SLSA ────────────────────────────────┐   │
│  │                                                     │   │
│  │  attest verify [flags]     Verify in-toto commit sig│   │
│  │  attest keygen             Generate Ed25519 keypair │   │
│  │  attest inspect <file>     Decode in-toto statement │   │
│  │                                                     │   │
│  └─────────────────────────────────────────────────────┘   │
│                                                            │
│  ┌─ REMEDIATION & AUTO-FIX ────────────────────────────┐   │
│  │                                                     │   │
│  │  fix [path] [--dry-run]    Interactive auto-fix TUI │   │
│  │  fix --auto                Auto-extract env secrets │   │
│  │                                                     │   │
│  └─────────────────────────────────────────────────────┘   │
│                                                            │
│  ┌─ LICENSE & ENTERPRISE ──────────────────────────────┐   │
│  │                                                     │   │
│  │  license status            Show active tier & caps  │   │
│  │  license apply <token>     Install enterprise key   │   │
│  │                                                     │   │
│  └─────────────────────────────────────────────────────┘   │
│                                                            │
│  ┌─ DIAGNOSTICS ───────────────────────────────────────┐   │
│  │                                                     │   │
│  │  doctor [--json]           System diagnostics       │   │
│  │                                                     │   │
│  └─────────────────────────────────────────────────────┘   │
│                                                            │
│  ┌─ OTHER ─────────────────────────────────────────────┐   │
│  │                                                     │   │
│  │  update                        Update CLI       │   │
│  │  version                       Show version      │   │
│  │  help                          Show this help   │   │
│  │                                                     │   │
│  └─────────────────────────────────────────────────────┘   │
│                                                            │
├────────────────────────────────────────────────────────────┤
│                                                            │
│  EXAMPLES:                                                 │
│    kuro deploy                                              │
│    kuro deploy --tls --ollama                               │
│    kuro setup tls --status                                  │
│    kuro health --watch                                      │
│    kuro scan ./my-project                                  │
│    kuro scan https://github.com/user/repo.git               │
│    kuro scan --remote ./my-project                         │
│                                                            │
╰────────────────────────────────────────────────────────────╯
`))
}
