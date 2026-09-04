# ─────────────────────────────────────────────────────────────────────────────
# SPDX-License-Identifier: AGPL-3.0-only
# Copyright (c) 2026 Sam Bleed
#
# Kuro Core v0.1.0 — Makefile (Standalone CLI & Local Proxy)
# Usage: make <target>
# ─────────────────────────────────────────────────────────────────────────────

.PHONY: build test test-unit test-coverage lint clean install proxy e2e-core help

SHELL := /bin/bash
BIN_DIR := bin
BINARY := $(BIN_DIR)/kuro

build:
	@echo "🔨 Compiling Kuro Core CLI..."
	@mkdir -p $(BIN_DIR)
	@cd cli && go build -ldflags="-s -w" -o ../$(BINARY) main.go
	@echo "✅ Built: $(BINARY)"

test: test-unit

test-unit:
	@echo "🧪 Running unit tests..."
	@go test -v -count=1 ./cli/... ./services/git-proxy/...

test-coverage:
	@echo "📊 Running tests with coverage..."
	@mkdir -p reports
	@go test -coverprofile=reports/coverage.out ./cli/... ./services/git-proxy/...
	@go tool cover -func=reports/coverage.out | tail -n 1
	@echo "Report saved to reports/coverage.out"

lint:
	@echo "🔍 Linting Go modules..."
	@go vet ./cli/... ./services/git-proxy/...
	@echo "✅ Vet checks passed"

install: build
	@echo "📦 Installing kuro binary to /usr/local/bin..."
	@install -m 755 $(BINARY) /usr/local/bin/kuro
	@echo "✅ Installed: /usr/local/bin/kuro"

# Local git-proxy (fail-closed pre-push gate) via the unified CLI.
# Optional env:
#   SCAN_MODE=local|api   (default local — runs kuro scan --json)
#   KURO_BIN=./bin/kuro   (path to CLI when SCAN_MODE=local)
proxy: build
	@echo "🚪 Starting local git-proxy on :8000 via ./bin/kuro proxy..."
	@echo "   Tip: SCAN_MODE=local KURO_BIN=$$(pwd)/bin/kuro (default local scan)"
	@echo "        SCAN_MODE=api for Enterprise API path"
	KURO_BIN=$$(pwd)/$(BINARY) $(BINARY) proxy

e2e-core: build
	@echo "🧪 Running Core-local E2E (no Postgres/NATS/API)..."
	@bash tests/e2e-core-local.sh

clean:
	@echo "🧹 Cleaning artifacts..."
	@rm -rf $(BIN_DIR) reports/
	@echo "✅ Clean completed"

help:
	@echo "Kuro Core — Available Targets:"
	@echo ""
	@echo "  make build         Compile standalone kuro CLI binary to bin/kuro"
	@echo "  make test          Run all unit tests across cli and git-proxy"
	@echo "  make test-coverage Generate test coverage report in reports/coverage.out"
	@echo "  make lint          Run go vet analysis"
	@echo "  make install       Install binary to /usr/local/bin/kuro"
	@echo "  make proxy         Build + run local git-proxy via ./bin/kuro proxy"
	@echo "  make e2e-core      Build + run Core-local E2E (tests/e2e-core-local.sh)"
	@echo "  make clean         Remove build artifacts"
	@echo "  make help          Show this menu"
