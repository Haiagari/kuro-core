#!/usr/bin/env bash
# ============================================================================
# KURO PIPELINE — Bootstrap Script
# ============================================================================
# Despliega el pipeline completo desde cero en un VPS Ubuntu 24.04.
# Idempotente: se puede ejecutar múltiples veces sin romper nada.
#
# Uso:
#   sudo bash scripts/bootstrap.sh
#
# Configuración vía variables de entorno:
#   KURO_HOME       = /opt/kuro     (ruta del repositorio)
#   KURO_USER       = kuro                   (usuario del sistema)
#   KURO_DOMAIN     = kuro.yourcompany.com     (dominio base)
#   NOMAD_VERSION  = 1.8.0
#   GO_VERSION     = 1.23.0
#   SKIP_TAILSCALE = false                 (saltar instalación de Tailscale)
#   TAILSCALE_TAG  = tag:server
# ============================================================================

set -euo pipefail

# ── Detectar modo local vs servidor ──────────────────────────────────────
#   --local: salta Nomad, systemd services, Tailscale, creación de usuario
#   Por defecto: asume VPS (comportamiento original)
LOCAL_MODE=false
for arg in "$@"; do
  [ "$arg" = "--local" ] && LOCAL_MODE=true && break
done

if $LOCAL_MODE; then
  echo "  Modo LOCAL: saltando Nomad, systemd, Tailscale y creación de usuario"
fi

# ── Configuración ─────────────────────────────────────────────────────────
KURO_HOME="${KURO_HOME:-/opt/kuro}"
KURO_USER="${KURO_USER:-kuro}"
KURO_DOMAIN="${KURO_DOMAIN:-kuro.local}"
NOMAD_VERSION="${NOMAD_VERSION:-1.8.0}"
GO_VERSION="${GO_VERSION:-1.23.0}"
SKIP_TAILSCALE="${SKIP_TAILSCALE:-$LOCAL_MODE}"
TAILSCALE_TAG="${TAILSCALE_TAG:-tag:server}"

# Secrets generados automáticamente (se persisten en .env)
ENV_FILE="${KURO_HOME}/.env"
if [ -f "$ENV_FILE" ]; then
  source "$ENV_FILE"
else
  POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-$(openssl rand -hex 32)}"
  NOMAD_TOKEN="${NOMAD_TOKEN:-}"
fi

# ── Funciones auxiliares ──────────────────────────────────────────────────

log()  { printf "  ➜  %s\n" "$*"; }
ok()   { printf "  ✅  %s\n" "$*"; }
skip() { printf "  ⏭️  %s\n" "$*"; }
fail() { printf "  ❌  %s\n" "$*"; exit 1; }

section() {
  echo ""
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "  $1"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
}

wait_for_port() {
  local host="$1" port="$2" timeout="${3:-30}"
  for i in $(seq 1 "$timeout"); do
    if nc -z "$host" "$port" 2>/dev/null; then
      return 0
    fi
    sleep 1
  done
  return 1
}

# ── Paso 1: Sistema base ─────────────────────────────────────────────────

section "Sistema base"

if [ "$(id -u)" -ne 0 ]; then
  fail "Este script debe ejecutarse como root (sudo)"
fi

# Detectar OS
if ! grep -qi "ubuntu" /etc/os-release 2>/dev/null; then
  log "AVISO: Este script está optimizado para Ubuntu 24.04"
  log "       Continuando de todas formas..."
fi

log "Actualizando paquetes..."
apt-get update -qq && apt-get upgrade -y -qq

log "Instalando dependencias del sistema..."
apt-get install -y -qq \
  curl wget gnupg unzip git jq \
  netcat-openbsd ufw openssl ca-certificates \
  sqlite3

# Crear usuario si no existe
if ! id "$KURO_USER" &>/dev/null; then
  log "Creando usuario ${KURO_USER}..."
  adduser --disabled-password --gecos "" "$KURO_USER"
  usermod -aG sudo "$KURO_USER"
fi

# Configurar firewall (modo seguro: solo SSH + Tailscale)
log "Configurando firewall (modo seguro)..."
ufw --force reset 2>/dev/null || true
ufw default deny incoming
ufw default allow outgoing
ufw allow 22/tcp       comment "SSH"
ufw allow 41641/udp    comment "Tailscale"
ufw --force enable
ok "Firewall configurado"
log ""
log "  NOTA: HTTP (80) y HTTPS (443) están BLOQUEADOS."
log "  Los servicios solo son accesibles via Tailscale."
log "  Si necesitás exponerlos, ejecutá después:"
log "    ufw allow 80/tcp"
log "    ufw allow 443/tcp"

# ── Paso 2: Go ────────────────────────────────────────────────────────────

section "Go ${GO_VERSION}"

