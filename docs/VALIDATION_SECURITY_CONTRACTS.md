# Security contract correction — validation record

Date: 2026-10-11. Base: `d4c83bd675d0279553524b6495d3511622e90f69` (`main`). Changes are proposed on `fix/core-security-contracts`; this record does not assert merge acceptance or a published release.

## Implemented

- Preserve scanner process failures despite stdout; narrowly accept Gitleaks finding exit 1.
- Reject recognized scanner analysis errors and malformed/native report mismatches; parse Checkov report objects/arrays and TruffleHog Git metadata.
- Separate online Trivy DB preparation from network-disabled vulnerability scans, validate DB schema/freshness and retain previous DB on failed preparation.
- Fail on invalid custom policies; combine filters, implement relative path globs, document and enforce mandatory safety floors.
- Always hash files for statistics; remove source-cache and local proxy approval bypasses.
- Require trusted-key attestation verification, DSSE PAE by default, expected-commit binding and explicit legacy compatibility. Inspection stays unverified.
- Align release filenames and source-version labeling; repair documented Core helper entrypoints.
- Align README, guides and diagram labels with implemented behavior; add a policy contract and a Docker CI scanner job.

## Executed locally

| Check | Result |
|---|---|
| `make test` with Go 1.26.4 | Pass |
| `make lint` (`go vet`) | Pass |
| `make build` | Pass |
| `go test -race ./cli/... ./services/git-proxy/...` | Pass |
| `python3 tests/test-install.py` | Pass: 3 tests; OS/architecture subcases cover Linux/macOS amd64/arm64 |
| `python3 tests/test-trivy-db-update.py` | Pass: staging/source isolation and previous-DB preservation fixtures |
| Changed shell scripts syntax checks | Pass |
| Local Markdown link existence and SVG XML checks | Pass |
| Changed SVG previews | Rendered and visually inspected |
| `make e2e-core` | Blocked: no usable Docker/Podman; script exits 2 |
| Native SAST/SCA/IaC/history smoke tests | Not executed locally; require container runtime |

Some tests use fake container executables and synthetic local release/DB fixtures. Those validate error propagation and command contracts, not actual scanner image execution. The real Git-note tests exercise signature verification and reject a signed note copied to another commit.

## Remaining acceptance evidence

The CI `Native Scanner Contracts` job pulls pinned image tags, prepares the DB without source mounts, runs Core clean/secret E2E and `tests/e2e-scanner-contracts.sh`. Its result must be checked separately before accepting real-image compatibility. Rootless Podman/SELinux, macOS runtime integration, native Windows/WSL and a production upstream-authenticated proxy were not validated here.

## Material behavior changes

- Previously cached clean paths now run scanners on every scan: correctness over previous short-circuit performance.
- Trivy-selected scans require prepared fresh data and block when unavailable/expired. Offline scan-time downloads are removed.
- `attest verify` without a key fails; raw-payload signatures require `--legacy-raw`; expected commit binding is required.
- Invalid custom policies fail instead of using a permissive fallback. Default safety floors cannot be lowered.
- Source builds report `dev`; release availability is distinct from historical source version labels.

No release tags, default-branch changes or merge actions are part of this correction. This work does not claim complete SLSA compliance, guaranteed vulnerability coverage, immutable source snapshots or organization-wide enforcement.
