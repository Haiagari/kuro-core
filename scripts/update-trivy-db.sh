#!/usr/bin/env sh
# Explicit online preparation. No project source is mounted into this container.
set -eu
image=docker.io/aquasec/trivy:0.57.0
cache_dir=${KURO_TRIVY_CACHE_DIR:-"$HOME/.kuro/trivy"}
case "$cache_dir" in /*) ;; *) cache_dir="$PWD/$cache_dir" ;; esac
if command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1; then
 runtime=docker
elif command -v podman >/dev/null 2>&1 && podman info >/dev/null 2>&1; then
 runtime=podman
else
 echo 'Docker or Podman must be installed and usable.' >&2; exit 1
fi
mkdir -p "$cache_dir"
# Serialize writers; a failed update keeps the previously installed DB.
lock="$cache_dir/.update-lock"
mkdir "$lock" 2>/dev/null || { echo 'Another update is active; inspect .update-lock if a previous update crashed.' >&2; exit 1; }
staging=$(mktemp -d "$cache_dir/.download-XXXXXX")
backup="$staging/previous-db"
cleanup() {
 if [ -d "$backup" ] && [ ! -d "$cache_dir/db" ]; then mv "$backup" "$cache_dir/db"; fi
 rm -rf "$staging"
 rmdir "$lock"
}
trap cleanup EXIT
trap 'exit 1' HUP INT TERM
"$runtime" pull "$image"
set -- run --rm --cap-drop=ALL --security-opt=no-new-privileges --memory=512m --cpus=1.0
if [ "$runtime" = podman ]; then set -- "$@" --userns=keep-id; fi
"$runtime" "$@" --user "$(id -u):$(id -g)" -v "$staging:/trivy-cache:Z" "$image" \
 image --cache-dir=/trivy-cache --download-db-only
[ -s "$staging/db/trivy.db" ] && [ -s "$staging/db/metadata.json" ] || { echo 'Incomplete DB download.' >&2; exit 1; }
if [ -d "$cache_dir/db" ]; then mv "$cache_dir/db" "$backup"; fi
mv "$staging/db" "$cache_dir/db"
printf 'Trivy DB prepared at %s/db. Scans reject expired metadata.\n' "$cache_dir"
