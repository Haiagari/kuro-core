# Architecture — Kuro Core v0.1.0

> **Edition**: Kuro Core (Standalone CLI & Local Proxy) · **Version**: v0.1.0

Kuro Core is a local-first security platform designed to catch secrets, vulnerabilities, IaC misconfigurations, and code quality issues directly on your workstation before commits are pushed to remote repositories.

---

## Core Philosophy: Local-First & Zero-Dependencies

Kuro Core operates entirely on your workstation without requiring background servers, databases, or external queues:
- **CLI & TUI**: Single Go binary coordinating scans and rendering interactive output via Bubbletea.
- **Local Pre-Push Proxy**: HTTP/TCP listener on `localhost:8000` that inspects git pushes and blocks leaks at the transport boundary.
- **Container Isolation**: Scanners run as ephemeral containers via Docker or Podman with `--network none` and `--read-only` mounts.

---

## The 6-Phase Pipeline

Every scan in Kuro Core follows a deterministic, 6-phase lifecycle implemented in `cli/internal/orchestrator`:

```
fetch ──► scope ──► scan ──► analyze ──► decide ──► report
```

1. **Fetch**: Validates target directory, initializes workspace context, and ensures local container runtimes are ready.
2. **Scope**: Inspects project manifests to selectively trigger relevant scanners:
   - Trivy activates when `package.json`, `go.mod`, or `requirements.txt` are detected.
   - Checkov activates when `Dockerfile`, `*.tf`, or `*.yaml` are present.
   - Gitleaks and Semgrep inspect all source files.
3. **Scan**: Spawns isolated Docker/Podman containers in parallel with strict memory caps (`--memory 1g`) and dropped Linux capabilities.
4. **Analyze**: Ingests raw JSON outputs from scanner containers and unifies them into structured findings.
5. **Decide**: Evaluates findings against `deploy/policies/default-policy.json`.
6. **Report**: Formats and renders findings in the interactive Bubbletea TUI, prints structured JSON for CI/CD, or outputs exit codes.

---

## Pre-Push Interception Architecture

The local Git Proxy (`services/git-proxy`) acts as an intermediate gate between your local git client and remote hosting providers:

```
[ git push ] ──► [ Local Git Proxy (:8000) ] ──► [ Fail-Closed Scanner ] ──► [ GitHub / GitLab ]
```

- **Fail-Closed Guarantee**: If scanning encounters an error or secrets are detected, the connection is closed with HTTP 403 and diagnostic messages are written to `stderr`.
- **Sideband Telemetry**: Streaming progress indicators are multiplexed back to the developer's terminal in real time.

---

## Enterprise Features

For distributed deployments requiring centralized web dashboards, PostgreSQL Row-Level Security, AWS Firecracker microVMs, eBPF kernel monitoring, and NATS JetStream scaling, refer to **Kuro Enterprise**.
