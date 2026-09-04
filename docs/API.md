# CLI & JSON Interface — Kuro Core v0.1.1

**Audience:** developers integrating Kuro Core into scripts, CI, or the local Git proxy.  
**Scope:** Core CLI contracts (commands, flags, JSON stdout, exit codes).

> **Enterprise / server HTTP API is out of scope for this document.**  
> Optional companion commands (`auth`, `scan --remote`, `webhook`, …) may call a remote Kuro API when configured, but Core’s supported integration surface is the **local CLI**. For multi-tenant HTTP APIs, see [Haiagari/kuro-enterprise](https://github.com/Haiagari/kuro-enterprise).

Related: [ARCHITECTURE.md](ARCHITECTURE.md) · [ATTESTATION.md](ATTESTATION.md) · [../QUICKSTART.md](../QUICKSTART.md)

---

## Process exit codes (`kuro scan`)

| Decision / condition | Exit code |
|---|---|
| `pass` / `approved` / `ok` | `0` |
| `review` | `2` |
| `block` / unknown / scan error | `1` |

Applies to TUI, text, and `--json` modes (`decision_exit.go`, `scanOutput`).

---

## `kuro scan`

```bash
kuro scan <path>|<url> [--json] [--history] [--tui] [--remote]
```

| Flag | Meaning |
|---|---|
| `--json` | Machine-readable JSON on stdout (no TUI) |
| `--history` | Full git history scanners (local only) |
| `--tui` | Force TUI; auto-enabled on TTY when not `--json` |
| `--remote` | Force remote adapter (needs API key — Enterprise companion) |

**Important:** flags may appear **after** the path (`kuro scan ./proj --json`). Core reorders argv so Go’s `flag` parser accepts this (v0.1.1).

### Local vs remote

| Target | Mode |
|---|---|
| `./dir`, `/abs`, `~/…`, existing directory | Local Docker/Podman |
| `--remote` or `https://` / `git@` / `ssh://` | Remote API (requires `kuro auth`) |

### JSON output (local success shape)

```json
{
  "target": "/tmp/project",
  "mode": "local",
  "status": "completed",
  "decision": "pass",
  "duration": "12.3s",
  "findings": [
    {
      "scanner": "gitleaks",
      "severity": "CRITICAL",
      "title": "…",
      "file": ".env",
      "line": 1
    }
  ]
}
```

On hard failure with nil result:

```json
{
  "status": "failed",
  "decision": "",
  "error": "…",
  "findings": []
}
```

**Pitfall for scripts:** parse the **root** object’s `decision`. Nested finding objects do not carry the gate decision (Core E2E teaches this).

### Examples

```bash
kuro scan ./my-project
kuro scan ./my-project --json; echo $?
kuro scan ./my-project --history
kuro scan --json ./my-project    # equivalent ordering
```

---

## `kuro doctor`

```bash
kuro doctor
kuro doctor --json
```

Core checks: container runtime, Git, scanner images, disk space, Kuro binary. Exit `0` allows warnings; critical failures are non-zero.

---

## `kuro fix`

```bash
kuro fix [path] [--dry-run|-d] [--auto|-a|-y]
```

Interactive remediation of hardcoded secrets. `--dry-run` previews; `--auto` applies without prompts. Suppressions → `.kuro-suppressions.json`.

---

## `kuro canary`

```bash
kuro canary generate [--type aws|github|slack|jwt|generic] [--format env|json|yaml|tf] [--memo …] [--output file]
kuro canary inject <dir> [--type …] [--format …]
kuro canary verify <token|file>
kuro canary list [dir]
```

---

## `kuro attest`

See [ATTESTATION.md](ATTESTATION.md).

```bash
kuro attest verify [--commit SHA] [--repo path] [--pubkey key|file] [--file envelope.json]
kuro attest keygen
kuro attest inspect <envelope.json>
```

---

## `kuro proxy`

```bash
kuro proxy [--addr :8000] [--upstream https://github.com]
```

| Env | Default | Purpose |
|---|---|---|
| `LISTEN_ADDR` | `:8000` | Bind address (`--addr` overrides) |
| `UPSTREAM_URL` | `https://github.com` | Forge upstream |
| `SCAN_MODE` | `local` | `local` → `kuro scan --json`; `api`/`remote`/`enterprise` → HTTP API |
| `KURO_BIN` | `kuro` on PATH | CLI used in local mode |
| `KURO_URL` / `KURO_API_KEY` | — | Enterprise API mode |

Behavior: fail-closed Smart HTTP; policy breach → **403** + findings on git stderr.

---

## Optional Enterprise companion CLI

These exist in the binary but are **not** the Core happy path:

`auth`, `status`, `backup`, `webhook`, `update`, `deploy`, `setup`, `health`, `up`, `license apply`, `scan --remote`

Document their HTTP contracts in Enterprise — not here.

---

## Integrating with CI (Core)

```bash
#!/usr/bin/env bash
set -euo pipefail
kuro doctor
set +e
kuro scan "$GITHUB_WORKSPACE" --json > scan.json
code=$?
set -e
# 0 pass, 2 review (policy choice), 1 block/error
if [ "$code" -eq 1 ]; then
  cat scan.json
  exit 1
fi
```

Ensure Docker/Podman is available to the job and images can be pulled once.

---

## Troubleshooting

| Issue | Fix |
|---|---|
| Scripts ignore `--json` after path | Upgrade to ≥ v0.1.1 |
| JSON parse grabs wrong `decision` | Decode root object only |
| Exit `1` with empty findings | Treat as error path; check `error` field |
| Proxy in API mode on Core laptop | Switch back to `SCAN_MODE=local` |

---

## See also

- [ARCHITECTURE.md](ARCHITECTURE.md)
- [SCANNER-ARCHITECTURE.md](SCANNER-ARCHITECTURE.md)
- [../tests/e2e-core-local.sh](../tests/e2e-core-local.sh)
