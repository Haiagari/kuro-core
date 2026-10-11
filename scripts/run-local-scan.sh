#!/usr/bin/env bash
set -euo pipefail
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
kuro_bin="${KURO_BIN:-$script_dir/../bin/kuro}"
if [[ $# -eq 0 ]]; then set -- .; fi
exec "$kuro_bin" scan "$@"
