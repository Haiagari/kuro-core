#!/usr/bin/env bash
# ============================================================================
# KURO PIPELINE — Deploy Orchestrator
# ============================================================================
# Orquesta el deploy completo del pipeline desde cero en un solo comando.
#
# Uso:
#   ./scripts/deploy.sh               # Deploy interactivo (pregunta por TLS)
#   ./scripts/deploy.sh --tls         # Con TLS (Let's Encrypt / BYO)
#   ./scripts/deploy.sh --ollama      # Con Ollama para dedup IA
#   ./scripts/deploy.sh --tls --ollama  # Completo
#   ./scripts/deploy.sh --ci          # Sin prompts (para CI/CD)
#   ./scripts/deploy.sh --help        # Esta ayuda
#
# Idempotente: seguro de ejecutar múltiples veces.
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$ROOT_DIR"

# ── Colors ──────────────────────────────────────────────────────────────────
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

ok()    { echo -e "  ${GREEN}✅${NC} $*"; }
warn()  { echo -e "  ${YELLOW}⚠️  ${NC} $*"; }
fail()  { echo -e "  ${RED}❌${NC} $*"; exit 1; }
log()   { echo -e "  ➜  $*"; }
header() {
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo -e "  ${BOLD}$1${NC}"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
}

# ── Parse args ──────────────────────────────────────────────────────────────
SETUP_TLS=false
SETUP_OLLAMA=false
CI_MODE=false

for arg in "$@"; do
    case "$arg" in
        --tls)     SETUP_TLS=true ;;
        --ollama)  SETUP_OLLAMA=true ;;
        --ci)      CI_MODE=true ;;
        --help|-h)
            echo "Uso: $0 [--tls] [--ollama] [--ci]"
            echo ""
            echo "Opciones:"
            echo "  --tls      Configurar TLS (Let's Encrypt / BYO / self-signed)"
            echo "  --ollama   Instalar Ollama + modelo nomic-embed-text"
            echo "  --ci       Non-interactive (sin prompts, defaults)"
            echo "  --help     Esta ayuda"
            echo ""
            echo "Ejemplos:"
            echo "  $0                         # Deploy básico interactivo"
            echo "  $0 --tls                   # Deploy + TLS"
            echo "  $0 --tls --ollama          # Completo"
            echo "  $0 --ci                    # Para CI/CD sin interacción"
            exit 0
            ;;
        *)
            echo -e "${RED}❌ Argumento desconocido: $arg${NC}"
            echo "Uso: $0 [--tls] [--ollama] [--ci]"
            exit 1
            ;;
    esac
done

# ── Welcome ──────────────────────────────────────────────────────────────────
echo ""
echo "╔══════════════════════════════════════════════════════════╗"
echo "║        Kuro v1.4.1 — Deploy Orchestrator        ║"
echo "║        $(date '+%Y-%m-%d %H:%M')"
echo "╚══════════════════════════════════════════════════════════╝"
echo ""

# ─────────────────────────────────────────────────────────────────────────────
# Paso 1: Prerequisites
# ─────────────────────────────────────────────────────────────────────────────
header "1/7 — Prerequisites"

for cmd in docker git openssl curl; do
    if command -v "$cmd" &>/dev/null 2>&1; then
        ok "$cmd disponible"
    else
        fail "$cmd no encontrado — instalalo primero"
    fi
done

docker compose version &>/dev/null || fail "Docker Compose no disponible (docker compose plugin required)"

ok "Todos los prerequisitos OK"

# ─────────────────────────────────────────────────────────────────────────────
# Paso 2: Environment (.env)
# ─────────────────────────────────────────────────────────────────────────────
header "2/7 — Environment"

if [ -f .env ]; then
    ok ".env ya existe ($(wc -l < .env) líneas)"
elif [ -f .env.example ]; then
    log "Generando .env desde .env.example..."
    bash "$SCRIPT_DIR/bootstrap-prod.sh"
    ok ".env generado"
    if [ "$CI_MODE" = false ]; then
        echo ""
        echo -e "  ${YELLOW}⚠️  EDITÁ .env con tus valores de producción:${NC}"
        echo "     code .env"
        echo "     # Al menos: POSTGRES_PASSWORD, KURO_API_SECRET, KURO_WEBHOOK_SECRET"
        echo ""
        echo -e "  ${CYAN}  ¿Continuar? [Enter]${NC}"
        read -r
    else
        warn "CI mode: usando defaults de .env.example (NO para producción)"
    fi
else
    warn "No se encuentra .env.example — saltando generación de .env"
fi

# ─────────────────────────────────────────────────────────────────────────────
# Paso 3: TLS (opcional)
# ─────────────────────────────────────────────────────────────────────────────
header "3/7 — TLS"

if [ "$SETUP_TLS" = true ]; then
    log "Ejecutando setup-tls.sh..."
    bash "$SCRIPT_DIR/setup-tls.sh"
    ok "TLS configurado"
elif [ "$CI_MODE" = false ]; then
    echo ""
    echo -e "  ¿Configurar TLS ahora?"
    echo -e "  ${CYAN}  1)${NC} Let's Encrypt (necesita dominio + puerto 80)"
    echo -e "  ${CYAN}  2)${NC} BYO (tus propios certificados)"
    echo -e "  ${CYAN}  3)${NC} Self-signed (dev/testing)"
    echo -e "  ${CYAN}  s)${NC} Skip — configurar después"
    echo ""
    echo -n "  Elegí [1/2/3/s]: "
    read -r tls_choice
    case "$tls_choice" in
        1) bash "$SCRIPT_DIR/setup-tls.sh" --letsencrypt ;;
        2) bash "$SCRIPT_DIR/setup-tls.sh" --byo ;;
        3) bash "$SCRIPT_DIR/setup-tls.sh" --generate ;;
        *) log "TLS skip — usá bash scripts/setup-tls.sh después" ;;
    esac
    echo ""
