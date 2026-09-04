#!/usr/bin/env bash
#
# Check Docker group GID and update .env if needed
#
# Usage:
#   ./scripts/check-docker-gid.sh

set -euo pipefail

echo "🔍 Checking Docker group GID on this host..."

# Find docker group GID
DOCKER_GID=$(getent group docker 2>/dev/null | cut -d: -f3 || echo "")

if [ -z "$DOCKER_GID" ]; then
    echo "❌ Error: docker group not found"
    echo ""
    echo "Possible causes:"
    echo "  1. Docker not installed"
    echo "  2. User not in docker group (run: sudo usermod -aG docker \$USER)"
    echo "  3. Docker installed via snap (different group name)"
    echo ""
    echo "Install Docker: https://docs.docker.com/engine/install/"
    exit 1
fi

echo "✅ Found docker group with GID: $DOCKER_GID"
echo ""

# Check if .env exists
if [ ! -f .env ]; then
    echo "⚠️  .env file not found"
    echo "   Creating from .env.example..."
    cp .env.example .env
    echo "   ✅ Created .env"
    echo ""
fi

# Check current DOCKER_GID in .env
CURRENT_GID=$(grep "^DOCKER_GID=" .env 2>/dev/null | cut -d= -f2 || echo "")

if [ -z "$CURRENT_GID" ]; then
    echo "📝 Adding DOCKER_GID=$DOCKER_GID to .env"
    echo "" >> .env
    echo "# Docker group GID (auto-detected)" >> .env
    echo "DOCKER_GID=$DOCKER_GID" >> .env
    echo "   ✅ Added DOCKER_GID to .env"
elif [ "$CURRENT_GID" != "$DOCKER_GID" ]; then
    echo "⚠️  DOCKER_GID mismatch!"
    echo "   Current in .env: $CURRENT_GID"
    echo "   Actual on host: $DOCKER_GID"
    echo ""
    read -p "Update .env with correct GID? (y/N) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        sed -i "s/^DOCKER_GID=.*/DOCKER_GID=$DOCKER_GID/" .env
        echo "   ✅ Updated DOCKER_GID in .env"
    fi
else
    echo "✅ DOCKER_GID in .env matches host ($DOCKER_GID)"
fi

echo ""
echo "Next steps:"
echo "  1. Rebuild worker: docker-compose build worker"
echo "  2. Restart:        docker-compose up -d worker"
echo "  3. Verify logs:    docker logs kuro-worker --tail 50"
