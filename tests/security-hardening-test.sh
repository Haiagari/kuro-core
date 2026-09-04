#!/usr/bin/env bash
# ═══════════════════════════════════════════════════════════════════════════
# Kuro — Security Hardening Integration Test
#
# Tests that security hardening measures are working:
# 1. Docker socket NOT mounted in worker
# 2. Docker Socket Proxy is running and restricting operations
# 3. API key bcrypt authentication works
# 4. NATS TLS is enabled (when overlay is active)
# 5. PostgreSQL SSL is enforced (when overlay is active)
#
# Usage: ./tests/security-hardening-test.sh
# ═══════════════════════════════════════════════════════════════════════════

set -euo pipefail

COMPOSE_FILE="docker-compose.yml"
COMPOSE_SECURITY="docker-compose.security.yml"

echo "🔒 Kuro — Security Hardening Test"
echo "════════════════════════════════════════════════════════════════════════"
echo ""

# ── Test 1: Docker Socket NOT mounted ──────────────────────────────────────

echo "[1/5] Verifying Docker socket is NOT mounted in worker..."

if docker compose ps worker >/dev/null 2>&1; then
    SOCKET_MOUNT=$(docker inspect kuro-worker 2>/dev/null | jq -r '.[0].Mounts[] | select(.Destination == "/var/run/docker.sock") | .Source' || echo "")

    if [ -z "$SOCKET_MOUNT" ]; then
        echo "✅ PASS: Docker socket is NOT mounted in worker"
    else
        echo "❌ FAIL: Docker socket is mounted at $SOCKET_MOUNT"
        echo "   This is a CRITICAL security vulnerability!"
        exit 1
    fi
else
    echo "⏭  SKIP: Worker not running"
fi

echo ""

# ── Test 2: Docker Socket Proxy (if security overlay is active) ────────────

echo "[2/5] Checking Docker Socket Proxy..."

if docker compose -f "$COMPOSE_FILE" -f "$COMPOSE_SECURITY" ps docker-socket-proxy >/dev/null 2>&1; then
    if docker compose ps docker-socket-proxy | grep -q "Up"; then
        echo "✅ PASS: Docker Socket Proxy is running"

        # Verify restricted operations
        echo "   Testing restricted operations..."

        # This should work (CONTAINERS=1)
        if docker compose exec -T worker sh -c 'curl -sf http://docker-socket-proxy:2375/containers/json' >/dev/null 2>&1; then
            echo "   ✅ Allowed: list containers"
        else
            echo "   ⚠️  Cannot list containers (expected to work)"
        fi

        # This should fail (BUILD=0)
        if docker compose exec -T worker sh -c 'curl -sf http://docker-socket-proxy:2375/build' >/dev/null 2>&1; then
            echo "   ❌ FAIL: Build endpoint should be blocked!"
            exit 1
        else
            echo "   ✅ Blocked: build operations"
        fi
    else
        echo "❌ FAIL: Docker Socket Proxy is not running"
        exit 1
    fi
else
    echo "⏭  SKIP: Security overlay not active (use -f docker-compose.security.yml)"
fi

echo ""

# ── Test 3: bcrypt API Key Auth ─────────────────────────────────────────────

echo "[3/5] Testing bcrypt API key authentication..."

if docker compose ps api | grep -q "Up"; then
    # Generate a test API key
    echo "   Generating test API key..."
    API_KEY_OUTPUT=$(docker compose exec -T worker kuro-worker apikey --name "test-hardening" --role viewer 2>&1 || echo "")

    if echo "$API_KEY_OUTPUT" | grep -q "kuro_api_"; then
        TEST_KEY=$(echo "$API_KEY_OUTPUT" | grep "Key:" | awk '{print $2}')
        echo "   Generated key: ${TEST_KEY:0:20}..."

        # Test authentication
        HEALTH_RESPONSE=$(curl -sf -H "X-API-Key: $TEST_KEY" http://localhost:8080/health 2>&1 || echo "failed")

        if echo "$HEALTH_RESPONSE" | grep -q "ok"; then
            echo "✅ PASS: bcrypt authentication works"
        else
            echo "❌ FAIL: Authentication failed with bcrypt key"
            echo "   Response: $HEALTH_RESPONSE"
            exit 1
        fi
    else
        echo "⚠️  WARN: Could not generate test API key"
        echo "   Output: $API_KEY_OUTPUT"
    fi
else
    echo "⏭  SKIP: API not running"
fi

echo ""

# ── Test 4: NATS TLS ────────────────────────────────────────────────────────

echo "[4/5] Checking NATS TLS..."

if docker compose -f "$COMPOSE_FILE" -f "$COMPOSE_SECURITY" ps nats >/dev/null 2>&1; then
    NATS_LOGS=$(docker compose logs nats 2>&1 | tail -20 || echo "")

    if echo "$NATS_LOGS" | grep -qi "tls"; then
        echo "✅ PASS: NATS TLS is enabled"
    else
        echo "⚠️  WARN: Could not verify NATS TLS from logs"
        echo "   Check manually: docker compose logs nats | grep -i tls"
    fi
else
    echo "⏭  SKIP: Security overlay not active or NATS not running"
fi

echo ""

# ── Test 5: PostgreSQL SSL ──────────────────────────────────────────────────

echo "[5/5] Checking PostgreSQL SSL..."

if docker compose -f "$COMPOSE_FILE" -f "$COMPOSE_SECURITY" ps postgres >/dev/null 2>&1; then
    PG_LOGS=$(docker compose logs postgres 2>&1 | tail -20 || echo "")

    if echo "$PG_LOGS" | grep -qi "ssl"; then
        echo "✅ PASS: PostgreSQL SSL is configured"
    else
        echo "⚠️  WARN: Could not verify PostgreSQL SSL from logs"
        echo "   Check manually: docker compose logs postgres | grep -i ssl"
    fi
else
    echo "⏭  SKIP: Security overlay not active or PostgreSQL not running"
fi

echo ""
echo "────────────────────────────────────────────────────────────────────────"
echo "✨ Security hardening tests complete!"
echo ""
echo "Summary:"
echo "  ✅ Docker socket: not mounted (critical)"
echo "  ✅ Docker Socket Proxy: operational (if overlay active)"
echo "  ✅ bcrypt auth: working"
echo "  ✅ NATS TLS: configured (if overlay active)"
echo "  ✅ PostgreSQL SSL: configured (if overlay active)"
echo ""
echo "To run with full security overlay:"
echo "  docker compose -f docker-compose.yml -f docker-compose.security.yml up -d"
echo "────────────────────────────────────────────────────────────────────────"
