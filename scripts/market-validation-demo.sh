#!/usr/bin/env bash
#
# Kuro Core Phase 0/1 market-validation demo.
# Uses only synthetic fixtures and the local CLI; it does not create or read secrets.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
KURO_BIN="${KURO_BIN:-$ROOT_DIR/bin/kuro}"
DEMO_DIR="$(mktemp -d "${TMPDIR:-/tmp}/kuro-market-validation.XXXXXX")"

cleanup() {
  rm -rf -- "$DEMO_DIR"
}
trap cleanup EXIT

if [[ ! -x "$KURO_BIN" ]]; then
  cat >&2 <<EOF
Kuro binary not found or not executable: $KURO_BIN
Build it first with: make build
Then rerun this demo, or set KURO_BIN to an existing kuro binary.
EOF
  exit 2
fi

run_kuro() {
  set +e
  COMMAND_OUTPUT="$("$KURO_BIN" "$@" 2>&1)"
  COMMAND_CODE=$?
  set -e
}

printf '%s\n' 'Kuro Core — Phase 0/1 local scan demo' '======================================='
printf 'Binary: %s\n' "$KURO_BIN"
printf '%s\n' 'Synthetic fixtures are created in a temporary directory and removed on exit.'
printf '%s\n\n' 'Prerequisite check: kuro doctor'

run_kuro doctor
printf '%s\n' "$COMMAND_OUTPUT"
printf 'doctor exit code: %s\n\n' "$COMMAND_CODE"
if [[ "$COMMAND_CODE" -ne 0 ]]; then
  cat >&2 <<'EOF'
Kuro doctor did not pass. Local scans require a usable Docker or Podman runtime
for the scanner containers. Start one, then rerun the demo.
EOF
  exit 2
fi

mkdir -p "$DEMO_DIR/clean" "$DEMO_DIR/blocked"
cat >"$DEMO_DIR/clean/main.go" <<'EOF'
package main

func main() {}
EOF

cat >"$DEMO_DIR/blocked/.env" <<'EOF'
# Synthetic fixture only; this is not a real credential.
SLACK_TOKEN=xoxb-123456789012-123456789012-abcdefghijklmnopqrstuvwx
EOF

printf '%s\n' 'Clean fixture: kuro scan PATH --json'
run_kuro scan "$DEMO_DIR/clean" --json
printf '%s\n' "$COMMAND_OUTPUT"
clean_code=$COMMAND_CODE
printf 'clean scan exit code: %s (expected 0 / decision pass)\n\n' "$clean_code"

printf '%s\n' 'Synthetic finding fixture: kuro scan PATH --json'
run_kuro scan "$DEMO_DIR/blocked" --json
printf '%s\n' "$COMMAND_OUTPUT"
blocked_code=$COMMAND_CODE
printf 'finding scan exit code: %s (expected 1 / decision block)\n\n' "$blocked_code"

if [[ "$clean_code" -ne 0 || "$blocked_code" -ne 1 ]]; then
  printf 'Unexpected result: expected clean=0 and finding=1; got clean=%s finding=%s\n' \
    "$clean_code" "$blocked_code" >&2
  exit 1
fi

cat <<'EOF'
Demo completed: local JSON scan behavior matched the expected pass/block exit codes.
See docs/market-validation/DEMO.md for optional attestation inspection/verification
and the limitations that must be stated during interviews.
EOF
