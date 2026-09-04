# Tests — Kuro Core

This directory contains end-to-end, chaos, and resilience test suites for **Kuro Core** (CLI and local Git proxy):

---

## Test Suites

### 1. Unit & Chaos Monkey Tests
```bash
# Run chaos and failure injection tests against the local proxy
./tests/chaos-test.sh

# Run all unit tests across the workspace
make test
```

### 2. End-to-End (E2E) Pre-Push Proxy Tests
Simulates real git push traffic against the local interception proxy on `localhost:8000`:
```bash
./tests/e2e-proxy.sh
```

### 3. Security Hardening Tests
Validates container runtime flags, dropped capabilities, and non-privileged UID constraints:
```bash
./tests/security-hardening-test.sh
```
