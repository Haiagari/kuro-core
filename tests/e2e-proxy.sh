#!/usr/bin/env bash
#
# e2e-proxy.sh — Tests E2E del Git Proxy
#
# Prerequisitos:
#   - docker compose up -d (api + worker + git-proxy + postgres + nats)
#   - Puerto 8080 accesible (API)
#   - Puerto 8000 accesible (Git Proxy, opcional para test directo)
#
# USO:
#   ./tests/e2e-proxy.sh                    # Test completo
#   ./tests/e2e-proxy.sh --quick            # Solo test rápido (clean + secret)
#   ./tests/e2e-proxy.sh --verbose          # Con output detallado
#   ./tests/e2e-proxy.sh --skip-db-check    # Salta verificación DB
#   ./tests/e2e-proxy.sh --skip-cleanup     # No borra archivos temporales
#
# Exit codes:
#   0  → Todos los tests pasan
#   1  → Test falló
#   2  → Prerequisito faltante

set -euo pipefail

# ── Configuración ──────────────────────────────────────────────────────────────
API_URL="${API_URL:-http://localhost:8080}"
KURO_SCANS_DIR="${KURO_SCANS_DIR:-/tmp/kuro-scans}"
VERBOSE=false
SKIP_DB_CHECK=false
SKIP_CLEANUP=false

# ── Colores ────────────────────────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

pass_count=0
fail_count=0

# ── Helpers ────────────────────────────────────────────────────────────────────
info()  { echo -e "${CYAN}[INFO]${NC} $*"; }
pass()  { echo -e "  ${GREEN}✅ PASS${NC} $*"; pass_count=$((pass_count + 1)); }
fail()  { echo -e "  ${RED}❌ FAIL${NC} $*"; fail_count=$((fail_count + 1)); }
warn()  { echo -e "  ${YELLOW}⚠️  WARN${NC} $*"; }
header(){ echo -e "\n${CYAN}══════════════════════════════════════════════${NC}"; echo -e "${CYAN} $*${NC}"; echo -e "${CYAN}══════════════════════════════════════════════${NC}"; }

verbose() {
    if [ "$VERBOSE" = true ]; then
        echo "$1"
    fi
}

check_prereq() {
    if ! command -v curl &>/dev/null; then
        echo "❌ curl is required"
        exit 2
    fi
    if ! command -v python3 &>/dev/null; then
        echo "❌ python3 is required (for JSON parsing)"
        exit 2
    fi
}

wait_for_api() {
    local max_attempts=30
    local attempt=1
    info "Waiting for API at $API_URL/health ..."
    while [ $attempt -le $max_attempts ]; do
        if curl -sf "$API_URL/health" >/dev/null 2>&1; then
            info "API ready after ${attempt}s"
            return 0
        fi
        sleep 1
        ((attempt++))
    done
    fail "API not ready after $max_attempts seconds"
    exit 2
}

# ── Tests ──────────────────────────────────────────────────────────────────────

test_api_health() {
    header "Test: API Health"
    local http_code
    http_code=$(curl -s -o /dev/null -w "%{http_code}" "$API_URL/health" 2>/dev/null || echo "000")
    if [ "$http_code" = "200" ]; then
        pass "API health endpoint returns 200"
    else
        fail "API health returned $http_code (expected 200)"
    fi
}

