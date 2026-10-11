# Baseline security debt audit

Date: 2026-10-11. Audited base: `29971b7e3388f6fc7662a6cb8ddf0d3814bd9d51`. Scanner metadata came from [CI run 38184547238](https://github.com/Haiagari/kuro-core/actions/runs/38184547238), using the trusted gate's blob snapshot and CI scanner mode. No matched credential values or raw scanner diagnostics are reproduced here.

## Mandatory gate verified

`Protect main` actively requires `Kuro PR Gate` from GitHub Actions (integration 15368), alongside lint and Gitleaks. Strict up-to-date checks remain enabled; bypass actors are empty. After activation, [probe PR #20](https://github.com/Haiagari/kuro-core/pull/20) was reopened and made ready for review to remove the draft restriction. Its lint and secret checks passed; [trusted gate run 38184488098](https://github.com/Haiagari/kuro-core/actions/runs/38184488098) rejected one synthetic Python finding. GitHub reported a conflict-free PR with merge state `blocked`. The probe was closed without merge; no unsafe code entered `main`, and no merge attempt was performed.

## Classification

These are finding records, not 27 distinct exploitable vulnerabilities. Multiple engines can report the same fixture. Classification is source review, not an automatic waiver or a credential-validity/exploit test.

| Category | Records | Disposition |
|---|---:|---|
| Deliberate test/demo fixtures | 10 | Retain detection coverage; no folder-wide exemption |
| False positives in prose, rule templates or protocol identifiers | 13 | Record precise cause; improve precision separately with regression cases |
| Confirmed vulnerable dependency version | 2 | Update x/text from v0.37.0 to v0.41.0 |
| Operational/transport hardening | 2 | Keep visible; targeted design and validation needed |
| Total baseline | 27 | Adoption does not assert a clean repository |

## Complete inventory

Locations refer to the audited base; line numbers can move in later revisions. G = Gitleaks, S = Semgrep, C = Checkov, T = Trivy.

| # | Engine / rule | Location | Classification and evidence |
|---:|---|---|---|
| 1 | C / CKV_DOCKER_2 | services/git-proxy/Dockerfile:1 | Hardening: image has no HEALTHCHECK; a meaningful liveness probe requires a real service endpoint/probe, not an always-success command |
| 2 | G / generic-api-key | cli/cmd/canary_test.go:19 | Fixture: assertion against a standard mock prefix in generated canary tests |
| 3 | G / generic-api-key | cli/cmd/fix_test.go:56 | Fixture: JavaScript remediation input |
| 4 | G / generic-api-key | cli/internal/orchestrator/testdata/gitleaks_findings.json:8 | Fixture: scanner sample record |
| 5 | G / generic-api-key | cli/internal/orchestrator/testdata/gitleaks_findings.json:16 | Fixture: scanner sample record |
| 6 | G / generic-api-key | docs/market-validation/INTERVIEW-GUIDE.md:11 | False positive: interview prose lists security/tooling costs; no credential assignment at the reported line |
| 7 | G / generic-api-key | tests/e2e-proxy.sh:178 | Fixture: explicit synthetic input in an Enterprise/API test |
| 8 | G / slack-bot-token | scripts/market-validation-demo.sh:58 | Fixture: deliberate synthetic demo token |
| 9 | G / slack-bot-token | tests/e2e-core-local.sh:146 | Fixture: deliberate synthetic Core E2E token |
| 10 | G / slack-bot-token | tests/e2e-scanner-contracts.sh:35 | Fixture: deliberate synthetic historical token |
| 11 | S / hardcoded-credential | cli/cmd/fix_test.go:36 | Fixture: Python remediation input; overlaps test coverage rather than production use |
| 12 | S / hardcoded-credential | tests/e2e-proxy.sh:178 | Fixture: same synthetic test input as record 7 |
| 13 | S / insecure-http-url | cli/cmd/attest_test.go:104 | False positive: DSSE reference-vector type identifier, not an HTTP request |
| 14 | S / insecure-http-url | deploy/security/semgrep/kuro-rules.yml:184 | False positive: detection pattern template, not executed application code |
| 15 | S / insecure-http-url | deploy/security/semgrep/kuro-rules.yml:185 | False positive: detection pattern template |
| 16 | S / insecure-http-url | deploy/security/semgrep/kuro-rules.yml:186 | False positive: detection pattern template |
| 17 | S / insecure-http-url | deploy/security/semgrep/kuro-rules.yml:187 | False positive: detection pattern template |
| 18 | S / insecure-http-url | docs/assets/logo/kuro-mark-mono.svg:1 | False positive: canonical SVG namespace identifier, not a network request |
| 19 | S / insecure-http-url | docs/assets/logo/kuro-mark.svg:1 | False positive: SVG namespace identifier |
| 20 | S / insecure-http-url | docs/assets/logo/kuro-wordmark-mono.svg:1 | False positive: SVG namespace identifier |
| 21 | S / insecure-http-url | docs/assets/logo/kuro-wordmark.svg:1 | False positive: SVG namespace identifier |
| 22 | S / insecure-http-url | docs/diagrams/architecture.svg:1 | False positive: SVG namespace identifier |
| 23 | S / insecure-http-url | docs/diagrams/git-proxy-flow.svg:1 | False positive: SVG namespace identifier |
| 24 | S / insecure-http-url | docs/diagrams/product-workflow.svg:1 | False positive: SVG namespace identifier |
| 25 | S / insecure-http-url | services/git-proxy/server/config_client.go:140 | Conditional transport risk: API-mode default uses plaintext to an internal service and may carry an API header. Core defaults to local mode. TLS policy needs an explicit compatibility decision |
| 26 | T / CVE-2026-56852 | cli/go.mod | Confirmed component advisory: x/text v0.37.0, invalid UTF-8 can trigger an infinite loop in unicode/norm; fixed in v0.39.0 |
| 27 | T / CVE-2026-56851 | cli/go.mod | Confirmed component advisory: x/text v0.37.0, crafted input can panic the Nickname profile in secure/precis; fixed in v0.41.0 |

## Targeted remediation

The proposed dependency change uses v0.41.0, which covers both fixes and retains the existing Go 1.26.4 minimum (upstream x/text v0.41.0 declares Go 1.25.0). It does not broadly update unrelated packages or introduce suppression rules. Module checksums are updated; workspace synchronization must remain clean.

Primary sources: [GO-2026-5970](https://pkg.go.dev/vuln/GO-2026-5970), [GO-2026-6629](https://pkg.go.dev/vuln/GO-2026-6629), [x/text v0.41.0 module](https://go.googlesource.com/text/+/refs/tags/v0.41.0/go.mod). Trivy severity is HIGH/UNKNOWN respectively; UNKNOWN is not an assertion of safety. Package-version detection establishes affected component presence, not demonstrated reachability or exploitation in Kuro. Vendored standard-library code/toolchain coverage is a separate consideration.

The fixture/false-positive records remain visible under the existing adoption policy. They have not been removed from scans, marked harmless automatically, or granted inline approvals. Changing their affected files can still block the gate. A future exception design should identify an exact record, justification, owner and expiry; rule-precision changes must retain vulnerable/benign regression coverage.

The two hardening items remain open. For the image, first implement/test a genuine liveness check and distinguish it from scanner readiness/security approval. For API mode, decide TLS validation and explicit local/internal compatibility before changing defaults. No endpoint or broad insecure-transport override is added merely to reduce scanner counts.

## Validation status

Inventory/enforcement evidence above is completed. Dependency remediation passed local build/tests/vet/race checks, module tidy/workspace synchronization and native CI on implementation commit `e40b1915dc883e2c5c146465d9566aa5ccd759d0`: [CI run 38185085418](https://github.com/Haiagari/kuro-core/actions/runs/38185085418), [trusted gate run 38185083898](https://github.com/Haiagari/kuro-core/actions/runs/38185083898), and [secret scanning run 38185085399](https://github.com/Haiagari/kuro-core/actions/runs/38185085399). The same-base gate comparison confirmed 27 base records, 25 candidate records and zero introduced records. The two x/text advisories are absent from the patched candidate; the 25 fixture/false-positive/hardening records remain visible. This remediation is proposed in PR #21 until reviewed and merged; no release is published by this audit.
