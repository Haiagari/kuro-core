#!/usr/bin/env bash
# ============================================================================
# ⚠️  LEGACY — Este script usa docker run (no Docker Compose).
# ⚠️  Para nuevos equipos, usar MAKE SETUP (docker compose):
# ⚠️    git clone <repo> && cd kuro
# ⚠️    cp .env.example .env && make setup
# ⚠️
# Este script se mantiene para referencia histórica y demos avanzadas
# que requieren SigNoz. No es el path recomendado.
# ============================================================================
# Uso:   bash scripts/setup.sh
# Requiere: Docker, Go 1.23+, curl, git, openssl (S3: Garage)
# Efecto: Deja el pipeline corriendo con datos de demo en ~5 minutos.
# Idempotente: Se puede ejecutar múltiples veces, recrea todo desde cero.
# ============================================================================
#
# DESIGN DECISIONS (v1.4.1)
# ============================================================================
# 1. Garage S3:
#    - Uses --entrypoint /garage with explicit config at /tmp/garage-config.toml.
#    - rpc_secret is randomly generated with openssl rand.
#    - Data dirs at /tmp/garage-data/ (writable inside container — NOT /var/lib/garage).
#    - Cluster layout configured via CLI (not admin API v0/ endpoints, which are
#      deprecated): layout assign -z dc1 -c 1000000000 $NODE_ID then apply --version 1.
#    - Keys and buckets created via /garage CLI, not admin API.
#    - Health check uses admin API Bearer token (not v0/status directly).
#
# 2. NATS:
#    - Runs docker rm -f kuro-nats nats-* before starting to kill any conflicting
#      containers from Nomad or previous manual runs (wildcard nats-* catches all).
#
# 3. Scanner images:
#    - Pre-pulled during setup (semgrep, trivy, gitleaks, checkov) in background
#      jobs to avoid pull latency during scan execution (saves ~2min per scan).
#    - Trivy vulnerability DB also pre-cached with --download-db-only.
#
# 4. API key registration:
#    - Uses openssl dgst -sha256 (not PostgreSQL encode(sha256(...), 'hex')).
#    - INSERT uses ON CONFLICT (key_hash) DO NOTHING for idempotency.
#    - Schema column is 'revoked' (changed from 'active' in earlier versions).
#
# 5. Dashboard:
#    - Served via Go embed.FS (not http.Dir, which broke when CWD changed).
#    - Binary embeds dashboard/index.html at compile time.
# ============================================================================
set -euo pipefail

# ── Configuración ───────────────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
KURO_HOME="$SCRIPT_DIR"

# Colores
BOLD='\033[1m'; DIM='\033[2m'; GREEN='\033[0;32m'
YELLOW='\033[0;33m'; RED='\033[0;31m'; CYAN='\033[0;36m'; NC='\033[0m'

# ── Funciones ───────────────────────────────────────────────────────────────
log()    { printf "  ${DIM}➜${NC}  %s\n" "$*"; }
ok()     { printf "  ${GREEN}✅${NC}  %s\n" "$*"; }
warn()   { printf "  ${YELLOW}⚠${NC}  %s\n" "$*"; }
fail()   { printf "  ${RED}❌${NC}  %s\n" "$*"; exit 1; }
header() { echo ""; echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"; printf "  ${BOLD}%s${NC}\n" "$1"; echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"; echo ""; }
check()  { local n="$1" c="$2"; if eval "$c" &>/dev/null; then printf "  ${GREEN}✅${NC}  %-20s respondiendo\n" "$n"; else printf "  ${RED}❌${NC}  %-20s NO RESPONDE\n" "$n"; fi; }

get_ip() { docker inspect "$1" --format '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' 2>/dev/null | tr -d '[:space:]' || echo ""; }

