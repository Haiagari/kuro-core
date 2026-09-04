# AGENTS.md — Kuro Core context for AI coding assistants

**Audience:** AI agents and human contributors automating work in this repository.  
**Scope:** [Haiagari/kuro-core](https://github.com/Haiagari/kuro-core) only — local-first AppSec gate. Do **not** assume Postgres, NATS, MinIO, or Enterprise dashboards are required.

Related: [CONTRIBUTING.md](CONTRIBUTING.md) · [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) · [QUICKSTART.md](QUICKSTART.md)

---

## 1. Product truth (must match code)

| Fact | Value |
|---|---|
| Product | **Kuro Core** — single-binary local AppSec gate |
| Repo | Always `Haiagari/kuro-core` (never `Haiagari/kuro`, `sambleed/*`, or `kuro-pipeline` unless historical notes) |
| Edition | Core = local Docker/Podman scanners; Enterprise = `Haiagari/kuro-enterprise` |
| Version docs target | **v0.1.1** |
| License | AGPL-3.0-only |
| Happy path | `kuro doctor` → `kuro scan` → `kuro fix` / `kuro canary` → `kuro proxy` |
| Install | `curl -sSL https://raw.githubusercontent.com/Haiagari/kuro-core/main/scripts/install.sh \| sh` (pin: `sh -s -- v0.1.1`); also `make build` / `make install` |
| Scan exit codes | pass=`0`, review=`2`, block/error=`1`; flags after path OK (`kuro scan PATH --json`) |
| Scanners | Gitleaks, Semgrep (**embedded** rules, not `--config=auto`), Trivy, Checkov |
| Hardening | `--network=none`, `--cap-drop=ALL`, `no-new-privileges`, `--memory=512m` |
| Proxy | Prefer `./bin/kuro proxy`; `SCAN_MODE=local` default; `SCAN_MODE=api` for Enterprise; `services/git-proxy` for Docker/standalone |
| E2E | Core: `make e2e-core` / `tests/e2e-core-local.sh`; Enterprise/API: `tests/e2e-proxy.sh` |

---

## 2. Directory map

```
kuro-core/
├── bin/                      # build output (gitignored)
├── cli/                      # main binary (Go + Bubbletea)
│   ├── cmd/                  # scan, fix, canary, doctor, proxy, attest, …
│   └── internal/
│       ├── doctor/
│       ├── orchestrator/     # 6-phase pipeline, containers, semgrep rules
│       ├── output/
│       └── tui/
├── deploy/policies/          # default-policy.json
├── deploy/security/          # scanner base configs
├── docs/                     # architecture, scanners, API, attestation
├── services/git-proxy/       # standalone proxy main / images
├── scripts/                  # install.sh + helpers
├── tests/                    # unit helpers, e2e-core-local, e2e-proxy, chaos
├── Makefile
└── go.work                   # cli + git-proxy modules
```

---

## 3. Technology stack

| Domain | Choice |
|---|---|
| Language | Go 1.26+ |
| CLI / TUI | Bubbletea + Lipgloss |
| Scanners | Gitleaks, Semgrep, Trivy, Checkov (+ TruffleHog in `--history`) |
| Proxy | Go Smart-HTTP (`git-receive-pack`) |
| Policy | Static JSON |
| License | AGPL-3.0-only |

---

## 4. Mandatory conventions

- **SemVer 2.0.0** for releases.
- **Conventional Commits:** `feat(scope):`, `fix(scope):`, `docs(scope):`, …
- **Zero attribution:** never add `Co-Authored-By` or AI-assisted trailer lines.
- **Fail-closed:** security gates block on error/parse failure.
- **Docs honesty:** do not document Enterprise as the default; mark server HTTP API out-of-scope in Core docs.
- **Do not create git tags** unless a human explicitly asks.

---

## 5. Safe change patterns

| Task | Prefer |
|---|---|
| Local scan behavior | `cli/internal/orchestrator/*` |
| Exit codes / flag parsing | `cli/cmd/scan.go`, `decision_exit.go` |
| Proxy UX | `cli/cmd/proxy.go` + `kuro/git-proxy/server` |
| Semgrep offline | Keep embedded `rules/semgrep-core.yml`; never restore `--config=auto` for Core |
| Docs | Update README + QUICKSTART + docs/* together when behavior changes |
| Tests | `make test`; behavior: `make e2e-core` |

---

## 6. Common pitfalls for agents

1. Wrong clone/install URL (`Haiagari/kuro` or `sambleed/…`) — always `Haiagari/kuro-core`.
2. Treating `tests/e2e-proxy.sh` as Core default — it is Enterprise/API.
3. Documenting memory as `1g` — runtime uses `--memory=512m`.
4. Claiming Core needs Postgres/NATS/MinIO — it does not.
5. Breaking `kuro scan PATH --json` flag reorder.

---

## 7. Quick verification commands

```bash
make build
./bin/kuro doctor
./bin/kuro scan /tmp/clean --json; echo $?    # expect 0 on pass
make test
make e2e-core   # needs Docker/Podman
```
