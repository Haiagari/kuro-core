# Changelog — Kuro Core

All notable changes to Kuro Core are documented in this file.
Format: [Keep a Changelog](https://keepachangelog.com/en/1.0.0/) — [Semantic Versioning](https://semver.org/)

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