wait_for() {
  local host="$1" port="$2" timeout="${3:-30}" method="${4:-tcp}"
  for i in $(seq 1 "$timeout"); do
    if [ "$method" = "http" ]; then
      curl -sf "http://${host}:${port}" &>/dev/null && return 0
    elif [ "$method" = "pg" ]; then
      docker exec kuro-postgres pg_isready -U kuro &>/dev/null && return 0
    else
      nc -z "$host" "$port" 2>/dev/null && return 0
    fi 2>/dev/null || true
    sleep 1
  done
  return 1
}

get_container_env() {
  docker inspect "$1" --format '{{range .Config.Env}}{{println .}}{{end}}' 2>/dev/null | grep "^$2=" | cut -d= -f2 || echo ""
}

cleanup() {
  log "Limpiando contenedores existentes..."
  docker rm -f kuro-postgres kuro-nats kuro-garage kuro-signoz-query 2>/dev/null || true
  docker rm -f kuro-api kuro-worker kuro-notifier 2>/dev/null || true
  pkill -f "bin/kuro-api|bin/kuro-worker|bin/kuro-notifier" 2>/dev/null || true
  rm -f "${KURO_HOME}/.api-pid" "${KURO_HOME}/.worker-pid" "${KURO_HOME}/.notifier-pid"
}

# =============================================================================
# Paso 1: Prerrequisitos
# =============================================================================
header "Paso 1/8: Prerrequisitos"

for cmd in docker go git curl openssl; do
  command -v "$cmd" &>/dev/null || fail "'$cmd' no está instalado"
done
ok "Docker $(docker --version | grep -oP '\d+\.\d+\.\d+' | head -1)"
ok "Go $(go version | grep -oP 'go\S+')"
ok "git, curl, openssl"

# ────────────────────────────────────────────────────────────────────────────
# Paso 2: Limpiar
# ────────────────────────────────────────────────────────────────────────────
header "Paso 2/8: Limpiando entorno"
cleanup
sleep 2
ok "Entorno limpio"

# ────────────────────────────────────────────────────────────────────────────
# Paso 3: Redes Docker
# ────────────────────────────────────────────────────────────────────────────
header "Paso 3/8: Redes Docker"

docker network rm kuro-public kuro-private kuro-data 2>/dev/null || true
docker network create kuro-public 2>/dev/null || true
docker network create kuro-private 2>/dev/null || true
docker network create kuro-data 2>/dev/null || true
ok "Redes: kuro-public, kuro-private, kuro-data"

# ────────────────────────────────────────────────────────────────────────────
# Paso 4: Iniciar servicios core
# ────────────────────────────────────────────────────────────────────────────
header "Paso 4/8: Servicios core"

POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-kuro_pg_$(openssl rand -hex 16)}"
# S3 fallback credentials (Garage admin API genera las reales; estos se usan si falla)
MINIO_ROOT_USER="${MINIO_ROOT_USER:-kuro}"
MINIO_ROOT_PASSWORD="${MINIO_ROOT_PASSWORD:-kurominio2026!}"

log "Iniciando PostgreSQL..."
docker run -d --name kuro-postgres \
  --network kuro-data \
  --restart unless-stopped \
  -e POSTGRES_USER=kuro \
  -e POSTGRES_PASSWORD="$POSTGRES_PASSWORD" \
  -e POSTGRES_DB=kuro \
  pgvector/pgvector:pg17 2>/dev/null || true

log "Iniciando NATS..."
# Kill any conflicting NATS containers first
docker rm -f kuro-nats nats-* 2>/dev/null || true
docker run -d --name kuro-nats \
  --network kuro-private \
  --restart unless-stopped \
  -p 4222:4222 \
  nats:2.10-alpine -js -sd /data -m 8222 2>/dev/null || true

