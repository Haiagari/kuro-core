# CLI and JSON interface — development branch

Core's supported integration is the local CLI. URL targets / `--remote` require an optional Enterprise API; server HTTP contracts are outside this document. This describes source development behavior, not a guarantee of published v0.1.1 assets.

## Scan

```bash
kuro scan <path> [--json] [--no-cache] [--history] [--tui]
```

Flags before or after the path work. JSON takes priority over TUI; TUI is automatic on a terminal and can be disabled with `--tui=false`. `--history` selects Git-history secret scanners instead of the normal working-tree fleet. `--no-cache` disables file statistics; scanners always execute regardless of statistics.

| Variable | Default | Purpose |
|---|---|---|
| KURO_MAX_CONCURRENCY | 2 | Active scanner container limit |
| KURO_POLICY_PATH | Embedded default | Custom JSON policy; invalid policy blocks |
| KURO_NO_CACHE | false | 1/true disables file statistics |
| KURO_CACHE_DIR | $HOME/.kuro/cache | File statistics storage; never authorizes a pass |
| KURO_TRIVY_CACHE_DIR | $HOME/.kuro/trivy | Prepared vulnerability database cache |

Policies: [POLICY.md](POLICY.md). Offline prerequisites: [SCANNER-ARCHITECTURE.md](SCANNER-ARCHITECTURE.md).

| Decision/condition | Exit |
|---|---:|
| pass / approved / ok | 0 |
| review | 2 |
| block / unknown / execution error | 1 |

## JSON contract

Read **stdout**, and decode the root decision; diagnostic progress goes to stderr. Successful policy-permitted example:

```json
{"target":"/tmp/project","mode":"local","status":"completed","decision":"pass","duration":"1.2s","findings":[]}
```

Finding example under the default policy:

```json
{"target":"/tmp/project","mode":"local","status":"completed","decision":"block","duration":"1.2s","findings":[{"scanner":"gitleaks","severity":"CRITICAL","title":"generic-api-key","file":".env","line":1}]}
```

Pipeline failure uses `status: failed`, `decision: block` and `error`. The defensive nil-result path uses an empty decision; both exit 1. Failures before orchestration (invalid CLI arguments or missing remote configuration) may emit stderr without a JSON envelope. Consumers must validate **both process exit and JSON**, never treating absent JSON, unknown decisions or failed status as approval. Current error results may omit findings from other scanners that completed before a failure.

The CLI envelope has target, mode, status, decision, duration (string), findings, and optional error. Finding fields are scanner, severity, title, file, line. No schema version, policy trace or full scanner coverage inventory is currently emitted.

## CI integration

```bash
#!/usr/bin/env bash
set -euo pipefail
kuro doctor
# Prepare Trivy's DB during a separate online setup step when dependencies select it.
set +e
kuro scan "$GITHUB_WORKSPACE" --json > scan.json
code=$?
set -e
cat scan.json
case "$code" in
  0) exit 0 ;;
  2) echo "Human review required" >&2; exit 2 ;;
  *) exit 1 ;;
esac
```

The example stops on review as well as block/error. A team that allows review in CI must explicitly define its human review process. Docker/Podman and available images/DB are prerequisites. These commands do not establish organization-wide enforcement.

## Other Core commands

```bash
kuro doctor [--json]
kuro fix [path] [--dry-run|-d] [--auto|-a|-y] [--report|-r <file>]
kuro canary generate [--type aws|github|slack|jwt|generic] [--format env|json|yaml|tf] [--output file]
kuro canary inject <dir> [--type ...] [--format ...]
kuro canary verify <token|file>
kuro canary list [dir]
kuro attest inspect <envelope.json>
kuro attest verify --pubkey <hex|file> [--commit SHA|ref] [--repo path] [--file envelope.json] [--legacy-raw]
kuro attest keygen
kuro proxy [--addr 127.0.0.1:8000] [--upstream https://github.com]
```

Doctor warnings may exit 0; it is not a successful vulnerability scan. Fix is heuristic unless supplied a JSON scan report; report findings must identify regular files inside the selected target. Missing or invalid reports and remediation failures exit 1 rather than falling back to heuristic scanning. Automatic replacements are limited to recognized quoted secrets in Go, Python and JS/TS string literals; use `--dry-run` first. Suppression records are not automatically consumed by the gate. Canary verification identifies synthetic markers, not provider activity or an external intrusion alert. Attestation trust and exit codes: [ATTESTATION.md](ATTESTATION.md).

## Proxy

Default LISTEN_ADDR is `:8000` (all interfaces), UPSTREAM_URL is `https://github.com`, SCAN_MODE is local, and KURO_BIN is `kuro` on PATH (with a local build fallback). Use a loopback address explicitly for workstation-only access. `KURO_URL` / `KURO_API_KEY` select Enterprise API configuration with SCAN_MODE=api/remote/enterprise.

Local pass forwards the receive-pack payload; review/block rejects it; scanner/process/JSON failures reject under default fail-closed. Local approvals are not cached. Direct pushes that bypass this remote are outside its control. Upstream authentication must be configured separately; the minimal remote URL example does not supply credentials.

## Optional companion commands

`auth`, `status`, `backup`, `webhook`, `update`, `deploy`, `setup`, `health`, `up`, `license apply` and `scan --remote` are auxiliary surfaces. Core guides do not rely on server deployment commands. Release updating requires published compatible artifacts; it is not an Enterprise HTTP contract.
