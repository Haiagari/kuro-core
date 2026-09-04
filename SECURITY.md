# Security Policy — Kuro Core

**Audience:** researchers and users reporting vulnerabilities in Kuro Core.  
**Scope:** [Haiagari/kuro-core](https://github.com/Haiagari/kuro-core) local CLI, orchestrator, and Git proxy. Enterprise server components are tracked in `Haiagari/kuro-enterprise`.

Related: [docs/SCANNER-ARCHITECTURE.md](docs/SCANNER-ARCHITECTURE.md) · [CONTRIBUTING.md](CONTRIBUTING.md)

---

## Supported versions

| Version | Supported |
|---------|-----------|
| v0.1.x  | Active development & security fixes |
| < v0.1  | Not supported |

Docs and installers currently target **v0.1.1**.

---

## Reporting a vulnerability

Kuro Core is a security tool — please report privately.

**Do not open a public GitHub issue** for a security vulnerability.

- **Preferred:** [GitHub Security Advisory](https://github.com/Haiagari/kuro-core/security/advisories/new)
- **Email:** [security@kuro.dev](mailto:security@kuro.dev)

You can expect:

1. **Acknowledgment** within 48 hours.
2. **Initial assessment** within 5 business days.
3. A **fix timeline** once confirmed.

Please include Kuro version (`kuro version`), OS, container runtime, reproduction steps, and impact.

---

## In scope (Core)

- **CLI & TUI** (`cli/`): scan lifecycle, remediation (`kuro fix`), canaries, attestation verify/keygen/inspect.
- **Local Git proxy** (`kuro proxy` / `services/git-proxy`): fail-closed blocking, local `SCAN_MODE`, sideband reporting.
- **Container isolation arguments** passed to Docker/Podman (`--network=none`, `--cap-drop=ALL`, `no-new-privileges`, memory/CPU limits, volume mounts).
- **Install script** (`scripts/install.sh`) and release artifact integrity as published from this repo.

---

## Out of scope

- Upstream bugs in scanner images (Gitleaks, Semgrep, Trivy, Checkov, TruffleHog) — report upstream; we pin versions and can bump.
- Vulnerabilities in repositories being scanned (findings are the product working as intended).
- Enterprise-only server stacks (Postgres, NATS, MinIO, Firecracker, multi-tenant HTTP API) — report to `Haiagari/kuro-enterprise`.
- Social engineering / physical access to a developer workstation.

---

## Hardening notes for reporters

Local scanners intentionally run with network disabled. Semgrep uses an **embedded** ruleset (`semgrep-core.yml`) rather than `--config=auto`. See [docs/SCANNER-ARCHITECTURE.md](docs/SCANNER-ARCHITECTURE.md) before filing “offline Semgrep broken” issues — that may be expected configuration.

---

## Safe harbor

Good-faith research that avoids privacy violations, service disruption, and data destruction is welcomed. Do not attempt to access other users’ data or production Enterprise tenants.
