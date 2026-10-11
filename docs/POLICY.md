# Gate policies — current development contract

The local CLI loads the embedded [default policy](../cli/internal/orchestrator/rules/default-policy.json), or the JSON file selected by `KURO_POLICY_PATH`. Missing/unreadable/malformed custom policies, unknown fields, unsupported filters and invalid actions fail the scan with `block` / exit 1. An invalid policy never silently falls back.

```json
{
  "name": "team-policy",
  "version": "1.0.0",
  "rules": [
    {"scanner": "checkov", "severity": "MEDIUM", "file_patterns": ["infra/**/*.tf"], "threshold": 1, "action": "BLOCK", "reason": "Review infrastructure failures"}
  ]
}
```

```bash
KURO_POLICY_PATH=./team-policy.json kuro scan ./project --json
```

| Field | Contract |
|---|---|
| `name`, `version` | Nonempty policy metadata |
| `rules` | Nonempty list |
| `scanner` | Optional: gitleaks, semgrep, trivy, checkov, trufflehog; case-insensitive |
| `severity` | Optional: CRITICAL, HIGH, MEDIUM, LOW, INFO, UNKNOWN; case-insensitive |
| `file_patterns` | Optional relative path globs; any one matching pattern satisfies this filter |
| `threshold` | Number of matching findings; omitted/0 means 1; negatives rejected |
| `action` | BLOCK, REVIEW, MANUAL_REVIEW, PASS; case-insensitive |
| `reason` | Optional explanatory text; currently not emitted as a decision trace |

All filters in one rule are combined with **AND**. Patterns match source paths relative to the scan root (`/src/` prefixes are removed). `*` stays within one path segment; `?` matches one non-slash character; `**` crosses directories, and `**/` also matches zero directories. Absolute paths, parent traversal, backslashes, bracket classes and brace expansion are rejected. `*.tf` matches root Terraform files; `**/*.tf` also matches nested files.

Patterns filter findings, not which scanners are scheduled. Core scheduling currently selects Checkov for Dockerfile/`.tf` and Trivy for the manifests listed in [architecture](ARCHITECTURE.md). A YAML-only directory does not automatically select Checkov.

## Precedence and mandatory floors

All matching rules are evaluated: **block > review > pass**. PASS is not an override. Critical findings and all Gitleaks/TruffleHog detections always block; any HIGH always requires review. A custom policy can add restrictions but cannot lower these floors. Thresholds above 1 cannot relax a mandatory floor.

Default policy additionally blocks all Trivy and Checkov findings. Grype and Syft are not supported Core scanners. Unverified TruffleHog detections still block; offline scanning never contacts providers to verify credentials.

## Consumer behavior

`kuro scan`: pass=0, review=2, block/error=1. `kuro proxy` blocks both review and block. CI must explicitly choose whether to stop on review; the documented safe example stops on every nonzero result. File caches never skip policy evaluation or scanner execution.
