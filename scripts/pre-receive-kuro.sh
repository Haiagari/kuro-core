#!/usr/bin/env bash
#
# Kuro — Server-Side Pre-Receive Hook
# Intercepts git pushes on the Git Server and scans
# them via Kuro API before any refs are updated.
#
# Installation:
#   1. Copy this script to the target repository's 'hooks/pre-receive' directory on the Git server.
#   2. Make it executable: chmod +x hooks/pre-receive
#   3. Configure KURO_API_URL and KURO_API_KEY environment variables on the Git server.
#
set -euo pipefail

KURO_API_URL="${KURO_API_URL:-http://localhost:8080}"
KURO_API_KEY="${KURO_API_KEY:-}"
PROXY_FAIL_MODE="${PROXY_FAIL_MODE:-open}" # 'open' to allow push on Kuro failure, 'closed' to block
TIMEOUT_SECONDS=30

if [[ -z "$KURO_API_KEY" ]]; then
    printf '%s\n' "[warn] KURO_API_KEY is not set on the Git server. Skipping pre-receive check."
    exit 0
fi

# Temp directory to hold extracted files for scanning
TEMP_BASE="/tmp/kuro-pre-receive"
mkdir -p "$TEMP_BASE"

# Read refs to update from stdin (format: <old-sha> <new-sha> <ref-name>)
while read -r old_sha new_sha ref_name; do
    # Ignore branch deletion pushes (new-sha is all zeros)
    if [[ "$new_sha" =~ ^0+$ ]]; then
        continue
    fi

    # Determine repository name from directory path on the server
    # Git server executes hooks within the bare repository directory (e.g. /data/git/repositories/owner/repo.git)
    REPO_DIR="$(pwd)"
    REPO_NAME=$(basename "$REPO_DIR")
    REPO_OWNER=$(basename "$(dirname "$REPO_DIR")")
    FULL_REPO_NAME="${REPO_OWNER}/${REPO_NAME%.git}"

    # Extract short branch name
    BRANCH_NAME="${ref_name#refs/heads/}"

    # Create temporary path for file extraction
    PUSH_ID="push-${new_sha}-$(date +%s%N)"
    SCAN_DIR="${TEMP_BASE}/${PUSH_ID}"
    mkdir -p "$SCAN_DIR"

    # Clean up files after hook execution
    trap 'rm -rf "$SCAN_DIR"' EXIT

    # Extract files at the target commit SHA
    if ! git archive --format=tar "$new_sha" | tar -x -C "$SCAN_DIR" 2>/dev/null; then
        # Fallback if git archive fails (e.g. invalid sha)
        printf '%s\n' "remote: [kuro] Error: Failed to extract files for commit ${new_sha}."
        if [[ "$PROXY_FAIL_MODE" == "closed" ]]; then
            exit 1
        fi
        continue
    fi

    # Build JSON payload matching the Kuro API Proxy Scan contract
    PAYLOAD=$(cat <<EOF
{
  "path": "$SCAN_DIR",
  "repo": "$FULL_REPO_NAME",
  "commit": "$new_sha",
  "branch": "$BRANCH_NAME"
}
EOF
)

    printf '%s\n' "remote: [kuro] 🔒 Scanning push ${new_sha:0:12} ($BRANCH_NAME) for secrets..."

    # Call Kuro Proxy Scan endpoint synchronously
    HTTP_RESPONSE=$(curl -s -w "%{http_code}" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "X-API-Key: $KURO_API_KEY" \
        --max-time "$TIMEOUT_SECONDS" \
        -d "$PAYLOAD" \
        "${KURO_API_URL}/api/v1/scans/proxy" || echo "500")

    HTTP_STATUS="${HTTP_RESPONSE: -3}"
    RESPONSE_BODY="${HTTP_RESPONSE:0:${#HTTP_RESPONSE}-3}"

    if [[ "$HTTP_STATUS" -eq 403 ]]; then
        # Scan rejected by Kuro policies (BLOCKED)
        printf '\n%s\n' "remote: ❌ ==============================================="
        printf '%s\n' "remote: ❌ KURO PIPELINE - PUSH BLOCKED"
        printf '%s\n' "remote: ❌ Secrets or critical vulnerabilities detected."
        printf '%s\n' "remote: ❌ Commit: ${new_sha:0:12}"
        printf 'remote: ❌ Repository: %s\n' "$FULL_REPO_NAME"
        
        # Parse and print findings details if returned
        if command -v jq >/dev/null 2>&1; then
            findings=$(echo "$RESPONSE_BODY" | jq -r '.findings[]? | "remote: ❌   - \(.severity) | \(.rule) | \(.file):\(.line)"' 2>/dev/null || true)
            if [[ -n "$findings" ]]; then
                printf '%s\n' "$findings"
            fi
        else
            printf '%s\n' "remote: ❌ Please inspect the scan dashboard for details."
        fi
        
        printf '%s\n' "remote: ❌ ==============================================="
        printf '%s\n' "remote: Fix the issues listed above and push again."
        exit 1
    elif [[ "$HTTP_STATUS" -eq 200 ]]; then
        # Scan approved (APPROVED)
        printf '%s\n' "remote: [kuro] ✅ Approved. No security blockers found."
    else
        # Error connecting or non-200/403 status code
        printf '%s\n' "remote: [kuro] ⚠️  Scan server error (HTTP $HTTP_STATUS)."
        if [[ "$PROXY_FAIL_MODE" == "closed" ]]; then
            printf '%s\n' "remote: ❌ Rejecting push due to PROXY_FAIL_MODE=closed"
            exit 1
        fi
        printf '%s\n' "remote: [kuro] Allowing push (fail-open mode)."
    fi
done

exit 0