log "Iniciando Garage S3..."
GARAGE_RPC_SECRET=$(openssl rand -hex 16)
rm -rf /tmp/garage-data; mkdir -p /tmp/garage-data/meta /tmp/garage-data/data
cat > /tmp/garage-config.toml << GARAGECONF
metadata_dir = "/tmp/garage-data/meta"
data_dir = "/tmp/garage-data/data"
db_engine = "lmdb"
block_size = 1048576
replication_mode = "none"
compression_level = 1
rpc_bind_addr = "[::]:3901"
rpc_secret = "$GARAGE_RPC_SECRET"
[s3_api]
s3_region = "us-east-1"
api_bind_addr = "[::]:3900"
root_domain = ".s3.kuro.local"
[admin]
api_bind_addr = "[::]:3903"
admin_token = "kuro-garage-admin-token"
metrics_token = "kuro-metrics-token"
GARAGECONF

docker run -d --name kuro-garage \
  --network kuro-data \
  --restart unless-stopped \
  -p 3900:3900 -p 3903:3903 \
  -v /tmp/garage-config.toml:/tmp/garage-config.toml:ro \
  --entrypoint /garage \
  dxflrs/garage:v1.0.1 \
  -c /tmp/garage-config.toml server 2>/dev/null || true

log "Iniciando SigNoz (tracing) — OTLP :4318, UI :3301..."
docker compose up -d signoz signoz-otel-collector 2>/dev/null || warn "SigNoz no arrancó (tracing será opcional)"

sleep 5
ok "Contenedores iniciados"

# ────────────────────────────────────────────────────────────────────────────
# Paso 5: Configurar servicios core
# ────────────────────────────────────────────────────────────────────────────
header "Paso 5/8: Configuración de servicios"

# ─── PostgreSQL ───
log "Esperando PostgreSQL..."
wait_for "" "" 30 "pg" || fail "PostgreSQL no arrancó"
ok "PostgreSQL listo"

# Agregar regla trust en pg_hba.conf para conexiones desde la red Docker
log "Configurando pg_hba.conf..."
POSTGRES_NET=$(docker inspect kuro-postgres --format '{{range $k, $v := .NetworkSettings.Networks}}{{(index $v.IPAddress)}}{{end}}' 2>/dev/null | cut -d. -f1-2)
if [ -n "$POSTGRES_NET" ]; then
  docker exec kuro-postgres bash -c "
    sed -i 's|host all all all scram-sha-256|host all all ${POSTGRES_NET}.0.0/16 trust\nhost all all all scram-sha-256|' /var/lib/postgresql/data/pg_hba.conf
    psql -U kuro -d kuro -c 'SELECT pg_reload_conf();' 2>/dev/null
  " 2>/dev/null || true
  ok "pg_hba.conf: trust para red ${POSTGRES_NET}.0.0/16"
fi

# Aplicar schema
if [ -f "${KURO_HOME}/schema/init.sql" ]; then
  log "Aplicando schema..."
  docker exec -i kuro-postgres psql -U kuro -d kuro < "${KURO_HOME}/schema/init.sql" 2>/dev/null || warn "Schema ya aplicado"
  # También crear tabla api_keys (si no está en init.sql)
  docker exec kuro-postgres psql -U kuro -d kuro -c "
    CREATE TABLE IF NOT EXISTS api_keys (
      id SERIAL PRIMARY KEY,
      key_hash TEXT NOT NULL UNIQUE,
      name TEXT NOT NULL,
      role TEXT NOT NULL DEFAULT 'viewer',
      active BOOLEAN DEFAULT true,
      created_at TIMESTAMPTZ DEFAULT now()
    );
  " 2>/dev/null || true
  ok "Schema aplicado"
fi

# ─── NATS: Crear streams (Go program) ───
log "Configurando NATS streams..."
NATS_IP=$(get_ip kuro-nats)

cat > /tmp/create-nats-streams.go << 'GOEOF'
package main

import (
	"fmt"
	"os"
	"github.com/nats-io/nats.go"
)

