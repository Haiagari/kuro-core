#!/bin/bash
# Kuro — Installer
# Uso: curl -sSL https://get.kuro.dev | bash
# O:   bash <(curl -sSL https://get.kuro.dev)
#
# No requiere flags. No hace preguntas. En 3 minutos tenés Kuro corriendo.

set -euo pipefail

# ── Colors ──────────────────────────────────────────────
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

echo -e "${BLUE}=========================================="
echo "  Kuro v1.4.1 — Installer"
echo -e "==========================================${NC}"
echo ""

# ── Step 1: Prerequisites ───────────────────────────────
echo -e "${YELLOW}[1/5]${NC} Verificando prerequisitos..."

# Docker
if ! command -v docker &>/dev/null; then
    echo -e "${RED}✗ Docker no está instalado${NC}"
    echo "  Instalalo: https://docs.docker.com/engine/install/"
    exit 1
fi

if ! docker info &>/dev/null; then
    echo -e "${RED}✗ Docker no está corriendo${NC}"
    echo "  Ejecutá: systemctl start docker (o el comando de tu OS)"
    exit 1
fi
echo -e "  ${GREEN}✓${NC} Docker $(docker --version | cut -d' ' -f3 | tr -d ',')"

# Docker Compose
if ! docker compose version &>/dev/null; then
    echo -e "${RED}✗ Docker Compose no está disponible${NC}"
    echo "  Actualizá Docker a una versión más reciente que incluya Compose v2"
    exit 1
fi
echo -e "  ${GREEN}✓${NC} Docker Compose $(docker compose version --short)"

# Git
if ! command -v git &>/dev/null; then
    echo -e "${RED}✗ Git no está instalado${NC}"
    exit 1
fi
echo -e "  ${GREEN}✓${NC} Git $(git --version | cut -d' ' -f3)"

# Puertos
REQUIRED_PORTS=(8080 4222 5432 9000)
BLOCKED=0
for port in "${REQUIRED_PORTS[@]}"; do
    if ss -tlnp "sport = :$port" 2>/dev/null | grep -q ":$port"; then
        echo -e "  ${RED}✗ Puerto $port está en uso${NC}"
        BLOCKED=1
    fi
done

if [ "$BLOCKED" = "1" ]; then
    echo -e "${RED}Liberá los puertos marcados y volvé a ejecutar el instalador${NC}"
    exit 1
fi
echo -e "  ${GREEN}✓${NC} Puertos 8080, 4222, 5432, 9000 disponibles"
echo ""

# ── Step 2: Descargar archivos ──────────────────────────
echo -e "${YELLOW}[2/5]${NC} Preparando archivos..."

REPO_DIR="${KURO_DIR:-$HOME/kuro-pipeline}"
if [ ! -d "$REPO_DIR" ]; then
    # Clonar el repo
    if command -v gh &>/dev/null && gh auth status &>/dev/null 2>&1; then
        gh repo clone Haiagari/kuro "$REPO_DIR"
    else
        git clone --depth 1 https://github.com/Haiagari/kuro.git "$REPO_DIR"
    fi
    echo -e "  ${GREEN}✓${NC} Repositorio clonado en $REPO_DIR"
else
    echo -e "  ${GREEN}✓${NC} Directorio $REPO_DIR ya existe"
fi

cd "$REPO_DIR"

# ── Step 3: Configuración ───────────────────────────────
echo -e "${YELLOW}[3/5]${NC} Configurando entorno..."

if [ ! -f .env ]; then
    # Generar secrets por defecto
    {
        echo "# Kuro — Environment"
        echo "# Generado por install.sh el $(date +%Y-%m-%d)"
        echo ""
        echo "# PostgreSQL"
        echo "DB_PASSWORD=kuro_$(openssl rand -hex 12)"
        echo ""
        echo "# Webhook"
        echo "WEBHOOK_SECRET=whs_$(openssl rand -hex 16)"
        echo ""
        echo "# JWT"
        echo "JWT_SECRET=jwt_$(openssl rand -hex 24)"
        echo ""
        echo "# MinIO"
        echo "S3_ACCESS_KEY=kuro_$(openssl rand -hex 8)"
        echo "S3_SECRET_KEY=kuro_$(openssl rand -hex 24)"
    } > .env
    echo -e "  ${GREEN}✓${NC} .env creado con secrets aleatorios"