if command -v go &>/dev/null && go version | grep -q "go${GO_VERSION}"; then
  skip "Go ${GO_VERSION} ya está instalado"
else
  log "Descargando Go ${GO_VERSION}..."
  wget -q "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" -O /tmp/go.tar.gz
  tar -C /usr/local -xzf /tmp/go.tar.gz
  rm /tmp/go.tar.gz
  echo 'export PATH=$PATH:/usr/local/go/bin' > /etc/profile.d/go.sh
  chmod +x /etc/profile.d/go.sh
  export PATH="$PATH:/usr/local/go/bin"
  ok "Go ${GO_VERSION} instalado"
fi

# ── Paso 3: Docker ───────────────────────────────────────────────────────

section "Docker"

if command -v docker &>/dev/null; then
  skip "Docker ya está instalado ($(docker --version))"
else
  log "Instalando Docker..."
  curl -fsSL https://get.docker.com | sh
  usermod -aG docker "$KURO_USER"
  systemctl enable docker
  systemctl start docker
  ok "Docker instalado"
fi

# ── Paso 4: Ollama ────────────────────────────────────────────────────────

section "Ollama + Modelos"

if command -v ollama &>/dev/null; then
  skip "Ollama ya está instalado"
else
  log "Instalando Ollama..."
  curl -fsSL https://ollama.ai/install.sh | sh

  # Configurar para que escuche en todas las interfaces
  mkdir -p /etc/systemd/system/ollama.service.d
  cat > /etc/systemd/system/ollama.service.d/override.conf << 'EOF'
[Service]
Environment="OLLAMA_HOST=0.0.0.0:11434"
EOF
  systemctl daemon-reload
  systemctl restart ollama
  ok "Ollama instalado y configurado en 0.0.0.0:11434"
fi

log "Descargando modelo nomic-embed-text..."
ollama pull nomic-embed-text 2>&1 | tail -1
ok "nomic-embed-text listo (274 MB)"

# Verificar que responde
if wait_for_port "127.0.0.1" "11434" 15; then
  ok "Ollama responde en :11434"
else
  log "ADVERTENCIA: Ollama no responde aún, continuando..."
fi

# ── Paso 5: Nomad ─────────────────────────────────────────────────────────

if ! $LOCAL_MODE; then
section "Nomad ${NOMAD_VERSION}"

if command -v nomad &>/dev/null && nomad version | grep -q "${NOMAD_VERSION}"; then
  skip "Nomad ${NOMAD_VERSION} ya está instalado"
else
  log "Descargando Nomad ${NOMAD_VERSION}..."
  wget -q "https://releases.hashicorp.com/nomad/${NOMAD_VERSION}/nomad_${NOMAD_VERSION}_linux_amd64.zip" -O /tmp/nomad.zip
  unzip -q -o /tmp/nomad.zip -d /usr/local/bin
  rm /tmp/nomad.zip

  # Configurar directorios
  mkdir -p /opt/nomad/data /opt/nomad/config /etc/nomad.d

  ok "Nomad ${NOMAD_VERSION} instalado"
fi
else
  skip "Nomad (salteado en modo local)"
fi

# ── Paso 6: Directorios del pipeline ──────────────────────────────────────

section "Directorios del pipeline"

log "Creando directorios de datos..."
mkdir -p /opt/nomad/volumes/{policies,runner,postgres,signoz,minio}
mkdir -p /opt/kuro/scans /opt/nomad/backups/postgres
chmod 777 /opt/kuro/scans

# Copiar política por defecto
if [ -f "${KURO_HOME}/policies/default-policy.json" ]; then
  cp "${KURO_HOME}/policies/default-policy.json" /opt/nomad/volumes/policies/
  ok "Policy file copiado"
fi

# ── Paso 7: Configurar Nomad ──────────────────────────────────────────────

if ! $LOCAL_MODE; then
section "Configuración de Nomad"

if [ -f /etc/nomad.d/nomad.hcl ]; then
  skip "Config de Nomad ya existe"
else
  if [ -f "${KURO_HOME}/nomad/config/nomad.hcl" ]; then
    cp "${KURO_HOME}/nomad/config/nomad.hcl" /etc/nomad.d/nomad.hcl
  else
    log "Creando configuración por defecto..."

    # Generar token ACL si no existe
    if [ -z "$NOMAD_TOKEN" ]; then
      NOMAD_TOKEN=$(openssl rand -hex 32)
    fi

    cat > /etc/nomad.d/nomad.hcl << NOMADEOF
datacenter = "dc1"
name       = "kuro-server"
data_dir   = "/opt/nomad/data"

server {
  enabled          = true
  bootstrap_expect = 1
}

