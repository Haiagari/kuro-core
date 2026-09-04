#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="${ROOT_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"

printf '%s\n' "[*] Running RC gate: enterprise certification + release authorization"

make -C "$ROOT_DIR" release-gate
make -C "$ROOT_DIR" enterprise-gate

printf '%s\n' "[ok] RC gate passed"
