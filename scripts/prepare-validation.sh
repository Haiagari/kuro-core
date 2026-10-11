#!/usr/bin/env bash
set -euo pipefail
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
exec "${KURO_BIN:-$script_dir/../bin/kuro}" doctor "$@"
