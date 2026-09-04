# CLI & Local Proxy Reference — Kuro Core v0.1.0

> **Edition**: Kuro Core (Standalone Edition) · **Version**: v0.1.0

---

## CLI Commands

### 1. `kuro scan`
Runs a multi-scanner audit over local source code:
```bash
kuro scan ./path/to/project             # Interactive Bubbletea TUI scan
kuro scan ./path/to/project --json      # Export findings as machine-readable JSON
kuro scan ./path/to/project --history   # Deep git history scan
```

### 2. `kuro fix`
Interactive terminal secret extraction and remediation:
```bash
kuro fix [path] --dry-run               # Preview transformations safely
kuro fix [path] --auto                  # Unattended batch migration of secrets to env vars
```

### 3. `kuro canary`
Generates and audits honeypot decoy credentials:
```bash
kuro canary generate --type aws         # Create decoy AWS credentials
kuro canary generate --type github      # Create decoy GitHub PAT
kuro canary verify <token>              # Verify canary authenticity via HMAC signature
kuro canary list [dir]                  # List active canaries in workspace
```

### 4. `kuro doctor`
Runs workstation diagnostics to ensure container runtimes and scanners are operational:
```bash
kuro doctor
kuro doctor --json
```

---

## Local Git Proxy (:8000)

Runs locally on port `8000` to intercept `git push` operations:

- **Protocol**: Smart HTTP (`/owner/repo.git/git-receive-pack`)
- **Mode**: Fail-Closed (`PROXY_FAIL_MODE=closed`)
- **Timeout**: Configurable via `PROXY_SCAN_TIMEOUT` (default: 30 seconds)
- **Exit Behavior**:
  - `HTTP 200`: All checks passed, push forwarded to upstream remote.
  - `HTTP 403`: Push rejected, scan findings streamed directly to git stderr.