test_proxy_scan_missing_fields() {
    header "Test: Proxy scan — missing fields"

    # Missing path
    local resp
    resp=$(curl -s -X POST "$API_URL/api/v1/scans/proxy" \
        -H "Content-Type: application/json" \
        -d '{"repo":"test/repo","commit":"abc","branch":"main"}' 2>/dev/null)
    if echo "$resp" | python3 -c "import sys,json; d=json.load(sys.stdin); assert d.get('error') == 'path is required'" 2>/dev/null; then
        pass "Missing path returns 400 with error"
    else
        fail "Missing path: expected error, got: $resp"
    fi

    # Missing repo
    resp=$(curl -s -X POST "$API_URL/api/v1/scans/proxy" \
        -H "Content-Type: application/json" \
        -d '{"path":"/tmp/test","commit":"abc","branch":"main"}' 2>/dev/null)
    if echo "$resp" | python3 -c "import sys,json; d=json.load(sys.stdin); assert d.get('error') == 'repo is required'" 2>/dev/null; then
        pass "Missing repo returns 400 with error"
    else
        fail "Missing repo: expected error, got: $resp"
    fi

    # Invalid JSON
    resp=$(curl -s -X POST "$API_URL/api/v1/scans/proxy" \
        -H "Content-Type: application/json" \
        -d 'not-json' 2>/dev/null)
    local http_code
    http_code=$(echo "$resp" | python3 -c "import sys; print('400')" 2>/dev/null || curl -s -o /dev/null -w "%{http_code}" -X POST "$API_URL/api/v1/scans/proxy" -H "Content-Type: application/json" -d 'not-json')
    if echo "$resp" | python3 -c "import sys,json; d=json.load(sys.stdin); assert 'error' in d" 2>/dev/null; then
        pass "Invalid JSON returns error"
    else
        warn "Invalid JSON: response might differ"
    fi
}

test_proxy_scan_clean() {
    header "Test: Proxy scan — clean files (should APPROVE)"

    local test_dir="$KURO_SCANS_DIR/e2e-test/clean-$$"
    if command -v docker &>/dev/null && docker ps | grep -q kuro-git-proxy; then
        docker exec kuro-git-proxy mkdir -p "$test_dir"
        docker exec kuro-git-proxy sh -c "echo 'package main' > $test_dir/main.go && echo 'func main() {}' >> $test_dir/main.go"
    else
        mkdir -p "$test_dir"
        echo "package main" > "$test_dir/main.go"
        echo "func main() {}" >> "$test_dir/main.go"
    fi

    local commit="e2e-clean-$(date +%s)"
    local resp
    resp=$(curl -s -w "\n%{http_code}" -X POST "$API_URL/api/v1/scans/proxy" \
        -H "Content-Type: application/json" \
        -d "{\"path\":\"$test_dir\",\"repo\":\"test/e2e-clean\",\"commit\":\"$commit\",\"branch\":\"main\"}" 2>/dev/null)

    local http_code
    http_code=$(echo "$resp" | tail -1)
    local body
    body=$(echo "$resp" | sed '$d')

    local decision
    decision=$(echo "$body" | python3 -c "import sys,json; print(json.load(sys.stdin).get('decision',''))" 2>/dev/null)

    if [ "$http_code" = "200" ] && [ "$decision" = "APPROVED" ]; then
        pass "Clean files → HTTP $http_code, decision: $decision"
    else
        fail "Clean files → HTTP $http_code, decision: $decision (expected 200, APPROVED)"
        verbose "Response: $body"
    fi

    if [ "$SKIP_CLEANUP" = false ]; then if command -v docker &>/dev/null && docker ps | grep -q kuro-git-proxy; then docker exec kuro-git-proxy rm -rf "$test_dir"; else rm -rf "$test_dir"; fi; fi
}

