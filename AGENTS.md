# AGENTS.md — Project Context & Architecture Guidelines for Kuro Core

This document serves as the authoritative context and architectural guide for AI coding assistants working on the **Kuro Core** repository.

---

## 1. Project Overview & Architecture

Kuro Core is an open-source, local-first zero-trust security gate and CLI designed to intercept git pushes, execute multi-scanner validation (Gitleaks, Semgrep, Trivy, Checkov), and remediate threats locally without server dependencies.

### Directory Structure

```
kuro-core/
├── bin/                             # Compiled binary outputs
├── cli/                             # CLI Tool & TUI (Go + Bubbletea)
│   ├── cmd/                         # CLI subcommands (scan, fix, canary, doctor)
│   └── internal/
│       ├── doctor/                  # Environment diagnostics engine
│       ├── orchestrator/            # 6-phase scan pipeline & Docker adapters
│       ├── output/                  # Formatting (JSON, SARIF, ANSI)
│       └── tui/                     # Interactive Bubbletea terminal interface
├── deploy/                          # Local policy rules
│   ├── policies/                    # Static default policies (default-policy.json)
│   └── security/                    # Scanner base configs (Gitleaks, Semgrep)
├── docs/                            # Technical documentation for Kuro Core
├── services/
│   └── git-proxy/                   # Local pre-push proxy listener (:8000)
├── tests/                           # Chaos and unit testing suites
├── Makefile                         # Unified build and test automation
└── go.work                          # Multi-module Go workspace (cli + git-proxy)
```

---

## 2. Technology Stack

| Domain | Technology / Spec |
|---|---|
| **Language** | Go 1.26+ |
| **CLI & TUI** | Bubbletea + Lipgloss |
| **Scanner Fleet** | Gitleaks, Semgrep, Trivy, Checkov (Ephemeral Docker/Podman) |
| **Interception Proxy** | Go HTTP/TCP Smart-Git Server (`git-receive-pack`) |
| **Policy Engine** | Static JSON policy evaluation |
| **License** | AGPL-3.0 |

---

## 3. Mandatory Governance & Conventions

- **Semantic Versioning 2.0.0**: Strictly follow `MAJOR.MINOR.PATCH`.
- **Conventional Commits**: Commit messages must follow format `feat(...)`, `fix(...)`, `docs(...)`.
- **Zero Attribution**: Never add "Co-Authored-By" or AI-assisted attribution to commits.
- **Fail-Closed Principle**: All security gates default to blocking if execution or parsing fails.
