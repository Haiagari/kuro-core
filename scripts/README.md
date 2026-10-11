# Helper scripts — Kuro Core

Run from a source checkout. Build with `make build`; local helpers use `bin/kuro` or KURO_BIN.

| Script | Current purpose |
|---|---|
| install.sh | Download published release archive and verify SHA-256; use source build if assets do not exist |
| update-trivy-db.sh | Explicit online DB preparation, without mounting source |
| market-validation-demo.sh | Synthetic clean/block demonstration |
| run-local-scan.sh | Forward arguments to the Core CLI scan |
| run-tests.sh | Go tests for CLI and proxy modules |
| prepare-validation.sh | Core doctor checks |
| run-validation.sh | Run the synthetic market-validation demo |
| setup-hooks.sh / pre-commit-secrets.sh | Optional Gitleaks hook; requires separately installed host Gitleaks |
| scan-container-image.sh | Legacy auxiliary image scanner; outside the hardened Core gate |
| check-docker-gid.sh | Docker socket permission diagnostics |

```bash
./scripts/run-local-scan.sh ./project --json
./scripts/run-tests.sh
./scripts/prepare-validation.sh
./scripts/run-validation.sh
./scripts/update-trivy-db.sh
```

The optional pre-commit hook is bypassable and currently skips when host Gitleaks is unavailable; it is not evidence of full Core enforcement. Check script prerequisites before use. No Core helper requires Postgres/NATS/MinIO. The alternate `deploy/install.sh` delegates to the same Core installer.

Release asset names omit the tag's leading `v`: tag `vX.Y.Z` uses `kuro_X.Y.Z_linux_amd64.tar.gz` and `kuro_X.Y.Z_checksums.txt`. Validate a real release before recommending it. The installer supports Linux/macOS amd64/arm64. See [Quickstart](../QUICKSTART.md), [scanners](../docs/SCANNER-ARCHITECTURE.md), [tests](../tests/README.md).
