# Architecture — Kuro Core v0.1.1

**Audience:** contributors, reviewers, and operators who need a precise picture of how Core works on a workstation.  
**Scope:** Kuro Core (`Haiagari/kuro-core`) — single binary + Docker/Podman. Postgres, NATS, MinIO, Firecracker, and multi-tenant HTTP APIs belong to [Haiagari/kuro-enterprise](https://github.com/Haiagari/kuro-enterprise).

Related: [SCANNER-ARCHITECTURE.md](SCANNER-ARCHITECTURE.md) · [API.md](API.md) · [../QUICKSTART.md](../QUICKSTART.md)

---

## Design principles

1. **Local-first** — default path never needs a Kuro server.
2. **Single binary** — CLI, TUI, remediation, canaries, attestation, and proxy live in `bin/kuro`.
3. **Fail-closed** — scan/proxy errors and policy breaches block rather than soft-fail.
4. **Hardened scanners** — ephemeral containers with `--network=none`, `--cap-drop=ALL`, `--security-opt=no-new-privileges`.
5. **Edition honesty** — Core docs do not pretend Enterprise is the default.

---

## High-level components

```
┌──────────────────────────────────────────────────────────────┐
│ Developer workstation                                        │
│                                                              │
│  kuro doctor / scan / fix / canary / attest / license        │
│           │                                                  │
│           ▼                                                  │
│  cli/internal/orchestrator  (6-phase pipeline)               │
│           │                                                  │
│           ▼                                                  │
│  Docker / Podman ──► Gitleaks · Semgrep · Trivy · Checkov    │
│                                                              │
│  git push ──► kuro proxy (:8000) ──► SCAN_MODE=local         │
│                     │                 └─► kuro scan --json   │
│                     └─► forge (GitHub / GitLab / …)          │
└──────────────────────────────────────────────────────────────┘
```

| Component | Location | Role |
|---|---|---|
| CLI entry | `cli/main.go`, `cli/cmd/*` | Subcommands, exit codes, help |
| Orchestrator | `cli/internal/orchestrator` | Fetch → scope → scan → analyze → decide → report |
| Local adapter | `adapter_local.go`, `container.go`, `scanners.go` | Containerized scanners |
| Semgrep rules | `cli/internal/orchestrator/rules/semgrep-core.yml` (embedded) | Offline SAST under `--network=none` |
| Policy | `deploy/policies/default-policy.json` | Gate decisions |
| Proxy library | `kuro/git-proxy/server` | Smart-HTTP pre-push gate |
| Proxy CLI | `cli/cmd/proxy.go` → `kuro proxy` | Preferred runtime |
| Standalone proxy | `services/git-proxy` | Docker / thin `main` for images |
| TUI | `cli/internal/tui` | Bubbletea interactive scan UI |

---

## Six-phase scan pipeline

Implemented in `cli/internal/orchestrator`:

```
fetch ──► scope ──► scan ──► analyze ──► decide ──► report
```

### 1. Fetch
- Resolve absolute path; require a directory.
- Detect runtime (`docker` preferred if daemon healthy, else `podman`).
- Kick off **async** image pulls (`gitleaks`, `semgrep`, `trivy`, `checkov`; plus TruffleHog in `--history` mode).

### 2. Scope
Default (working tree):
- Always: **Gitleaks**, **Semgrep**
- **Trivy** if lockfiles present (`go.mod`, `package-lock.json`, `yarn.lock`, `requirements.txt`, `Gemfile.lock`, `Cargo.lock`)
- **Checkov** if `Dockerfile` or `*.tf` present

History mode (`kuro scan --history`):
- `gitleaks-history` + `trufflehog-history` only (longer timeouts).

Optional file-change cache under `$HOME/.kuro/cache`.

### 3. Scan
- Parallel container runs via `runContainer`.
- Hardening flags (see [SCANNER-ARCHITECTURE.md](SCANNER-ARCHITECTURE.md)).
- Per-scanner timeout: 5m (15m in history mode). Overall CLI context ~35m.

### 4. Analyze
- Parse tool JSON (`parsers.go`).
- Deduplicate (SHA-256 fingerprint + Jaccard similarity).
- Aggregate severity counts.

### 5. Decide
- Evaluate against static JSON policy (`deploy/policies/default-policy.json`).
- Decisions: `pass` | `review` | `block`.

### 6. Report
- TUI (auto on TTY), text summary, or `--json`.
- Exit codes: pass=`0`, review=`2`, block/error=`1` (`cli/cmd/decision_exit.go`).
- Flags after path work: `kuro scan PATH --json` (reorder in `scan.go`).

---

## Local vs remote adapters

| Mode | Trigger | Needs |
|---|---|---|
| **Local (Core default)** | Local filesystem path | Docker/Podman |
| **Remote / URL** | `--remote` or `https://…` / `git@…` target | API key (`kuro auth`) + Enterprise/server |

Core happy path is **always local**. Remote mode is an optional companion for Enterprise.

---

## Pre-push proxy architecture

```
git client ──Smart HTTP──► kuro proxy (:8000)
                              │
                              ├─ SCAN_MODE=local (default)
                              │     └─ exec: kuro scan --json  (KURO_BIN / PATH)
                              │
                              └─ SCAN_MODE=api|remote|enterprise
                                    └─ HTTP to KURO_URL + KURO_API_KEY
                              │
                    pass ──► forward to UPSTREAM_URL (default https://github.com)
                    fail ──► HTTP 403 + stderr sideband findings
```

Preferred invocation:

```bash
./bin/kuro proxy --addr :8000 --upstream https://github.com
# make proxy
```

Env knobs: `LISTEN_ADDR`, `UPSTREAM_URL`, `SCAN_MODE`, `KURO_BIN`, `KURO_URL`, `KURO_API_KEY`, `PROXY_FAIL_MODE` (fail-closed), `PROXY_SCAN_TIMEOUT`.

`services/git-proxy` remains for container images; logic is shared via the importable server package.

---

## Remediation, deception, attestation

| Capability | Command | Notes |
|---|---|---|
| Secret remediation | `kuro fix` | Heuristic walk + env-var rewrite; `--dry-run` / `--auto` |
| Canaries | `kuro canary` | HMAC-tagged honeypots; manifest `.canary-manifest.json` |
| Attestation | `kuro attest` | Ed25519 in-toto envelopes; git notes `refs/notes/kuro-attestation` |

Details: [ATTESTATION.md](ATTESTATION.md), [API.md](API.md).

---

## Repository layout (Core-relevant)

```
kuro-core/
├── cli/                 # Binary entry + cmd + internal/*
├── services/git-proxy/  # Standalone proxy main / Docker
├── deploy/policies/     # default-policy.json
├── deploy/security/     # scanner base configs
├── docs/                # Architecture, scanners, API, attestation
├── scripts/             # install.sh + helpers
├── tests/               # e2e-core-local, e2e-proxy, chaos, hardening
├── Makefile             # build, test, proxy, e2e-core, install
└── go.work              # cli + git-proxy modules
```

---

## Troubleshooting

| Issue | Guidance |
|---|---|
| “No container runtime” | Install/start Docker or Podman; `kuro doctor` |
| Proxy cannot scan | Set `KURO_BIN` or put `kuro` on `PATH`; confirm `SCAN_MODE=local` |
| Semgrep fails offline | Expected to use embedded rules — not `--config=auto`. See scanner doc |
| Confusing Core vs Enterprise E2E | Use `make e2e-core`; reserve `tests/e2e-proxy.sh` for API stacks |

---

## See also

- [SCANNER-ARCHITECTURE.md](SCANNER-ARCHITECTURE.md)
- [API.md](API.md)
- [../AGENTS.md](../AGENTS.md)
- [../tests/README.md](../tests/README.md)
