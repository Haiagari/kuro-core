<div align="center">

# 🛡️ KURO CORE

**Local-First Zero-Trust Security Gate & Multi-Scanner CLI**  
*Intercepts and validates every `git push` with multi-scanner SAST/SCA/Secrets, interactive terminal remediation, and honeypot canary deception — 100% self-contained on your machine with zero server dependencies.*

[![Go Version](https://img.shields.io/badge/Go-1.26-00ADD8?style=flat-square&logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-AGPL--3.0-blue?style=flat-square)](LICENSE)
[![Version](https://img.shields.io/badge/Release-v1.4.2-emerald?style=flat-square)](https://github.com/Haiagari/kuro/releases)

<br/>

```bash
# 🚀 Build and run Kuro Core CLI in seconds
git clone https://github.com/Haiagari/kuro.git && cd kuro
make build
./bin/kuro scan ./my-project
```

</div>

---

## 📑 Table of Contents

- [Overview](#-overview)
- [Architecture & How It Works](#-architecture--how-it-works)
- [Open-Core Editions (Community vs Enterprise)](#-open-core-editions)
- [Core Features](#-core-features)
  - [1. Multi-Scanner Parallel Engine](#1-multi-scanner-parallel-engine)
  - [2. Pre-Push Interception Proxy](#2-pre-push-interception-proxy)
  - [3. Interactive Terminal Auto-Remediation](#3-interactive-terminal-auto-remediation)
  - [4. Cyber Deception & Canary Tokens](#4-cyber-deception--canary-tokens)
- [CLI Command Reference](#-cli-command-reference)
- [Building & Installing](#-building--installing)
- [Documentation & Commercial Suite](#-documentation--commercial-suite)

---

## ⚡ Overview

**Kuro Core** is an open-source, local-first AppSec gatekeeper designed for individual developers and teams who want military-grade code security without sending code to the cloud or setting up complex infrastructure.

Running directly on your workstation via Docker or Podman, it coordinates 4 industry-standard security scanners, prevents leaks before they leave your computer, and gives you actionable remediation right inside your terminal.

---

## 🏛️ Architecture & How It Works

```mermaid
flowchart TD
    DevWorkstation["💻 Developer Workstation"] -->|git push origin main| GitProxy["🛡️ Local Git Proxy (:8000)\n(Fail-Closed & Leak Prevention)"]
    DevWorkstation -->|kuro scan / kuro fix| KuroCLI["⚡ Kuro CLI & Bubbletea TUI\n(6-Phase Orchestrator)"]

    subgraph KuroEngine["🔒 Local Container Isolation"]
        KuroCLI -->|Isolated Docker/Podman (--network none)| Scanners
        GitProxy -->|Pre-Push Scan| KuroCLI
        subgraph Scanners["🔍 Multi-Scanner Fleet"]
            Gitleaks["Gitleaks (Secrets)"]
            Semgrep["Semgrep (SAST)"]
            Trivy["Trivy (SCA/CVE)"]
            Checkov["Checkov (IaC/Terraform)"]
        end
    end

    Scanners -->|JSON Results| LocalAnalyzer["📊 Local Fingerprint Deduplication\n(SHA-256 & Jaccard)"]
    LocalAnalyzer --> Policy["📜 Policy Gate (default-policy.json)"]
    Policy -->|PASS / BLOCK| Verdict["✅ Terminal Report / Git Push Verdict"]
```

---

## 💎 Open-Core Editions

Kuro follows a transparent **Open-Core** distribution model:

| Capability | Kuro Core (Open-Source) | Kuro Enterprise (Commercial) |
|---|:---:|:---:|
| **Target Architecture** | Local-First / Single-Binary CLI | Distributed Multi-Tenant Platform |
| **License** | AGPL-3.0 | Commercial EULA |
| **Multi-Scanner Engine** (Gitleaks, Semgrep, Trivy, Checkov) | ✅ Included | ✅ Included |
| **Interactive Terminal TUI & Auto-Fix** (`kuro fix`) | ✅ Included | ✅ Included |
| **Cyber Deception & Canary Tokens** (`kuro canary`) | ✅ Included | ✅ Included |
| **Deduplication Engine** | ✅ SHA-256 Fingerprint & Jaccard | ✅ **AI Vector (Ollama + pgvector HNSW)** |
| **Web Command Center & Threat Studio** (Next.js 16) | ❌ Terminal TUI / JSON / SARIF | ✅ **Next.js 16 + React 19 + Recharts** |
| **Centralized Event & Notification Dispatcher** | ❌ Local stdout / exit codes | ✅ **Multi-Channel Webhooks (Slack/Discord/Telegram)** |
| **PostgreSQL Multi-Tenant Row-Level Security (RLS)** | ❌ Single-tenant / Local SQLite | ✅ **Active RLS Enforcement** |
| **Zero-Knowledge Finding Vault & Blind Indexing** | ❌ Plaintext storage | ✅ **AES-256-GCM + HMAC Index** |
| **in-toto / SLSA Cryptographic Commit Attestation** | ❌ | ✅ **Ed25519 Git Notes Signing** |
| **WebAssembly & OPA Rego Dynamic Policy Engine** | ❌ Static JSON rules | ✅ **Zero-Downtime Hot-Reloading** |
| **eBPF Linux Kernel Container Runtime Auditing** | ❌ | ✅ **Syscall Containment (SIGKILL)** |
| **Monorepo Smart Diff Delta Scanning (<50ms)** | ❌ Full tree scans | ✅ **Selective Tree-Walking** |
| **NATS JetStream Distributed Concurrency Locks** | ❌ In-memory mutex | ✅ **Multi-Node Cluster Scaling** |

---

## 🛠️ Core Features

### 1. Multi-Scanner Parallel Engine
Kuro Core coordinates multiple containerized scanners running in parallel with `--network none` and read-only mounts:
- **Gitleaks**: High-speed detection of API keys, tokens, and credentials.
- **Semgrep**: AST-based static application security testing (SAST).
- **Trivy**: Software Composition Analysis (SCA) for vulnerable open-source dependencies.
- **Checkov**: Infrastructure as Code (IaC) misconfiguration audits for Dockerfiles and Terraform.

### 2. Pre-Push Interception Proxy
A lightweight HTTP/TCP proxy that runs on `localhost:8000`. By configuring your git remote to point to the proxy, any `git push` is inspected before forwarding to GitHub/GitLab. If hardcoded secrets or critical vulnerabilities exist, the push is aborted with an error message in stderr.

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

## 💻 CLI Command Reference

```bash
# 1. Scan a project directory
kuro scan ./my-project
kuro scan ./my-project --json
kuro scan ./my-project --history

# 2. Interactive threat remediation & secret extraction
kuro fix ./my-project --dry-run
kuro fix ./my-project --auto

# 3. Canary token management
kuro canary generate --type aws
kuro canary verify <token>

# 4. Environment and runtime diagnostics
kuro doctor

# 5. Check active edition status
kuro license status
```

---

## 🔨 Building & Installing

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

## 📚 Documentation & Commercial Suite

- [Quickstart Guide](QUICKSTART.md) — Step-by-step tutorial for local usage.
- [Scanner Architecture](docs/SCANNER-ARCHITECTURE.md) — Adapter specifications.
- [Attestation Reference](docs/ATTESTATION.md) — SLSA and signature specs.

> 🏢 **Need Centralized Management?**  
> If you need multi-tenant PostgreSQL RLS, Web Dashboard, AWS Firecracker microVMs, or kernel-level eBPF containment, explore **Kuro Enterprise**.