else
    log "TLS skip (usá --tls para configurar)"
fi

# ─────────────────────────────────────────────────────────────────────────────
# Paso 4: Docker Compose
# ─────────────────────────────────────────────────────────────────────────────
header "4/7 — Starting Services"

log "Levantando servicios core..."
docker compose up -d postgres nats garage api worker notifier backup 2>&1

log "Esperando que los servicios estén healthy..."
sleep 10
FAILED=0
for svc in postgres nats garage api worker; do
    STATUS=$(docker compose ps --format '{{.Status}}' "$svc" 2>/dev/null || echo "unknown")
    if echo "$STATUS" | grep -qiE "healthy|up|running"; then
        ok "$svc — $STATUS"
    else
        warn "$svc — $STATUS (puede necesitar más tiempo)"
        FAILED=$((FAILED + 1))
    fi
done

if [ "$FAILED" -gt 0 ]; then
    warn "$FAILED servicio(s) no están healthy aún"
    log "docker compose ps para ver estado completo"
fi

# ─────────────────────────────────────────────────────────────────────────────
# Paso 5: Database + Garage buckets
# ─────────────────────────────────────────────────────────────────────────────
header "5/7 — Database & Storage"

if [ -f "$SCRIPT_DIR/init-docker.sh" ]; then
    log "Ejecutando init-docker.sh..."
    bash "$SCRIPT_DIR/init-docker.sh"
    ok "Schema + Garage buckets configurados"
else
    warn "init-docker.sh no encontrado — saltando"
fi

# ─────────────────────────────────────────────────────────────────────────────
# Paso 6: API Key
# ─────────────────────────────────────────────────────────────────────────────
header "6/7 — API Key"

KURO_KEY_DIR="${KURO_KEY_DIR:-$HOME/.kuro}"
mkdir -p "$KURO_KEY_DIR"

API_KEY_OUT=$(bash "$SCRIPT_DIR/bootstrap-api-key.sh 2>&1" || true)
ADMIN_KEY=$(echo "$API_KEY_OUT" | grep -oP 'API Key:\s*\K\S+' | head -1)

if [ -n "$ADMIN_KEY" ]; then
    echo "$ADMIN_KEY" > "$KURO_KEY_DIR/api_key"
    chmod 600 "$KURO_KEY_DIR/api_key"
    ok "API key guardada en $KURO_KEY_DIR/api_key"
else
    warn "No se pudo extraer la API key automáticamente"
    echo "$API_KEY_OUT"
fi

# ─────────────────────────────────────────────────────────────────────────────
# Paso 7: Verification
# ─────────────────────────────────────────────────────────────────────────────
header "7/7 — Verification"

if [ -f "$SCRIPT_DIR/smoke-test.sh" ]; then
    log "Ejecutando smoke test..."
    bash "$SCRIPT_DIR/smoke-test.sh" || warn "Smoke test reportó warnings (revisar arriba)"
else
    warn "smoke-test.sh no encontrado — saltando"
fi

# ── Ollama (optional) ───────────────────────────────────────────────────────
if [ "$SETUP_OLLAMA" = true ]; then
    log "Instalando Ollama..."
    make ollama 2>&1 || warn "Ollama setup tuvo problemas"
fi

# ── Final Summary ───────────────────────────────────────────────────────────
PROTO="${KURO_HTTP_PROTO:-http}"
DASHBOARD_URL="${PROTO}://localhost:8080/dashboard/"

echo ""
echo "╔══════════════════════════════════════════════════════════╗"
echo "║          ✅  Deploy Complete                             ║"
echo "╚══════════════════════════════════════════════════════════╝"
echo ""
echo -e "  ${BOLD}Dashboard:${NC}    $DASHBOARD_URL"
echo -e "  ${BOLD}Garage Admin:${NC}  http://localhost:3903"
echo -e "  ${BOLD}API Key:${NC}       $KURO_KEY_DIR/api_key"
echo ""
echo -e "  ${BOLD}Comandos útiles:${NC}"
echo "    docker compose ps               — estado de servicios"
echo "    docker compose logs -f worker   — logs del worker"
echo "    make test                       — tests E2E"
echo "    bash scripts/smoke-test.sh      — smoke test"
echo ""
echo -e "  ${YELLOW}⚠️  POST-DEPLOY (según necesites):${NC}"
echo "    bash scripts/setup-tls.sh       — configurar TLS si no lo hiciste"
echo "    bash scripts/setup-ollama.sh -f  — agregar Ollama para dedup IA"
echo "    bash scripts/setup-key-rotation.sh — rotación periódica de keys"
echo ""

if [ -n "${ADMIN_KEY:-}" ]; then
    echo -e "  ${BOLD}Probar con un scan:${NC}"
    echo "    curl -X POST ${PROTO}://localhost:8080/scans/trigger \\"
    echo "      -H \"X-API-Key: $ADMIN_KEY\" \\"
    echo '      -H "Content-Type: application/json" \'
    echo '      -d '"'"'{"repository_url":"https://github.com/WebGoat/WebGoat.git"}'"'"''
    echo ""
fi

# ── Save deploy marker ──────────────────────────────────────────────────────
echo "$(date -Iseconds) | deploy complete | proto=$PROTO | tls=$SETUP_TLS | ollama=$SETUP_OLLAMA" >> "$ROOT_DIR/.deploy-history"
