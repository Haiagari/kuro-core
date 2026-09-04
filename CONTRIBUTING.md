# Contributing to Kuro Core

**Audience:** humans and agents opening PRs against [Haiagari/kuro-core](https://github.com/Haiagari/kuro-core).  
**Scope:** Core local CLI + proxy. Server/Enterprise features belong in `Haiagari/kuro-enterprise`.

Related: [AGENTS.md](AGENTS.md) · [QUICKSTART.md](QUICKSTART.md) · [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) · [SECURITY.md](SECURITY.md)

---

## Prerequisites

| Tool | Version | Why |
|---|---|---|
| Go | 1.26+ | Build CLI + git-proxy modules |
| Docker 24+ or Podman 4+ | Scanner containers + Core E2E |
| Make | — | `build`, `test`, `proxy`, `e2e-core` |
| Git | — | Notes / proxy workflows |

---

## Development setup

```bash
git clone https://github.com/Haiagari/kuro-core.git
cd kuro-core
make build
./bin/kuro version          # expect v0.1.1 (or your branch ldflags)
./bin/kuro doctor
```

Optional: `sudo make install` to place `kuro` on `/usr/local/bin`.

Workspace layout uses `go.work` covering `cli/` and `services/git-proxy/`.

---

## Day-to-day commands

```bash
make build          # → bin/kuro
make test           # go test ./cli/... ./services/git-proxy/...
make lint           # go vet
make test-coverage  # reports/coverage.out
make proxy          # build + ./bin/kuro proxy (SCAN_MODE=local)
make e2e-core       # Core-local E2E (no Postgres/NATS/API)
make clean
```

Manual smoke:

```bash
./bin/kuro scan ./some-clean-dir --json; echo $?
./bin/kuro proxy --addr :8000
```

---

## Repository map

```
kuro-core/
├── cli/                 # CLI + TUI + orchestrator
├── services/git-proxy/  # Standalone proxy binary / Docker
├── deploy/policies/     # default-policy.json
├── docs/                # Architecture, scanners, API, attestation
├── scripts/             # install.sh and helpers
├── tests/               # e2e-core-local, e2e-proxy, chaos, hardening
├── Makefile
└── go.work
```

---

## Coding guidelines

1. **Fail-closed** by default for gates and proxy.
2. Keep local scanners **offline-capable** (`--network=none`; Semgrep embedded rules).
3. Preserve scan exit codes: pass=`0`, review=`2`, block/error=`1`.
4. Preserve flag-after-path behavior for `kuro scan`.
5. Prefer `./bin/kuro proxy` in docs and Make targets; keep `services/git-proxy` for images.
6. Do not introduce Core hard deps on Postgres/NATS/MinIO.
7. Update docs when user-facing behavior changes (README, QUICKSTART, docs/*).

---

## Tests expected before merge

| Check | Command |
|---|---|
| Unit | `make test` |
| Vet | `make lint` |
| Core E2E (when scanners available) | `make e2e-core` |

`tests/e2e-proxy.sh` is for Enterprise/API stacks — not required for pure Core CLI changes unless you touch API scan mode.

---

## Pull requests

1. Branch from `main` (e.g. `fix/scan-json-flags`).
2. Conventional Commits: `feat(scope):`, `fix(scope):`, `docs(scope):`, `refactor(scope):`, `test(scope):`.
3. **No** `Co-Authored-By` / AI attribution trailers.
4. Fill `.github/PULL_REQUEST_TEMPLATE.md`.
5. Link related issues; describe Core vs Enterprise impact.
6. Do **not** create release git tags unless maintainers ask.

---

## Documentation PRs

When editing markdown:

- Use repo URL `Haiagari/kuro-core` only.
- Align version references to **v0.1.1** unless documenting history.
- Cross-link ARCHITECTURE / SCANNER / API / ATTESTATION as appropriate.
- Include troubleshooting and Core vs Enterprise scope on major pages.

---

## Security reports

Do not file public issues for vulnerabilities — see [SECURITY.md](SECURITY.md).

---

## License

By contributing you agree your work is licensed under **AGPL-3.0-only** (see [LICENSE](LICENSE)).
