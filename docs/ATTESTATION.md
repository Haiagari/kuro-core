# Cryptographic Attestation — Kuro Core v0.1.1

**Audience:** developers and release engineers verifying supply-chain gate attestations offline.  
**Scope:** Core CLI (`kuro attest`). Signing infrastructure that *issues* production attestations may live in Enterprise; Core focuses on **verify / keygen / inspect**.

| Spec | Value |
|---|---|
| Statement | in-toto Statement v1 / SLSA-style provenance |
| Predicate type (conceptual) | `https://kuro.dev/attestation/security-gate/v1` |
| Signature | Ed25519 |
| Git notes ref | `refs/notes/kuro-attestation` |

Related: [API.md](API.md) · [ARCHITECTURE.md](ARCHITECTURE.md) · [../QUICKSTART.md](../QUICKSTART.md)

---

## Overview

When a security gate approves a commit, an in-toto statement (decision, scanners, policy hash, issuer, timestamp) can be wrapped in a signed DSSE-like JSON envelope and stored either:

- as a **git note** on `refs/notes/kuro-attestation`, or
- as a **standalone JSON file**.

`kuro attest verify` reads that envelope, optionally checks the Ed25519 signature against a public key, and prints the predicate. No server is required for verification.

```
Git commit (HEAD)
        │
        ▼
kuro attest verify [--pubkey …]
        │
        ├─ git notes --ref=refs/notes/kuro-attestation show <commit>
        │     or --file envelope.json
        ▼
Decode payload → optional Ed25519 verify → print decision / scanners / issued_at
```

---

## Commands

### Generate a keypair (local / test)

```bash
kuro attest keygen
```

Prints:

- Key ID (SHA-256 fingerprint prefix)
- Public key (hex, safe to distribute / set as `KURO_ATTESTATION_PUBLIC_KEY`)
- Private key (hex — keep secret; Enterprise signing often uses `KURO_ATTESTATION_PRIVATE_KEY`)

### Verify a commit note

```bash
# Public key from env or flag (hex string or path to file containing hex)
export KURO_ATTESTATION_PUBLIC_KEY="$(cat attestation.pub)"

kuro attest verify
kuro attest verify --commit abc1234 --repo /path/to/repo
kuro attest verify --pubkey /path/to/attestation.pub
kuro attest verify --commit HEAD --pubkey "$KURO_ATTESTATION_PUBLIC_KEY"
```

Flags:

| Flag | Default | Meaning |
|---|---|---|
| `--commit` / `-c` | `HEAD` | Commit SHA |
| `--repo` / `-r` | `.` | Git repository path |
| `--pubkey` / `-p` | `$KURO_ATTESTATION_PUBLIC_KEY` | 32-byte Ed25519 public key (hex or file) |
| `--file` / `-f` | — | Standalone envelope JSON instead of git notes |

**Expected success output:** `VERIFIED KURO CRYPTOGRAPHIC ATTESTATION` plus decision, scan ID, scanners, issuer, issued-at, commit digest.

**Exit codes:**

| Situation | Code |
|---|---|
| Signature OK and decision `PASS` / `APPROVED` | `0` |
| Missing note / malformed envelope / bad signature / invalid key | `1` |
| Signature OK but decision is not PASS/APPROVED | `2` (warning) |

If `--pubkey` / env is omitted, Core still decodes and prints the statement but **skips** cryptographic verification.

### Verify a standalone file

```bash
kuro attest verify --file ./kuro-attestation.json --pubkey ./attestation.pub
```

### Inspect (decode only)

```bash
kuro attest inspect ./kuro-attestation.json
```

Pretty-prints the base64-decoded in-toto statement from the envelope. Does not require a public key.

---

## Envelope shape (conceptual)

```json
{
  "payloadType": "application/vnd.in-toto+json",
  "payload": "<base64 in-toto statement>",
  "signatures": [
    { "keyid": "…", "sig": "<base64 Ed25519 over raw statement bytes>" }
  ]
}
```

Decoded statement subjects typically include a commit digest; the Kuro predicate includes fields such as `decision`, `scan_id`, `scanners`, `policy_hash`, `issued_at`, `issuer`.

---

## Prerequisites & pitfalls

| Pitfall | Guidance |
|---|---|
| No note on commit | Ensure notes were pushed/fetched: `git fetch origin refs/notes/kuro-attestation:refs/notes/kuro-attestation` |
| Invalid public key | Must be 32-byte hex (64 hex chars), not PEM |
| Verify without pubkey | Decodes only — do not treat as cryptographic proof |
| Expecting Core to *mint* production attestations | Issuance is typically an Enterprise/gatekeeper concern; Core ships verify/keygen/inspect |

---

## See also

- [API.md](API.md) — CLI surface summary
- [ARCHITECTURE.md](ARCHITECTURE.md) — where attestation sits in the product
- [../SECURITY.md](../SECURITY.md) — reporting crypto bugs
