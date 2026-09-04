#!/usr/bin/env bash
set -euo pipefail

# Test runner wrapper that ensures high rate limits for the full suite.
# Prevents flaky 429 failures caused by rate limit accumulation across tests.

export KURO_API_RATE_LIMIT_DEFAULT="100000 per minute"
export KURO_API_RATE_LIMIT_LOGIN="100000 per minute"
export PYTEST_DISABLE_PLUGIN_AUTOLOAD=1

exec .venv/bin/python -m pytest "$@"