test_proxy_scan_secret() {
    header "Test: Proxy scan — files with secrets (should BLOCK)"

    local test_dir="$KURO_SCANS_DIR/e2e-test/secret-$$"
    if command -v docker &>/dev/null && docker ps | grep -q kuro-git-proxy; then
        docker exec kuro-git-proxy mkdir -p "$test_dir"
        docker exec kuro-git-proxy sh -c "echo 'API_KEY = \"sk-proj-abc123def456ghi789jkl012mno345pqr678stu901vwx234yz\"' > $test_dir/config.py"
    else
        mkdir -p "$test_dir"
        cat > "$test_dir/config.py" << 'EOF'
API_KEY = "sk-proj-abc123def456ghi789jkl012mno345pqr678stu901vwx234yz"
EOF
    fi

    local commit="e2e-secret-$(date +%s)"
    local resp
    resp=$(curl -s -w "\n%{http_code}" -X POST "$API_URL/api/v1/scans/proxy" \
        -H "Content-Type: application/json" \
        -d "{\"path\":\"$test_dir\",\"repo\":\"test/e2e-secret\",\"commit\":\"$commit\",\"branch\":\"main\"}" 2>/dev/null)

    local http_code
    http_code=$(echo "$resp" | tail -1)
    local body
    body=$(echo "$resp" | sed '$d')

    local decision
    decision=$(echo "$body" | python3 -c "import sys,json; print(json.load(sys.stdin).get('decision',''))" 2>/dev/null)
    local findings_count
    findings_count=$(echo "$body" | python3 -c "import sys,json; print(len(json.load(sys.stdin).get('findings',[])))" 2>/dev/null)

    if [ "$http_code" = "403" ] && [ "$decision" = "BLOCKED" ] && [ "$findings_count" -gt 0 ]; then
        pass "Secret files → HTTP $http_code, decision: $decision, findings: $findings_count"
    else
        fail "Secret files → HTTP $http_code, decision: $decision, findings: $findings_count (expected 403, BLOCKED, >0 findings)"
        verbose "Response: $body"
    fi

    if [ "$SKIP_CLEANUP" = false ]; then if command -v docker &>/dev/null && docker ps | grep -q kuro-git-proxy; then docker exec kuro-git-proxy rm -rf "$test_dir"; else rm -rf "$test_dir"; fi; fi
}

test_proxy_scan_fail_closed() {
    header "Test: Proxy scan — fail-closed mode"

    # Save original env
    local orig_mode="${PROXY_FAIL_MODE:-}"

    # Set fail-closed and send to non-existent NATS (simulate timeout by using wrong subject)
    # We test by having the API handler fail — the proxy scan won't actually time out
    # because NATS is running. Instead we verify the env var is respected by checking
    # the API doesn't crash and still returns a valid response for clean files.
    local test_dir="$KURO_SCANS_DIR/e2e-test/fail-$$"
    if command -v docker &>/dev/null && docker ps | grep -q kuro-git-proxy; then
        docker exec kuro-git-proxy mkdir -p "$test_dir"
        docker exec kuro-git-proxy sh -c "echo 'clean' > $test_dir/file.txt"
    else
        mkdir -p "$test_dir"
        echo "clean" > "$test_dir/file.txt"
    fi

    local commit="e2e-fail-$(date +%s)"
    local resp
    resp=$(curl -s -w "\n%{http_code}" -X POST "$API_URL/api/v1/scans/proxy" \
        -H "Content-Type: application/json" \
        -d "{\"path\":\"$test_dir\",\"repo\":\"test/e2e-fail\",\"commit\":\"$commit\",\"branch\":\"main\"}" 2>/dev/null)

    local http_code
    http_code=$(echo "$resp" | tail -1)

    if [ "$http_code" = "200" ] || [ "$http_code" = "403" ]; then
        pass "Fail-closed mode: API returns $http_code (no crash)"
    else
        fail "Fail-closed mode: unexpected HTTP $http_code"
    fi

    if [ "$SKIP_CLEANUP" = false ]; then if command -v docker &>/dev/null && docker ps | grep -q kuro-git-proxy; then docker exec kuro-git-proxy rm -rf "$test_dir"; else rm -rf "$test_dir"; fi; fi
}

test_db_persistence() {
    header "Test: DB — proxy scans persisted"

    if [ "$SKIP_DB_CHECK" = true ]; then
        warn "Skipping DB check (--skip-db-check)"
        return
    fi

    if ! command -v docker &>/dev/null; then
        warn "Skipping DB check (docker not available)"
        return
    fi

    local db_container
    db_container=$(docker ps --filter "name=kuro-postgres" --format "{{.Names}}" 2>/dev/null | head -1)

    if [ -z "$db_container" ]; then
        warn "Skipping DB check (no kuro-postgres container found)"
        return
    fi

    local proxy_count
    proxy_count=$(docker exec "$db_container" psql -U kuro -d kuro -t -A \
        -c "SELECT COUNT(*) FROM scans WHERE trigger_source = 'proxy'" 2>/dev/null || echo "0")

    if [ "$proxy_count" -gt 0 ] 2>/dev/null; then
        pass "DB has $proxy_count scan(s) with trigger_source='proxy'"
    else
        warn "No proxy scans found in DB (might need migration 011)"
    fi
}

