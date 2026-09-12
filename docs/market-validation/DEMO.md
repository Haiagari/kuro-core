# Kuro Core Phase 0/1 Demo

This kit is a short, repeatable validation artifact for conversations with developers and security-minded engineering teams. It demonstrates the current **local CLI gate**; it is not a production deployment or an organizational-control claim.

## Run the synthetic demo

Prerequisites:

- Docker 24+ or Podman 4+ installed and usable.
- A built binary at `./bin/kuro` (`make build`), or an executable path supplied with `KURO_BIN`.
- Scanner images available or pullable on the first run.

From the repository root:

```bash
./scripts/market-validation-demo.sh
# Or, with another existing binary:
KURO_BIN=/path/to/kuro ./scripts/market-validation-demo.sh
```

The script:

1. Runs `kuro doctor`.
2. Creates a clean synthetic Go fixture and a separate synthetic `.env` finding fixture under `mktemp -d`.
3. Runs `kuro scan PATH --json` for each fixture. Flags after the path are supported.
4. Captures exit codes without aborting on the expected finding result.
5. Removes the temporary directory on exit.

Expected gate contract:

| Fixture | JSON decision | Exit code |
| --- | --- | ---: |
| Clean synthetic project | `pass` | `0` |
| Synthetic finding | `block` (or `blocked`) | `1` |

`review` is a valid policy outcome with exit code `2`. Any scan or execution error is fail-closed and uses exit code `1`. For scripts, inspect the root JSON object's `decision`; finding objects are not the gate decision.

## Optional attestation discussion

Attestation is a separate verification surface, not an issuance step in this demo. Given an existing standalone envelope supplied by the customer or their gatekeeper:

```bash
./bin/kuro attest inspect ./attestation.json
./bin/kuro attest verify --file ./attestation.json
./bin/kuro attest verify --file ./attestation.json --pubkey ./attestation.pub
```

- `inspect` decodes and pretty-prints the in-toto statement; it does not verify a signature.
- `verify` can read a standalone envelope. With `--pubkey` (or `KURO_ATTESTATION_PUBLIC_KEY`) it checks the Ed25519 signature. Without a public key it decodes the statement but skips cryptographic verification; do not present that as cryptographic proof.
- The commands above inspect or verify an existing envelope. They do not mint a production attestation, and the demo does not claim that a local scan automatically issues one.

## What this validates

Use the demo to test whether a prospect values:

- A local, machine-readable pass/block result that can be placed in a developer workflow.
- A fail-closed result when a scanner or policy check errors.
- Optional offline inspection or signature verification of an attestation produced elsewhere.
- A low-friction path to try the behavior on synthetic content before discussing a pilot.

Observe actions rather than agreement: do they run it, provide a representative repository, ask to repeat it in their workflow, or commit budget and an owner?

## Explicit limitations to state

- **Docker/Podman is a prerequisite.** Core runs the scanners in local containers. First use may need to pull images; scans themselves use hardened, network-disabled scanner containers.
- **The proxy is optional.** `kuro proxy` is a local Smart-HTTP remote that can gate pushes before forwarding to an upstream forge. It is not required for direct local scans.
- **The proxy materializes push data temporarily.** Its receive-pack path writes the request, pack, and extracted files into temporary work directories for scanning. A deployment must account for temporary disk space and filesystem permissions; this is not durable repository storage.
- **Local mode is local.** The proxy's default `SCAN_MODE=local` invokes `kuro scan --json`; API/Enterprise mode is a separate configuration and product surface.
- **No organizational-enforcement claim.** This kit does not prove policy adoption, repository coverage, CI coverage, audit readiness, or centralized enforcement. Those require a scoped pilot with the customer's workflow, ownership, and evidence.
- **Attestation issuance is out of scope.** Core documents the local verify/inspect surface; do not imply that this demo creates signed production attestations.

## Suggested demo close

Ask: “Which real workflow would you put this result into, what would make the result actionable, and what evidence would you need before paying for a pilot?” Record the workflow, owner, next artifact, and dated next step—not just positive feedback.
