# Attestation inspection and verification — development branch

Core verifies an existing envelope; a scan does not automatically issue an attestation. These commands do not demonstrate a SLSA level or validate a complete SLSA provenance predicate.

## Inspect untrusted content

```bash
kuro attest inspect ./attestation.json
```

Decodes a supported envelope and valid JSON payload. Output is explicitly **UNVERIFIED**. No signature, decision authenticity or commit binding is checked.

## Verify a signature and commit binding

```bash
# HEAD is resolved in the selected repository:
kuro attest verify --repo ./project --pubkey ./attestation.pub
# A standalone file needs an expected full commit digest, or a resolvable ref:
kuro attest verify --file ./attestation.json --commit <FULL_COMMIT_SHA> --pubkey ./attestation.pub
```

`--pubkey` or `KURO_ATTESTATION_PUBLIC_KEY` is required: a trusted 32-byte Ed25519 public key encoded as 64 hex characters, or a file containing it. The key must be obtained independently from a trusted issuer. `keyid` is only a hint; it is never a trust authority.

| Option | Default |
|---|---|
| `--commit` / `-c` | HEAD |
| `--repo` / `-r` | . |
| `--file` / `-f` | Read Git note on `refs/notes/kuro-attestation` |
| `--pubkey` / `-p` | KURO_ATTESTATION_PUBLIC_KEY; absence is an error |
| `--legacy-raw` | false; explicit historical format compatibility |

Verification checks the supported payload type, at least one signature against the supplied key, the statement and predicate types, required security-gate fields, and a subject digest matching the expected full SHA-1/SHA-256 commit. A note copied onto a different commit is rejected. Standalone files with a full digest can be verified without a local repository. Repository identity, issuer authorization beyond the trusted key, predicate freshness and the expected policy hash are not independently enforced.

| Outcome | Exit |
|---|---:|
| Valid signature/binding with PASS or APPROVED (case-insensitive) | 0 |
| Valid signature/binding with REVIEW, BLOCK, BLOCKED or DENY | 2 |
| Missing key/note, malformed content, unknown decision, bad signature or wrong commit | 1 |

## Signature format

Default verification uses Ed25519 over **DSSE Pre-Authentication Encoding (PAE)**:

`DSSEv1 <type-byte-length> <payloadType> <payload-byte-length> <payload-bytes>`

Envelope `payloadType` must be `application/vnd.in-toto+json`. Payload `_type` must be `https://in-toto.io/Statement/v1`, and `predicateType` must be `https://kuro.dev/attestation/security-gate/v1`. Each subject requires a name and full `digest.commit`. The predicate requires decision, scan_id, scanners, issuer, issued_at and a SHA-256 policy_hash. Standard/URL-safe padded or unpadded base64 is accepted. Any valid signature under the supplied key satisfies the single-key trust model.

Historical Core envelopes signed raw statement bytes instead of PAE. They are rejected by default. Use `--legacy-raw` only for a knowingly trusted historical issuer; output labels this format as **not DSSE**. No automatic downgrade is attempted.

## Keys and Git notes

```bash
kuro attest keygen
# Fetch existing notes when using the note workflow:
git fetch origin refs/notes/kuro-attestation:refs/notes/kuro-attestation
```

Key generation prints the private key to the terminal. Keep it out of logs and source control. Core does not provide managed key storage, issuance, rotation or revocation.

Protocol reference: [DSSE protocol](https://github.com/secure-systems-lab/dsse/blob/master/protocol.md). Related: [API](API.md), [architecture](ARCHITECTURE.md), [security reporting](../SECURITY.md).
