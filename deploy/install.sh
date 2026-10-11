#!/usr/bin/env sh
# Compatibility entrypoint for Core; no Enterprise services are installed.
set -eu
script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
exec sh "$script_dir/../scripts/install.sh" "$@"
