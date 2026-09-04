#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="${ROOT_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
# ponytail: legacy path, kept for compatibility
DECISION_FILE="$ROOT_DIR/reports/final/zero-trust-decision.json"
# ponytail: legacy name, kept for compatibility
WARNING_MODE="${ZERO_TRUST_WARNING_MODE:-block}"
PYTHON_BIN="${PYTHON_BIN:-$ROOT_DIR/.venv/bin/python}"
if [ ! -x "$PYTHON_BIN" ]; then
  PYTHON_BIN="python3"
fi

if [ ! -f "$DECISION_FILE" ]; then
  printf '%s\n' "[blocked] decision file not found: $DECISION_FILE"
  exit 1
fi

DECISION="$("$PYTHON_BIN" - "$DECISION_FILE" <<'PY'
import json
import sys
from pathlib import Path
path = Path(sys.argv[1])
data = json.loads(path.read_text(encoding="utf-8"))
print(data.get("decision", "BLOCKED"))
PY
)"
TARGET="$("$PYTHON_BIN" - "$DECISION_FILE" <<'PY'
import json
import sys
from pathlib import Path
path = Path(sys.argv[1])
data = json.loads(path.read_text(encoding="utf-8"))
print(data.get("target", "unknown"))
PY
)"

case "$DECISION" in
  APPROVED)
    printf '%s\n' "[ok] Deploy authorized by Security Gate (target=$TARGET)"
    exit 0
    ;;
  WARNING)
    if [ "$WARNING_MODE" = "allow" ]; then
      printf '%s\n' "[ok] Deploy allowed with warning by Security Gate (target=$TARGET)"
      exit 0
    fi
    printf '%s\n' "[blocked] Deploy blocked by Security Gate warning policy (target=$TARGET)"
    exit 1
    ;;
  BLOCKED)
    printf '%s\n' "[blocked] Deploy blocked by Security Gate (target=$TARGET)"
    exit 1
    ;;
  ERROR)
    printf '%s\n' "[blocked] Deploy blocked by Security Gate scanner error (target=$TARGET)"
    exit 2
    ;;
  MANUAL_REVIEW)
    printf '%s\n' "[blocked] Deploy blocked by Security Gate manual review (target=$TARGET)"
    exit 3
    ;;
  *)
    printf '%s\n' "[blocked] Deploy blocked by Security Gate unknown decision '$DECISION' (target=$TARGET)"
    exit 1
    ;;
esac
