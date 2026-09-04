#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

REPORT_DIR="$PROJECT_ROOT/reports/ecosystem-pilot-gate"
SUMMARY_FILE="$REPORT_DIR/summary.md"
DECISION_FILE="$REPORT_DIR/decision.json"

mkdir -p "$REPORT_DIR"

GATE_PASSED=true
FAILURES=()
declare -A STEP_STATUS=()

run_step() {
    local step_key="$1"
    local step_name="$2"
    shift 2
    printf "\n========================================\n"
    printf "[*] %s\n" "$step_name"
    printf "========================================\n"
    if "$@"; then
        printf "[ok] %s PASSED\n" "$step_name"
        STEP_STATUS["$step_key"]="true"
    else
        printf "[FAIL] %s FAILED\n" "$step_name"
        GATE_PASSED=false
        FAILURES+=("$step_name")
        STEP_STATUS["$step_key"]="false"
    fi
}

run_expected_blocked_step() {
    local step_key="$1"
    local step_name="$2"
    shift 2
    printf "\n========================================\n"
    printf "[*] %s\n" "$step_name"
    printf "========================================\n"
    local output
    if output=$("$@" 2>&1); then
        printf '%s\n' "$output"
        printf "[FAIL] %s FAILED (expected BLOCKED, got success)\n" "$step_name"
        GATE_PASSED=false
        FAILURES+=("$step_name")
        STEP_STATUS["$step_key"]="false"
    else
        printf '%s\n' "$output"
        if printf '%s' "$output" | grep -q "Final decision: BLOCKED"; then
            printf "[ok] %s PASSED (blocked as expected)\n" "$step_name"
            STEP_STATUS["$step_key"]="true"
        else
            printf "[FAIL] %s FAILED (did not block as expected)\n" "$step_name"
            GATE_PASSED=false
            FAILURES+=("$step_name")
            STEP_STATUS["$step_key"]="false"
        fi
    fi
}

# Step 1: Readiness check
run_step readiness "Readiness Check" make -C "$PROJECT_ROOT" readiness

# Step 2: Full test suite
run_step full_tests "Full Test Suite" make -C "$PROJECT_ROOT" test

# Step 3: Contract tests
run_step contract_tests "Contract Tests" make -C "$PROJECT_ROOT" test-contracts

# Step 4: Sandbox smoke tests
run_step sandbox_smoke "Sandbox Smoke Tests" make -C "$PROJECT_ROOT" integration-sandbox-smoke

# Step 5: Security regression tests
run_step security_regression "Security Regression Tests" make -C "$PROJECT_ROOT" test-regression-security

# Step 6: validate-secure and validate-vulnerable
run_step validate_secure "Validate Secure" make -C "$PROJECT_ROOT" validate-secure
run_expected_blocked_step validate_vulnerable "Validate Vulnerable" make -C "$PROJECT_ROOT" validate-vulnerable

# Step 7: Evidence verification
run_step evidence_verify "Evidence Verify (latest)" "$PROJECT_ROOT/kuro" evidence verify --latest

# Step 8: Config doctor
run_step config_doctor "Config Doctor" "$PROJECT_ROOT/kuro" config doctor

# Generate decision.json
TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
mkdir -p "$REPORT_DIR"
FAILURES_JSON="[]"
if [ ${#FAILURES[@]} -gt 0 ]; then
    FAILURES_JSON="["
    for idx in "${!FAILURES[@]}"; do
        if [ "$idx" -gt 0 ]; then
            FAILURES_JSON+=", "
        fi
        FAILURES_JSON+="\"${FAILURES[$idx]}\""
    done
    FAILURES_JSON+="]"
fi
cat > "$DECISION_FILE" <<EOF
{
  "gate": "ecosystem-pilot",
  "phase": 8,
  "timestamp": "$TIMESTAMP",
  "passed": $GATE_PASSED,
  "failures": $FAILURES_JSON,
  "steps": {
    "readiness": ${STEP_STATUS[readiness]:-false},
    "full_tests": ${STEP_STATUS[full_tests]:-false},
    "contract_tests": ${STEP_STATUS[contract_tests]:-false},
    "sandbox_smoke": ${STEP_STATUS[sandbox_smoke]:-false},
    "security_regression": ${STEP_STATUS[security_regression]:-false},
    "validate_secure": ${STEP_STATUS[validate_secure]:-false},
    "validate_vulnerable": ${STEP_STATUS[validate_vulnerable]:-false},
    "evidence_verify": ${STEP_STATUS[evidence_verify]:-false},
    "config_doctor": ${STEP_STATUS[config_doctor]:-false}
  }
}
EOF

# Generate summary.md
{
    printf "# Ecosystem Pilot Gate Report\n\n"
    printf "**Timestamp:** %s\n\n" "$TIMESTAMP"
    printf "**Result:** %s\n\n" "$(if $GATE_PASSED; then echo "PASSED"; else echo "FAILED"; fi)"
    printf "## Steps Executed\n\n"
    printf "1. Readiness Check\n"
    printf "2. Full Test Suite\n"
    printf "3. Contract Tests\n"
    printf "4. Sandbox Smoke Tests\n"
    printf "5. Security Regression Tests\n"
    printf "6. Validate Secure\n"
    printf "7. Validate Vulnerable\n"
    printf "8. Evidence Verify (latest)\n"
    printf "9. Config Doctor\n\n"

    if [ ${#FAILURES[@]} -gt 0 ]; then
    printf "## Failures\n\n"
    for f in "${FAILURES[@]}"; do
            printf -- "- %s\n" "$f"
        done
        printf "\n"
    fi

    printf "## Decision\n\n"
    if $GATE_PASSED; then
        printf "Gate PASSED. Ecosystem pilot integrations can proceed to the next phase.\n"
    else
        printf "Gate FAILED. Resolve all failures before proceeding.\n"
    fi
} > "$SUMMARY_FILE"

printf "\n========================================\n"
if $GATE_PASSED; then
    printf "[ok] Ecosystem Pilot Gate PASSED\n"
else
    printf "[FAIL] Ecosystem Pilot Gate FAILED\n"
fi
printf "Report: %s\n" "$SUMMARY_FILE"
printf "Decision: %s\n" "$DECISION_FILE"
printf "========================================\n"
