# Helper Scripts — Kuro Core

This directory contains lightweight development and validation scripts for **Kuro Core**:

```
scripts/
├── install.sh                # One-command installer for Kuro Core CLI
├── pre-commit-secrets.sh     # Git pre-commit hook running local Gitleaks check
├── run-local-scan.sh         # Helper to run local scans directly
├── run-tests.sh              # Local test harness runner
├── run-validation.sh         # Code validation and sanity checks
├── scan-container-image.sh   # Container vulnerability scanner
├── setup-hooks.sh            # Installs local git hooks into .git/hooks/
└── check-docker-gid.sh       # Diagnostics for docker socket permissions
```
