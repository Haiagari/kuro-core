# Tests — Kuro Core

**Audience:** contributors validating CLI, orchestrator, and proxy behavior.  
**Scope:** this repository’s unit, chaos, hardening, and E2E suites for **Kuro Core**.

Related: [../QUICKSTART.md](../QUICKSTART.md) · [../CONTRIBUTING.md](../CONTRIBUTING.md) · [../docs/SCANNER-ARCHITECTURE.md](../docs/SCANNER-ARCHITECTURE.md)

---

## Prerequisites

| Suite | Needs |
|---|---|
| Unit (`make test`) | Go 1.26+ |
| Core E2E (`make e2e-core`) | `bin/kuro` + Docker **or** Podman |
| Enterprise/API proxy E2E | Extra API stack — **not** Core default |
| Hardening / chaos | Runtime-dependent; see each script header |

---

## 1. Unit tests

```bash
make test
# equivalent: go test -v -count=1 ./cli/... ./services/git-proxy/...

make lint            # go vet
make test-coverage   # reports/coverage.out
```

---

## 2. Core-local E2E (preferred for Core)

**No Postgres / NATS / API.** Exercises the standalone CLI path:

`doctor` → clean scan (`pass` / exit `0`) → secret scan (`block` / exit `1`).

```bash
make e2e-core
# or: bash tests/e2e-core-local.sh
# optional: KURO_BIN=/path/to/kuro bash tests/e2e-core-local.sh
```

**Notes:**

- Parses the **root** JSON `decision` field (not nested findings).
- Secret fixture uses a **synthetic Slack bot token** — AWS documentation `EXAMPLE` keys are allowlisted by Gitleaks.
- Exit `2` from the script means missing binary/runtime prerequisites.

---

## 3. Enterprise / API proxy E2E

```bash
./tests/e2e-proxy.sh
```

This path targets API/`SCAN_MODE=api` style stacks. It is **not** the Core happy path. Prefer `make e2e-core` when validating local-first changes.

---

## 4. Chaos & hardening

```bash
./tests/chaos-test.sh
./tests/security-hardening-test.sh
```

Hardening tests validate container flags / capability drops / non-privileged constraints consistent with `runContainer` (`--network=none`, `--cap-drop=ALL`, `no-new-privileges`, etc.).

---

## Expected scan exit codes (product contract)

| Decision | `kuro scan` exit |
|---|---|
| pass | `0` |
| review | `2` |
| block / error | `1` |

---

## Troubleshooting

| Symptom | Fix |
|---|---|
| E2E exit 2 | `make build`; start Docker/Podman |
| Secret case unexpectedly passes | Do not use AWS EXAMPLE keys; use Slack token fixture |
| Confusion with proxy E2E failures | Confirm whether you meant Core (`e2e-core-local`) vs API (`e2e-proxy`) |
| Semgrep fails offline in E2E | Embedded rules required — see scanner architecture doc |

---

## See also

- [../docs/API.md](../docs/API.md)
- [../docs/ARCHITECTURE.md](../docs/ARCHITECTURE.md)
- [../AGENTS.md](../AGENTS.md)
