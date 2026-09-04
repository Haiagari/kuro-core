<div align="center">

# KURO CORE

**Local-First Zero-Trust Security Gate & Multi-Scanner CLI**  
*Intercepts and validates every `git push` with multi-scanner SAST/SCA/Secrets, interactive terminal remediation, and honeypot canary deception — 100% self-contained on your machine with zero server dependencies.*

[![Go Version](https://img.shields.io/badge/Go-1.26-00ADD8?style=flat-square&logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-AGPL--3.0-blue?style=flat-square)](LICENSE)
[![Version](https://img.shields.io/badge/Release-v0.1.0-emerald?style=flat-square)](https://github.com/Haiagari/kuro-core/releases)

<br/>

```bash
# Build and run Kuro Core CLI in seconds
git clone https://github.com/Haiagari/kuro-core.git && cd kuro-core
make build
./bin/kuro doctor
./bin/kuro scan ./my-project
```

</div>

---

## Table of Contents

- [Overview](#overview)
- [Architecture & How It Works](#architecture--how-it-works)
- [Core Features](#core-features)
  - [1. Multi-Scanner Parallel Engine](#1-multi-scanner-parallel-engine)
  - [2. Pre-Push Interception Proxy](#2-pre-push-interception-proxy)
  - [3. Interactive Terminal Auto-Remediation](#3-interactive-terminal-auto-remediation)
  - [4. Cyber Deception & Canary Tokens](#4-cyber-deception--canary-tokens)
- [CLI Command Reference](#cli-command-reference)
- [Building & Installing](#building--installing)
- [Documentation](#documentation)

---

## Overview

**Kuro Core** is an open-source, local-first AppSec gatekeeper designed for individual developers and teams who want military-grade code security without sending code to the cloud or setting up complex infrastructure.

Running directly on your workstation via Docker or Podman, it coordinates 4 industry-standard security scanners, prevents leaks before they leave your computer, and gives you actionable remediation right inside your terminal.

For multi-tenant dashboards, NATS, and Firecracker sandboxes, see **Kuro Enterprise** (`Haiagari/kuro-enterprise`) — not required for Core.

---

## Architecture & How It Works

```mermaid
flowchart TB
    subgraph Client["Developer Workstation"]
        Dev["Developer Shell / IDE"]
        GitClient["Git Client (git push)"]
        CLICmd["kuro scan / kuro fix / kuro canary"]
    end

    subgraph Boundary["Transport Interception Boundary"]
        GitProxy["Local Git Proxy (:8000)\nkuro proxy (Fail-Closed)"]
    end

    subgraph CoreEngine["Kuro Core Engine (cli/internal/orchestrator)"]
        direction TB
        subgraph Pipeline["6-Phase Deterministic Pipeline"]
            P1["1. Fetch\n(Workspace Validation)"]
            P2["2. Scope\n(Manifest & Target Filter)"]
            P3["3. Scan\n(Parallel Executor)"]
            P4["4. Analyze\n(Parser & Aggregator)"]
            P5["5. Decide\n(Policy Gate Evaluation)"]
            P6["6. Report\n(Formatter & TUI)"]
            P1 --> P2 --> P3 --> P4 --> P5 --> P6
        end

        subgraph Analysis["Local Analysis & Remediation"]
            Dedup["Deduplication Engine\n(SHA-256 Fingerprint + Jaccard Similarity)"]
            PolicyGate["Policy Engine\n(deploy/policies/default-policy.json)"]
            AutoFix["Remediation Engine\n(Secret Replacement & .env.example)"]
        end
    end

    subgraph Sandbox["Container Execution Sandbox (--network none)"]
        direction LR
        subgraph Scanners["Isolated Scanner Containers (Read-Only Mount)"]
            S_Gitleaks["Gitleaks\n(Secrets Detection)"]
            S_Semgrep["Semgrep\n(SAST Engine)"]
            S_Trivy["Trivy\n(SCA & Dependency CVEs)"]
            S_Checkov["Checkov\n(IaC / Dockerfile / Terraform)"]
        end
        Runtime["Docker / Podman Daemon\n(Dropped Capabilities, Memory Limits)"]
    end

    subgraph Output["Decision & Upstream"]
        TUI["Bubbletea Interactive TUI / JSON / SARIF"]
        RemoteGit["Remote Git Forge\n(GitHub / GitLab / Bitbucket)"]
        AbortPush["Push Blocked (HTTP 403 + Stderr Report)"]
    end

    %% Flow connections
    GitClient -->|"Smart HTTP/TCP Forward"| GitProxy
    GitProxy -->|"Invoke Pre-Push Scan"| P1
    Dev --> CLICmd
    CLICmd --> P1

    P3 -->|"Spawn Ephemeral Containers"| Runtime
    Runtime --> Scanners
    Scanners -->|"Raw Output JSON"| P4

    P4 --> Dedup
    Dedup --> P5
    P5 --> PolicyGate
    PolicyGate -->|"Violations Found"| AutoFix
    AutoFix -.->|"Interactive Fix"| Dev

    P6 --> TUI
    PolicyGate -->|"Pass (Zero Violations)"| GitProxy
    PolicyGate -->|"Block (Policy Breach)"| AbortPush
    GitProxy -->|"Forward Push"| RemoteGit
```

---

## Core Features

### 1. Multi-Scanner Parallel Engine
Kuro Core coordinates multiple containerized scanners running in parallel with `--network none` and read-only mounts:
- **Gitleaks**: High-speed detection of API keys, tokens, and credentials.
- **Semgrep**: AST-based static application security testing (SAST).
- **Trivy**: Software Composition Analysis (SCA) for vulnerable open-source dependencies.
- **Checkov**: Infrastructure as Code (IaC) misconfiguration audits for Dockerfiles and Terraform.

### 2. Pre-Push Interception Proxy
A lightweight HTTP/TCP proxy that runs on `localhost:8000` via **`kuro proxy`** (same binary as the CLI). By configuring your git remote to point to the proxy, any `git push` is inspected before forwarding to GitHub/GitLab. If hardcoded secrets or critical vulnerabilities exist, the push is aborted with an error message in stderr. Docker/standalone images still build from `services/git-proxy`.

### 3. Interactive Terminal Auto-Remediation
Run `kuro fix` to inspect detected secrets interactively:
- Automatically replaces hardcoded secrets with `os.Getenv(...)`, `process.env[...]`, or configuration references.
- Generates `.env.example` templates safely.
- Supports dry-run previews (`--dry-run`) or fully unattended batch execution (`--auto`).

### 4. Cyber Deception & Canary Tokens
Plant decoy credentials in test fixtures to detect unauthorized intrusion or insider threats:
```bash
# Generate a fake AWS credential honeypot
kuro canary generate --type aws --format env

# Verify if a leaked token was generated by Kuro
kuro canary verify AKIAIOSFODNN7EXAMPLE
```

---

## CLI Command Reference

Core happy path (local, zero server):

```bash
# Diagnostics
kuro doctor

# Scan a project directory
kuro scan ./my-project
kuro scan ./my-project --json
kuro scan ./my-project --history

# Fail-closed local Git proxy (pre-push gate)
kuro proxy
kuro proxy --addr :8000 --upstream https://github.com

# Interactive threat remediation & secret extraction
kuro fix ./my-project --dry-run
kuro fix ./my-project --auto

# Canary token management
kuro canary generate --type aws
kuro canary verify <token>

# Attestation (in-toto / SLSA)
kuro attest verify
kuro attest keygen

# License / edition
kuro license status
```

Optional server / Enterprise-companion commands (`auth`, `deploy`, `setup`, `health`, `up`, `backup`, `webhook`, `scan --remote`) talk to a Kuro server stack. Prefer **Kuro Enterprise** for that path; Core’s default is fully local.

---

## Building & Installing

### Compile binary
```bash
make build
# Binary output: bin/kuro
```

### Run tests
```bash
make test
```

### Install globally
```bash
sudo make install
```

---

## Documentation

- [Quickstart Guide](QUICKSTART.md) — Step-by-step tutorial for local workstation usage.
- [Scanner Architecture](docs/SCANNER-ARCHITECTURE.md) — Multi-scanner engine and container sandbox specifications.
- [Attestation Reference](docs/ATTESTATION.md) — Cryptographic in-toto and SLSA provenance verification.
