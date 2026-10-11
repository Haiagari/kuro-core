#!/usr/bin/env bash
# Native scanner smoke tests beyond the clean/secret Core demo.
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
kuro_bin="${KURO_BIN:-$root/bin/kuro}"
fixture_dir=$(mktemp -d)
trap 'rm -rf "$fixture_dir"' EXIT
expect_block() {
 local target="$1" scanner="$2"
 local code=0
 "$kuro_bin" scan "$target" --json --no-cache > "$fixture_dir/result.json" || code=$?
 python3 - "$fixture_dir/result.json" "$code" "$scanner" <<'PY'
import json, sys
with open(sys.argv[1]) as f: result = json.load(f)
assert sys.argv[2] == '1', (sys.argv[2], result)
assert result.get('status') == 'completed', result
assert result.get('decision') == 'block', result
assert any(f.get('scanner') == sys.argv[3] for f in result.get('findings', [])), result
PY
}
# An intentionally vulnerable dependency; registry/network lookups remain disabled.
mkdir "$fixture_dir/sca"
printf 'PyYAML==5.3\n' > "$fixture_dir/sca/requirements.txt"
expect_block "$fixture_dir/sca" trivy
mkdir "$fixture_dir/iac"
printf 'FROM python:3.12\nUSER root\n' > "$fixture_dir/iac/Dockerfile"
expect_block "$fixture_dir/iac" checkov
mkdir "$fixture_dir/sast"
printf 'value = eval(input())\n' > "$fixture_dir/sast/example.py"
expect_block "$fixture_dir/sast" semgrep
mkdir "$fixture_dir/history"
git -C "$fixture_dir/history" init -q
git -C "$fixture_dir/history" config user.name 'Kuro Test'
git -C "$fixture_dir/history" config user.email 'test@example.invalid'
printf 'SLACK_TOKEN=xoxb-123456789012-123456789012-abcdefghijklmnopqrstuvwx\n' > "$fixture_dir/history/secret.env"
git -C "$fixture_dir/history" add secret.env
git -C "$fixture_dir/history" commit -qm 'synthetic secret'
git -C "$fixture_dir/history" rm -q secret.env
git -C "$fixture_dir/history" commit -qm 'remove working-tree secret'
code=0
"$kuro_bin" scan "$fixture_dir/history" --history --json --no-cache > "$fixture_dir/history.json" || code=$?
python3 - "$fixture_dir/history.json" "$code" <<'PY'
import json, sys
with open(sys.argv[1]) as f: result = json.load(f)
assert sys.argv[2] == '1', (sys.argv[2], result)
assert result.get('status') == 'completed' and result.get('decision') == 'block', result
assert any(f.get('scanner') == 'gitleaks' for f in result.get('findings', [])), result
PY
printf 'Native scanner contract smoke tests passed.\n'
