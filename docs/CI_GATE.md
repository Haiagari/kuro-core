# Trusted Kuro PR Gate

The local hook gives early feedback. This opt-in GitHub layer scans the proposed merge snapshot even if the author used `git push --no-verify`. It protects integration into `main`, not initial upload of source/secrets to GitHub or other branches.

## Acceptance policy

Core contains intentionally unsafe scanner fixtures and may have existing dependency/IaC findings. The gate adopts a **no new findings** policy rather than silently marking the entire repository clean:

- Scan both base and proposed merge with the same base-built Kuro, default policy, scanner versions and prepared Trivy DB.
- Pass only if both scans complete consistently and every candidate finding already exists at the same path/line/rule/severity/scanner in a **byte-identical whole file** on the base. Dependency findings additionally bind package/version detail and a digest of recognized dependency manifests/lockfiles; changing one invalidates dependency-debt inheritance.
- New findings and changes to any affected file block, even when the finding stays on the same line. Renames also require resolution. This deliberately favors safety over convenience.
- Existing findings in unchanged files remain visible as counted debt. No automatic test-folder exemptions or inline approval mechanism is added.
- Scanner errors, malformed reports, unresolved merge parents, symlinks/submodules or unmappable finding paths block. If base itself cannot be analyzed, the gate fails; an error is never a baseline exception.

This stage is stricter than Core's local severity policy for introduced findings: **any** introduced finding blocks. It is not a guarantee of complete scanner coverage, and does not yet scan each intermediate PR commit for removed secrets.

## Trust boundary

`kuro-pr-gate.yml` uses `pull_request_target` to load the workflow from the base. Only the exact base SHA is checked out and executed: toolchain build, Python gate and policy. Candidate Git objects are fetched and checked against the event's head/base SHA; their proposed merge parents must match. Files are materialized directly from blobs, without checkout filters, `.gitattributes` export exclusions, hooks, submodules, symlinks or execution of candidate builds/scripts.

`kuro scan --ci` uses builtin Gitleaks rules instead of candidate config/ignore files or allow comments, disables Semgrep inline/Git/Semgrep ignores, and ignores candidate Trivy ignore files. New Checkov suppressions in changed Terraform/Dockerfile sources fail before scanning. Existing Checkov suppressions in unchanged base files remain part of adopted debt; no claim is made that they were rescanned without suppression.

Containers have no network, read-only source, restricted resources/capabilities and no GitHub token. Online image/DB preparation has no source mount. Host subprocesses receive a restricted environment. Raw scanner reports/diagnostics are not logged or uploaded; the gate publishes fixed check summaries, counts and introduced-finding metadata (rule/path/line, without matched source or secret values).

The workflow has `contents: read` and `checks: write` only, references no repository secrets, uses immutable action SHAs, disables shared Go caching and persists no Git credentials. A custom check named **Kuro PR Gate** is created on both the exact PR head SHA and verified proposed-merge SHA, since the target-event workflow runs on the base context and GitHub may evaluate test-merge checks. Missing/unfinished publication on the evaluated SHA cannot satisfy a required successful check.

This trusts the base, its dependencies and scanner images, GitHub Actions and administrators. Administrators can change/disable rules. GitHub Actions as an expected check source is not an identity unique to this workflow; malicious contributors with write access who can create other privileged workflows require stronger organization-level workflow controls. This stage does not claim tamper resistance against repository administrators or all write-access collaborators.

## Activate after merge

The new target-event workflow is unavailable on the base until this implementation is merged. The initial PR therefore runs bootstrap/native acceptance in ordinary CI; that bootstrap executes the reviewed PR implementation and is **validation**, not a trusted enforcement check.

1. Merge the reviewed implementation; open/update a new PR and confirm **Kuro PR Gate** is emitted for its current head.
2. In **Settings → Rules → Rulesets → Protect main → Require status checks**, add **Kuro PR Gate**, retaining `Lint and Test` and `Gitleaks Secret Scan`.
3. Select **GitHub Actions** as the expected source when available, keep branches up to date before merge and avoid bypass actors. Do not select `Trusted Kuro Gate Runner` as a substitute for the explicit head/merge-bound check.
4. Use a synthetic vulnerable PR to verify its check fails and GitHub blocks merge; a clean PR must pass. Never publish actual credentials for testing.

Workflow installation alone is not enforcement. The connector used for development cannot modify repository administration/rulesets. The owner must activate the required check in GitHub. Until then, the check is advisory.

If the base or head moves during scanning, update the PR branch and rerun. Canceled runs may leave an in-progress check; the latest complete run on the current head is required. Changes to workflow/policy/CLI are evaluated with the previous trusted base version and take effect only after review/merge.

## Validation

`python3 tests/test-pr-gate.py` covers bypassed hooks, unchanged debt, same-line changes, archive-ignore evasion, unsupported links, suppressions, malformed/error reports and token isolation. `python3 tests/e2e-pr-gate.py` uses actual scanner images and a synthetic `git push --no-verify`; candidate ignores cannot hide the introduced Python finding. CI also runs a bootstrap scan of this implementation's proposed merge tree. Local validation includes 14 Python tests, Go tests/build/vet/race checks and actionlint validation of both changed workflows. Actual production check publication and branch-rule enforcement still require merge and owner activation.
