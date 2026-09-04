# E2E Tests — Kuro v1.4.1

> **Test Coverage**: 9 E2E tests + 1 smoke test (~6.5 min runtime)  
> **Unit Tests**: 130+ tests, 57% global coverage

## v1.4.1 Code Quality Improvements

### New Error Handling Patterns (testable)
- **`encodeJSON()` helper**: All `json.NewEncoder(w).Encode()` calls replaced with error-logging wrapper. Tests should verify the helper doesn't break existing behavior.
- **`rows.Err()` checks**: 20 new checks after `rows.Next()` loops. Tests can validate that DB iteration errors propagate correctly instead of being silently ignored.
- **`defer recover()`**: 19 goroutines now protected. Tests for background goroutines should ensure recovered panics don't mask failures.

### New Graceful Shutdown Paths (testable)
- **NATS subscriptions**: `Unsubscribe()` on shutdown for ResultsConsumer, DLQ consumer, and Notifier pull subscription. Tests should verify subscriptions are cleaned up.
- **`backup-dedup`**: Now handles SIGTERM/SIGINT. Tests should verify graceful cancellation.
- **UsageBatcher**: Still uses `SetFinalizer` as cleanup path (no explicit `Close()` in API main).

### Concurrency Changes (testable)
- **`gitcache.evict()`**: Path removal now happens outside the lock. Tests should verify no TOCTOU race between eviction and concurrent `EnsureRepo` calls.
- **S3 client singleton**: Created once at startup. Integration tests should verify artifact uploads still work with a single client.
- **DB pool config**: Worker now uses `pgxpool.NewWithConfig()` with explicit MaxConns=8. Tests should verify no pool exhaustion under concurrent scan scenarios.

### CI/CD Changes
- All workflows now have explicit `permissions:` blocks (least privilege)
- Docker images pinned to versions instead of `:latest`
- `persist-credentials: false` on all checkout steps

## Setup

1. **Start services**:
   ```bash
   docker compose up -d
   ```

2. **Generate test API key**:
   ```bash
   bash scripts/bootstrap-api-key.sh
   # Copy the "kuro_live_..." key from output
   ```

3. **Set environment variable**:
   ```bash
   export KURO_TEST_API_KEY=kuro_live_YOUR_KEY_HERE
   ```

   Or create `tests/.env`:
   ```bash
   cp tests/.env.example tests/.env
   # Edit tests/.env and set KURO_TEST_API_KEY
   ```

## Run Tests

```bash
# All E2E tests
go test -v ./tests -timeout 10m

# Single test
go test -v ./tests -run TestKuroCLI_AuthConfig

# Smoke test only
go test -v ./tests -run TestSmoke
```

## Security

**NEVER commit API keys to git.**

- ✅ Use `KURO_TEST_API_KEY` environment variable
- ✅ Keep `tests/.env` in `.gitignore`
- ❌ DO NOT hardcode keys in source files

If you accidentally commit a key:
1. Revoke it immediately in the database
2. Generate a new one with `bootstrap-api-key.sh`
3. Use `git filter-repo` or force push to clean history

## Test Coverage

| Test | Duration | Description |
|------|----------|-------------|
| `TestKuroCLI_AuthConfig` | <1s | Config creation and permissions |
| `TestKuroCLI_ScanWithFindings` | ~95s | WebGoat scan (Java repo) |
| `TestKuroCLI_ScanCleanRepo` | ~10s | Hello-World scan (no findings) |
| `TestKuroCLI_StatusCommand` | <1s | Status polling |
| `TestKuroCLI_VersionCommand` | <1s | Version output |
| `TestKuroCLI_HelpCommand` | <1s | Help text |
| `TestKuroCLI_JSONOutput` | ~95s | JSON format validation |
| `TestKuroCLI_InvalidRepo` | <1s | Error handling |
| `TestKuroCLI_UpdateCommand` | <1s | Update check |
| `TestSmoke` | ~6s | Fast health check (12 assertions) |

**Total runtime**: ~6.5 minutes (9 E2E tests + 1 smoke test)

## Troubleshooting

**Error: `KURO_TEST_API_KEY environment variable not set`**
- Solution: `export KURO_TEST_API_KEY=$(bash scripts/bootstrap-api-key.sh | grep kuro_live | head -1 | awk '{print $3}')`

**Error: `401 Unauthorized`**
- Solution: Key may be revoked or expired. Generate a new one with `bootstrap-api-key.sh`

**Error: `connection refused`**
- Solution: Ensure services are running: `docker compose ps`

**Tests timeout**
- Solution: Increase timeout: `go test -v ./tests -timeout 15m`
