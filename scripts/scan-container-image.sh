#!/usr/bin/env bash
set -euo pipefail

IMAGE_REF="${1:-${KURO_IMAGE_REF:-}}"
REPORTS_DIR="${REPORTS_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/reports}"
OUTPUT="$REPORTS_DIR/container-image-scan.json"

# ── Validation ──────────────────────────────────────────────────────────

# R2-a: Reject empty values
if [ -z "$IMAGE_REF" ]; then
  printf '%s\n' "[blocked] image reference not provided"
  exit 1
fi

# R2-b: Reject values starting with dash (argument injection)
if [[ "$IMAGE_REF" == -* ]]; then
  printf '%s\n' "[blocked] invalid image reference (starts with '-'): $IMAGE_REF"
  exit 1
fi

# R2-c: Reject values without a colon or slash (no valid image format)
if [[ "$IMAGE_REF" != *:* && "$IMAGE_REF" != */* ]]; then
  printf '%s\n' "[blocked] invalid image reference format (no tag or registry): $IMAGE_REF"
  exit 1
fi

# ── Scanning ────────────────────────────────────────────────────────────

mkdir -p "$REPORTS_DIR"

if command -v trivy >/dev/null 2>&1; then
  trivy image --format json --output "$OUTPUT" "$IMAGE_REF"
  printf '%s\n' "$OUTPUT"
  exit 0
fi

if command -v docker >/dev/null 2>&1; then
  digest="$(docker image inspect --format '{{index .RepoDigests 0}}' "$IMAGE_REF" 2>/dev/null || true)"
  python - "$OUTPUT" "$IMAGE_REF" "$digest" <<'PY'
import json
import sys
from pathlib import Path

output = Path(sys.argv[1])
image_ref = sys.argv[2]
digest = sys.argv[3]
output.write_text(
    json.dumps(
        {
            "image": image_ref,
            "digest": digest,
            "results": [],
            "status": "scan_skipped_trivy_unavailable",
        },
        indent=2,
    ),
    encoding="utf-8",
)
PY
  printf '%s\n' "$OUTPUT"
  exit 0
fi

printf '%s\n' "[blocked] neither trivy nor docker is available"
exit 1
