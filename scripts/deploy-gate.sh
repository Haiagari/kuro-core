#!/usr/bin/env bash
set -euo pipefail

# Use Kuro CLI to run the deploy gate
# Exit codes:
# 0 = ready
# 1 = blocked by security policy
# 2 = misconfigured
# 3 = dependency unavailable
# 4 = manual review required

ROOT_DIR="${ROOT_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
KURO_BIN="$ROOT_DIR/kuro"

if [ ! -x "$KURO_BIN" ]; then
  printf '%s\n' "[error] kuro binary not found or not executable: $KURO_BIN"
  exit 2
fi

# Run the command and pass any extra arguments
"$KURO_BIN" deploy-gate "$@"
