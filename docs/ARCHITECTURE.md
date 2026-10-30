# Architecture — current development branch

Kuro Core is a local CLI, TUI and opt-in Smart-HTTP proxy. It requires Docker/Podman, not Postgres, NATS or MinIO. Enterprise remote mode is a separate integration.

## Components

| Component | Source | Responsibility |
|---|---|---|
| CLI | `cli/main.go`, `cli/cmd/` | Commands, display and process exit codes |
| Orchestrator | `cli/internal/orchestrator/` | Scan phases, error propagation and policy |
| Local adapter | `adapter_local.go`, `scanners.go`, `container.go` | Scoping and isolated scanner execution |
| Rules | `cli/internal/orchestrator/rules/` | Embedded Semgrep rules and default policy |
| Git proxy | `services/git-proxy/server/` | Materialize push content and evaluate local/remote decision |
| TUI | `cli/internal/tui/` | Bubbletea progress display |

## Scan lifecycle

1. **Fetch:** require a directory, choose Docker/Podman and start asynchronous image pulls. Pulls use host networking; scan containers do not.
2. **Scope:** always select Gitleaks/Semgrep; add Trivy for `go.mod`, `package-lock.json`, `yarn.lock`, `requirements.txt`, `Gemfile.lock`, `Cargo.lock`; add Checkov for Dockerfile/`.tf`. Traversal errors block. YAML-only and unsupported package-manager projects do not automatically receive those checks.
3. **Scan:** bound concurrency (default 2), run containers under the controls in [scanner architecture](SCANNER-ARCHITECTURE.md), parse native reports and aggregate errors. A configured Trivy scan requires fresh prepared DB data. Unexpected process failures, timeouts and recognized analysis/parse errors block.
4. **Analyze:** aggregate findings and severity counts. No finding deduplication is implemented. Policy evaluation currently occurs here.
5. **Decide:** expose `pass`, `review` or `block` under the [policy contract](POLICY.md). Invalid policies block before scanning. Statistics may be recorded after pass; they never authorize future scans.
6. **Report:** TUI, text or JSON. pass=0, review=2, block/error=1. Flags after a scan path are supported.

`--history` selects Gitleaks Git history plus TruffleHog Git scanning **instead of** working-tree SAST/SCA/IaC. It is not a superset of the default scan. Source is read live, and the CLI is not an immutable source snapshot or centralized enforcement system.

## Local and remote modes

| Trigger | Mode | Prerequisites |
|---|---|---|
| Local path | Core local | Docker/Podman; prepared Trivy DB when selected |
| URL or `--remote` | Enterprise companion | API configuration and key |

## Git proxy

```bash
make proxy
# Or after a build:
KURO_BIN="$PWD/bin/kuro" ./bin/kuro proxy --addr 127.0.0.1:8000
```

The default listener is `:8000` (all interfaces); choose an explicit loopback address for workstation-only use. Git pushes routed to its Smart-HTTP remote are buffered/materialized in temporary directories, scanned, and forwarded only when approved. Review is blocked as well as block. Scan/process/JSON errors fail closed under the default configuration. The local client does not cache approvals. Enterprise API mode retains its separately configured client behavior.

```bash
git remote add proxy http://127.0.0.1:8000/<owner>/<repo>.git
git push proxy main
```

A direct push to another remote bypasses the proxy. This is an opt-in developer gate, not proof of organization-wide enforcement. Temporary disk capacity, upstream authentication and the operating environment must be validated for a deployment. Successful upstream negotiation/metadata traffic is distinct from forwarding a rejected receive-pack payload.

Environment: `LISTEN_ADDR`, `UPSTREAM_URL`, `SCAN_MODE`, `KURO_BIN`, and Enterprise `KURO_URL`/`KURO_API_KEY`. Advanced proxy behavior, including `PROXY_FAIL_MODE`, must be reviewed before deployment; explicitly enabling fail-open relaxes the default gate.

## Additional commands

`kuro fix` is a heuristic interactive text workflow, not a Bubbletea remediation UI and not guaranteed to process every scanner finding. It can also consume a `kuro scan --json` report; report paths must resolve to regular files inside the selected target, and malformed reports fail closed. Automatic replacements are limited to recognized quoted secrets in Go, Python, and JS/TS string literals. Review diffs and separately revoke real leaked credentials. Suppression records do not automatically establish a policy exception. `kuro canary` creates synthetic HMAC-marked tokens; monitoring and external alerts require other infrastructure. `kuro attest` inspects or verifies envelopes produced elsewhere; see its [trust and format limits](ATTESTATION.md).

Related: [Quickstart](../QUICKSTART.md), [CLI/JSON](API.md), [tests](../tests/README.md), [synthetic demo](market-validation/DEMO.md), [interview guide](market-validation/INTERVIEW-GUIDE.md).
