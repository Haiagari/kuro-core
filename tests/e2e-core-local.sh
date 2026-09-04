#!/usr/bin/env bash
#
# e2e-core-local.sh — Kuro Core local E2E (NO Postgres / NATS / API)
#
# This exercises the standalone Core CLI path only:
#   doctor → scan clean (pass/0) → scan secret (block/1)
#
# For Enterprise / API+proxy stacks, use tests/e2e-proxy.sh instead.
#
# Prerequisites:
#   - Built binary at bin/kuro (run `make build` first), OR set KURO_BIN
#   - docker or podman available and usable (scanners run in containers)
#
# Usage:
#   ./tests/e2e-core-local.sh
#   make e2e-core
#
# Exit codes:
#   0  → All checks passed
#   1  → Test assertion failed
#   2  → Missing prerequisite (binary / container runtime)

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
KURO_BIN="${KURO_BIN:-$ROOT_DIR/bin/kuro}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

pass_count=0
fail_count=0

info()  { echo -e "${CYAN}[INFO]${NC} $*"; }
pass()  { echo -e "  ${GREEN}✅ PASS${NC} $*"; pass_count=$((pass_count + 1)); }
fail()  { echo -e "  ${RED}❌ FAIL${NC} $*"; fail_count=$((fail_count + 1)); }
warn()  { echo -e "  ${YELLOW}⚠️  WARN${NC} $*"; }
header(){ echo -e "\n${CYAN}══════════════════════════════════════════════${NC}"; echo -e "${CYAN} $*${NC}"; echo -e "${CYAN}══════════════════════════════════════════════${NC}"; }

have_runtime() {
  if command -v docker &>/dev/null; then
    if docker info &>/dev/null; then
      echo docker
      return 0
    fi
  fi
  if command -v podman &>/dev/null; then
    if podman info &>/dev/null; then
      echo podman
      return 0
    fi
  fi
  return 1
}

check_prereq() {
  if [ ! -x "$KURO_BIN" ]; then
    echo "❌ Missing executable: $KURO_BIN"
    echo "   Run: make build"
    exit 2
  fi

  local runtime
  if ! runtime="$(have_runtime)"; then
    echo "❌ Neither docker nor podman is available/usable"
    echo "   Core local scans require a container runtime for scanner images."
    exit 2
  fi
  info "Using container runtime: $runtime"
  info "Using binary: $KURO_BIN"
}

test_doctor() {
  header "Test: kuro doctor (warn OK; fail only on critical)"

  set +e
  out="$("$KURO_BIN" doctor 2>&1)"
  code=$?
  set -e

  echo "$out" | sed 's/^/  | /'

  if [ "$code" -eq 0 ]; then
    pass "doctor exit 0 (warnings allowed)"
  else
    fail "doctor exited $code (expected 0; critical failures only)"
  fi
}

test_scan_clean() {
  header "Test: scan clean temp dir → pass + exit 0"

  local tmp
  tmp="$(mktemp -d "${TMPDIR:-/tmp}/kuro-e2e-clean.XXXXXX")"
  cat > "$tmp/main.go" <<'EOF'
package main

func main() {}
EOF

  set +e
  out="$("$KURO_BIN" scan --json "$tmp" 2>&1)"
  code=$?
  set -e

  echo "$out" | sed 's/^/  | /'

  local decision
  decision="$(echo "$out" | python3 -c "import sys,json
text=sys.stdin.read()
dec=json.JSONDecoder()
i=0
decision=''
while True:
    j=text.find('{', i)
    if j<0: break
    try:
        obj, _ = dec.raw_decode(text[j:])
        if isinstance(obj, dict) and 'decision' in obj:
            decision=obj.get('decision','')
            break
    except json.JSONDecodeError:
        pass
    i=j+1
print(decision)" 2>/dev/null || true)"

  if [ "$code" -eq 0 ] && [ "$decision" = "pass" ]; then
    pass "clean scan → exit $code, decision=$decision"
  else
    fail "clean scan → exit $code, decision=$decision (expected 0 / pass)"
  fi

  rm -rf "$tmp"
}

test_scan_secret() {
  header "Test: scan temp dir with fake secret → block + exit 1"

  local tmp
  tmp="$(mktemp -d "${TMPDIR:-/tmp}/kuro-e2e-secret.XXXXXX")"
  cat > "$tmp/.env" <<'EOF'
# Synthetic slack bot token — AWS docs EXAMPLE keys are allowlisted by gitleaks.
SLACK_TOKEN=xoxb-123456789012-123456789012-abcdefghijklmnopqrstuvwx
EOF

  set +e
  out="$("$KURO_BIN" scan --json "$tmp" 2>&1)"
  code=$?
  set -e

  echo "$out" | sed 's/^/  | /'

  local decision
  decision="$(echo "$out" | python3 -c "import sys,json
text=sys.stdin.read()
dec=json.JSONDecoder()
i=0
decision=''
while True:
    j=text.find('{', i)
    if j<0: break
    try:
        obj, _ = dec.raw_decode(text[j:])
        if isinstance(obj, dict) and 'decision' in obj:
            decision=obj.get('decision','')
            break
    except json.JSONDecodeError:
        pass
    i=j+1
print(decision)" 2>/dev/null || true)"

  if [ "$code" -eq 1 ] && { [ "$decision" = "block" ] || [ "$decision" = "blocked" ]; }; then
    pass "secret scan → exit $code, decision=$decision"
  else
    fail "secret scan → exit $code, decision=$decision (expected 1 / block)"
  fi

  rm -rf "$tmp"
}

main() {
  echo -e "${CYAN}"
  echo "  ╔══════════════════════════════════════════╗"
  echo "  ║   Kuro Core — Local E2E (no server)      ║"
  echo "  ╚══════════════════════════════════════════╝"
  echo -e "${NC}"
  echo "  Enterprise/API proxy tests: tests/e2e-proxy.sh"
  echo ""

  check_prereq
  test_doctor
  test_scan_clean
  test_scan_secret

  echo ""
  header "Results"
  echo -e "  ${GREEN}PASS: $pass_count${NC}"
  if [ "$fail_count" -gt 0 ]; then
    echo -e "  ${RED}FAIL: $fail_count${NC}"
    exit 1
  fi
  echo -e "  ${GREEN}FAIL: 0${NC}"
  exit 0
}

main "$@"