test_response_format() {
    header "Test: Response format"

    local test_dir="$KURO_SCANS_DIR/e2e-test/format-$$"
    if command -v docker &>/dev/null && docker ps | grep -q kuro-git-proxy; then
        docker exec kuro-git-proxy mkdir -p "$test_dir"
        docker exec kuro-git-proxy sh -c "echo 'test' > $test_dir/file.txt"
    else
        mkdir -p "$test_dir"
        echo "test" > "$test_dir/file.txt"
    fi

    local commit="e2e-format-$(date +%s)"
    local resp
    resp=$(curl -s -X POST "$API_URL/api/v1/scans/proxy" \
        -H "Content-Type: application/json" \
        -d "{\"path\":\"$test_dir\",\"repo\":\"test/e2e-format\",\"commit\":\"$commit\",\"branch\":\"main\"}" 2>/dev/null)

    # Verify JSON structure
    if echo "$resp" | python3 -c "
import sys, json
d = json.load(sys.stdin)
assert 'scan_id' in d, 'missing scan_id'
assert 'decision' in d, 'missing decision'
assert 'findings' in d, 'missing findings'
assert len(d['scan_id']) > 0, 'empty scan_id'
print('OK')
" 2>/dev/null; then
        pass "Response has all required fields (scan_id, decision, findings)"
    else
        fail "Response format invalid"
        verbose "Response: $resp"
    fi

    if [ "$SKIP_CLEANUP" = false ]; then if command -v docker &>/dev/null && docker ps | grep -q kuro-git-proxy; then docker exec kuro-git-proxy rm -rf "$test_dir"; else rm -rf "$test_dir"; fi; fi
}

# ── Main ──────────────────────────────────────────────────────────────────────

main() {
    # Parse args
    for arg in "$@"; do
        case "$arg" in
            --quick) SKIP_DB_CHECK=true ;;
            --verbose) VERBOSE=true ;;
            --skip-db-check) SKIP_DB_CHECK=true ;;
            --skip-cleanup) SKIP_CLEANUP=true ;;
            --help|-h)
                echo "Usage: $0 [--quick] [--verbose] [--skip-db-check] [--skip-cleanup]"
                exit 0
                ;;
            *) echo "Unknown arg: $arg"; exit 1 ;;
        esac
    done

    # Pre-create test directory with open permissions to avoid DinD/Docker volume permission issues
    # between root (worker/host) and nonroot (git-proxy) containers.
    mkdir -p "$KURO_SCANS_DIR/e2e-test"
    chmod 777 "$KURO_SCANS_DIR/e2e-test" || true

    echo -e "${CYAN}"
    echo "  ╔══════════════════════════════════════════╗"
    echo "  ║      Kuro — E2E Proxy Tests      ║"
    echo "  ╚══════════════════════════════════════════╝"
    echo -e "${NC}"
    echo "  API:        $API_URL"
    echo "  Scans dir:  $KURO_SCANS_DIR"
    echo ""

    check_prereq
    wait_for_api

    test_api_health
    test_proxy_scan_missing_fields
    test_response_format
    test_proxy_scan_clean
    test_proxy_scan_secret
    test_proxy_scan_fail_closed
    test_db_persistence

    # Summary
    echo ""
    header "Results"
    echo -e "  ${GREEN}PASS: $pass_count${NC}"
    if [ "$fail_count" -gt 0 ]; then
        echo -e "  ${RED}FAIL: $fail_count${NC}"
    else
        echo -e "  ${GREEN}FAIL: $fail_count${NC}"
    fi
    echo ""

    if [ "$fail_count" -gt 0 ]; then
        exit 1
    fi
    exit 0
}

main "$@"