else
    echo -e "  ${GREEN}✓${NC} .env ya existe"
fi

# Aplicar migraciones de schema si no están aplicadas
if [ -f docs/schema/migrations/001_nullable_commit_sha.sql ]; then
    echo -e "  ${GREEN}✓${NC} Migraciones listas (se aplican al levantar Postgres)"
fi
echo ""

# ── Step 4: Levantar servicios ──────────────────────────
echo -e "${YELLOW}[4/5]${NC} Iniciando servicios..."
echo "  (Esto puede tomar 30-60 segundos la primera vez)"

docker compose up -d --wait 2>&1 | while IFS= read -r line; do
    echo "  $line"
done

# Verificar health de cada servicio
SERVICES=("kuro-postgres" "kuro-nats" "kuro-minio" "kuro-api")
ALL_HEALTHY=true
for svc in "${SERVICES[@]}"; do
    if docker ps --filter "name=$svc" --filter "health=healthy" --format "{{.Names}}" | grep -q "$svc"; then
        echo -e "  ${GREEN}✓${NC} $svc: healthy"
    else
        echo -e "  ${YELLOW}⚠${NC} $svc: revisar estado (puede estar arrancando)"
        ALL_HEALTHY=false
    fi
done

# Esperar a que todo esté healthy si no lo está ya
if [ "$ALL_HEALTHY" = false ]; then
    echo ""
    echo "  Esperando que todos los servicios estén healthy..."
    sleep 10
    docker compose ps 2>/dev/null | tail -n +2 | while IFS= read -r line; do
        echo "  $line"
    done
fi
echo ""

# ── Step 5: Bootstrap API Key ──────────────────────────
echo -e "${YELLOW}[5/5]${NC} Generando API key de administrador..."

# Esperar a que Postgres esté listo
for i in $(seq 1 30); do
    if docker exec kuro-postgres pg_isready -U kuro -d kuro &>/dev/null; then
        break
    fi
    sleep 1
done

# Ejecutar bootstrap
API_KEY=$(docker exec kuro-postgres psql -U kuro -d kuro -t -A -c "
    SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'api_keys');
" 2>/dev/null || echo "false")

if [ "$API_KEY" = "false" ]; then
    echo -e "  ${YELLOW}⚠${NC} Tabla api_keys no encontrada. Ejecutando schema..."
    docker compose exec -T postgres psql -U kuro -d kuro -f /docker-entrypoint-initdb.d/init.sql 2>/dev/null || true
fi

# Generar API key
ADMIN_KEY="kuro_live_$(openssl rand -hex 24)"
KEY_HASH=$(echo -n "$ADMIN_KEY" | openssl dgst -sha256 | sed 's/^.* //')

docker exec kuro-postgres psql -U kuro -d kuro -c "
    INSERT INTO api_keys (key_hash, name, role, is_active)
    VALUES ('$KEY_HASH', 'admin-install', 'admin', true)
    ON CONFLICT (key_hash) DO NOTHING;
" > /dev/null 2>&1

echo ""
echo -e "${GREEN}=========================================="
echo "  ✅  Kuro está listo"
echo -e "==========================================${NC}"
echo ""
echo -e "  ${CYAN}Dashboard:${NC}  http://localhost:8080/dashboard"
echo -e "  ${CYAN}API Key:${NC}     $ADMIN_KEY"
echo -e "  ${CYAN}CLI:${NC}         kuro scan <repo-url>"
echo ""
echo -e "  ${YELLOW}Para configurar el CLI:${NC}"
echo "    kuro auth $ADMIN_KEY"
echo "    kuro scan https://github.com/tu-organizacion/tu-repo"
echo ""
echo -e "  ${YELLOW}Para ver servicios:${NC}"
echo "    docker compose ps"
echo ""
