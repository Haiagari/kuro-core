package main

import (
	"fmt"
	"os"
	"strings"

	"kuro/cli/cmd"
)

var Version = "v0.1.1"

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
	case "proxy":
		cmd.RunProxy(os.Args[2:])
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

╭───────────────────────────────────────────────────────────╮
│                                                            │
│                        KURO CORE                            │
│              Local-first security gate  ` + Version + `              │
│                                                            │
├───────────────────────────────────────────────────────────┤
│                                                            │
│  ┌─ CORE (local, zero server) ─────────────────────────┐   │
│  │                                                     │   │
│  │  scan <path> [--json] [--history]                   │   │
│  │    Scan locally via Docker/Podman scanners          │   │
│  │  fix [path] [--dry-run|--auto]                      │   │
│  │    Interactive secret remediation (Bubbletea TUI)   │   │
│  │  canary generate|inject|verify|list                 │   │
│  │    Honeypot / canary token deception                │   │
│  │  attest verify|keygen|inspect                       │   │
│  │    in-toto / SLSA provenance verification           │   │
│  │  doctor [--json]                                    │   │
│  │    Runtime diagnostics (Docker/Podman, tools)       │   │
│  │  license status|apply <token>                       │   │
│  │    Show tier / apply enterprise key                 │   │
│  │                                                     │   │
│  └────────────────────────────────────────────────────┘   │
│                                                            │
│  ┌─ LOCAL GIT PROXY ───────────────────────────────────┐   │
│  │                                                     │   │
│  │  proxy [--addr :8000] [--upstream URL]              │   │
│  │    Fail-closed pre-push gate (default :8000)        │   │
│  │  Then point a remote at http://localhost:8000/...   │   │
│  │                                                     │   │
│  └────────────────────────────────────────────────────┘   │
│                                                            │
│  ┌─ OPTIONAL (server / Enterprise companion) ──────────┐   │
│  │                                                     │   │
│  │  auth · status · backup · webhook · update          │   │
│  │  deploy · setup · health · up                       │   │
│  │  scan --remote | scan <url>   (needs API key)       │   │
│  │  Prefer Kuro Enterprise for multi-tenant stacks     │   │
│  │                                                     │   │
│  └────────────────────────────────────────────────────┘   │
│                                                            │
│  ┌─ OTHER ─────────────────────────────────────────────┐   │
│  │  version · help                                     │   │
│  └────────────────────────────────────────────────────┘   │
│                                                            │
├───────────────────────────────────────────────────────────┤
│  EXAMPLES:                                                 │
│    kuro doctor                                             │
│    kuro scan ./my-project                                  │
│    kuro scan ./my-project --json                           │
│    kuro fix ./my-project --dry-run                         │
│    kuro canary generate --type aws --format env            │
│    kuro proxy                                               │
│                                                            │
╰───────────────────────────────────────────────────────────╯
`))
}
