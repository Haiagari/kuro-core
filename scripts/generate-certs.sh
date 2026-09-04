#!/usr/bin/env bash
# ═══════════════════════════════════════════════════════════════════════════
# Kuro — TLS Certificate Generator
#
# Generates self-signed certificates for internal TLS communication:
# - NATS JetStream (mutual TLS)
# - PostgreSQL (server TLS)
#
# Usage: ./scripts/generate-certs.sh
# ═══════════════════════════════════════════════════════════════════════════

set -euo pipefail

CERTS_DIR="./certs"
DAYS=3650  # 10 years (internal use only)

echo "📜 Generating TLS certificates for Kuro internal services"
echo ""

# Create certs directory
mkdir -p "$CERTS_DIR"

# ── NATS JetStream TLS ────────────────────────────────────

echo "[1/2] Generating NATS TLS certificate..."

openssl req -x509 \
  -newkey rsa:4096 \
  -nodes \
  -keyout "$CERTS_DIR/nats-key.pem" \
  -out "$CERTS_DIR/nats-cert.pem" \
  -days $DAYS \
  -subj "/C=US/ST=State/L=City/O=Kuro/OU=Security/CN=nats" \
  -addext "subjectAltName=DNS:nats,DNS:localhost,IP:127.0.0.1"

chmod 600 "$CERTS_DIR/nats-key.pem"
chmod 644 "$CERTS_DIR/nats-cert.pem"

echo "✅ NATS certificate: $CERTS_DIR/nats-cert.pem"
echo "✅ NATS key: $CERTS_DIR/nats-key.pem"
echo ""

# ── PostgreSQL TLS ─────────────────────────────────────────

echo "[2/2] Generating PostgreSQL TLS certificate..."

openssl req -x509 \
  -newkey rsa:4096 \
  -nodes \
  -keyout "$CERTS_DIR/postgres-key.pem" \
  -out "$CERTS_DIR/postgres-cert.pem" \
  -days $DAYS \
  -subj "/C=US/ST=State/L=City/O=Kuro/OU=Security/CN=postgres" \
  -addext "subjectAltName=DNS:postgres,DNS:localhost,IP:127.0.0.1"

chmod 600 "$CERTS_DIR/postgres-key.pem"
chmod 644 "$CERTS_DIR/postgres-cert.pem"

echo "✅ PostgreSQL certificate: $CERTS_DIR/postgres-cert.pem"
echo "✅ PostgreSQL key: $CERTS_DIR/postgres-key.pem"
echo ""

echo "────────────────────────────────────────────────────"
echo "✨ All certificates generated successfully!"
echo ""
echo "Next steps:"
echo "  1. Start Kuro with security overlay:"
echo "     docker compose -f docker-compose.yml -f docker-compose.security.yml up -d"
echo ""
echo "  2. Update connection strings to use TLS:"
echo "     - NATS: nats://nats:4222?tls=true"
echo "     - PostgreSQL: ?sslmode=require"
echo "────────────────────────────────────────────────────"
