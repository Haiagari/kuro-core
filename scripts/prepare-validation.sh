#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MODE="${1:-secure}"

printf '%s\n' "[info] Preparing validation mode: $MODE"

if [ -x "$ROOT_DIR/scripts/clean-reports.sh" ]; then
  "$ROOT_DIR/scripts/clean-reports.sh"
fi

if [ -x "$ROOT_DIR/scripts/check-readiness.sh" ]; then
  "$ROOT_DIR/scripts/check-readiness.sh"
fi

required_tools=(python3 pip git)
optional_tools=(pytest semgrep gitleaks trivy checkov)

printf '%s\n' "[info] Checking required tools"
for tool in "${required_tools[@]}"; do
  if command -v "$tool" >/dev/null 2>&1; then
    printf '%s\n' "[ok] $tool"
  else
    printf '%s\n' "[missing] $tool"
  fi
done

printf '%s\n' "[info] Checking security tools"
for tool in "${optional_tools[@]}"; do
  if command -v "$tool" >/dev/null 2>&1; then
    printf '%s\n' "[ok] $tool"
  else
    printf '%s\n' "[skip] $tool not installed"
  fi
done

case "$MODE" in
  secure)
    printf '%s\n' "[info] Validation target: app-validation/fixed"
    printf '%s\n' "[info] Use: make validate-secure"
    ;;
  vulnerable)
    printf '%s\n' "[info] Validation target: app-validation/vulnerable"
    printf '%s\n' "[info] Use: make validate-vulnerable"
    ;;
  *)
    printf '%s\n' "[warn] Unknown mode, defaulting to secure guidance"
    printf '%s\n' "[info] Use: make validate-secure"
    ;;
esac

printf '%s\n' "[done] Validation preparation complete"
