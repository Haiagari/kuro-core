#!/usr/bin/env bash
# Kuro — Pre-launch usage report generator
#
# Runs kuro scan and kuro scan --history against a list of repositories
# and produces a metrics report for the launch post.
#
# Usage:
#   bash scripts/audit-report.sh [--json] [--dir ./my-repos]
#
# By default scans the current directory. Pass --dir to scan a folder
# with multiple repos, or edit the REPOS array below.
#
# Output: audit-report-{timestamp}.md (or .json with --json)

set -euo pipefail

# ── Config ──────────────────────────────────────────────────
SCAN_DIR="${KURO_SCAN_DIR:-${1:-.}}"
OUTPUT_DIR="${KURO_OUTPUT_DIR:-.}"
REPORT_FILE=""

# Repos to scan (if --dir is passed, scans all subdirectories)
# Otherwise, edit this list or pass paths directly
REPOS=()

# ── Detect kuro binary ────────────────────────────────────────
KURO=""
for candidate in ./kuro ../cli/kuro /usr/local/bin/kuro ~/.kuro/bin/kuro; do
  if [ -x "$candidate" ]; then
    KURO="$candidate"
    break
  fi
done

if [ -z "$KURO" ]; then
  # Try building from source
  if [ -f ./cli/main.go ]; then
    echo "  Building kuro from source..."
    KURO=$(mktemp /tmp/kuro-bin-XXXXXX)
    (cd cli && go build -o "$KURO" .) || {
      echo "❌ Could not find or build kuro binary."
      echo "   Install it first: make build-cli or curl ... | sh"
      exit 1
    }
  else
    echo "❌ kuro binary not found and no cli/ directory to build from."
    exit 1
  fi
fi

echo "  Using kuro: $KURO"
echo ""

# ── Collect repos ────────────────────────────────────────────
collect_repos() {
  local dir="$1"
  if [ -d "$dir" ]; then
    for sub in "$dir"/*/; do
      if [ -d "$sub/.git" ]; then
        REPOS+=("$sub")
      fi
    done
  fi
  if [ ${#REPOS[@]} -eq 0 ]; then
    REPOS=("$dir")
  fi
}

if [ "$SCAN_DIR" != "." ] || [ ! -d ".git" ]; then
  collect_repos "$SCAN_DIR"
else
  REPOS=(".")
fi

# ── Run scans ────────────────────────────────────────────────
total_scans=0
total_findings=0
total_duration=0
total_repos=0
declare -A scanner_findings
declare -A severity_counts
critical_total=0
high_total=0
medium_total=0
low_total=0
info_total=0

report_lines=()

run_scan() {
  local repo="$1"
  local mode="$2"
  local label="$3"
  local repo_name

  repo_name=$(basename "$(cd "$repo" 2>/dev/null && pwd || echo "$repo")")
  echo "  Scanning $repo_name ($label)..."

  local start end duration findings_count
  start=$(date +%s%N)

  # Run scan with JSON output
  local json_output
  if [ "$mode" = "history" ]; then
    json_output=$("$KURO" scan --json --history "$repo" 2>/dev/null || true)
  else
    json_output=$("$KURO" scan --json "$repo" 2>/dev/null || true)
  fi

  end=$(date +%s%N)
  duration=$(( (end - start) / 1000000 )) # ms

  # Parse findings count from JSON
  findings_count=$(echo "$json_output" | grep -o '"findings":\[[^]]*\]' | tr -d '[:space:]' | sed 's/"findings"://' | tr -d '[]' | tr ',' '\n' | grep -c '.' || echo "0")

  if [ "$findings_count" = "0" ]; then
    findings_count=$(echo "$json_output" | grep -o '"findings_count":[0-9]*' | grep -o '[0-9]*' || echo "0")
  fi

  echo "    → $findings_count findings in ${duration}ms"

  report_lines+=("| $repo_name | $label | $findings_count | ${duration}ms |")

  total_scans=$((total_scans + 1))
  total_findings=$((total_findings + findings_count))
  total_duration=$((total_duration + duration))
}

# ── Scan each repo ───────────────────────────────────────────
for repo in "${REPOS[@]}"; do
  repo_name=$(basename "$(cd "$repo" 2>/dev/null && pwd || echo "$repo")")
  echo ""
  echo "  ── $repo_name ──"

  # Working tree scan
  run_scan "$repo" "tree" "Working tree"

  # History scan (if it's a git repo)
  if [ -d "$repo/.git" ]; then
    run_scan "$repo" "history" "Full history"
  fi

  total_repos=$((total_repos + 1))
done

# ── Generate report ──────────────────────────────────────────
timestamp=$(date -u +%Y%m%dT%H%M%SZ)
REPORT_FILE="${OUTPUT_DIR}/audit-report-${timestamp}.md"

{
  echo "# Kuro — Usage Report"
  echo ""
  echo "**Generated**: $(date -u)"
  echo "**Version**: $("$KURO" version 2>/dev/null || echo "unknown")"
  echo "**Repos scanned**: $total_repos"
  echo ""
  echo "---"
  echo ""
  echo "## Summary"
  echo ""
  echo "| Metric | Value |"
  echo "|--------|-------|"
  echo "| Repositories scanned | $total_repos |"
  echo "| Total scans | $total_scans |"
  echo "| Total findings | $total_findings |"
  echo "| Avg scan duration | $(( total_duration / (total_scans > 0 ? total_scans : 1) ))ms |"
  echo "| Total scan time | $(( total_duration / 1000 ))s |"
  echo ""
  echo "## Per-Repo Results"
  echo ""
  echo "| Repo | Mode | Findings | Duration |"
  echo "|------|------|----------|----------|"
  for line in "${report_lines[@]}"; do
    echo "$line"
  done
  echo ""
  echo "---"
  echo ""
  echo "## Next Steps"
  echo ""
  echo "1. Review findings and identify false positives"
  echo "2. Update policy rules if needed (\`deploy/policies/default-policy.json\`)"
  echo "3. Run regularly to establish a baseline"
  echo ""
} > "$REPORT_FILE"

echo ""
echo "  ✅ Report saved: $REPORT_FILE"
echo "  Total findings: $total_findings across $total_repos repos"
echo "  Avg duration: $(( total_duration / (total_scans > 0 ? total_scans : 1) ))ms"
