# Tests — Kuro Core

## Checks for contributors

```bash
make test             # CLI/proxy Go suites; Go 1.26.4+
make lint             # go vet
make build
python3 tests/test-install.py   # Local release fixtures; no network
python3 tests/test-trivy-db-update.py  # DB staging/source isolation/failure fixtures
make e2e-core          # Actual scanner containers; Docker/Podman required
./scripts/update-trivy-db.sh
./tests/e2e-scanner-contracts.sh   # Native SAST/SCA/IaC/history smoke cases
```

The Go suites include scanner process failures with parseable output, malformed native reports, Trivy DB metadata freshness, invalid/custom policy behavior, cache safety, local proxy errors, and real Git-note attestation signature/commit binding. Some subprocess tests use deliberately fake container executables to inject failures; passing them does not validate real scanner image compatibility or container isolation.

Installer tests run the real shell installer against local archives/checksums and cover supported OS/architecture combinations, checksum mismatch and unavailable releases. Python 3 is needed only for this test harness.

`make e2e-core` exercises doctor, a clean fixture (pass/0) and synthetic Slack secret (block/1). The separate scanner-contracts script exercises vulnerable Python dependency, Dockerfile, Semgrep and Git-history fixtures. CI includes a dedicated Docker job for these contracts. Neither suite certifies a production proxy deployment. Prepare a Trivy DB separately for tests with dependency manifests. Exit 2 from the E2E **script** means missing prerequisites, distinct from review/2 from `kuro scan`.

`tests/e2e-proxy.sh` and `tests/security-hardening-test.sh` are legacy Enterprise/API-stack scripts; the latter checks server infrastructure, not Core container flags. Do not present them as successful Core hardening validation. `tests/chaos-test.sh --unit-only` runs proxy fault injection; other modes include HTTP mock scenarios.

## Product exit contract

pass=0, review=2, block/error=1. Local proxy rejects review and block. A successful unit suite is not a scanner coverage or deployment certification. Record exactly which runtime-dependent checks ran, failed or were skipped.

Related: [CONTRIBUTING](../CONTRIBUTING.md), [scanner contract](../docs/SCANNER-ARCHITECTURE.md), [CLI](../docs/API.md).
