#!/bin/bash
# Kuro — Backup PostgreSQL a MinIO
# Ejecuta pg_dump, comprime, sube a MinIO, limpia backups >30 días.
#
# Uso manual: ./deploy/scripts/backup.sh
# Uso cron:   Ejecutado por el servicio kuro-backup en docker-compose

set -euo pipefail

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

DATE=$(date +%Y%m%d-%H%M%S)
BACKUP_FILE="kuro-pg-${DATE}.sql.gz"
BUCKET="kuro-backups"
RETENTION_DAYS=30

echo -e "${YELLOW}[backup]${NC} Iniciando backup de PostgreSQL..."

# ── 1. pg_dump ──────────────────────────────────────────
echo -e "  ${YELLOW}→${NC} Ejecutando pg_dump..."
if ! PGPASSWORD="${DB_PASSWORD:-kuro_pg_dev}" pg_dump \
    -h "${DB_HOST:-postgres}" \
    -U "${DB_USER:-kuro}" \
    -d "${DB_NAME:-kuro}" \
    --no-owner \
    --compress=9 \
    -f "/tmp/${BACKUP_FILE}"; then
    echo -e "${RED}[backup]${NC} Error: pg_dump falló (ver detalle arriba)"
    exit 1
fi

SIZE=$(du -h "/tmp/${BACKUP_FILE}" | cut -f1)
echo -e "  ${GREEN}✓${NC} Backup creado: ${BACKUP_FILE} (${SIZE})"

# ── 2. Subir a MinIO ────────────────────────────────────
S3_ENDPOINT="${S3_ENDPOINT:-http://garage:9000}"
AWS_ACCESS_KEY_ID="${S3_ACCESS_KEY:-kuroadmin}"
AWS_SECRET_ACCESS_KEY="${S3_SECRET_KEY:-kuroadmin123}"
export AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY
export AWS_DEFAULT_REGION="${AWS_DEFAULT_REGION:-us-east-1}"

# Crear bucket si no existe
if ! aws --endpoint-url "$S3_ENDPOINT" s3 ls "s3://${BUCKET}" &>/dev/null; then
    aws --endpoint-url "$S3_ENDPOINT" s3 mb "s3://${BUCKET}" 2>/dev/null || true
fi

if aws --endpoint-url "$S3_ENDPOINT" \
    s3 cp "/tmp/${BACKUP_FILE}" "s3://${BUCKET}/${BACKUP_FILE}" \
    --quiet 2>/dev/null; then
    echo -e "  ${GREEN}✓${NC} Subido a MinIO: s3://${BUCKET}/${BACKUP_FILE}"
else
    echo -e "  ${RED}✗${NC} Error subiendo a MinIO"
    rm -f "/tmp/${BACKUP_FILE}"
    exit 1
fi

# ── 3. (Opcional) Reflejar a S3 externo ─────────────────
OFFSITE_ENDPOINT="${OFFSITE_S3_ENDPOINT:-}"
if [ -n "$OFFSITE_ENDPOINT" ]; then
    export AWS_ACCESS_KEY_ID="${OFFSITE_ACCESS_KEY:-}"
    export AWS_SECRET_ACCESS_KEY="${OFFSITE_SECRET_KEY:-}"
    OFFSITE_BUCKET="${OFFSITE_BUCKET:-kuro-backups}"
    echo -e "  ${YELLOW}→${NC} Reflejando backup a S3 externo: ${OFFSITE_ENDPOINT}"
    if aws --endpoint-url "$OFFSITE_ENDPOINT" s3 cp "/tmp/${BACKUP_FILE}" "s3://${OFFSITE_BUCKET}/${BACKUP_FILE}" --quiet 2>/dev/null; then
        echo -e "  ${GREEN}✓${NC} Backup reflejado: s3://${OFFSITE_BUCKET}/${BACKUP_FILE}"
    else
        echo -e "  ${RED}✗${NC} Error reflejando backup externo (no crítico)"
    fi
fi

rm -f "/tmp/${BACKUP_FILE}"

# ── 4. Limpiar backups > 30 días (local) ────────────────
echo -e "  ${YELLOW}→${NC} Limpiando backups anteriores a ${RETENTION_DAYS} días..."

OLD_BACKUPS=$(aws --endpoint-url "$S3_ENDPOINT" \
    s3 ls "s3://${BUCKET}/" 2>/dev/null | grep "kuro-pg-" | awk '{print $4}')

# Calcular cutoff como YYYYMMDD numérico (compatible con Alpine/busybox)
CUTOFF_YMD=$(date -d "@$(($(date +%s) - ${RETENTION_DAYS} * 86400))" +%Y%m%d 2>/dev/null || \
    date -j -v-${RETENTION_DAYS}d +%Y%m%d 2>/dev/null || \
    echo "")

if [ -z "$CUTOFF_YMD" ]; then
    echo -e "  ${YELLOW}⚠${NC} No se pudo calcular cutoff, saltando limpieza"
else
    for backup in $OLD_BACKUPS; do
        # Extraer fecha del nombre: kuro-pg-20260603-120000.sql.gz → 20260603
        BACKUP_DATE=$(echo "$backup" | sed 's/kuro-pg-//; s/-.*//')
        if [ -n "$BACKUP_DATE" ] && [ "$BACKUP_DATE" -lt "$CUTOFF_YMD" ] 2>/dev/null; then
            aws --endpoint-url "$S3_ENDPOINT" s3 rm "s3://${BUCKET}/${backup}" --quiet 2>/dev/null || true
            echo -e "  ${YELLOW}→${NC} Eliminado: ${backup}"
        fi
    done
fi

echo -e "${GREEN}[backup]${NC} Backup completado exitosamente"
