# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| v1.5.x  | ✅ Active development |
| v1.4.x  | ✅ Security fixes only |
| < v1.4  | ❌ Not supported |

## Security Audit History

| Date | Version | Scope | Result |
|------|---------|-------|--------|
| 2026-08-03 | v1.5.0 | Full codebase security audit | 43 issues found and fixed — RLS enforcement, fail-closed gates, SSRF hardening, clone URL validation, multi-ref scanning, CSV injection prevention, operator-controlled suppression |

See [CHANGELOG.md](CHANGELOG.md) for the full list of security fixes.

## Reporting a Vulnerability

Kuro is a security tool — we take vulnerability reports seriously.

**Do not open a public GitHub issue** for a security vulnerability. Instead, report it privately:

- **GitHub Security Advisory**: [Report a vulnerability](https://github.com/Haiagari/kuro/security/advisories/new) (preferred)
- **Email**: [sam@kuro.dev](mailto:sam@kuro.dev)

You can expect:

1. **Acknowledgment** within 48 hours of your report.
2. **Initial assessment** within 5 business days (confirmed vulnerability vs. expected behavior).
3. **Fix timeline** communicated within 10 business days of confirmation.
4. **CVE assignment** and public disclosure after the fix is released.

We appreciate coordinated disclosure. Please give us reasonable time to fix and release before publishing details.

## Scope

The following components are in scope for security reports:

- All Go services under `services/` (api, worker, git-proxy, notifications, backup-dedup)
- The CLI (`cli/`)
- The dashboard (`dashboard/`)
- Docker Compose configurations (`docker-compose*.yml`)
- Scanner integrations (Gitleaks, TruffleHog, Semgrep, Checkov, Trivy, Syft, Grype)
- Authentication and authorization (API keys, JWT, OAuth2 GitHub, RBAC)
- Multi-tenant isolation (organization scoping, RLS enforcement)
- Clone URL validation and token injection prevention
- Webhook SSRF protection

Out of scope:

- Third-party scanner vulnerabilities (report to the respective project)
- Dependency CVEs already tracked by Trivy/Grype scans
- The infrastructure this tool scans (your repos are your responsibility)

## Security Posture

Kuro implements defense-in-depth across four security gates:

1. **Pre-Push Gate** — Git proxy intercepts push, runs Gitleaks + Semgrep, blocks before code leaves the machine.
2. **Post-Push Gate** — Webhook triggers full 7-scanner pipeline, blocks PR merge via commit status checks.
3. **History Scan** — Scheduled full-history scans catch secrets committed before Kuro was installed.
4. **Platform Monitoring** — GitHub/GitLab native secret scanning complements Kuro for code already on the platform.

### Defense-in-Depth (v1.5.0)

v1.5.0 enforces defense-in-depth at every layer:

| Layer | Mechanism |
|-------|-----------|
| **Database isolation** | RLS policies enforce tenant filtering at the database level. All services connect as `kuro_app` (non-superuser, no `BYPASSRLS`). |
| **Application isolation** | All repository queries include explicit `organization_id` predicates as defense-in-depth. |
| **Fail-closed gates** | `PROXY_FAIL_MODE` defaults to `closed` — pushes blocked on scan failure. |
| **Clone URL validation** | `validateCloneURL` restricts clone operations to allowlisted hosts (`KURO_ALLOWED_GIT_HOSTS`). |
| **Token injection prevention** | `IsTokenAllowedHost` blocks token injection to arbitrary endpoints. |
| **Operator-controlled suppression** | Scanner suppression only via env vars — in-repo `.kuro.yml` suppression files not honored. |
| **Multi-ref push scanning** | `parsePushRefs` handles edge cases in push ref parsing that could bypass scanning. |
| **Webhook SSRF hardening** | `resolveOrgID` validates organization resolution; webhook endpoint rejects non-allowed hosts. |
| **CSV injection prevention** | Export endpoints sanitize cell values to prevent formula injection in spreadsheet clients. |

See [ARCHITECTURE.md](docs/ARCHITECTURE.md) for the full threat model.
