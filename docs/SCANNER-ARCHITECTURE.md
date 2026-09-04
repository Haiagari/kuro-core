# Scanner Architecture — Kuro Core v0.1.1

**Audience:** engineers extending or debugging the local multi-scanner fleet.  
**Scope:** Core local Docker/Podman execution only. Enterprise sandbox (Firecracker, remote workers) is out of scope.

Related: [ARCHITECTURE.md](ARCHITECTURE.md) · [API.md](API.md) · [../QUICKSTART.md](../QUICKSTART.md)

---

## Overview

The orchestrator’s **LocalAdapter** (`cli/internal/orchestrator`) runs industry scanners as ephemeral containers, parses JSON, deduplicates findings, and feeds the policy gate.

```
LocalAdapter
   ├─ runGitleaks / runGitleaksHistory
   ├─ runSemgrep          ← embedded offline ruleset (NOT --config=auto)
   ├─ runTrivy            ← prefers --offline-scan
   ├─ runCheckov
   └─ runTrufflehogHistory (history mode only)
            │
            ▼
      runContainer(...)   ← shared hardening flags
```

---

## Pinned images

Defined in `adapter_local.go` (Docker Hub via `docker.io/` prefix for Podman):

| Scanner | Image | Role |
|---|---|---|
| Gitleaks | `docker.io/zricethezav/gitleaks:v8.30.1` | Secrets (working tree `dir` or history `detect`) |
| Semgrep | `docker.io/semgrep/semgrep:1.165.0` | SAST with Core embedded rules |
| Trivy | `docker.io/aquasec/trivy:0.57.0` | SCA / lockfile CVEs |
| Checkov | `docker.io/bridgecrew/checkov:3.2.400` | IaC (Dockerfile, Terraform) |
| TruffleHog | `docker.io/trufflesecurity/trufflehog:3.81.0` | History-mode secret hunting |

Images are pulled asynchronously during **Fetch** so the first scan can progress while layers download.

---

## Container hardening

Every scanner invocation goes through `runContainer` (`container.go`):

```text
docker|podman run --rm
  --network=none
  --cap-drop=ALL
  --security-opt=no-new-privileges
  --memory=512m
  --cpus=1.0
  -v <host>:<container mount>
  <image> <args...>
```

| Control | Purpose |
|---|---|
| `--network=none` | No exfiltration / registry / rule downloads during scan |
| `--cap-drop=ALL` | Drop Linux capabilities |
| `no-new-privileges` | Block privilege escalation |
| `--memory=512m` / `--cpus=1.0` | Resource caps per container |
| `:ro,Z` mounts | Read-only source (SELinux-friendly). TruffleHog history uses `:Z` (needs write for cache) |

**Implication:** tools that expect network at scan time must be configured for offline use (see Semgrep & Trivy below).

---

## Semgrep — embedded offline ruleset

Core **does not** pass `--config=auto` (that requires network and breaks under `--network=none`).

Instead (`semgrep_run.go` + `semgrep_rules.go`):

1. Write embedded bytes (`semgrepCoreRules`) to a temp host file `semgrep-core.yml`.
2. Mount that directory at `/kuro-rules:ro,Z`.
3. Run:

```text
semgrep scan
  --config=/kuro-rules/semgrep-core.yml
  --metrics=off
  --max-memory 350
  --json
  /src
```

Source of truth for rule content: `cli/internal/orchestrator/rules/semgrep-core.yml` (high-signal subset suitable for Core/E2E).

**Pitfall:** Do not “fix” Semgrep by restoring `--config=auto` in Core — it will fail offline and break `make e2e-core`.

---

## Per-scanner invocation notes

### Gitleaks
- Working tree: `dir --report-format=json --report-path=/proc/self/fd/1 /src`
- History: `detect --source=/src ...`
- Exit code 1 with findings is treated as normal when stdout is present.

### Trivy
- Primary: `trivy fs --format=json --skip-db-update --offline-scan /src`
- If empty/error (DB not cached): retry without `--offline-scan`
- Non-critical: may return empty findings rather than failing the whole scan

### Checkov
- `checkov --directory=/src --output=json --soft-fail`
- Exit code `2` (no IaC files) → empty findings, not an error

### TruffleHog (history only)
- `filesystem /src --json --no-update --concurrency 1`
- Volume `:Z` (writable) for temp cache

---

## Scoping & parallelism

From `Scope` / `Run`:

- Parallel goroutines per selected scanner.
- Progress lines on stderr: `├─ gitleaks... N findings (t.s)`.
- File cache under `$HOME/.kuro/cache` can skip unchanged files for stats (best-effort).

---

## Finding pipeline

1. **Parse** — tool-specific JSON → unified `Finding` structs.
2. **Dedup** — SHA-256 over `scanner + rule_id + file_path + line_number`; Jaccard similarity collapses near-duplicates.
3. **Decide** — `deploy/policies/default-policy.json`.
4. **Report** — TUI / text / JSON + exit codes (`0` / `2` / `1`).

JSON shape and exit codes: [API.md](API.md).

---

## Offline constraints checklist

| Requirement | How Core satisfies it |
|---|---|
| No network during scan | `--network=none` |
| Semgrep without registry | Embedded `semgrep-core.yml` |
| Trivy without forced online | `--offline-scan` first; optional retry |
| TruffleHog no update | `--no-update` |
| Host safety | cap-drop, no-new-privileges, memory/CPU limits, mostly read-only mounts |

First-time **image pulls** still need network outside the scan containers (async `pull` in Fetch).

---

## Troubleshooting

| Symptom | Cause | Action |
|---|---|---|
| Semgrep empty / error offline | Someone switched to `--config=auto` | Restore embedded rules path |
| Trivy always empty | Vuln DB never downloaded | Allow one online retry path; pre-pull image + DB on a networked host |
| Permission / SELinux mount errors | Volume labels | Core uses `:Z` / `:ro,Z` — check Podman SELinux policy |
| Proxy local scan fails | Hardened scanners missing images | `kuro doctor`; pull pinned tags above |
| E2E secret fixture “passes” | Using AWS `EXAMPLE` keys | Use synthetic Slack token (see `tests/e2e-core-local.sh`) |

---

## See also

- [ARCHITECTURE.md](ARCHITECTURE.md)
- [../tests/README.md](../tests/README.md) — `security-hardening-test.sh`, `e2e-core-local.sh`
- [../SECURITY.md](../SECURITY.md)