func main() {
	addr := os.Args[1]
	nc, err := nats.Connect(addr)
	if err != nil {
		fmt.Println("CONNECT_ERROR:", err)
		os.Exit(1)
	}
	defer nc.Close()

	js, err := nc.JetStream()
	if err != nil {
		fmt.Println("JS_ERROR:", err)
		os.Exit(1)
	}

	for _, cfg := range []*nats.StreamConfig{
		{Name: "scans", Subjects: []string{"scans.>"}, Retention: nats.LimitsPolicy, MaxAge: 86400000000000, Storage: nats.FileStorage, Replicas: 1},
		{Name: "results", Subjects: []string{"results.>"}, Retention: nats.LimitsPolicy, MaxAge: 259200000000000, Storage: nats.FileStorage, Replicas: 1},
	} {
		if existing, err := js.StreamInfo(cfg.Name); err == nil {
			fmt.Printf("EXISTS: %s (%d msgs)\n", cfg.Name, existing.State.Msgs)
		} else if _, err := js.AddStream(cfg); err != nil {
			fmt.Printf("ERROR: %s - %v\n", cfg.Name, err)
		} else {
			fmt.Printf("CREATED: %s\n", cfg.Name)
		}
	}
}
GOEOF

mkdir -p /tmp/nats-streams && cd /tmp/nats-streams
cp /tmp/create-nats-streams.go main.go 2>/dev/null || true
if [ ! -f go.mod ]; then go mod init nats-streams 2>/dev/null; fi
go mod tidy 2>&1 | tail -1 || true
NATS_STREAMS_OUT=$(go run main.go "nats://${NATS_IP}:4222" 2>&1) || NATS_STREAMS_OUT="FAILED"
cd "$KURO_HOME"
echo "$NATS_STREAMS_OUT" | while IFS= read -r line; do log "  NATS: $line"; done
ok "NATS streams configurados"

# ─── Pre-cachear imágenes Docker de scanners ───
log "Pre-cacheando imágenes Docker de scanners (ahorra ~2min por scan)..."
for img in "semgrep/semgrep:1.165.0" "aquasec/trivy:0.57.0" "zricethezav/gitleaks:v8.30.1" "bridgecrew/checkov:3.2.400" "trufflesecurity/trufflehog:3.81.0" "anchore/syft:v1.45.1" "anchore/grype:v0.114.0"; do
  docker pull -q "$img" 2>/dev/null &
done
wait
ok "Imágenes de scanners cacheadas"

# ─── Garage S3: configurar cluster + bucket ───
log "Configurando Garage S3..."
sleep 6  # Wait for Garage to start

# Get node ID and configure layout
NODE_ID=$(docker exec kuro-garage /garage -c /tmp/garage-config.toml status 2>&1 | grep "HEALTHY" -A5 | grep -oP '^\w+' || echo "")
if [ -n "$NODE_ID" ]; then
  docker exec kuro-garage /garage -c /tmp/garage-config.toml layout assign -z dc1 -c 1000000000 $NODE_ID 2>/dev/null || true
  docker exec kuro-garage /garage -c /tmp/garage-config.toml layout apply --version 1 2>/dev/null || true
  sleep 2

  # Create key and bucket
  KEY_OUT=$(docker exec kuro-garage /garage -c /tmp/garage-config.toml key create kuro-worker 2>&1 || echo "")
  S3_ACCESS_KEY=$(echo "$KEY_OUT" | grep "Key ID:" | awk '{print $3}')
  S3_SECRET_KEY=$(echo "$KEY_OUT" | grep "Secret key:" | awk '{print $3}')

  docker exec kuro-garage /garage -c /tmp/garage-config.toml bucket create scans-artifacts 2>/dev/null || true
  docker exec kuro-garage /garage -c /tmp/garage-config.toml bucket allow --read --write --owner scans-artifacts --key kuro-worker 2>/dev/null || true
  ok "Garage S3 configurado (key: ${S3_ACCESS_KEY})"
else
  warn "Garage no disponible, usando credenciales fallback"
  S3_ACCESS_KEY="${MINIO_ROOT_USER:-kuro}"
  S3_SECRET_KEY="${MINIO_ROOT_PASSWORD:-kurominio2026!}"
