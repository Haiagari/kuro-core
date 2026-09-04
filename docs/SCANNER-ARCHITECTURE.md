# Scanner Architecture — Kuro Core v1.4.2

> **Edition**: Kuro Core (Standalone Edition) · **Version**: v1.4.2

This document describes the containerized scanner engine used in **Kuro Core**, covering the local adapter interface, Docker/Podman runtime execution, and finding deduplication.

---

## ⚡ Local Multi-Scanner Fleet

Kuro Core directly coordinates 4 specialized, containerized scanners on your workstation:

```
┌─────────────────────────────────────────────────────────────┐
│                    Kuro Core Orchestrator                   │
│         (cli/internal/orchestrator/adapter_local.go)        │
└──────────────────────────────┬──────────────────────────────┘
                               │
            ┌──────────────────┼──────────────────┐
            ▼                  ▼                  ▼
       ┌──────────┐       ┌──────────┐       ┌──────────┐
       │ Gitleaks │       │ Semgrep  │       │  Trivy   │ ... Checkov
       │ (Secrets)│       │  (SAST)  │       │  (SCA)   │
       └──────────┘       └──────────┘       └──────────┘
```

- **Gitleaks** (`zricethezav/gitleaks:v8.30.1`): Scans for API keys, bearer tokens, private keys, and high-entropy secrets.
- **Semgrep** (`semgrep/semgrep:1.165.0`): AST-based static code analysis for injection vulnerabilities, hardcoded logic, and OWASP Top 10.
- **Trivy** (`aquasec/trivy:0.57.0`): Scans package lockfiles (`package-lock.json`, `go.mod`, `requirements.txt`) against known CVE databases.
- **Checkov** (`bridgecrew/checkov:3.2.400`): Audits Infrastructure as Code (Dockerfiles, Terraform `.tf`) for security misconfigurations.

---

## 🔒 Runtime Sandboxing & Execution Constraints

Local scans execute with strict container isolation to protect host integrity:
1. **Network Disabled**: Containers are invoked with `--network none` to prevent exfiltration.
2. **Read-Only Mounts**: Target code is mounted read-only into `/src:ro`.
3. **Memory Limits**: Bounded to `--memory 1g` per scanner container.
4. **Dropped Capabilities**: Runs with reduced Linux capabilities and non-privileged UID mapping.

---

## 📊 Local Finding Deduplication

1. **Fingerprinting**: Each finding generates a deterministic SHA-256 hash based on `scanner + rule_id + file_path + line_number`.
2. **Jaccard Similarity**: Findings with identical messages across overlapping line windows are unified to reduce terminal noise.