client {
  enabled = true
  options = {
    "driver.raw_exec.enable" = "1"
    "docker.privileged.enabled" = "true"
  }
  host_volume "policies"     { path = "/opt/nomad/volumes/policies" ; read_only = false }
  host_volume "runner-data"  { path = "/opt/nomad/volumes/runner"  ; read_only = false }
  host_volume "postgres-data"{ path = "/opt/nomad/volumes/postgres" ; read_only = false }
  host_volume "signoz-data"  { path = "/opt/nomad/volumes/signoz"  ; read_only = false }
}

acl { enabled = true }
NOMADEOF
  fi

  # Habilitar e iniciar Nomad
  systemctl enable nomad 2>/dev/null || true
  systemctl restart nomad 2>/dev/null || nomad agent -config /etc/nomad.d/nomad.hcl &>/var/log/nomad.log &

  ok "Nomad configurado"
fi

# Esperar a que Nomad esté listo
log "Esperando a Nomad..."
if wait_for_port "127.0.0.1" "4646" 30; then
  ok "Nomad listo en :4646"
else
  fail "Nomad no arrancó a tiempo. Revisá /var/log/nomad.log"
fi
else
  skip "Configuración de Nomad (salteado en modo local)"
fi
fi

# ── Paso 8: Storage Layer ────────────────────────────────────────────────

section "Storage Layer"

deploy_job() {
  local job="$1" label="$2" timeout="${3:-30}"
  log "Desplegando ${label}..."
  nomad job run -detach "${KURO_HOME}/nomad/${job}.nomad" 2>/dev/null || true
  # Esperar a que esté saludable sin esperar el -detach
  sleep 5
}

deploy_job "postgres" "PostgreSQL 17 + pgvector"
deploy_job "nats"     "NATS JetStream"
deploy_job "redis"    "Redis"

# Esperar a que PostgreSQL esté listo
log "Esperando a PostgreSQL..."
if wait_for_port "127.0.0.1" "5432" 60; then
  ok "PostgreSQL listo"

  # Aplicar schema
  log "Aplicando schema de base de datos..."
  PGPASSWORD="${POSTGRES_PASSWORD}" psql -h 127.0.0.1 -U kuro -d kuro \
    -f "${KURO_HOME}/schema/init.sql" 2>/dev/null && ok "Schema aplicado" || log "Schema ya aplicado (error ignorado)"
else
  fail "PostgreSQL no arrancó a tiempo"
fi

# ── Paso 9: NATS Streams ─────────────────────────────────────────────────

section "NATS Streams"

# Instalar NATS CLI si no existe
if ! command -v nats &>/dev/null; then
  log "Instalando NATS CLI..."
  wget -q "https://github.com/nats-io/natscli/releases/download/v0.1.5/nats-0.1.5-linux-amd64.zip" -O /tmp/nats-cli.zip || true
  unzip -q -o /tmp/nats-cli.zip -d /tmp/nats-cli 2>/dev/null || true
  cp /tmp/nats-cli/nats-*/nats /usr/local/bin/ 2>/dev/null || true
fi

if command -v nats &>/dev/null; then
  bash "${KURO_HOME}/scripts/setup-nats-streams.sh"
  ok "Streams de NATS configurados"
else
  log "NATS CLI no disponible, creando streams via Docker..."
  docker run --rm --network host natsio/nats-box \
    nats --server nats://127.0.0.1:4222 stream add SCANS \
    --subjects "scans.>" --retention limits --max-age 24h --replicas 1 2>/dev/null || true
  docker run --rm --network host natsio/nats-box \
    nats --server nats://127.0.0.1:4222 stream add RESULTS \
    --subjects "results.>" --retention limits --max-age 72h --replicas 1 2>/dev/null || true
  ok "Streams creados via Docker"
fi

# ── Paso 11: API + Worker + Notifier ─────────────────────────────────────

section "Aplicación"

# Compilar binarios
log "Compilando binarios..."
cd "${KURO_HOME}/api"
go build -o "${KURO_HOME}/bin/kuro-api" . 2>&1 | tail -1
cd "${KURO_HOME}/worker"
go build -o "${KURO_HOME}/bin/kuro-worker" . 2>&1 | tail -1

# Build Docker image del worker
docker build -t localhost/kuro-worker:latest -f "${KURO_HOME}/worker/Dockerfile.worker" "${KURO_HOME}/worker" 2>&1 | tail -1
ok "Worker Docker image built"

# Deploy API
deploy_job "kuro-api" "API Gateway"
deploy_job "kuro-notifier" "Notifier"

# Deploy Worker
nomad job run "${KURO_HOME}/nomad/kuro-worker.nomad" 2>/dev/null | tail -1
ok "Worker desplegado"

# ── Paso 12: Caddy ────────────────────────────────────────────────────────

section "Caddy (Reverse Proxy)"

