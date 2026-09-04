#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="${ROOT_DIR:-$(cd "$SCRIPT_DIR/.." && pwd)}"
MODE="${1:-secure}"
PYTHON_BIN="${PYTHON_BIN:-$ROOT_DIR/.venv/bin/python}"
if [ ! -x "$PYTHON_BIN" ]; then
  PYTHON_BIN="python3"
fi

case "$MODE" in
  secure)
    VALIDATION_TARGET="secure-app"
    TEST_TARGET="$ROOT_DIR/app-validation/fixed"
    SCAN_TARGET="secure"
    ;;
  vulnerable)
    VALIDATION_TARGET="vulnerable-lab"
    TEST_TARGET="$ROOT_DIR/app-validation/vulnerable"
    SCAN_TARGET="vulnerable"
    ;;
  *)
    printf '%s\n' "[blocked] unknown validation mode: $MODE"
    exit 1
    ;;
esac

export ROOT_DIR VALIDATION_TARGET

run_step() {
  local label="$1"
  shift
  printf '%s\n' "[info] $label"
  if "$@"; then
    printf '%s\n' "[ok] $label"
    return 0
  fi
  printf '%s\n' "[warn] $label failed; continuing to collect evidence"
  return 0
}

run_tests() {
  PYTEST_DISABLE_PLUGIN_AUTOLOAD=1 "$PYTHON_BIN" -m pytest --junitxml="$ROOT_DIR/reports/pytest-report.xml" "$TEST_TARGET"
}

run_scans() {
  ROOT_DIR="$ROOT_DIR" bash "$SCRIPT_DIR/run-local-scan.sh" "$SCAN_TARGET"
}

printf '%s\n' "[info] Starting validation flow: $MODE"

ROOT_DIR="$ROOT_DIR" "$SCRIPT_DIR/clean-reports.sh"
ROOT_DIR="$ROOT_DIR" "$SCRIPT_DIR/check-readiness.sh"

run_step "Running tests" run_tests
run_step "Running scans" run_scans
  run_step "Generating metrics" "$PYTHON_BIN" "$SCRIPT_DIR/collect-metrics.py"
  run_step "Generating report" "$PYTHON_BIN" "$SCRIPT_DIR/generate-final-report.py"
  run_step "Evaluating policy" "$PYTHON_BIN" "$SCRIPT_DIR/validate-policy.py"

if ROOT_DIR="$ROOT_DIR" bash "$SCRIPT_DIR/release-gate.sh"; then
  printf '%s\n' "[done] Validation flow completed with authorized release gate"
  exit 0
fi

printf '%s\n' "[done] Validation flow completed with blocked release gate"
exit 1
