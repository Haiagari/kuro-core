# Helper Scripts — Kuro Core

**Audience:** developers and CI maintainers working in [Haiagari/kuro-core](https://github.com/Haiagari/kuro-core).  
**Scope:** local Core helpers. These scripts do **not** start Postgres/NATS/MinIO.

Related: [../QUICKSTART.md](../QUICKSTART.md) · [../tests/README.md](../tests/README.md) · [../README.md](../README.md)

---

## Inventory

| Script | Purpose |
|---|---|
| `install.sh` | One-command installer for the Kuro Core CLI (release binaries) |
| `pre-commit-secrets.sh` | Git pre-commit hook running a local Gitleaks-oriented check |
| `setup-hooks.sh` | Installs local git hooks into `.git/hooks/` |
| `run-local-scan.sh` | Convenience wrapper for local scans |
| `run-tests.sh` | Local test harness runner |
| `run-validation.sh` / `prepare-validation.sh` | Validation / sanity helpers |
| `scan-container-image.sh` | Container image vulnerability helper |
| `check-docker-gid.sh` | Diagnostics for Docker socket group permissions |

---

## Installer (primary user entry)

```bash
# Latest from main branch script (downloads release asset)
curl -sSL https://raw.githubusercontent.com/Haiagari/kuro-core/main/scripts/install.sh | sh

# Pin Core v0.1.1
curl -sSL https://raw.githubusercontent.com/Haiagari/kuro-core/main/scripts/install.sh | sh -s -- v0.1.1
```

Always use the **Haiagari/kuro-core** raw URL — not historical `Haiagari/kuro` or personal forks.

After install:

```bash
kuro version
kuro doctor
```

---

## Hooks

```bash
./scripts/setup-hooks.sh
# installs scripts/pre-commit-secrets.sh into .git/hooks/pre-commit (typical)
```

**Pitfall:** Hooks complement but do not replace `kuro scan` / `kuro proxy`. Prefer the full Core happy path for gate coverage.

---

## Local scan / tests

```bash
./scripts/run-local-scan.sh
./scripts/run-tests.sh
# Prefer Make when possible:
make test
make e2e-core
```

---

## Docker socket diagnostics

If `kuro doctor` or scans fail with permission errors on `/var/run/docker.sock`:

```bash
./scripts/check-docker-gid.sh
```

Ensure your user can talk to the Docker/Podman daemon, then re-run `kuro doctor`.

---

## Troubleshooting

| Issue | Fix |
|---|---|
| Installer 404 / wrong binary | Confirm URL is `Haiagari/kuro-core`; pin `v0.1.1` |
| Hook fires but push still leaks | Use `kuro proxy` with `SCAN_MODE=local` |
| Validation scripts expect server stack | Use Core paths (`make e2e-core`); Enterprise uses different repos/scripts |

---

## See also

- [../CONTRIBUTING.md](../CONTRIBUTING.md)
- [../docs/ARCHITECTURE.md](../docs/ARCHITECTURE.md)