fi

# Pre-cachear Trivy DB para evitar descarga en cada scan
log "Pre-cacheando Trivy vulnerability DB..."
docker run --rm aquasec/trivy:0.57.0 image --download-db-only --quiet 2>/dev/null || true
ok "Trivy DB cacheada"

# ────────────────────────────────────────────────────────────────────────────
# Paso 6: Build
# ────────────────────────────────────────────────────────────────────────────
header "Paso 6/8: Compilación"

VERSION="1.4.1"
COMMIT=$(git -C "${KURO_HOME}" log -1 --format=%h 2>/dev/null || echo "unknown")
DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

log "Compilando API..."
LDFLAGS="-X kuro/api/version.Version=$VERSION -X kuro/api/version.Commit=$COMMIT -X kuro/api/version.Date=$DATE"
cd "${KURO_HOME}/api" && go build -ldflags "$LDFLAGS" -o "${KURO_HOME}/bin/kuro-api" . 2>&1 | tail -1
ok "API compilada"

log "Compilando Worker..."
LDFLAGS="-X kuro/worker/version.Version=$VERSION -X kuro/worker/version.Commit=$COMMIT -X kuro/worker/version.Date=$DATE"
cd "${KURO_HOME}/worker" && go build -ldflags "$LDFLAGS" -o "${KURO_HOME}/bin/kuro-worker" . 2>&1 | tail -1
ok "Worker compilado"

log "Compilando Notifier..."
LDFLAGS="-X kuro/notifications/version.Version=$VERSION -X kuro/notifications/version.Commit=$COMMIT -X kuro/notifications/version.Date=$DATE"
cd "${KURO_HOME}/notifications" && go build -ldflags "$LDFLAGS" -o "${KURO_HOME}/bin/kuro-notifier" . 2>&1 | tail -1 || warn "Notifier opcional no compilado"
ok "Notifier compilado"

docker build -t localhost/kuro-worker:latest \
  -f "${KURO_HOME}/worker/Dockerfile.worker" \
  "${KURO_HOME}/worker" 2>&1 | tail -1 || warn "Docker image del worker no construida (solo necesaria para Nomad)"
ok "Builds completados"

# ────────────────────────────────────────────────────────────────────────────
# Paso 7: Iniciar API + Worker + Notifier
# ────────────────────────────────────────────────────────────────────────────
header "Paso 7/8: Iniciar pipeline"

NATS_IP=$(get_ip kuro-nats)
POSTGRES_IP=$(get_ip kuro-postgres)
GARAGE_IP=$(get_ip kuro-garage)
MINIO_IP="${GARAGE_IP}"  # backward compat

# Obtener password de PostgreSQL desde el container
PGPASS=$(get_container_env kuro-postgres POSTGRES_PASSWORD)
[ -z "$PGPASS" ] && PGPASS="$POSTGRES_PASSWORD"

# Generar API key
ADMIN_API_KEY="kuro_api_$(openssl rand -hex 32)"
echo "$ADMIN_API_KEY" > "${KURO_HOME}/.admin-key"
chmod 600 "${KURO_HOME}/.admin-key"
mkdir -p "${KURO_HOME}/logs"

# ─── API ───
log "Iniciando API..."
# Use 127.0.0.1 for DB connection (Docker port mapping works via localhost)
export DB_URL=...://kuro:${PGPASS}@127.0.0.1:5432/kuro?sslmode=disable"
export NATS_URL="nats://${NATS_IP}:4222"
export OTEL_EXPORTER_OTLP_ENDPOINT="${OTEL_EXPORTER_OTLP_ENDPOINT:-localhost:4318}"
export OTEL_SERVICE_NAME="kuro-api"
export OTEL_DEPLOYMENT_ENVIRONMENT="${OTEL_DEPLOYMENT_ENVIRONMENT:-development}"
export TLS_CERT_FILE="${TLS_CERT_FILE:-}"
export TLS_KEY_FILE="${TLS_KEY_FILE:-}"
nohup "${KURO_HOME}/bin/kuro-api" > "${KURO_HOME}/logs/api.log" 2>&1 &
API_PID=$!
echo "$API_PID" > "${KURO_HOME}/.api-pid"
ok "API iniciada (PID $API_PID)"

