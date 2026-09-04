<div align="center">

# Kuro Core

**Local-first, single-binary AppSec gate**  
Intercepts and validates every `git push` with multi-scanner SAST / SCA / Secrets, interactive remediation, and honeypot canaries — 100% self-contained on your machine. No Postgres, NATS, or MinIO required.

[![Go Version](https://img.shields.io/badge/Go-1.26-00ADD8?style=flat-square&logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-AGPL--3.0--only-blue?style=flat-square)](LICENSE)
[![Version](https://img.shields.io/badge/Release-v0.1.1-emerald?style=flat-square)](https://github.com/Haiagari/kuro-core/releases)

```bash
curl -sSL https://raw.githubusercontent.com/Haiagari/kuro-core/main/scripts/install.sh | sh
# pin: ... | sh -s -- v0.1.1
kuro doctor && kuro scan ./my-project
```

</div>

---

## Audience

| You are… | Start here |
|---|---|
| Developer who wants local pre-push security | [QUICKSTART.md](QUICKSTART.md) |
| Contributor / AI agent working in this repo | [AGENTS.md](AGENTS.md), [CONTRIBUTING.md](CONTRIBUTING.md) |
| Security / architecture reviewer | [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md), [docs/SCANNER-ARCHITECTURE.md](docs/SCANNER-ARCHITECTURE.md) |

---

## Core vs Enterprise

| | **Kuro Core** (`Haiagari/kuro-core`) | **Kuro Enterprise** (`Haiagari/kuro-enterprise`) |
|---|---|---|
| Runtime | Single binary + Docker/Podman | Server stack (API, workers, dashboards) |
| Datastores | None | Postgres, NATS, MinIO, … |
| Happy path | `doctor` → `scan` → `fix` / `canary` → `proxy` | Central policy, multi-tenant API |
| Proxy scan mode | `SCAN_MODE=local` (default) | `SCAN_MODE=api` |

This repository is **Core only**. Server HTTP APIs and multi-tenant dashboards are out of scope here; see Enterprise when you need them.

---

## Features

- **Multi-scanner fleet** — Gitleaks, Semgrep (embedded offline ruleset), Trivy, Checkov in hardened containers (`--network=none`, `--cap-drop=ALL`, `no-new-privileges`).
- **Fail-closed Git proxy** — `kuro proxy` on `:8000` blocks leaking pushes before they reach GitHub/GitLab.
- **Interactive remediation** — `kuro fix` extracts hardcoded secrets to env vars (`--dry-run`, `--auto`).
- **Canary deception** — `kuro canary generate|inject|verify|list` for honeypot credentials.
- **Attestation** — `kuro attest verify|keygen|inspect` for in-toto / SLSA-style Ed25519 envelopes.
- **Deterministic exit codes** — pass=`0`, review=`2`, block/error=`1` (flags work after the path: `kuro scan PATH --json`).

---

## Install

### Release installer (recommended)

```bash
curl -sSL https://raw.githubusercontent.com/Haiagari/kuro-core/main/scripts/install.sh | sh

# Pin a release:
curl -sSL https://raw.githubusercontent.com/Haiagari/kuro-core/main/scripts/install.sh | sh -s -- v0.1.1
```

### Build from source

```bash
git clone https://github.com/Haiagari/kuro-core.git
cd kuro-core
make build          # → bin/kuro
sudo make install   # → /usr/local/bin/kuro
```

**Prerequisites:** Docker 24+ or Podman 4+ for local scanners. Go 1.26+ only if building from source.

---

## Quick usage (happy path)

```bash
kuro doctor                         # runtime + scanner image checks
kuro scan ./my-project              # interactive TUI on a TTY
kuro scan ./my-project --json       # machine-readable; flags OK after path
kuro fix ./my-project --dry-run     # preview secret remediation
kuro canary generate --type aws --format env
./bin/kuro proxy                    # fail-closed pre-push gate (:8000)
```

Expected scan exit codes:

| Decision | Exit code |
|---|---|
| pass | `0` |
| review | `2` |
| block / error | `1` |

Optional Core-local E2E (no Postgres/NATS/API):

```bash
make e2e-core
# equivalent: bash tests/e2e-core-local.sh
```

`tests/e2e-proxy.sh` is the **Enterprise/API** proxy path — not the Core default.

---

## Architecture (summary)

```
Developer ──► kuro scan / fix / canary
                 │
                 ▼
        6-phase orchestrator (fetch → scope → scan → analyze → decide → report)
                 │
                 ▼
   Hardened Docker/Podman containers (Gitleaks · Semgrep · Trivy · Checkov)

git push ──► kuro proxy (:8000, SCAN_MODE=local) ──► kuro scan --json ──► forge
```

Full detail: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) · scanners: [docs/SCANNER-ARCHITECTURE.md](docs/SCANNER-ARCHITECTURE.md).

---

## CLI reference (Core)

```bash
kuro doctor [--json]
kuro scan <path> [--json] [--history] [--tui]   # --remote needs Enterprise API key
kuro fix [path] [--dry-run|--auto]
kuro canary generate|inject|verify|list
kuro attest verify|keygen|inspect
kuro proxy [--addr :8000] [--upstream https://github.com]
kuro license status|apply <token>
kuro version | kuro help
```

Optional companion commands (`auth`, `deploy`, `setup`, `health`, `up`, `backup`, `webhook`, `scan --remote`) talk to a Kuro **server** stack. Prefer [Haiagari/kuro-enterprise](https://github.com/Haiagari/kuro-enterprise) for that path.

### Local Git proxy

```bash
export PATH="$PWD/bin:$PATH"   # or: export KURO_BIN=$PWD/bin/kuro
./bin/kuro proxy
# or: make proxy

git remote add proxy http://localhost:8000/<owner>/<repo>.git
git push proxy main
```

- Default `SCAN_MODE=local` shells out to `kuro scan --json`.
- Set `SCAN_MODE=api` (aliases: `remote`, `enterprise`) plus `KURO_URL` / `KURO_API_KEY` for Enterprise API mode.
- `services/git-proxy` remains for Docker/standalone images; day-to-day prefer `./bin/kuro proxy`.

---

## Documentation index

| Doc | Purpose |
|---|---|
| [QUICKSTART.md](QUICKSTART.md) | Full tutorial: install → doctor → scan → fix → canary → proxy → e2e |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | CLI, orchestrator phases, proxy, containers |
| [docs/SCANNER-ARCHITECTURE.md](docs/SCANNER-ARCHITECTURE.md) | Images, hardening, Semgrep embedded rules, offline constraints |
| [docs/API.md](docs/API.md) | CLI / JSON contract, exit codes; Enterprise HTTP marked out-of-scope |
| [docs/ATTESTATION.md](docs/ATTESTATION.md) | `kuro attest` guide |
| [AGENTS.md](AGENTS.md) | Context for AI coding agents |
| [CONTRIBUTING.md](CONTRIBUTING.md) | Dev setup, tests, PR conventions |
| [SECURITY.md](SECURITY.md) | Vulnerability reporting |
| [CHANGELOG.md](CHANGELOG.md) | Release history |
| [scripts/README.md](scripts/README.md) | Helper scripts |
| [tests/README.md](tests/README.md) | Unit, Core E2E, proxy E2E, hardening |

---

## License

[AGPL-3.0-only](LICENSE) — see also [NOTICE](NOTICE).
