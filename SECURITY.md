# Security Policy — Kuro Core

## Supported Versions

| Version | Supported |
|---------|-----------|
| v0.1.x  | Active development & security fixes |
| < v0.1  | Not supported |

---

## Reporting a Vulnerability

Kuro Core is a local-first security tool — we take vulnerability reports seriously.

**Do not open a public GitHub issue** for a security vulnerability. Instead, report it privately:

- **GitHub Security Advisory**: [Report a vulnerability](https://github.com/Haiagari/kuro-core/security/advisories/new) (preferred)
- **Email**: [security@kuro.dev](mailto:security@kuro.dev)

You can expect:

1. **Acknowledgment** within 48 hours.
2. **Initial assessment** within 5 business days.
3. **Fix timeline** communicated upon confirmation.

---

## Scope

The following components are in scope for Kuro Core:

- **CLI & TUI Engine** (`cli/`): Execution lifecycle, Bubbletea TUI, secret remediation (`kuro fix`), deception (`kuro canary`).
- **Local Pre-Push Proxy** (`services/git-proxy`): Packet inspection, fail-closed blocking, sideband telemetry.
- **Local Container Isolation**: Arguments, volumes, and flags passed to Docker/Podman runtimes (`--network none`, `--memory 1g`).

Out of scope:
- Upstream vulnerabilities in underlying scanner images (Gitleaks, Semgrep, Trivy, Checkov).
- Infrastructure of target repositories being scanned.
