# Contributing to Kuro Core

Thank you for contributing to Kuro Core! This document provides guidelines for setting up your local development environment, running tests, and submitting changes.

---

## 🛠️ Development Setup

### Prerequisites
- **Go**: 1.26+
- **Docker** or **Podman**: 24+ (Docker) / 4+ (Podman)
- **Make**: For running build automation

### Build & Run Locally
```bash
# Clone the repository
git clone https://github.com/Haiagari/kuro.git
cd kuro

# Compile the standalone CLI binary
make build

# The binary is located at bin/kuro
./bin/kuro version
```

---

## 🧪 Testing

Kuro Core requires all tests and vet checks to pass before merging:

```bash
# Run all unit and chaos tests
make test

# Run code linters
make lint

# Generate test coverage report
make test-coverage
```

---

## 📁 Repository Structure

```
kuro-core/
├── bin/                      # Compiled binary outputs (gitignored)
├── cli/                      # Standalone CLI and TUI implementation
│   ├── cmd/                  # Subcommands (scan, fix, canary, doctor)
│   └── internal/
│       ├── orchestrator/     # 6-phase scan pipeline & Docker adapters
│       └── tui/              # Bubbletea terminal interface
├── deploy/
│   └── policies/             # Static JSON policy rules (default-policy.json)
├── docs/                     # Technical documentation & architecture
├── services/
│   └── git-proxy/            # Standalone fail-closed local Git proxy (:8000)
├── go.work                   # Go multi-module workspace (cli + git-proxy)
├── Makefile                  # Build, test, and install targets
└── README.md                 # Project overview and usage
```

---

## 📜 Commit Conventions

We follow the **Conventional Commits** specification:
- `feat(scope): ...`
- `fix(scope): ...`
- `docs(scope): ...`
- `refactor(scope): ...`
- `test(scope): ...`

*Note: Do not add "Co-Authored-By" or AI attribution lines to commit messages.*
