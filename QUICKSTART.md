# Quickstart — Kuro Core (development branch)

This guide describes the current source tree, including changes not released as v0.1.1. Core runs locally; Docker or Podman is required. Enterprise servers are optional and separate.

## Build and check

Supported release platforms: Linux/macOS, amd64/arm64. Windows binaries are not configured; WSL support has not been verified here. Building requires Go **1.26.4 or newer**, Git and Make. Scanners require Docker 24+ or Podman 4+ with a usable runtime.

Build from source while release binaries are unavailable:

```bash
git clone https://github.com/Haiagari/kuro-core.git
cd kuro-core
make build
export PATH="$PWD/bin:$PATH"
kuro doctor
```

`doctor` checks runtime, Git, scanner images and disk. Image warnings can occur before the first pull. It does not certify scanner coverage or Trivy DB freshness. Network is needed to obtain images; scan containers use `--network=none`.

## First scan

```bash
mkdir -p /tmp/kuro-demo-clean
printf 'package main\nfunc main() {}\n' > /tmp/kuro-demo-clean/main.go
kuro scan /tmp/kuro-demo-clean --json
# Expected exit 0 and root decision "pass".
```

A synthetic secret should block:

```bash
mkdir -p /tmp/kuro-demo-secret
# Assemble an explicitly synthetic token instead of storing it as a literal.
printf 'SLACK_TOKEN=xoxb-%s-%s-%s\n' 123456789012 123456789012 abcdefghijklmnopqrstuvwx > /tmp/kuro-demo-secret/.env
kuro scan /tmp/kuro-demo-secret --json
# Expected exit 1 and decision "block". Inspect status/error to distinguish failures.
```

| Result | Exit | Meaning |
|---|---:|---|
| `pass` | 0 | Applicable checks completed and the active policy permits the findings |
| `review` | 2 | Human review required; the local proxy blocks |
| `block` / error | 1 | Policy rejection or incomplete analysis |

A pass is not proof that a repository has no vulnerabilities. It is limited to the selected mode, scanners, rules and available vulnerability data. Every scan executes the applicable scanners; cached file statistics never authorize a pass. `--no-cache` disables those statistics.

## Repositories with dependencies

Before scanning a repository whose dependency manifests select Trivy, explicitly prepare its database on a networked host:

```bash
./scripts/update-trivy-db.sh
kuro scan ./my-project --json
```

The helper mounts only the database download directory, not project source. Scans mount that cache at `/trivy-cache` and run without network. Missing, invalid or expired DB metadata blocks the scan. The default cache is `$HOME/.kuro/trivy`; override with `KURO_TRIVY_CACHE_DIR`. Refresh before `NextUpdate` and at least every 24 hours. Copy its `db/` directory to an offline host when needed. Avoid updating it during a scan. See [scanner details](docs/SCANNER-ARCHITECTURE.md).

## Fix / remediate

`kuro fix` detects a bounded set of common hardcoded credential formats during a local tree walk. It can also consume the `findings` array from a `kuro scan --json` report with `--report`; report file paths must resolve to regular files inside the selected target directory. Invalid or missing report input fails closed rather than falling back to local scanning.

```bash
# Preview a local heuristic scan
kuro fix /path/to/project --dry-run

# Apply supported replacements without prompts
kuro fix /path/to/project --auto

# Consume a prior scan report
kuro fix /path/to/project --report /path/to/scan.json --dry-run
```

Automatic replacements are limited to recognized secrets in quoted Go, Python, and JS/TS string literals. Other findings are not rewritten. Suppressions are recorded in `.kuro-suppressions.json`. Review every change; replacement does not revoke a leaked credential or erase Git history.

## Release installation (when assets are published)

```bash
curl -fsSL https://raw.githubusercontent.com/Haiagari/kuro-core/main/scripts/install.sh | sh
# Select an actually published tag:
# curl -fsSL https://raw.githubusercontent.com/Haiagari/kuro-core/main/scripts/install.sh | sh -s -- vX.Y.Z
```

The installer downloads archives and validates SHA-256 checksums. A tag alone does not guarantee release assets. See [Releases](https://github.com/Haiagari/kuro-core/releases); use source build if none are available. A checksum from the same release detects corruption; it does not independently establish publisher authenticity.

## Next tutorials

- [CLI and CI integration](docs/API.md)
- [Policies and safety floors](docs/POLICY.md)
- [Architecture and limits](docs/ARCHITECTURE.md)
- [Attestation verification](docs/ATTESTATION.md)
- [Synthetic demo](docs/market-validation/DEMO.md)
- [Contributor checks](tests/README.md)

`kuro fix` is heuristic and requires diff review; use `--dry-run` first. A replacement does not revoke a leaked credential or remove it from Git history. Canary generation does not provide an external alerting service. The proxy is an opt-in Smart-HTTP route; pushes directly to another remote bypass it.

### Optional GitHub merge gate

See [Trusted Kuro PR Gate](docs/CI_GATE.md) for base-controlled CI scanning and activation as a required merge check. Its adoption policy blocks introduced findings and changes to affected files; unchanged existing findings remain counted debt. Installation alone does not activate branch enforcement.

The [baseline debt audit](docs/SECURITY_DEBT.md) records the 27 finding classifications, targeted dependency remediation and verified mandatory merge enforcement.
