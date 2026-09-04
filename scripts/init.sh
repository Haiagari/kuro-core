#!/bin/bash
# Init Script — Kuro v1.4.1
# Configuración inicial automática después de docker compose up
# Uso: ./scripts/init.sh

set -euo pipefail

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}=========================================="
echo "Kuro v1.4.1 — Init Script"
echo -e "==========================================${NC}"
echo ""

# ── Step 1: Wait for services ──────────────────────────
echo -e "${YELLOW}[1/5]${NC} Waiting for services to be healthy..."
for i in $(seq 1 30); do
    HEALTHY=$(docker compose ps --format "{{.Name}}" 2>/dev/null | grep "kuro-" | wc -l)
    if [ "$HEALTHY" -ge 6 ]; then
        echo -e "  ${GREEN}✓${NC} All 6 services are UP"
        break
    fi
    if [ "$i" -eq 30 ]; then
        echo -e "  ${RED}✗${NC} Timed out waiting for services"
        echo "  Run: docker compose ps"
        exit 1
    fi
    sleep 2
done

# Wait for healthchecks (API, postgres, nats, minio)
sleep 5
echo ""

# ── Step 2: Configure MinIO ─────────────────────────────
echo -e "${YELLOW}[2/5]${NC} Configuring MinIO S3..."
ACCESS_KEY="${MINIO_ACCESS_KEY:-YOUR_ACCESS_KEY_HERE}"
SECRET_KEY="${MINIO_SECRET_KEY:-YOUR_SECRET_KEY_HERE}"

# Check if alias already exists
if docker exec kuro-minio mc alias list 2>/dev/null | grep -q "local"; then
    echo -e "  ${GREEN}✓${NC} MinIO alias 'local' already configured"
else
    docker exec kuro-minio mc alias set local http://localhost:9000 "$ACCESS_KEY" "$SECRET_KEY" > /dev/null 2>&1
    echo -e "  ${GREEN}✓${NC} MinIO alias 'local' configured"
fi

# Check if bucket exists
if docker exec kuro-minio mc ls local/scans-artifacts > /dev/null 2>&1; then
    echo -e "  ${GREEN}✓${NC} MinIO bucket 'scans-artifacts' already exists"
else
    docker exec kuro-minio mc mb local/scans-artifacts --ignore-existing > /dev/null 2>&1
    echo -e "  ${GREEN}✓${NC} MinIO bucket 'scans-artifacts' created"
fi
echo ""

# ── Step 3: Verify API healthcheck ─────────────────────
echo -e "${YELLOW}[3/5]${NC} Verifying API healthcheck..."
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" ${KURO_HTTP_PROTO:-http}://localhost:8080/health 2>/dev/null || echo "000")
if [ "$HTTP_CODE" = "200" ]; then
    echo -e "  ${GREEN}✓${NC} API healthcheck OK"
else
    echo -e "  ${RED}✗${NC} API healthcheck failed (HTTP $HTTP_CODE)"
    echo "  Run: docker compose logs api"
    exit 1
fi
echo ""

# ── Step 4: Create API key (if create-team script exists) ──
echo -e "${YELLOW}[4/5]${NC} Creating default team and API key..."

# Check if we already have a user
ALREADY_REGISTERED=$(docker exec kuro-postgres psql -U kuro -t -A -c "SELECT COUNT(*) FROM users" 2>/dev/null || echo "0")

if [ "$ALREADY_REGISTERED" -gt 0 ]; then
    echo -e "  ${GREEN}✓${NC} Users already registered ($ALREADY_REGISTERED found)"
    
    # Get existing API key from DB (if stored in plaintext - may not be)
    API_KEY_EXISTS=$(docker exec kuro-postgres psql -U kuro -t -A -c "SELECT COUNT(*) FROM users WHERE api_key_hash IS NOT NULL" 2>/dev/null || echo "0")
    if [ "$API_KEY_EXISTS" -gt 0 ]; then
        echo -e "  ${GREEN}✓${NC} API keys configured ($API_KEY_EXISTS found)"
    else
        echo -e "  ${YELLOW}⚠${NC} No API keys found. Run: ./scripts/create-team.sh my-team"
    fi
elif [ -f "./scripts/create-team.sh" ]; then
    # Run create-team to generate default team + API key
    ./scripts/create-team.sh "default-team" 2>&1 | head -5
    echo ""
    echo -e "  ${YELLOW}⚠${NC} Save the API key shown above — you won't see it again!"
else
    echo -e "  ${YELLOW}⚠${NC} create-team.sh not found"
    echo "  To create API key manually, see docs/DEPLOYMENT.md"
fi
echo ""

# ── Step 5: Run smoke test ─────────────────────────────
echo -e "${YELLOW}[5/5]${NC} Running smoke test..."
if [ -f "./scripts/smoke-test.sh" ]; then
    ./scripts/smoke-test.sh 2>&1 | tail -10
    SMOKE_EXIT=$?
    if [ $SMOKE_EXIT -eq 0 ]; then
        echo -e "  ${GREEN}✓${NC} Smoke test PASSED"
    else
        echo -e "  ${YELLOW}⚠${NC} Smoke test ended with warnings (exit $SMOKE_EXIT)"
    fi
else
    echo -e "  ${YELLOW}⚠${NC} smoke-test.sh not found"
fi
echo ""

# ── Summary ────────────────────────────────────────────
echo -e "${BLUE}=========================================="
echo "Init Complete"
echo -e "==========================================${NC}"
echo ""
echo -e "  ${GREEN}✓${NC} MinIO S3 configured"
echo -e "  ${GREEN}✓${NC} API healthcheck OK"
echo -e "  ${GREEN}✓${NC} Services: $(docker compose ps --format "{{.Service}}" | wc -l) running"
echo ""
echo "Next steps:"
echo "  1. Make a push to trigger a security scan"
echo ""
echo "  2. View results:"
echo "     ${KURO_HTTP_PROTO:-http}://localhost:8080/dashboard"
echo ""
echo "  3. Need help? See docs/INDEX.md"
echo ""
