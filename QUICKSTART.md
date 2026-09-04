# Quickstart — Kuro Core v0.1.1

**Audience:** developers setting up Kuro Core on a workstation for the first time.  
**Scope:** local-first Core only (Docker/Podman scanners, no Postgres/NATS/MinIO). For multi-tenant dashboards and central APIs, use [Haiagari/kuro-enterprise](https://github.com/Haiagari/kuro-enterprise).

This tutorial walks the full happy path:

`install` → `doctor` → `scan` (clean + secret) → `fix` → `canary` → `proxy` → optional `e2e-core`

---

## Prerequisites

| Requirement | Notes |
|---|---|
| Docker 24+ **or** Podman 4+ | Required for scanner containers |
| Network (first run) | Pull scanner images once; scans themselves use `--network=none` |
| Go 1.26+ | Only if building from source |
| `curl` / `git` / `make` | Installer and source build |

---

## 1. Install

### Option A — release script

```bash
curl -sSL https://raw.githubusercontent.com/Haiagari/kuro-core/main/scripts/install.sh | sh

# Pin v0.1.1:
curl -sSL https://raw.githubusercontent.com/Haiagari/kuro-core/main/scripts/install.sh | sh -s -- v0.1.1
```

Confirm:

```bash
kuro version
# → kuro version v0.1.1
```

### Option B — from source

```bash
git clone https://github.com/Haiagari/kuro-core.git
cd kuro-core
make build
export PATH="$PWD/bin:$PATH"
./bin/kuro version
```

Optional system install: `sudo make install`.

---

## 2. Doctor

```bash
kuro doctor
# or: kuro doctor --json
```

**What it checks (Core):** container runtime, Git, scanner images, disk space, Kuro binary. It does **not** require Postgres, NATS, or MinIO.

**Expected:** exit `0`. Warnings (e.g. images not yet pulled) are OK; critical failures fail the run.

**Pitfall:** If Docker is installed but the daemon is not running, doctor fails. Start Docker/Podman and retry.

---

## 3. Scan a clean project

```bash
mkdir -p /tmp/kuro-demo-clean && printf 'package main\n\nfunc main() {}\n' > /tmp/kuro-demo-clean/main.go

kuro scan /tmp/kuro-demo-clean
# JSON (flags after path are supported):
kuro scan /tmp/kuro-demo-clean --json
echo $?   # expect 0 on pass
```

**Expected outcomes:**

- Interactive TUI on a TTY (unless `--json`).
- JSON includes top-level `"decision": "pass"`.
- Exit code `0`.

**Pitfall:** First scan may be slow while images pull asynchronously. Re-run after images are cached. Semgrep does **not** use `--config=auto`; Core embeds `semgrep-core.yml` so it works offline under `--network=none`.

---

## 4. Scan a fixture with a secret (expect block)

```bash
mkdir -p /tmp/kuro-demo-secret
# Synthetic Slack bot token — AWS docs EXAMPLE keys are allowlisted by Gitleaks
printf 'SLACK_TOKEN=xoxb-123456789012-123456789012-abcdefghijklmnopqrstuvwx\n' > /tmp/kuro-demo-secret/.env

kuro scan /tmp/kuro-demo-secret --json
echo $?   # expect 1 (block)
```

**Expected:** `"decision": "block"` (or `blocked`) and exit `1`.

Exit code cheat sheet:

| Decision | Code |
|---|---|
| pass | 0 |
| review | 2 |
| block / error | 1 |

---

## 5. Fix / remediate

`kuro fix` walks the tree for obvious hardcoded credentials (e.g. `AKIA…`) and offers interactive or automatic remediation.

```bash
# Preview only
kuro fix /path/to/project --dry-run

# Apply recommended replacements (env var extraction)
kuro fix /path/to/project --auto
```

Supports Go (`os.Getenv`), Python (`os.environ.get`), JS/TS (`process.env`), and generic `${ENV}` fallbacks. Suppressions land in `.kuro-suppressions.json`.

**Pitfall:** Fix is heuristic — always review diffs, especially with `--auto`.

---

## 6. Canary tokens

```bash
kuro canary generate --type aws --format env
kuro canary generate --type github --format json --output /tmp/canary.json
kuro canary inject ./tests/fixtures --type slack --format env
kuro canary verify /tmp/canary.json
kuro canary list
```

Types: `aws` (default), `github`, `slack`, `jwt`, `generic`. Formats: `env`, `json`, `yaml`, `tf`.

---

## 7. Local Git proxy (fail-closed pre-push)

Preferred one-binary path:

```bash
export PATH="$PWD/bin:$PATH"    # or: export KURO_BIN=$PWD/bin/kuro
./bin/kuro proxy
# equivalents: make proxy
# optional: ./bin/kuro proxy --addr :8000 --upstream https://github.com
```

Default `SCAN_MODE=local` runs `kuro scan --json` via the CLI. For Enterprise API mode:

```bash
SCAN_MODE=api KURO_URL=http://api:8080 KURO_API_KEY=... ./bin/kuro proxy
```

Wire a remote and push:

```bash
git remote add proxy http://localhost:8000/<owner>/<repo>.git
git push proxy main
```

Policy violations → HTTP 403 + findings on stderr. Clean → forwarded to upstream.

> Docker/standalone still builds from `services/git-proxy`; day-to-day prefer `./bin/kuro proxy`.

**Pitfall:** In local mode the proxy must find `kuro` on `PATH` or via `KURO_BIN`.

---

## 8. Optional: Core-local E2E

```bash
make e2e-core
# or: bash tests/e2e-core-local.sh
```

Runs doctor + clean scan (pass/0) + secret scan (block/1) with **no** Postgres/NATS/API.

Enterprise/API proxy E2E remains `tests/e2e-proxy.sh` — do not confuse the two.

---

## Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| `doctor` fails on runtime | Docker daemon down / no Podman | Start runtime; re-run `kuro doctor` |
| Semgrep / scanners hang or empty | Image not pulled; offline DB | Pull images; Trivy retries without offline when DB missing |
| `scan PATH --json` ignored flags | Fixed in v0.1.1 | Upgrade; flags after path are reordered |
| Proxy always 403 / error | `KURO_BIN` wrong; scanners missing | Export `KURO_BIN`; run `kuro doctor` |
| Looking for dashboards / Postgres | Wrong edition | Use Core for local; Enterprise for server stack |

---

## Next reading

- [README.md](README.md) — product overview & CLI index  
- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) — pipeline & proxy  
- [docs/SCANNER-ARCHITECTURE.md](docs/SCANNER-ARCHITECTURE.md) — hardening & images  
- [docs/API.md](docs/API.md) — JSON outputs & exit codes  
- [tests/README.md](tests/README.md) — test suites  
