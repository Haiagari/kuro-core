# Changelog — Kuro Core

All notable changes to Kuro Core are documented in this file.
Format: [Keep a Changelog](https://keepachangelog.com/en/1.0.0/) — [Semantic Versioning](https://semver.org/)

---

## [Unreleased]

### Added
- **Core doctor checks**: `kuro doctor` now validates Container Runtime, Git, Scanner Images, Disk Space, and Kuro Binary (no Postgres/NATS/MinIO).
- **git-proxy local scan mode**: default `SCAN_MODE=local` runs `kuro scan --json` via the Core CLI; set `SCAN_MODE=api` for the Enterprise API path.
- **Core-local E2E**: `tests/e2e-core-local.sh` and `make e2e-core` exercise doctor + clean/secret scans without Postgres/NATS/API (Enterprise proxy E2E remains `tests/e2e-proxy.sh`).
- **Makefile targets**: `make proxy` runs local git-proxy; `make e2e-core` builds and runs Core-local E2E.

### Changed
- **Docs & CLI help aligned to Core positioning**: clone/install URLs now point to `Haiagari/kuro-core`; `kuro help` leads with local-first commands (`doctor`, `scan`, `fix`, `canary`, `attest`) and demotes server/Enterprise companion commands.
- **Quickstart happy path**: doctor → scan → fix/canary → local git-proxy.
- **Local scanner containers hardened**: `runContainer` adds `--network=none`, `--cap-drop=ALL`, and `--security-opt=no-new-privileges`.
- **Scan CLI exit codes**: `kuro scan` now exits `0` on pass, `2` on review, `1` on block/error (all display modes); JSON mode handles a nil result safely when an error is present.

---

## [0.1.0] - 2026-09-04

### Added
- **Initial Open-Core Standalone Release**: Lightweight, zero-dependency CLI security gate and local git pre-push proxy.
- **Multi-Scanner Parallel Fleet**: Coordinated execution of Gitleaks, Semgrep, Trivy, and Checkov in isolated Docker/Podman containers (`--network none`).
- **Interactive Terminal Threat Remediation (`kuro fix`)**: Bubbletea TUI for interactive inspection and automated replacement of hardcoded credentials with environment variables.
- **Cyber Deception Honeypots (`kuro canary`)**: Generation and verification of HMAC-signed canary tokens (AWS, GitHub, Slack, JWT) for intrusion detection.
- **Fail-Closed Git Pre-Push Proxy**: Local Smart-HTTP proxy on `localhost:8000` blocking pushes containing credentials or policy violations.
- **Cryptographic Attestation Verification (`kuro attest`)**: Built-in verification of Ed25519-signed in-toto and SLSA provenance statements stored in git notes.
- **Deterministic Deduplication Engine**: Local SHA-256 fingerprinting combined with Jaccard token similarity for clean reporting without AI/server dependencies.