for i in $(seq 1 20); do
  if curl -sf ${KURO_HTTP_PROTO:-http}://localhost:8080/health &>/dev/null; then
    ok "API respondiendo en ${KURO_HTTP_PROTO:-http}://localhost:8080"
    break
  fi
  sleep 1
done

# ─── TLS (optional) ───
if [ ! -f "${KURO_HOME}/certs/server.crt" ] && [ -n "${GENERATE_TLS:-}" ]; then
    log "Generando certificados TLS..."
    bash "${KURO_HOME}/scripts/generate-tls-certs.sh"
    export TLS_CERT_FILE="${KURO_HOME}/certs/server.crt"
    export TLS_KEY_FILE="${KURO_HOME}/certs/server.key"
    ok "TLS configurado"
fi

# ─── Worker ───
log "Iniciando Worker..."
export DB_URL=...://kuro:${PGPASS}@127.0.0.1:5432/kuro?sslmode=disable"
export NATS_URL="nats://${NATS_IP}:4222"
export S3_ENDPOINT="${S3_ENDPOINT:-localhost:3900}"
export S3_ACCESS_KEY="${S3_ACCESS_KEY:-$MINIO_ROOT_USER}"
export S3_SECRET_KEY="${S3_SECRET_KEY:-$MINIO_ROOT_PASSWORD}"
export S3_BUCKET="${S3_BUCKET:-scans-artifacts}"
export S3_REGION="${S3_REGION:-us-east-1}"
# MinIO fallback (legacy)
export MINIO_ENDPOINT="${MINIO_ENDPOINT:-}"
export MINIO_ACCESS_KEY="${MINIO_ACCESS_KEY:-$MINIO_ROOT_USER}"
export MINIO_SECRET_KEY="${MINIO_SECRET_KEY:-$MINIO_ROOT_PASSWORD}"
export POLICY_FILE_PATH="${KURO_HOME}/policies/default-policy.json"
export KURO_HOME="/home/sam/Imágenes/kuro"
export OTEL_EXPORTER_OTLP_ENDPOINT="${OTEL_EXPORTER_OTLP_ENDPOINT:-localhost:4318}"
export OTEL_SERVICE_NAME="kuro-worker"
export OTEL_DEPLOYMENT_ENVIRONMENT="${OTEL_DEPLOYMENT_ENVIRONMENT:-development}"
nohup "${KURO_HOME}/bin/kuro-worker" > "${KURO_HOME}/logs/worker.log" 2>&1 &
WORKER_PID=$!
echo "$WORKER_PID" > "${KURO_HOME}/.worker-pid"
ok "Worker iniciado (PID $WORKER_PID)"

sleep 3
if tail -3 "${KURO_HOME}/logs/worker.log" 2>/dev/null | grep -q "Waiting for tasks"; then
  ok "Worker listo, esperando tareas..."
fi

# ─── Notifier ───
if [ -f "${KURO_HOME}/bin/kuro-notifier" ]; then
  log "Iniciando Notifier..."
  export NATS_URL="nats://${NATS_IP}:4222"
  nohup "${KURO_HOME}/bin/kuro-notifier" > "${KURO_HOME}/logs/notifier.log" 2>&1 &
  echo $! > "${KURO_HOME}/.notifier-pid"
  ok "Notifier iniciado"
fi

# ────────────────────────────────────────────────────────────────────────────
# Paso 8: Datos de demo
# ────────────────────────────────────────────────────────────────────────────
header "Paso 8/8: Datos de demo"

