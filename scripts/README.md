# Scripts — Kuro v1.4.1

```
scripts/
├── deploy.sh          → make deploy        → kuro deploy
├── setup.sh           → make setup
├── setup-tls.sh       → make setup-tls
├── setup-ollama.sh    → make setup-ollama
├── setup-runner.sh    → make setup-runner
├── setup-firewall.sh
├── setup-hooks.sh
├── setup-nats-streams.sh
├── health-check.sh    → make health
├── check-readiness.sh → make health
├── smoke-test.sh      → make smoke
├── smoke-prod.sh      → make smoke
├── run-tests.sh       → make test
├── run-integration-tests.sh → make test-integration
├── run-validation.sh  → make validate
├── deploy-gate.sh     → make release-gate
├── rc-gate.sh         → make rc-gate
├── release-gate.sh    → make release
├── init.sh / init-docker.sh / bootstrap.sh / bootstrap-prod.sh
├── pre-commit-secrets.sh (git hook)
├── scan-container-image.sh
├── prepare-validation.sh / ecosystem-pilot-gate.sh
│
├── legacy/            → Scripts one-shot or historical

├── backup.sh*         → ⚠️ DEPRECATED: use `kuro backup`
├── restore-platform.sh* → ⚠️ DEPRECATED: use `kuro backup restore`
├── register-webhook.sh* → ⚠️ DEPRECATED: use `kuro webhook`
├── bootstrap-api-key.sh* → ⚠️ DEPRECATED: use `kuro auth`
├── rotate-api-keys.sh* → ⚠️ DEPRECATED: use `kuro auth`
├── run-local-scan.sh* → ⚠️ DEPRECATED: use `kuro scan`
└── test-webhook.sh*   → ⚠️ DEPRECATED: use `kuro webhook test`

Scripts marked with * are deprecated — the Go CLI (`cli/kuro`) replaces them.
Scripts in `legacy/` are one-shot or historical, not needed to operate Kuro.
