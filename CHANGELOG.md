# Changelog

All notable changes to Kuro are documented in this file.
Format: [Keep a Changelog](https://keepachangelog.com/en/1.0.0/) — [Semantic Versioning](https://semver.org/)

---

## [1.6.0] - 2026-08-31

### 🔒 Security

- **Zero-Knowledge Finding Vault**: Encrypts sensitive code snippets and detected secrets at rest with AES-256-GCM authenticated envelope encryption (`services/worker/crypto/blindindex.go`).
- **Salted HMAC-SHA256 Blind Indexing**: Enables $O(\log N)$ exact-match indexed queries and cross-scan deduplication over encrypted data without plaintext exposure.
- **gVisor (`runsc`) Container Sandboxing**: Isolates scanner workloads inside Google's gVisor user-space kernel with `CapDrop: ["ALL"]` and `no-new-privileges:true`.
- **eBPF Kernel Syscall Containment**: Intercepts unauthorized kernel syscalls (`ptrace`, `bpf`, `socket`, `mount`), executes instant `SIGKILL` termination, and generates `CONTAINER_BREACH_SUSPECTED` audit events.
- **Git Chaos Monkey & Fail-Closed Suite**: Low-level Go fault injection resilience engine testing network partitions, flapping backends, 5xx panics, and corrupted payloads (`services/git-proxy/chaos_test.go`, `make test-chaos`).
- **Cyber Deception & Secret Honeypots**: `kuro canary` generates cryptographically authentic, HMAC-signed decoy credentials (AWS, GitHub, Slack, JWT) with zero real blast radius.

### ✨ Added

- **in-toto SLSA Cryptographic Attestation**: Digitally signs approved scan verdicts with Ed25519 keys and attaches immutable provenance envelopes to `refs/notes/kuro-attestation` (`kuro attest verify`).
- **Monorepo Smart Diff Delta Scanner**: Sub-second selective tree-walking routing changed files to relevant scanners, with $<50\text{ms}$ instant auto-approvals on non-code/documentation changes.
- **WebAssembly & OPA Rego Dynamic Policy Engine**: Sandboxed microsecond rule evaluation with zero-downtime hot-reloading (`services/worker/policy/wasm_engine.go`).
- **Interactive Terminal Threat Remediation TUI (`kuro fix`)**: AST/Regex automated secret extractor transforming hardcoded credentials into environment variable accessors with import injections.
- **NATS JetStream Distributed Concurrency Locks**: Atomic locking and deduplication preventing redundant container spins on identical commit SHAs across multi-worker clusters.
- **Real-Time Git Pre-Push Telemetry Stream**: Sideband 2 Smart HTTP stream rendering live scanner progress and colorized ANSI banners during `git push`.
- **AI Vector Deduplication Visualizer**: Interactive Next.js 16 Threat Studio component comparing raw scanner alerts vs. AI-consolidated root incidents.
- **Cryptographic License Engine & Feature Gating**: Offline Ed25519-signed license validation (`kuro license status`, `kuro license apply`).
- **1-Line Self-Hosted Platform Installer**: Automated deployment script (`scripts/install-platform.sh`) bootstrapping complete containerized stack in under 2 minutes.

---

## [1.5.0] - 2026-08-03

### 🔒 Security

- **RLS tenant isolation enforced**: Services now connect via `kuro_app` non-superuser role — RLS policies actively filter by tenant (was: inert under `kuro` superuser with BYPASSRLS)
- **`.kuro.yml` suppression disabled**: In-repo scanner suppression files are no longer honored; suppression is operator-controlled via env vars only (prevents malicious repos from suppressing scans)
- **Token exfiltration & SSRF hardened**: Clone URL validation via `validateCloneURL` restricts to allowed hosts; `IsTokenAllowedHost` blocks token injection to arbitrary endpoints
- **Multi-ref bypass fixed**: `parsePushRefs` correctly handles edge cases in push ref parsing that could bypass scanning
- **Fail-closed gates**: `PROXY_FAIL_MODE` defaults to `closed` — pushes blocked on scan failure (was: fail-open)
- **Webhook SSRF hardened**: `resolveOrgID` validates organization resolution; webhook endpoint rejects non-allowed hosts
- **CSV injection prevented**: Export endpoints sanitize cell values to prevent formula injection in spreadsheet clients

### 🐛 Fixed

- **SQL alias bug**: Query alias mismatch in scan listing caused incorrect column mapping
- **SSE memory leak**: Event stream handlers now clean up resources on disconnect
- **Policy severity mapping**: Severity normalization was dropping findings at MEDIUM boundary
- **Backup deadlock**: Concurrent backup deduplication no longer deadlocks on shared hash cache
- **CLI exit codes**: `kuro scan` now returns non-zero on BLOCK (was: always 0)
- **Dashboard filter persistence**: Scan type and severity filters survive page navigation
- **Notifier DSN migration**: Notifier and backup-dedup services now use `kuro_app` role instead of `kuro` superuser for DB connections
- **CSV export encoding**: Unicode and special characters no longer corrupt exported findings

### 🔄 Changed

- **New env vars (operator-controlled)**:
  - `DB_USER_APP` / `DB_PASSWORD_APP` — non-superuser DB role for all services (default: `kuro_app`)
  - `KURO_ALLOWED_GIT_HOSTS` — comma-separated hostnames for clone URL validation + token injection (default: `github.com,gitlab.com,bitbucket.org`)
  - `KURO_DISABLED_SCANNERS` — comma-separated scanner names to suppress (default: none)
  - `KURO_IGNORED_PATHS` — comma-separated glob patterns for path suppression (default: none)
  - `PROXY_FAIL_MODE` default changed from `open` to `closed`
- **Database role migration**: All services connect as `kuro_app` (non-superuser), not the `kuro` superuser. Migrations 018 (`kuro_app` role) and 019 (null-safe RLS policies) applied.
- **NATS subjects**: Proxy full scans and DLQ retries publish to `scans.new.bulk` instead of `scans.new`

### ✨ Added

- **`validateCloneURL`**: Enforces allowed hostnames on clone URLs before VCS operations
- **`IsTokenAllowedHost`**: Validates token injection targets against allowed hosts
- **`parsePushRefs`**: Correct ref parsing for push events, handles edge cases in multi-ref pushes
- **`resolveOrgID`**: Validates organization ID resolution from webhook payloads
- **Per-endpoint API status tracking**: Dashboard shows real-time API health per endpoint

---

## [1.4.1] - 2026-07-27

### 🐛 Fixed

- **NATS auto-reconnect**: API and Notifier now reconnect automatically if NATS restarts (was: permanent failure until manual restart)
- **Crash fixes**: `git-proxy` no longer panics on upstream errors (missing `return` after `http.Error`)
- **Goroutine safety**: `defer recover()` added to 19 unprotected goroutines
- **DB pool tuning (worker)**: `MaxConns` increased from 4→8 with proper lifetime config to avoid bottlenecks under concurrent scans
- **S3 client reuse**: No longer creates fresh AWS SDK config + HTTP transport per scan
- **NATS subscription cleanup**: Subscriptions are unsubscribed on graceful shutdown (no orphaned consumers)
- **`rows.Err()` checks**: Added after 20 `rows.Next()` loops that silently ignored DB iteration errors
- **`json.Encode` error logging**: Helper `encodeJSON()` added — 52 call sites now log write failures
- **Shared HTTP client**: Notifier reuses a single `http.Client` with timeout instead of 6 disposable ones
- **`context.Background()` fixed**: S3 upload now receives the scan context (respects cancellation)
- **Signal handling**: `backup-dedup` now handles SIGTERM/SIGINT for graceful shutdown
- **CI/CD security hardening**: Script injection fixed in Forgejo workflows. All 8 workflows have `permissions:` blocks (least privilege). `persist-credentials: false` on all checkouts. Docker images pinned to versions
- **Language audit**: 100% English across all code, comments, docs, CLI, notifications, and Semgrep rules — 200+ Spanish instances translated across 50+ files
- **Dead code removed**: Forgejo support (3 workflows, remote, env vars). GitLab webhook. `openspec/` SDD artifacts. Stale docs. Commercial licensing files. Dashboard mock-data
- **Security cleanup**: Hardcoded `Admin123!!` removed from docs. `docker-compose.security.yml` deleted. "Zero-Trust" renamed to "Security Hardening"
- **RLS verified empirically**: Confirmed `kuro` user has BYPASSRLS — RLS policies are defense-in-depth for future roles, not active isolation today
- **TeamScopeMiddleware test**: Added test for session-authenticated users documenting that team scoping only applies to API key auth

### 🚀 Added

- **Phase 0 — Foundation**: Version consistency fixes, SECURITY.md, issue/PR templates, CODE_OF_CONDUCT.md, Makefile test targets, README badges, doc consistency sweep
- **TruffleHog verified auto-BLOCK**: Removed `--no-verification`, captured `Verified` bool, policy engine auto-BLOCKs on live secrets
- **History scan mode**: `kuro scan --history` flag for full git log scanning with Gitleaks detect + TruffleHog filesystem
- **Periodic scheduler**: `scheduled_scans` table, worker goroutine checks every 60s, publishes to NATS
- **Dashboard scan_type filter**: Findings filterable by push/scheduled/manual
- **Four-gate security model**: Documented in ARCHITECTURE.md + README

### 📦 Published

- **ghcr.io images**: API, Worker, Notifier, and Backup-Dedup images rebuilt and published as v1.4.1

---

## [1.4.0] - 2026-07-10

### ♻️ Refactor

#### Scanner Adapter Architecture
- **ScannerAdapter interface**: New unified interface with 16 methods covering identity (`Name`, `Description`, `Capabilities`), configuration (`Configure`, `EnvVars`), runtime (`Image`, `Cmd`, `Volumes`, `Run`), execution lifecycle (`DependsOn`, `DependsOnCapabilities`, `Rollback`), and resource metadata (`DefaultTag`, `DocsURL`, `EnvVarDefs`, `ResourceSpec`). All 7 scanners implement this interface.
- **Catalog** (`worker/scanners/scanner.go:112-171`): Read-only discovery layer auto-generated from adapters via `NewCatalog(adapters)`. Always available regardless of configuration. Methods: `Get`, `List`, `ByCapability`.
- **Registry** (`worker/scanners/scanner.go:214-265`): Holds configured (initialized) adapters. Thread-safe with `sync.RWMutex`. `NewConfiguredRegistry` registers only scanners that pass `Configure()`.
- **Capability flags**: 5 capabilities — `sbom`, `secret-scan`, `sast`, `sca`, `sandbox`. Used for dependency resolution and pipeline decisions without scanner name coupling.
- **ScannerFunc**: Backward-compatible wrapper for function-based scanners (deprecated).
- **Configurer**: Deprecated interface, folded into ScannerAdapter.

#### Planner + Pipeline
- **Planner interface** (`worker/pipeline/planner.go:187-207`): `Plan(ctx, enabled)` → `ExecutionPlan` with ordered stages, `Review(ctx, enabled)` → `ReviewPayload` with warnings.
- **dagPlanner** (`worker/pipeline/planner.go:213-390`): Uses Kahn's algorithm for topological sort resolving `DependsOn` and `DependsOnCapabilities`. Validates all deps are registered and enabled. Detects cycles. **Soft ordering** (`scannerSoftOrdering`) splits stages where one scanner should follow another (e.g. Semgrep before Grype) without forcing it as a hard dependency.
- **ExecutionPlan**: Ordered sequence of `ExecutionStage` structs. Scanners within a stage have no transitive dependency and MAY run in parallel.
- **Pipeline interface** (`worker/pipeline/planner.go:577-590`): `Execute(ctx, plan)` → `PipelineResult`, `Rollback(ctx)` → error.
- **sequentialPipeline** (`worker/pipeline/planner.go:596-756`): Runs stages sequentially, scanners within a stage concurrently (bounded by 3-way semaphore). Tracks `StepResult` per scanner with status: `pending`, `running`, `succeeded`, `failed`, `rolled-back`.
- **Rollback**: Reverse-iterates `stepResults` calling `ScannerAdapter.Rollback()` for each succeeded step. Marks rolled-back steps with `rolled-back` status.
- **Progress callbacks**: `PipelineOption` functional options (`WithProgress`, `WithRuntime`, `WithRepoPath`, `WithLogger`, `WithScanID`). `ProgressFunc` receives `ProgressEvent` at each lifecycle transition.
- **ReviewPayload** (`worker/pipeline/planner.go:148-160`): Shows planned stages, total scanners, and `ReviewWarning` entries (capability gaps, missing env var defaults) before execution.

### ✨ Features

#### `kuro doctor` command
- 8 system health checks (PostgreSQL, NATS, MinIO, Docker, scanner images, versions, disk, backup)
- Table and JSON output formats
- Dependency cascading (Docker fail → skip scanner checks)
- Exit codes: 0 = healthy, 1 = CRITICAL failure

#### Content-addressable backup (backup-dedup)
- SHA-256 hashing with deduplication — identical backups use zero additional storage
- S3 storage layout: `hashes/`, `data/`, `pins/` prefixes
- Pin support for permanent backup preservation
- Configurable retention (default 30 days)
- Off-site S3 mirror support

#### Actionable errors
- Every scanner error includes a suggestion: "what happened" + "what to do"
- Error classification: image not found, OOM (exit 137), segfault (exit 139), timeout, volume missing
- Consistent format across Docker, Podman, and Sandbox runtimes

#### CLI TUI (Bubbletea base)
- Optional TUI mode with `--tui` flag or auto-detect on TTY
- Phase progress, spinner, log viewer
- Compatible with existing text mode

### 🧪 Tests
- 17 integration tests (build-tagged: `go test -tags=integration ./worker/pipeline/...`)
- Catalog → Registry → Planner → Pipeline → Rollback full lifecycle tests
- Actionable error detection tests
- All tests passing: unit + integration + existing suite

### 📚 Documentation
- ARCHITECTURE.md rewritten with new scanner architecture
- SCANNER-ARCHITECTURE.md: new standalone reference guide
- AGENTS.md updated with ScannerAdapter + Planner sections
- Removed stale known issues


## [1.3.1] - 2026-07-10

### 🔒 Security

#### Multi-tenant data isolation
- **Report endpoints**: `GET /reports/summary`, `GET /reports/trends`, and `GET /reports/audit` now explicitly filter by `organization_id` — cross-tenant data leakage via analytics endpoints is eliminated.
- **`vulnerability_lifecycle` RLS**: Row-Level Security enabled on the table with a null-safe SELECT policy that allows background tasks (where `app.tenant_id` is unset) to operate without bypassing tenant isolation for authenticated requests.
- **`vulnerability_lifecycle` unique index**: Active unique index now includes `organization_id` as `(organization_id, repository_url, fingerprint) WHERE status = 'active'`, preventing cross-tenant fingerprint collisions.
- **Cache-hit findings copy**: The `ResultsConsumer` cache-hit path now joins the `scans` table and filters by `organization_id` before cloning findings — a cached scan from one tenant can no longer be replicated into another tenant's scope.
- **Lifecycle remediation UPDATE**: Uses validated `orgID` extracted from the authenticated context instead of trusting the `organization_id` value present in the NATS payload directly.

### ⚡ SSE (`GET /scans/{id}/events`)

- **Middleware isolation**: SSE handler is now isolated from the global `Throttle(20)` and `Timeout(30s)` middleware — long-lived event streams are no longer subject to the global request throttle or 30-second timeout.
- **Ownership validation**: Scan ownership is validated against the tenant context before the stream starts; returns 404 if the scan does not belong to the caller's organization.
- **Connection reuse**: Reuses the context-injected `*pgxpool.Conn` from `OrganizationMiddleware` for LISTEN/NOTIFY — no second connection is acquired or pinned.
- **Context data race fix**: `handlerCtx` is captured once from `r.Context()` to prevent concurrent data races on the request context.
- **NATS deadlock fix**: NATS log subscription channel uses a non-blocking send to prevent deadlocking the NATS client when the channel is full.
- **Query storm prevention**: Notification wait loop breaks only on a matching scan ID, preventing unnecessary DB query storms from unrelated notifications.

### 🐧 Worker Sandbox (Linux)

- **Unprivileged user namespace fix**: `UidMappings` and `GidMappings` added to `SysProcAttr` in the worker's sandbox configuration — maps host UID/GID to container root (0) to fix `permission denied` errors in unprivileged user namespaces.

### 📄 Documentation

- **ARCHITECTURE.md**: Updated SSE middleware isolation, multi-tenancy RLS, and sandbox isolation sections.
- **API.md**: Updated SSE endpoint behavior and ownership validation notes.
- **SECURITY_PLAN.md**: Updated multi-tenant isolation layer details.
- **DATA_SCHEMA.md**: Updated `vulnerability_lifecycle` index and RLS policy documentation.
- **AGENTS.md**: Updated SSE, multi-tenancy, and sandbox architecture notes.

---

## [1.3.0] - 2026-07-09

### 🏗️ Architecture

#### Database Decoupling & Event-Driven Dispatch
- **Repository Pattern**: Introduced `ScanRepository` and `FPRepository` domain interfaces in `worker/domain/ports.go` to abstract all DB mutations and queries from worker core business logic.
- **Stateless Worker Execution**: Removed all database write operations (`SaveFindings`, `CompleteScan`, `PersistProxyScanResult`) from `worker/pipeline/executor.go` and `worker/quick.go`.
- **Enriched Message Contracts**: Enriched the `results.completed` NATS JetStream event payload with additional workspace metadata (`commit_sha`, `branch`, `trigger_source`, `scan_path`, `organization_id`).
- **Asynchronous Persistence Consumer**: Implemented `ResultsConsumer` as a durable NATS background worker inside `api/jobs` to asynchronously ingest scan results and persist findings using pgx `CopyFrom` bulk ingestion.
- **Asynchronous Cache-Hit Delegation**: Delegated cache-hit findings cloning to the asynchronous API persistence consumer when a `cached_scan_id` is supplied in the results event.

#### Git Clone Cache & Volume Path Alignment
- **Host Bind-Mount Alignments**: Replaced container-internal named volumes (`git-cache-*`) with host-filesystem bind mounts (`/tmp/kuro-git-cache-*`) in `docker-compose.yml` and `docker-compose.workers.yml`.
- **Path Translation**: Added `GIT_CACHE_DIR_HOST` env parameter allowing `RepoCache` to translate container workspace paths back to host paths for secure read-only container bind mounting in scanner runs.
- **Reference Cache Restored**: Fully re-enabled reference bare clone caching (`RepoCache`) in the main `PipelineExecutor`, reducing network traffic and scanning cold starts.

### 🧪 Testing

- **Repository Test Suite Fix**: Resolved API repositories test suite compilation failures (`team_filter_test.go`) arising from prior database pagination signature drifts.

## [1.2.0] - 2026-07-08

### 📄 Documentation

#### README redesign
- **Consistent English**: removed all Spanish mixed content (sections, bash comments, callouts)
- **Progressive disclosure**: restructured top-to-bottom — headline → how it works → quick start → deep dives
- **Architecture diagrams**: replaced Mermaid code blocks with generated professional diagrams
  - `docs/images/layer1-prepush.jpg` — Layer 1 Pre-Push Protection flow
  - `docs/images/layer2-postpush.jpg` — Layer 2 Post-Push Full Scan flow
- **Notifications section**: integrated real Discord and Telegram alert screenshots side-by-side
- **Observability section**: integrated real SigNoz service latency and trace explorer screenshots
- **Rate limiting table**: converted key-value pairs to tier table (Default / Admin / IP fallback)
- **Stack management**: consolidated duplicate "Gestión de servicios" section into Quick Start

#### Full documentation overhaul — all `.md` files
Applied consistent cognitive-load principles across the entire docs tree:
- Progressive disclosure (answer first, context after)
- Tables over prose walls
- GitHub-style callouts (`> [!NOTE]`, `> [!WARNING]`, `> [!IMPORTANT]`) for critical info
- Consistent English throughout — no mixed-language content

**QUICKSTART.md**
- Fixed duplicate `### 4.` step numbering
- Removed `PolicyReader` / NATS TTL implementation detail (belongs in ARCHITECTURE)
- Clear LOCAL vs REMOTE separation with "What you'll have" outcome statements
- Bash comments translated to English

**CONTRIBUTING.md**
- Fixed Next.js version drift: `15` → `16`
- Added `golangci-lint` section (enforced in CI but previously undocumented)
- Added dashboard-only contributor path (pnpm, no Go required)
- Converted issue reporting from bullet list to structured markdown template
- Added "Verify setup" step after `make setup`

**docs/ARCHITECTURE.md**
- Added Quick Reference table at the top for immediate scanning
- Converted bullet lists to tables: file cache behavior, Git Proxy features, dashboard features, PostgreSQL tables
- Added callouts: 100% protection requirement, Grype disabled warning, DB CRUD vs worker policy mismatch
- Removed emoji markers from flow diagram prose

**docs/API.md**
- Added Quick Reference table: base URLs, auth methods, format, middleware chain, metrics ports
- Flattened `<details>` collapse blocks to flat `####` headings — endpoints scannable without clicks
- Converted rate limiting bullet list to tier table
- Added `> [!IMPORTANT]` callout for API key shown-only-once behavior
- Removed duplicated `# Header (recommended)` comment line
- Added `> [!WARNING]` for `PROXY_FAIL_MODE=closed` fail-closed behavior

**docs/DEPLOYMENT.md**
- Added rollback procedure (was missing — checklist said "backup" but no restore instructions)
- Promoted webhook + branch protection section to top-level prominence (was buried at line 194)
- Added `> [!IMPORTANT]` callout: branch protection is mandatory for merge blocking
- Documented v1.0.0 → v1.2.0 upgrade gap (file claimed v1.2.0 but only covered v0.3.3 → v1.0.0)

**docs/OPERATIONS.md**
- Added Ollama, NATS stream, and MinIO health check commands
- Removed duplicate Quick Commands table / Stack Management bash block (same content twice)
- Added cron scheduling example for cleanup script
- Expanded troubleshooting table with Grype and OAuth causes

**docs/DATA_SCHEMA.md**
- Added FK relationship map at the top (replaces the need for an ERD)
- Fixed composite index bug: column `duplicate` → `is_duplicate` (column does not exist in DDL)
- Clarified `webhooks` (per-team, legacy) vs `admin_webhooks` (global, active) distinction
- Removed changelog-style "Known Schema Changes" section from reference doc
- Added duplicate-ratio diagnostic query

**docs/ENTERPRISE.md**
- Fixed factual error: Grype listed as "active by default" — it is **disabled** by default
- Moved AGPL-3.0 obligation explanation to top (key decision point for enterprise buyers)
- Replaced feature bullet list with comparison table (Community vs Enterprise)

**docs/SECURITY_PLAN.md**
- Fixed internal inconsistency: "9 layers" in executive summary vs "7 layers" in middleware diagram
- Converted opening summary to structured table (# / Layer / Mechanism)
- Added `> [!WARNING]` callout for rate limiting single-instance limitation
- Converted microsegmentation bullet list to Service / Network Exposure table
- Added `> [!IMPORTANT]` callout for `--network none` scanner isolation

### 🔒 Security

- **Dependabot triage**: reviewed and resolved all 20 historical Dependabot alerts
  - 15 alerts were already auto-closed by GitHub (files `requirements.txt` and `app-demo/` removed in earlier commits)
  - 5 open alerts for `github.com/docker/docker` CVEs (12–16) dismissed as `not_used`:
    - CVE-2026-33997 — plugin privilege validation off-by-one (worker does not install plugins)
    - CVE-2026-34040 — AuthZ plugin bypass (worker does not use AuthZ plugins)
    - CVE-2026-34041 — `PUT /containers/{id}/archive` RCE (worker does not call archive endpoint)
    - CVE-2026-34042 — `docker cp` race condition — empty file creation (worker does not use docker cp)
    - CVE-2026-34043 — `docker cp` race condition — bind mount redirection (worker does not use docker cp)
  - No fix available in stable `github.com/docker/docker` v28.x module line; Moby v29.3.1 not yet in Go module proxy
  - `golang.org/x/net` already at v0.55.0 (fixed) in both `api/go.mod` and `worker/go.mod`

---

## [1.1.0] - 2026-07-07

### 🐛 Fixed

- **Webhook secret**: Cached at startup instead of reading `os.Getenv` per request
- **Webhook org_id**: Fallback extracting owner from repo `full_name` if middleware doesn't provide org
- **Webhook goroutine**: Fire-and-forget protected with `r.Context().Err()` check
- **Scan detail**: DB errors in severity/top findings now return errors (previously silenced → phantom data)
- **List scans**: 5 correlated subqueries replaced with `LEFT JOIN LATERAL` + `COUNT(*) FILTER` (N+1 eliminated)
- **argIdx en ListScans**: Now increments correctly (was a time bomb)
- **DLQ consumer**: JetStream context refreshable after NATS reconnect
- **Worker shutdown**: `Unsubscribe()` NATS before `wg.Wait()` eliminates data race
- **FP cache TTL**: Only updates if `loadFPReasons` succeeds
- **Docker runner**: `io.Copy`/`Close` errors during image pull now logged
- **Backup service**: Moved to `backend` network (previously `frontend`, couldn't reach postgres/MinIO)

### ⚡ Performance

- **List scans query**: 5 subqueries per row → 1 LATERAL JOIN
- **Docker timeout**: Removed redundant second `context.WithTimeout`
- **make scan**: `sleep 90` replaced with 5s polling with visual indicator

### 🔧 Changed

- **Backup**: No longer runs 24/7 in polling loop. `make backup-run` on demand
- **Trivy-updater**: Moved to `profile: full`, `restart: no`. Only with `make up-full`
- **Startup modes**: `make up-minimal` (postgres+nats+api), `make up` (+worker), `make up-full` (+trivy)
- **TLS**: Enabled by default (auto-generates self-signed cert). `KURO_DISABLE_TLS=1` for HTTP
- **Nomad**: Removed from boot (`systemctl disable nomad`)
- **S3 variables**: Unified to `S3_ACCESS_KEY`/`S3_SECRET_KEY` (previously mixed with `GARAGE_*`)
- **DOCKER_GID**: Auto-detect commented in `.env.example` instead of hardcoded
- **Secrets**: `make check-env` runs before `make setup` — rejects CHANGEME
- **Bootstrap**: `scripts/bootstrap.sh --local` skips Nomad/systemd/Tailscale
- **Gitignore**: Compiled binaries excluded (`bin/`, `git-proxy/`, `notifications/`)

### 📄 Documentation

- README with startup modes table
- QUICKSTART updated with `make check-env` and corrected steps
- CHANGELOG updated with all changes

## [1.0.0] - 2026-07-01

### Stable Release — Production Ready

**Status**: **PRODUCTION** — Stable v1.0.0 release. OAuth2 GitHub login, server-side pagination, health check excluded from rate limiting, reports endpoints, notification channels API with toggle, dashboard fully connected to real data.

### Added

#### Authentication
- **OAuth2 GitHub login**: Full Authorization Code Flow. Login at `GET /auth/login`, callback at `GET /auth/callback`. Redirects to dashboard with httpOnly `kuro_session` cookie on success. Configurable via `OAUTH_CLIENT_ID`, `OAUTH_CLIENT_SECRET`, `OAUTH_REDIRECT_URL`.
- **HandleSessionLogin auto-create**: First login via API key auto-creates user + `user_roles` entry in the database.
- **RLS policies**: Added INSERT/UPDATE policies on `users` table for API key auth flow.
- **Logout simplified**: No longer requires `OAUTH_CLIENT_ID` to be set — always functional.

#### API Endpoints
- **GET /health excluded from rate limiting**: Moved outside RateLimitMiddleware for monitoring tool access.
- **Server-side pagination**: `GET /scans?limit=20&offset=0` returns `{ data, total }` (default 20, max 200). `GET /findings?limit=50&offset=0&severity=&scanner=` returns `{ data, total }` (default 50, max 500).
- **Reports endpoints**: `GET /reports/summary` (scan statistics), `GET /reports/trends` (trend data), `GET /reports/audit` (audit log).
- **Notification channels API**: `GET /admin/webhooks` (list), `POST /admin/webhooks` (create), `DELETE /admin/webhooks/{id}` (delete), `PATCH /admin/webhooks/{id}/toggle` (activate/deactivate). Supported types: `slack`, `discord`, `telegram`.
- **API Keys management**: `GET /api-keys` (list), `POST /api-keys` (create), `DELETE /api-keys/{id}` (revoke).
- **Policies management**: `GET /policies` (list), `POST /policies` (create).
- **Users listing**: `GET /users` (list all users).
- **Scan detail**: `GET /scans/{id}` returns findings_by_severity.
- **URL validation**: `POST /scans/trigger` validates URL scheme (http/https/ssh/git) and host format.

#### Dashboard
- **Quick Scan**: Button in header opens modal with URL input to trigger scans directly from the UI.
- **Onboarding banner**: Dismissible welcome banner for first-time users (localStorage-backed, SSR-safe).
- **Health indicator**: Shows API status (green/red) in header, auto-updates every 60 seconds.
- **Findings tooltips**: Severity and scanner badges show tooltips explaining their meaning.
- **Scan list finding counts**: Each scan row shows `total_findings`, `critical`/`high`/`medium`/`low` counts.
- **Finding Details redesign**: Severity-colored header, impact summary, code snippet with file path, fix suggestions.
- **Scan Details redesign**: Fetches real data from `/scans/{id}`, shows `findings_by_severity` from API.
- **Policies page redone**: Shows real worker rules from `deploy/policies/default-policy.json` as visual cards, expandable for details.
- **Reports page connected**: Shows real data from `/reports/summary` with stats grid and severity bar chart.
- **Settings page**: Notification channels management (Slack/Discord/Telegram) with add/test/delete/toggle functionality.
- **Dev mode**: Dashboard runs with `pnpm dev` for hot reload. No Docker rebuild needed for frontend changes.

#### Worker
- **Partial scanner success**: Findings survive individual scanner errors — a failed scanner doesn't lose results from successful ones.
- **Bypassed git-cache**: Temporarily bypassed due to hostPath bug (`EnsureRepo` returns container-internal paths). Uses `vcs.CloneRepo` directly instead.
- **Scanners memory limit increased**: From 768m to 1g per container.
- **docker-proxy permissions expanded**: Added AUTH, EXEC, POST, ALLOW_RESTART capabilities.
- **Trivy DB fixes**: Volume mount changed from `:ro` to `:rw`. `trivy-updater` moved to frontend network with DNS `8.8.8.8`.
- **Grype disabled**: Temporarily disabled since Trivy covers vulnerability scanning. Needs DB volume setup similar to trivy.

#### Git Proxy
- **Thin pack fix**: Fetches upstream objects before unpacking to handle thin packs correctly.
- **TeeReader**: Full body forwarding for robust packfile transmission.
- **GITHUB_USER env var**: Support for Basic Auth in addition to token-based auth.

#### Findings
- **CodeSnippet field**: Added to Finding struct and returned in API responses. Findings detail shows snippet when available.
- **cve_id field**: Added to Finding struct for vulnerability scanner results.

### Changed
- **Rate limits increased**: From 100 RPM / 20 burst to 2000 RPM / 500 burst. Admin: 5000 RPM / 1000 burst.
- **Rate limit env vars**: `RATE_LIMIT_RPM`, `RATE_LIMIT_BURST`, `RATE_LIMIT_RPM_ADMIN`, `RATE_LIMIT_BURST_ADMIN`.
- **OAuth callback**: Now redirects to dashboard with session cookie (previously returned JSON directly).
- **Dashboard DataProvider**: Uses sequential `fetchAll()` instead of `Promise.all()` for API calls.
- **Findings store**: `fetchFindings` handles both array and `{ data, total }` paginated response formats.
- **Finding interface**: Includes `snippet`, `remediation`, `rule_id` fields in TypeScript types.
- **Retry limits**: Backoff adjusted (5s/25s/125s default).
- **Dashboard sidebar**: Footer removed, Settings now navigates to `/settings`.
- **Documentation**: All files updated to reflect v1.0.0 with complete endpoint tables, env var docs, and architecture updates.

### Fixed
- **Findings API missing code_snippet**: `code_snippet` was not included in the SELECT query — now returned properly.
- **Scan list finding counts**: Scans endpoint now returns `total_findings`, `critical`, `high`, `medium`, `low` per scan.
- **Git cache EnsureRepo**: Returning container paths as host paths — temporarily bypassed with `vcs.CloneRepo`.
- **Gitleaks custom config too restrictive**: Backed up the custom config (29 rules didn't cover all default 150+ rules).
- **docker-proxy missing permissions**: Scanner containers couldn't start without AUTH/EXEC/POST capabilities.
- **Trivy DB not downloading**: Internal network + no DNS prevented DB download. Fixed with frontend network + `8.8.8.8`.
- **Grype DB not available**: Disabled as fallback until volume setup is implemented.
- **Git proxy unpack-objects failing**: Thin packs from GitHub need upstream objects before unpacking — fixed with pre-fetch.
- **Dashboard 429 rate limit**: Health check was rate-limited and limits were too low. Excluded `/health` and increased limits.
- **Auth session creation (403)**: Missing `user_roles` insert on first login via API key — `HandleSessionLogin` now creates both.
- **OAuth callback returning JSON**: Now redirects to dashboard with session cookie as expected.
- **Logout button not working**: Sidebar "Log out" now correctly calls `POST /auth/logout`.
- **Findings detail "No snippet available"**: Shown when API didn't return `code_snippet` — now handled gracefully.
- **Repo URL stripping**: `github.com` was incorrectly stripped from URLs in scans table.
- **Scan status type mismatch**: Dashboard store remapped API status values incorrectly — fixed.
- **localStorage SSR**: Onboarding banner no longer breaks during server-side rendering.
- **fetchFindings pagination**: Store now correctly handles `{ data, total }` format from API.
- **API keys revoked**: Now properly filtered from view (don't appear as "Revoked").
- **Stale test of `?api_key=`**: Updated to expect 401 as intended.

### Known Issues
- **Git-cache hostPath bug**: `EnsureRepo` returns container-internal paths. Temporarily bypassed with `vcs.CloneRepo` direct clone.
- **Grype disabled**: Needs DB volume setup similar to trivy. Trivy covers vulnerability scanning for now.
- **Findings dedup fingerprint**: Same code patterns produce identical fingerprints across different files.
- **Custom gitleaks config too restrictive**: 29 rules vs 150+ defaults — backed up but needs reconciliation.
- **Rate limiting is in-memory Token Bucket**: Single-instance only. Not distributed — needs Redis for multi-node.
- **Policies DB CRUD exists but unused**: Worker reads `deploy/policies/default-policy.json` directly, not the DB policies CRUD.
- **Grype image tag**: Uses `:latest` — needs pinning to specific version for reproducible builds.

---

## [1.0.0-rc.2] - 2026-06-30

### Fixed

- **Falla 1 & 2 (Smart-HTTP Ref Command Parsing)**: Synchronous interception on the Git server (`scripts/pre-receive-kuro.sh`) and safe parsing of Git Smart-HTTP protocol in `git-proxy/main.go` to extract exact commit SHA and branch, eliminating insecure heuristics.
- **Falla 3 (Git Cache Isolation)**: Prevented concurrent working directory collisions in `worker/gitcache/cache.go` by using individual private `--shared` temporary clones per scan.
- **Falla 4 (API Keys CPU DoS Mitigation)**: Replaced Bcrypt with high-speed SHA-256 (`subtle.ConstantTimeCompare`) for API key authentication, maintaining transparent compatibility with legacy Bcrypt keys.
- **Falla 5 (Auth Middleware Memory Leak)**: Fixed goroutine leak in `usageBatcher` using a `stop` channel and garbage collection via `runtime.SetFinalizer`.
- **Falla 6 (Webhook Double Scanning)**: Decoupled synchronous quick scans and asynchronous full scans in GitHub and GitLab webhooks to prevent cache collisions.
- **Falla 7 (Connection Pool RLS Leak)**: Prevented tenant_id leakage in PostgreSQL by manually resetting the session variable (`app.tenant_id`) in `context.Background()` when returning connections.
- **Falla 8 (Real-time SSE with Postgres LISTEN/NOTIFY)**: Replaced periodic database polling with native real-time push notifications for Server-Sent Events.
- **Falla 9 (Scanner Failure Propagation)**: Ensured infrastructure failure propagation at `RunScannerWithRetry` level without interrupting healthy parallel scanners.
- **Falla 10 (Zombie Docker/Podman Containers)**: Integrated `AutoRemove: true` when instantiating Docker containers in `worker/scanners/docker.go`.
- **Falla 11 (Multi-tenant VCS & Proxy Leak)**: Added real multi-tenant support for per-organization VCS URLs/tokens in `executor.go`. Also fixed tenant leak in `persistProxyScanResult` by propagating ScanJob's `OrganizationID`.

### Security & Robustness

- **In-memory Rate Limiter DoS Mitigation**: Migrated synchronous rate limiter to an in-memory RAM Token Bucket algorithm with inactive key expiration every 5 minutes, protecting PostgreSQL database connections from accidental or intentional saturation.
- **Git Proxy Scan Cache**: Added a decorator client in the Git HTTP proxy that caches recently approved commit responses for 1 hour, eliminating redundant requests and streamlining developer interactions.
- **Async Ollama Fallback in Worker**: Optimized startup availability check (maximum 3 seconds) and configured immediate failure (`Ollama is offline`) in the embedding pool to force the Worker to immediately degrade to deterministic local semantic similarity (Jaccard) without hanging on 30-second network timeouts.
- **Webhook Payload Limit**: Added 10MB maximum size protection (`http.MaxBytesReader`) on GitHub, GitLab, and Git Proxy webhook endpoints to prevent DoS attacks via memory exhaustion.
- **VCS Retry & Dashboard URL**: Implemented automatic HTTP retries (3 attempts) for VCS posts with backoff, and reconfigured the commit status check redirect URL to properly point to the Dashboard interface (port `:3000`) instead of the raw API JSON.

## [1.0.0-rc.1] - 2026-06-29

### Release Candidate 1 — Enterprise Readiness

**Status**: **PRODUCTION (RC)** — Complete scanner isolation, 100% parser coverage, Real Multi-tenancy (Org + Team), Dashboard connected to real data.

### Added

- **`POST /auth/session` endpoint**: Exchanges API key for JWT session stored in httpOnly `kuro_session` cookie. Replaces localStorage-based API key storage in the dashboard.
- **Auth batcher**: `last_used_at` updates now use a batcher (flush every 5s) instead of a goroutine per request.
- **Off-site S3 backup mirror**: Optional env vars `OFFSITE_S3_ENDPOINT`, `OFFSITE_ACCESS_KEY`, `OFFSITE_SECRET_KEY`, `OFFSITE_BUCKET` for remote disaster recovery.
- **Git Proxy graceful shutdown**: SIGTERM triggers clean shutdown (was `log.Fatal` with no shutdown).
- **golangci-lint v2**: Added to CI pipeline (`ci.yml`) for automated Go linting.
- **Git build args**: Dockerfiles now accept `GIT_COMMIT` and `BUILD_DATE` build args for ldflags injection.
- **Parser Coverage (100%)**: 68 new unit tests in `worker/scanners/` under *Strict TDD* mode. Tests validate dual parsing (e.g., Checkov old/new), failure mitigation (invalid Trufflehog NDJSON), and log cleanup (Gitleaks ASCII art) without requiring Docker daemon.
- **Multi-tenancy P2 (Teams)**: New migration `012_team_isolation.sql` with strict `team_id` isolation. Added `AND team_id = $N` filters in Postgres repositories (`scans`, `findings`), services, and handlers. Implemented and verified `TeamScopeMiddleware` with 16 new unit tests.
- **Dashboard Pagination**: Scans Table now includes server-side pagination, exact vulnerability counts (e.g., `3C 5H 7M`), and scan duration time.
- **New Diagrams**: Complete migration from D2 to SVG (`.drawio`). New architecture, proxy, and webhooks diagrams.

### Changed

- **Worker RAM**: Increased from 512m to 1G in docker-compose.
- **Scanner container memory**: Increased from 512m to 768m per container.
- **Scanner concurrency**: Max 3 concurrent scanners (semaphore channel in executor.go).
- **Semgrep CPU limit**: Now runs with `--jobs 2`.
- **PostgreSQL `shared_buffers`**: Increased from 256MB to 512MB.
- **Ollama startup**: Worker now waits for Ollama to be reachable (pings `/api/tags` at 5s intervals for up to 2 minutes) before starting with embeddings.
- **Docker image tags**: All ghcr images pinned from `:latest` to `:v1.0.0-rc.1`.
- **Health endpoint**: Returns JSON `{"status":"ok","db":"connected","nats":"connected"}` with DB/NATS status (returns 503 if either is down).
- **Dashboard auth**: `fetchAPI()` no longer reads from localStorage — uses httpOnly cookie via `credentials: 'include'`. SSE uses `{ withCredentials: true }`.
- **Login page**: POSTs to `/auth/session` instead of saving API key to localStorage.
- **Dockerfile cleanup**: Removed `ARG CACHEBUST=1` (was invalidating build cache without benefit).
- **Dotfiles**: All `v0.3.3` version references updated to `v1.0.0-rc.1`.
- **Isolation & Security**: Versions of the7 tools (Trufflehog, Syft, Grype, etc.) pinned semantically (`:latest` → `vX.Y.Z`) in `setup.sh` and `smoke-test.sh`. Removed 'latest' dependency that broke the pipeline in production.
- **Dashboard Metrics**: Statistics cards (`stats-cards.tsx`) and charts (`findings-chart.tsx`) now consume directly from `/api/reports/summary` and `/api/reports/trends`, eliminating dependency on truncated lists (default limit=50) in the frontend.

### Fixed

- **Multi-tenancy P1 (Org Isolation Bug)**: Fixed a critical bug in `OrganizationMiddleware` (`api/handlers/organization.go`) that tried to look up organizations using the bcrypt hash of the API Key instead of the already authenticated `user_id`. Postgres RLS now works correctly.
- **Dashboard Period Filters**: Period filters in "Top Vulnerabilities" (week, month, year) now filter real data instead of being dead-code.
- **API Key Generation & Deadlock**: Fixed a critical deadlock in `getRepoScores` (`api/handlers/security.go`) that was performing N+1 blocking queries, causing timeouts (500s) when trying to list organizations. The query was rewritten using native aggregations (`SUM(CASE WHEN...)`) in Postgres.
- **Dashboard API Parsing & Infinite Redirect**: Fixed an infinite 401 redirect loop in `/login`. Also fixed a parsing bug where the dashboard expected `api_key` but the API returned `key`, causing the modal to freeze (now covered by automated E2E tests with Playwright).
- **Dashboard Decision Enum Bug**: Standardized `decision` values across the React application (`APPROVED`, `BLOCKED`, `MANUAL_REVIEW`), eliminating inconsistencies (`approve`, `PASS`) that caused latent bugs.
- **Fake buttons removed**: Removed the fake AI button ("Ask AI") that called a non-existent route, and replaced the misleading static "Platforms" section with "Coming soon".

### Documentation

- **API.md**: Added `POST /auth/session` endpoint docs, updated `/health` response format.
- **ARCHITECTURE.md**: Updated auth flow, worker/scanner resources, Postgres tuning, Git Proxy shutdown, scanner concurrency.
- **DEPLOYMENT.md**: Updated resource tables, off-site backup config, golangci-lint CI note, Postgres tuning guidance.
- **AGENTS.md**: Added `/auth/session` endpoint, updated date.
- **worker/CONFIG.md**: Added Ollama startup wait behavior docs.
- **QUICKSTART.md**: Updated health check response format.
- **Version references**: Complete documentation updated to reflect `v1.0.0-rc.1`.

---

## [0.4.0] - 2026-06-14

### Added

- **CLI orchestrator**: 6 phases (fetch → scope → scan → analyze → decide → report) with two modes
- **Local mode**: `kuro scan ./proyecto` runs scanners via Docker/Podman directly, without server
- **Remote mode**: `kuro scan https://...` sends to Kuro server
- **`kuro setup images`**: downloads the4 scanner images with pinned versions
- **File cache**: `~/.kuro/cache/file-scan.json` skips unchanged files (SHA-256)
- **Lazy pull**: images download in background during the fetch phase
- **Orchestrator tests**: 63 tests, 47% coverage in `cli/internal/orchestrator/`
- **Tests in empty packages**: `api/db`, `api/config`, `git-proxy`, `worker/domain`

### Changed

- **Parallel scanners**: Gitleaks, Semgrep, Trivy, Checkov run in simultaneous goroutines (4x faster)
- **Resource limits**: each container runs with `--memory=512m --cpus=1.0`
- **Scanner timers**: each scanner shows its real time in the output
- **Gitleaks v8**: updated to API `dir` + `/proc/self/fd/1` for JSON output
- **Checkov**: version 3.2.534 → 3.2.400 (non-existent tag); exit code 2 handled (no IaC)
- **Semgrep**: removed `--no-git-ignore` (respects `.gitignore`)
- **Pinned images**: all with `docker.io/` prefix for Podman compatibility
- **Diagrams**: updated architecture-overview, proxy-flow, pipeline-flow with orchestrator
- **README**: new "CLI Orchestrator" section with modes, phases, examples

### Fixed

- **Git status**: removed vestigial `.git/` from testdata repos
- **Pre-commit**: added testdata to gitleaks allowlist
- **`runContainer`**: returns stdout instead of stderr when scanner exits with code 1
- **Trivy**: graceful fallback when DB is not cached
- **Checkov**: exit code 2 (no IaC files) treated as empty findings
- **Parser tests**: updated to Gitleaks v8 format

---

## [0.3.3] - 2026-06-11

### Changed

- **README**: rewritten in English with Table of Contents, cleaner structure, link to `.env.example` and `CONTRIBUTING.md`
- **CONTRIBUTING**: expanded from 54 lines to a full guide — prerequisites, project structure, Go/TS conventions, testing table, PR template, issue reporting
- **Dashboard design system** (`globals.css`): replaced default shadcn neutral palette with a security-themed dark design — deep navy/slate backgrounds, cyan primary accent, severity-mapped chart colors (red→amber→yellow→cyan→green)
- **Dashboard layout** (`layout.tsx`): fixed leftover "Square UI" placeholder title → "Kuro — Security Dashboard"
- **Dashboard header** (`header.tsx`): fixed hardcoded `yourusername` GitHub URL
- **Dashboard cards** (`stats-cards.tsx`, `security-score-card.tsx`): applied `card-hover-glow` utility for consistent hover effect
- **SVG diagrams**: rewrote all three main diagrams from scratch as clean hand-crafted SVG (dark mode, security palette). Size reduced 97%:
  - `architecture-overview.svg`: 483 KB → 19 KB
  - `proxy-flow.svg`: 681 KB → 12 KB
  - `webhook-flow.svg`: 700 KB → 16 KB
- **`docker-build.yml`**: migrated from Docker Hub (`docker.io/Haiagari`) to `ghcr.io` using `github.repository_owner`; bumped `build-push-action` v5→v6; added `git-proxy` image to build matrix
- **`kuro-doctor.yml`**: removed every-6-hours cron schedule (binary not present, was always returning score 100 and burning CI minutes); tightened guard to check for binary existence
- **`dependabot.yml`**: removed `pip` ecosystem (no Python deps); added `gomod` for `/git-proxy` and `npm` for `/dashboard`

### Fixed

- **ARCHITECTURE.md**: broken SVG references (`pipeline-flow-proxy.svg`, `pipeline-flow-webhook.svg` → `proxy-flow.svg`, `webhook-flow.svg`); dead link to `HOW-IT-WORKS.md`
- **All docs**: version string inconsistencies (`v0.3.1` titles with `v0.3.3` content) — unified to `v0.3.3` throughout
- **All docs**: broken footer links pointing to `INDEX.md` (never existed) → `../README.md`
- **OPERATIONS.md**: broken footer (text mixed into code block); resource table version label `v2.7.0` → `v0.3.3`
- **DEPLOYMENT.md**: dead link to `HOW-IT-WORKS.md` → `ARCHITECTURE.md`
- **`ci.yml`**: removed dead Python/pytest job (leftover from a previous incarnation — no `setup.py` or `kuro_pipeline/` package exists)
- **`ci.yml`**: fixed smoke test URL port `5002` → `8080`; replaced non-existent `garage` service with `minio`; wired `go.sum` cache paths; `e2e` now depends on `go` instead of the removed `test` job; unified smoke script name
- **`ci.yml`**: added missing `worker` service to the `docker compose up` stack for the E2E proxy test to prevent `nats: no responders available` error
- **`tests/e2e-proxy.sh`**: fixed Docker-in-Docker path resolution issue where test files created in the CI container were invisible to sibling containers; test files are now created/cleaned via `docker exec kuro-git-proxy`
- **`tests/e2e-proxy.sh`**: fixed Bash arithmetic gotcha (`((pass_count++))`) that aborted the test silently under `set -e`
- **`secret-scanning.yml`**: pinned TruffleHog from `@main` to `@v3.88.22` (supply chain hardening); added `trufflehog` to `summary` job needs so its result is reported

---

## [0.3.2] - 2026-06-11

### Added

- `GITHUB_TOKEN` + `GITHUB_USER` documented in `.env.example` with scopes instructions
- Script `tests/e2e-proxy.sh`: automatic proxy flow test, verifies 200 APPROVED / 403 BLOCKED
- Target `make test-e2e-proxy` in Makefile
- `docker-compose.ollama.yml` updated with granite-embedding-97m-r2, realistic resources (512 MB)
- `make ollama` updated: downloads model and restarts worker
- `PROXY_FAIL_MODE`, `PROXY_SCAN_TIMEOUT` documented in DEPLOYMENT.md

### Changed

- **Git Proxy Architecture**: The proxy no longer executes Docker scanners directly.
  Now extracts push files, copies them to a shared volume (`/tmp/kuro-scans/proxy/{push-id}/files/`),
  and delegates scanning to the Worker via NATS Request-Reply (scans.quick). The Worker runs
  Gitleaks + critical Semgrep in <30s, persists in DB with trigger_source='proxy', and
  triggers async full scan with all7 scanners.
- **API**: New endpoint `POST /api/v1/scans/proxy` replaces `/internal/pre-receive`.
  The API no longer executes Docker containers — only publishes to NATS and waits for response.
- **Worker**: ScanJob extended with `ScanPath` (proxy flow: read files from disk instead of cloning)
  and `TriggerSource` (scan origin: proxy, webhook, manual, api).
- **DB**: Migration 011 adds `trigger_source`, `push_id`, `scan_path` to `scans` table.

### Added

- `PROXY_FAIL_MODE=open|closed`: controls whether the proxy lets the push through (fail-open)
  or blocks it (fail-closed) when the Worker doesn't respond in time.
- `PROXY_SCAN_TIMEOUT`: proxy scan timeout (default 30s).
- `SCAN_WORKDIR_HOST` in git-proxy: shared volume path with the Worker.
- `persistProxyScanResult()` in Worker: persists proxy flow findings in DB with trigger_source='proxy'.
- `publishProxyFullScan()` in Worker: publishes scans.new for async full scan post-proxy.

### Removed

- `api/handlers/pre_receive.go` (122 lines): `runAllScanners()`, `runGitleaks()` removed.
- Endpoint `POST /internal/pre-receive` replaced by `POST /api/v1/scans/proxy`.
- Docker Socket dependency in git-proxy (no longer needs DOCKER_HOST).
- Fixer interface in provider.go
- OLLAMA_FIX_MODEL env var
- Bloated documentation: reduced from 50+ .md files to12 essential ones
- Legacy scripts, evidence, openspec, docs/business, docs/product
- NATS cluster (nats-2, nats-3, redundant volumes)

### Fixed

- AuthMiddleware now respects JWT session before requesting API key
- JWT hand-rolled replaced by golang-jwt/jwt v5 with RegisteredClaims (validation of alg, exp, iss)
- panic() in getJWTSecret and scanner registry replaced by return error
- Rate limiter centralized in PostgreSQL (works with N API replicas)
- HandleAskAI no longer a mock with time.Sleep -- uses NATS request-reply to worker with real Ollama
- Docker socket direct in worker-2/3 replaced by docker-socket-proxy
- Images alpine:latest -> alpine:3.20, trivy:latest -> 0.57.0, docker-socket-proxy -> v0.4.2
- Secrets without defaults in docker-compose.yml (.env required)
- deploy.resources duplicated with real mem_limit for docker compose up
- Documentation corrected: dedup threshold 0.92 (was 0.85), SHA-256 formula corrected, exaggerated claims removed

### Added

- NATS cluster3 nodes with JetStream R3 (Replicas:3)
- rate_limits table for centralized rate limiting
- Migration 008_rate_limits.sql
- ai.ask subscription in worker for AI questions via NATS
- docker-socket-proxy in main compose

---

## [0.3.1] - 2026-06-10

### Added

- Git Proxy: intercepts `git push`, runs gitleaks, forwards to GitHub only if approved
- Endpoint `POST /internal/pre-receive` for scanner integration in the proxy
- Authentication via GITHUB_TOKEN for upstream forwarding
- Migration 010: embedding dimension change 768 -> 384 for granite-97m

### Changed

- Embedding model: nomic-embed-text (137M, 768d, 275MB) -> granite-embedding-97m-multilingual-r2 (97M, 384d, ~100MB)
- NATS: from 3-node cluster to single node (was overkill for alpha)
- Networks: separation into backend (internal) + frontend
- Resource limits: deploy.resources duplicated with real mem_limit for docker compose up
- Version files updated from dev to 0.3.1 in api/worker/notifications/cli

### Removed

- FixGenerator and all correction suggestion code (used embedding model as generative)
- HandleAskAI and ai.ask subscription (Security Chat, nobody used it having ChatGPT)

---

## [0.2.0] - 2026-05-22

### Added

- **5 integrated scanners**: Gitleaks, Semgrep, Trivy, Checkov, TruffleHog
- **Hybrid deduplication**: SHA256 + Ollama embeddings + pgvector
- **Policy Engine**: 3 rules (BLOCK gitleaks, BLOCK CRITICAL, REVIEW 3+ HIGH)
- **Garage S3 (AWS SDK v2)**: SigV4 auth, legacy MinIO fallback
- **Multi-layer auth**: API Key + OAuth2 GitHub + JWT + HMAC-SHA256
- **Dynamic RBAC**: 4 roles (admin, security, operator, viewer)
- **Rate limiting**: token bucket, 100 req/min per user
- **Prometheus /metrics**: Worker (9090), API (8080)
- **OpenTelemetry traces**: optional via `OTEL_EXPORTER_OTLP_ENDPOINT`
- **Dashboard SPA**: served from API at `/dashboard/`

### Improved

- **Modern S3**: AWS SDK v2 with SigV4 replaces legacy MinIO
- **Robust auth**: multi-layer covers APIs, webhooks, OAuth
- **Security**: rate limiting prevents abuse

---

## [0.1.0] - 2026-05-20

### Initial release

### Added

- **API Gateway (Go + Chi)**: 33 REST endpoints
- **Worker (Go)**: async scanner pipeline
- **Notifier (Go)**: Slack, Discord, Telegram, Plane.so
- **PostgreSQL 17 + pgvector**: 11 tables
- **NATS JetStream**: message bus, auto-created SCANS stream
- **Docker Compose**: 8 services (postgres, nats, garage, api, worker, notifier, backup, ollama)
- **Healthchecks**: all services with health endpoints
- **Resource limits**: configurable per service
- **Backup service**: daily 2am, 30 day retention

### Core Features

- Git repo scanning (HTTP/SSH)
- Secret, vulnerability, misconfiguration detection
- Policy Engine with automatic decisions
- Configurable notifications
- S3 Artifacts