# Registrar API key admin en BD
log "Registrando API key de admin..."
ADMIN_HASH=$(echo -n "${ADMIN_API_KEY}" | openssl dgst -sha256 | sed 's/.* //')
docker exec kuro-postgres psql -U kuro -d kuro -c "
  INSERT INTO api_keys (key_hash, name, role)
  VALUES ('${ADMIN_HASH}', 'admin', 'admin')
  ON CONFLICT (key_hash) DO NOTHING;
" 2>/dev/null || warn "No se pudo registrar API key"
ok "API key admin registrada"

# Guardar configuración
cat > "${KURO_HOME}/.setup-env" << ENVEOF
# Kuro — Configuración generada por setup.sh
# Fecha: $(date -u +"%Y-%m-%dT%H:%M:%SZ")
POSTGRES_PASSWORD=${PGPASS}
ADMIN_API_KEY=${ADMIN_API_KEY}
MINIO_ROOT_USER=${MINIO_ROOT_USER}
MINIO_ROOT_PASSWORD=${MINIO_ROOT_PASSWORD}
S3_ENDPOINT=${S3_ENDPOINT:-localhost:3900}
S3_ACCESS_KEY=${S3_ACCESS_KEY:-$MINIO_ROOT_USER}
S3_SECRET_KEY=${S3_SECRET_KEY:-$MINIO_ROOT_PASSWORD}
S3_BUCKET=${S3_BUCKET:-scans-artifacts}
S3_REGION=${S3_REGION:-us-east-1}
API_PID=${API_PID}
WORKER_PID=${WORKER_PID}
ENVEOF
chmod 600 "${KURO_HOME}/.setup-env"

# ─── Backup schedule ───
if ! crontab -l 2>/dev/null | grep -q "backup.sh"; then
    (crontab -l 2>/dev/null || true; echo "0 3 * * * cd ${KURO_HOME} && bash scripts/backup.sh >> ${KURO_HOME}/logs/backup.log 2>&1") | crontab -
    ok "Backup automático: diario 3 AM"
fi

# ────────────────────────────────────────────────────────────────────────────
# Health Check Final
# ────────────────────────────────────────────────────────────────────────────
header "✅ Pipeline listo"

echo ""
echo "  ${BOLD}Servicios:${NC}"
echo "  ─────────────────────────────────────────────────────"
echo "  API:        ${KURO_HTTP_PROTO:-http}://localhost:8080"
echo "  Garage S3:  http://localhost:3900  (S3 API — credenciales en .setup-env)"
echo ""
echo "  ${BOLD}Credenciales:${NC}"
echo "  ─────────────────────────────────────────────────────"
echo "  Admin API Key: ${ADMIN_API_KEY}"
echo "  Guardada en:   ${KURO_HOME}/.admin-key"
echo ""
echo "  ${BOLD}Próximos pasos:${NC}"
echo "  ─────────────────────────────────────────────────────"
echo "  Demo completo:  bash scripts/demo.sh"
echo "  Ver salud:      bash scripts/health-check.sh"
echo ""

# Verificar
ok "Verificando servicios..."
check "PostgreSQL"  "docker exec kuro-postgres pg_isready -U kuro"
check "NATS"        "curl -sf http://${NATS_IP}:8222/healthz"
check "Garage S3"   "curl -sf http://127.0.0.1:3903/v0/status -H 'Authorization: Bearer kuro-garage-admin-token' 2>/dev/null || curl -sf http://127.0.0.1:3900 2>/dev/null || true"
check "API"         "curl -sf ${KURO_HTTP_PROTO:-http}://localhost:8080/health"
check "Worker"      "kill -0 ${WORKER_PID} 2>/dev/null"
check "SigNoz"     "curl -sf http://localhost:3301/api/v1/health 2>/dev/null || true"

echo ""
printf "  ${BOLD}Pipeline listo en ~5 minutos${NC}\n"
echo ""