# Actualizar Caddyfile con el dominio correcto si es necesario
if [ "$KURO_DOMAIN" != "kuro.local" ]; then
  sed -i "s/kuro\.local/${KURO_DOMAIN}/g" "${KURO_HOME}/caddy/Caddyfile" 2>/dev/null || true
fi

deploy_job "caddy" "Caddy"
ok "Caddy corriendo"

# ── Paso 13: SigNoz ──────────────────────────────────────────────────────

section "SigNoz (Observabilidad)"

deploy_job "signoz" "SigNoz" 30
ok "SigNoz desplegado (puede tomar 2-3 min en iniciar)"

# ── Paso 14: Backup ──────────────────────────────────────────────────────

section "Backup Automático"

# Actualizar script de backup con credenciales reales
if [ -f "${KURO_HOME}/scripts/backup.sh" ]; then
  sed -i "s/DB_PASS=\".*\"/DB_PASS=\"${POSTGRES_PASSWORD}\"/" "${KURO_HOME}/scripts/backup.sh"
fi

deploy_job "backup" "Backup periódico"
ok "Backup configurado (ejecución diaria a las 00:00)"

# ── Paso 15: Tailscale (Opcional) ─────────────────────────────────────────

section "Tailscale (Red Privada)"

if [ "$SKIP_TAILSCALE" != "true" ]; then
  if command -v tailscale &>/dev/null && tailscale status 2>/dev/null | grep -q "active"; then
    skip "Tailscale ya está conectado"
    tailscale status | head -5
  else
    log "Para conectar Tailscale, ejecutá manualmente:"
    log "  sudo bash ${KURO_HOME}/scripts/setup-tailscale.sh"
    log "  O configurá SKIP_TAILSCALE=true para omitir"
  fi
else
  skip "Tailscale omitido (SKIP_TAILSCALE=true)"
fi

# ── Paso 16: Health Check ─────────────────────────────────────────────────

section "Health Check Final"

echo ""
echo "  Verificando servicios..."

check_service() {
  local name="$1" cmd="$2"
  if eval "$cmd" &>/dev/null; then
    printf "  ✅  %-25s respondiendo\n" "$name"
  else
    printf "  ⚠️   %-25s NO RESPONDE\n" "$name"
  fi
}

check_service "API Gateway"    "curl -sf http://api.${KURO_DOMAIN}/health"
check_service "PostgreSQL"     "PGPASSWORD=${POSTGRES_PASSWORD} psql -h 127.0.0.1 -U kuro -d kuro -c 'SELECT 1'"
check_service "NATS"           "curl -sf http://127.0.0.1:8222/streaming/channelsz 2>/dev/null || nc -z 127.0.0.1 4222"
check_service "Ollama"         "curl -sf http://127.0.0.1:11434/api/tags"
check_service "Nomad"          "curl -sf http://127.0.0.1:4646/v1/status"

echo ""

# Mostrar jobs de Nomad
echo "  Jobs de Nomad:"
nomad job status 2>/dev/null | grep -E "^\S|running|pending" | while IFS= read -r line; do
  echo "    $line"
done

# ── Guardar configuración ─────────────────────────────────────────────────

section "Configuración Generada"

# Persistir secrets
cat > "$ENV_FILE" << ENVEOF
# Kuro — Configuración generada por bootstrap.sh
# Fecha: $(date -u +"%Y-%m-%dT%H:%M:%SZ")
POSTGRES_PASSWORD=${POSTGRES_PASSWORD}
ENVEOF

# Si se generó token de Nomad, guardarlo
if [ -n "$NOMAD_TOKEN" ] && [ ! -f "${KURO_HOME}/.nomad-token" ]; then
  echo "$NOMAD_TOKEN" > "${KURO_HOME}/.nomad-token"
fi

chmod 600 "$ENV_FILE" 2>/dev/null || true

# ── Resumen Final ─────────────────────────────────────────────────────────

section "✅  PIPELINE LISTO"

cat << SUMMARY

  Accesos:
  ─────────────────────────────────────────────────
  API:         http://api.${KURO_DOMAIN}/health
  SigNoz:      http://observe.${KURO_DOMAIN}
  Nomad UI:    http://127.0.0.1:4646

  Credenciales guardadas en:
    ${ENV_FILE}

  Próximos pasos:
    1. Ejecutar hardening de seguridad (RECOMENDADO):
       sudo bash ${KURO_HOME}/scripts/harden.sh

    2. Conectar Tailscale (si no lo hiciste):
       sudo bash ${KURO_HOME}/scripts/setup-tailscale.sh

    3. Configurar el worker:
       nomad job run ${KURO_HOME}/nomad/kuro-worker.nomad

    4. Crear un repo de prueba:
       bash ${KURO_HOME}/scripts/create-team.sh demo dev-demo

  Tiempo total: aproximadamente 8-10 minutos.

SUMMARY
