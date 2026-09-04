# Deploy directory — Kuro Core

**Audience:** contributors looking for Core policy and security config assets.  
**Scope:** local Core artifacts only. This folder is **not** a full Enterprise server deploy.

| Path | Purpose |
|---|---|
| `policies/` | Static gate policy (`default-policy.json`) used by the local orchestrator |
| `security/` | Scanner-oriented base configs (e.g. Gitleaks / Semgrep helpers) |
| `install.sh` | Legacy/alternate installer helper — prefer [`scripts/install.sh`](../scripts/install.sh) documented in the README |

For product install and happy path, see:

- [../README.md](../README.md)
- [../QUICKSTART.md](../QUICKSTART.md)
- [../docs/ARCHITECTURE.md](../docs/ARCHITECTURE.md)

Enterprise server stacks (Postgres, NATS, MinIO, dashboards) live in [Haiagari/kuro-enterprise](https://github.com/Haiagari/kuro-enterprise), not here.
