#!/usr/bin/env bash
# ⚠️ DEPRECATED: Usá `kuro scan` en vez de este script
set -euo pipefail

ROOT_DIR="${ROOT_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPORTS_DIR="$ROOT_DIR/reports"
PYTHON_BIN="${PYTHON_BIN:-$ROOT_DIR/.venv/bin/python}"
if [ ! -x "$PYTHON_BIN" ]; then
  PYTHON_BIN="python3"
fi

if [ -d "$ROOT_DIR/.venv/bin" ]; then
  export PATH="$ROOT_DIR/.venv/bin:$PATH"
fi
if [ -d "$ROOT_DIR/.bin" ]; then
  export PATH="$ROOT_DIR/.bin:$PATH"
fi

mkdir -p "$REPORTS_DIR"

TARGET_ARG="${1:-${TEST_APP:-secure}}"
case "$TARGET_ARG" in
  secure)
    TARGET_DIR="$ROOT_DIR/app-validation/fixed"
    VALIDATION_TARGET="secure-app"
    ;;
  vulnerable)
    TARGET_DIR="$ROOT_DIR/app-validation/vulnerable"
    VALIDATION_TARGET="vulnerable-lab"
    ;;
  *)
    if [[ "$TARGET_ARG" = /* ]]; then
      TARGET_DIR="$TARGET_ARG"
    else
      TARGET_DIR="$ROOT_DIR/$TARGET_ARG"
    fi
    VALIDATION_TARGET="$TARGET_ARG"
    ;;
esac

export VALIDATION_TARGET

run_if_available() {
  local tool="$1"
  shift

  if command -v "$tool" >/dev/null 2>&1; then
    "${@}" || true
  else
    printf '%s\n' "[skip] $tool is not installed"
  fi
}

printf '%s\n' "[info] Running local security scans"

if command -v pytest >/dev/null 2>&1; then
  PYTEST_DISABLE_PLUGIN_AUTOLOAD=1 "$PYTHON_BIN" -m pytest --junitxml="$REPORTS_DIR/pytest-report.xml" "$TARGET_DIR" || true
else
  printf '%s\n' "[skip] pytest is not installed"
fi

run_if_available semgrep semgrep scan \
  --config "$ROOT_DIR/deploy/security/semgrep/custom-rules.yml" \
  --json \
  --output "$REPORTS_DIR/semgrep-report.json" \
  "$TARGET_DIR"

run_if_available gitleaks gitleaks detect \
  --no-git \
  --source "$TARGET_DIR" \
  --report-format json \
  --report-path "$REPORTS_DIR/gitleaks-report.json" \
  --config "$ROOT_DIR/deploy/security/gitleaks/gitleaks.toml"

run_if_available trivy trivy fs \
  --config "$ROOT_DIR/deploy/security/trivy/trivy.yaml" \
  --output "$REPORTS_DIR/trivy-report.json" \
  "$TARGET_DIR"

run_if_available checkov checkov -d "$TARGET_DIR" \
  -o json > "$REPORTS_DIR/checkov-report.json"

"$PYTHON_BIN" "$SCRIPT_DIR/collect-metrics.py" || true

"$PYTHON_BIN" "$SCRIPT_DIR/normalize-scan-reports.py" || true
"$PYTHON_BIN" "$SCRIPT_DIR/generate-final-report.py" || true
"$PYTHON_BIN" "$SCRIPT_DIR/validate-policy.py" || true
"$PYTHON_BIN" "$SCRIPT_DIR/evaluate-alerts.py" || true

printf '%s\n' "[done] Local scan completed"
