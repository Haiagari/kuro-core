# Scanner architecture — current development branch

Scope: Core Docker/Podman execution. Related: [architecture](ARCHITECTURE.md), [CLI](API.md), [policies](POLICY.md).

## Images and coverage

| Scanner | Image tag | Scope |
|---|---|---|
| Gitleaks | docker.io/zricethezav/gitleaks:v8.30.1 | Working tree; Git history in history mode |
| Semgrep | docker.io/semgrep/semgrep:1.165.0 | Embedded Core SAST rules, not the full registry |
| Trivy | docker.io/aquasec/trivy:0.57.0 | Dependency vulnerability analysis with a prepared local DB |
| Checkov | docker.io/bridgecrew/checkov:3.2.400 | Selected when Dockerfile or `.tf` exists |
| TruffleHog | docker.io/trufflesecurity/trufflehog:3.81.0 | Git source in history mode; provider verification disabled |

Tags pin versions but are not immutable digests. Core does not currently attest image provenance. Image pulls occur outside scan containers; asynchronous pulls can race the first scan, and a failed scan should be retried after images are available.

## Execution controls

Every scan container uses `--rm`, `--network=none`, `--cap-drop=ALL`, `--security-opt=no-new-privileges`, `--memory=512m`, `--cpus=1.0`. Source mounts are read-only with `:ro,Z`; Semgrep rules are also read-only. Trivy's separate database/result cache is writable with `:Z`. Containers use the caller UID/GID to read private source and bundled-rule directories without changing host permissions, with an ephemeral home at `/tmp`. Rootless Podman uses `--userns=keep-id`. A root caller still runs as root; the root filesystem is writable.

`KURO_MAX_CONCURRENCY` defaults to 2. Per-scanner timeout is 5 minutes, or 15 in history mode. Source is read live; a read-only mount prevents scanner writes but does not prevent other host processes modifying files. Avoid edits during scans. Container isolation reduces exposure; it is not a guarantee of zero exfiltration through all host channels or immunity to runtime vulnerabilities.

## Error handling and native reports

The container runner preserves process errors even when stdout exists. Gitleaks exit 1 is accepted only with a successfully parsed, nonempty findings report and no canceled context. Other configured scanners must exit 0. Checkov's `--soft-fail` delegates finding policy to Core; it does not suppress infrastructure/parse errors in Core.

- Gitleaks: `dir --report-format=json --report-path=/proc/self/fd/1 /src`, or `detect --source=/src` for history.
- Semgrep: `semgrep scan --config=/kuro-rules/semgrep-core.yml --metrics=off --strict --max-memory 350 --json /src`. Reported analysis errors block. No `--config=auto`.
- Checkov: configured for Terraform/Dockerfile with `--skip-download`; native report object or report array with `results.failed_checks`; parsing errors block. A flat finding array is rejected.
- TruffleHog: `git file:///src --json --no-update --no-verification --concurrency 1`. Malformed NDJSON blocks. Offline detections are not proof that a credential is active.
- Trivy: vulnerability scanner only; offline invocation described below. Missing report structure or process failure blocks.

Reports are normalized into findings and evaluated by the active policy. Core currently performs **no finding deduplication or Jaccard similarity analysis**; all emitted findings count toward thresholds.

## Trivy database lifecycle

Run the explicit online preparation helper from the repository root:

```bash
./scripts/update-trivy-db.sh
# Optional separate storage:
KURO_TRIVY_CACHE_DIR=/absolute/db-cache ./scripts/update-trivy-db.sh
```

The helper pulls the pinned Trivy image and downloads its main vulnerability DB into a staging directory. It mounts **no source directory**, serializes update writers and replaces the previous `db/` only after a successful download. A failed download retains the previous DB. Do not refresh while scans use the cache; interruptions may leave `.update-lock` for operator inspection.

Default cache: `$HOME/.kuro/trivy`. Supply the same `KURO_TRIVY_CACHE_DIR` to scans if changed. For air-gapped hosts copy `db/trivy.db` and `db/metadata.json` from a networked preparation host, preserving the layout.

The scan checks a nonempty DB file and schema-2 metadata. `UpdatedAt` and `NextUpdate` must be present, not future/invalid, and the DB must be younger than 24 hours and before `NextUpdate`. Missing/incompatible/expired data blocks. Core does not independently validate DB provenance; a corrupt DB subsequently causes a Trivy execution failure.

Invocation uses the prepared cache mounted at `/trivy-cache`:

```text
fs --scanners=vuln --format=json --exit-code=0 --cache-dir=/trivy-cache
   --skip-db-update --skip-java-db-update --offline-scan /src
```

There is **no online fallback during scanning**. Java archive analysis requiring a Java DB is not configured by this preparation helper. The current scoper targets Go, npm/yarn, Python, Ruby and Rust manifests; it does not promise every language/package-manager combination.

Upstream reference: [Trivy v0.57 air-gap requirements](https://github.com/aquasecurity/trivy/blob/v0.57.0/docs/docs/advanced/air-gap.md).

## File statistics cache

`$HOME/.kuro/cache` (or `KURO_CACHE_DIR`) stores content hashes after a policy-permitted scan. Hash checks read content, including files with preserved size/mtime. This is **statistics only**: every scan runs the selected scanners and active policy. Historical approvals are never reused after source, policy, rules, images or DB changes. `--no-cache` / `KURO_NO_CACHE=1` disable statistics storage/use. Trivy's DB cache is separate and still required.

## Troubleshooting

| Symptom | Action |
|---|---|
| Missing images / initial scan failure | Pull the pinned image tags using Docker/Podman, then retry |
| Missing or expired Trivy DB | Run the preparation helper on a networked host or copy fresh DB files |
| Semgrep parse/config error | Fix source or embedded rules; do not replace the rules with an online registry |
| Checkov parsing error | Fix the IaC input; do not treat it as a clean result |
| SELinux/permission error | Inspect runtime permissions and `:Z` labeling; do not remove isolation flags |
| Synthetic secret unexpectedly passes | Use the Slack fixture in [Quickstart](../QUICKSTART.md) rather than AWS EXAMPLE keys |
