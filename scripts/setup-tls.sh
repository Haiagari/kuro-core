#!/usr/bin/env bash
# Kuro — TLS Configuration Script
# Configura certificados TLS para la API Gateway.
#
# Soporta:
#   1. Let's Encrypt (via certbot)  — recomendado para producción
#   2. BYO (bring your own certs)   — certificados existentes
#   3. Auto-generados               — self-signed (ya integrado en api/main.go)
#
# Uso:
#   bash scripts/setup-tls.sh                   # modo interactivo
#   bash scripts/setup-tls.sh --letsencrypt     # Let's Encrypt automático
#   bash scripts/setup-tls.sh --byo <cert> <key> # BYO explícito
#   bash scripts/setup-tls.sh --status           # ver estado actual
set -euo pipefail

# ─── Configuración ──────────────────────────────────────────────────────────

KURO_HOME="${KURO_HOME:-$(cd "$(dirname "$0")/.." && pwd)}"
CERT_DIR="${KURO_HOME}/certs"
ENV_FILE="${KURO_HOME}/.env"
COMPOSE_FILE="${KURO_HOME}/docker-compose.yml"

# ─── Colores ────────────────────────────────────────────────────────────────

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info()  { printf "${BLUE}[INFO]${NC} %s\n" "$1"; }
log_ok()    { printf "${GREEN}[OK]${NC} %s\n" "$1"; }
log_warn()  { printf "${YELLOW}[WARN]${NC} %s\n" "$1"; }
log_error() { printf "${RED}[ERROR]${NC} %s\n" "$1"; }

# ─── Help ───────────────────────────────────────────────────────────────────

usage() {
    cat <<EOF
Uso: bash scripts/setup-tls.sh [opción]

Opciones:
  --letsencrypt <domain>   Configurar Let's Encrypt (requiere certbot + puerto 80 libre)
  --byo <cert> <key>       Usar certificados existentes
  --generate               Generar self-signed (wrapper de generate-tls-certs.sh)
  --status                 Mostrar estado actual de TLS
  --disable                Volver a HTTP (quita TLS)
  -h, --help               Mostrar esta ayuda

Sin opciones: modo interactivo
EOF
    exit 0
}

# ─── Funciones ──────────────────────────────────────────────────────────────

# Detecta el protocolo actual (http o https) según KURO_DISABLE_TLS y TLS_CERT_FILE
detect_proto() {
    local disable_tls
    disable_tls=$(grep -E "^KURO_DISABLE_TLS=" "${COMPOSE_FILE}" 2>/dev/null | head -1 | sed 's/.*"\(.*\)"/\1/' || echo "")
    local cert_file
    cert_file=$(grep -E "^TLS_CERT_FILE=" "${ENV_FILE}" 2>/dev/null | cut -d= -f2 || echo "")

    if [ -n "$cert_file" ] && [ -f "$cert_file" ]; then
        echo "https (custom cert)"
    elif [ "$disable_tls" != "1" ]; then
        echo "https (self-signed)"
    else
        echo "http"
    fi
}

# Verifica que un certificado sea válido
validate_cert() {
    local cert_file="$1"
    local key_file="$2"

    if [ ! -f "$cert_file" ]; then
        log_error "Certificado no encontrado: $cert_file"
        return 1
    fi
    if [ ! -f "$key_file" ]; then
        log_error "Clave privada no encontrada: $key_file"
        return 1
    fi

    # Verificar que el certificado es PEM válido
    if ! openssl x509 -in "$cert_file" -noout 2>/dev/null; then
        log_error "Certificado inválido (no es PEM): $cert_file"
        return 1
    fi

    # Verificar que la clave coincide con el certificado
    local cert_mod key_mod
    cert_mod=$(openssl x509 -in "$cert_file" -noout -modulus 2>/dev/null | openssl md5)
    key_mod=$(openssl rsa -in "$key_file" -noout -modulus 2>/dev/null)
    if [ -z "$key_mod" ]; then
        # Puede ser EC key
        key_mod=$(openssl ec -in "$key_file" -noout -modulus 2>/dev/null || echo "")
    fi
    key_mod=$(echo "$key_mod" | openssl md5)

    if [ "$cert_mod" != "$key_mod" ]; then
        log_error "El certificado y la clave no coinciden"
        return 1
    fi

    # Verificar que no está expirado
    if ! openssl x509 -in "$cert_file" -checkend 0 >/dev/null 2>&1; then
        log_error "El certificado está EXPIRADO"
        return 1
    fi

    local days_left
    days_left=$(openssl x509 -in "$cert_file" -noout -enddate 2>/dev/null | cut -d= -f2 | xargs -I{} date -d "{}" +%s 2>/dev/null | awk -v now="$(date +%s)" '{printf "%.0f", ($1 - now) / 86400}')
    log_ok "Certificado válido ($days_left días restantes)"

    return 0
}

# Escribe variables TLS en .env
write_env_tls() {
    local cert_file="$1"
    local key_file="$2"

    # Normalizar a absoluto si es relativo
    case "$cert_file" in
        /*) ;;
        *) cert_file="${KURO_HOME}/${cert_file}" ;;
    esac
    case "$key_file" in
        /*) ;;
        *) key_file="${KURO_HOME}/${key_file}" ;;
    esac

    # Actualizar o agregar en .env
    if grep -q "^TLS_CERT_FILE=" "${ENV_FILE}" 2>/dev/null; then
        sed -i "s|^TLS_CERT_FILE=.*|TLS_CERT_FILE=${cert_file}|" "${ENV_FILE}"
    else
        echo "# TLS Configuration" >> "${ENV_FILE}"
        echo "TLS_CERT_FILE=${cert_file}" >> "${ENV_FILE}"
    fi

    if grep -q "^TLS_KEY_FILE=" "${ENV_FILE}" 2>/dev/null; then
        sed -i "s|^TLS_KEY_FILE=.*|TLS_KEY_FILE=${key_file}|" "${ENV_FILE}"
    else
        echo "TLS_KEY_FILE=${key_file}" >> "${ENV_FILE}"
    fi

    log_ok "Variables TLS escritas en ${ENV_FILE}"
}

# Elimina KURO_DISABLE_TLS de docker-compose (para producción)
remove_disable_tls() {
    if grep -q "KURO_DISABLE_TLS" "${COMPOSE_FILE}" 2>/dev/null; then
        # Comentar la línea para mantenerla como referencia
        sed -i 's/^\(.*KURO_DISABLE_TLS.*\)/# \1  # Deshabilitado por setup-tls.sh/' "${COMPOSE_FILE}"
        log_ok "KURO_DISABLE_TLS deshabilitado en docker-compose.yml"
    fi
}

# Restaura KURO_DISABLE_TLS (para development)
restore_disable_tls() {
    local target="${COMPOSE_FILE}"
    if [ -L "${target}" ] && [ -f "${COMPOSE_FILE}.optimized.yml" ]; then
        # Si es symlink, editar el archivo real
        target="$(readlink -f "${COMPOSE_FILE}")"
    fi

    if grep -q "KURO_DISABLE_TLS" "${target}" 2>/dev/null; then
        log_info "KURO_DISABLE_TLS ya está presente en compose"
    else
        # Buscar línea de API_PORT para agregar después
        sed -i '/API_PORT/a\      KURO_DISABLE_TLS: "1"' "${target}"
        log_ok "KURO_DISABLE_TLS restaurado"
    fi
}

# ─── Modo interactivo ───────────────────────────────────────────────────────

interactive() {
    echo ""
    echo "╔═══════════════════════════════════════════════╗"
    echo "║      Kuro — TLS Configuration         ║"
    echo "╚═══════════════════════════════════════════════╝"
    echo ""
    echo "Protocolo actual: $(detect_proto)"
    echo ""
    echo "Seleccioná una opción:"
    echo "  1) Let's Encrypt (recomendado para producción)"
    echo "  2) BYO — Usar certificados existentes"
    echo "  3) Generar self-signed (desarrollo/testing)"
    echo "  4) Deshabilitar TLS (volver a HTTP)"
    echo "  5) Solo mostrar estado — salir"
    echo ""
    read -rp "Opción [1-5]: " opt

    case "$opt" in
        1)
            read -rp "Dominio (ej: kuro.example.com): " domain
            setup_letsencrypt "$domain"
            ;;
        2)
            read -rp "Ruta al certificado (.crt/.pem): " cert
            read -rp "Ruta a la clave privada (.key): " key
            setup_byo "$cert" "$key"
            ;;
        3)
            setup_generate
            ;;
        4)
            setup_disable
            ;;
        5)
            show_status
            ;;
        *)
            log_error "Opción inválida"
            exit 1
            ;;
    esac
}

# ─── Let's Encrypt ──────────────────────────────────────────────────────────

setup_letsencrypt() {
    local domain="$1"

    if [ -z "$domain" ]; then
        log_error "Dominio requerido"
        exit 1
    fi

    if ! command -v certbot &>/dev/null; then
        log_error "certbot no encontrado. Instalálo primero:"
        log_error "  sudo apt install certbot"
        log_error "  sudo certbot certonly --standalone -d ${domain}"
        exit 1
    fi

    log_info "Solicitando certificado Let's Encrypt para ${domain}..."
    log_info "Asegurate de que el puerto 80 esté libre y ${domain} apunte a este servidor"

    if sudo certbot certonly --standalone -d "$domain" --non-interactive --agree-tos --email "admin@${domain}"; then
        local cert_file="/etc/letsencrypt/live/${domain}/fullchain.pem"
        local key_file="/etc/letsencrypt/live/${domain}/privkey.pem"

        write_env_tls "$cert_file" "$key_file"
        remove_disable_tls

        log_ok "Let's Encrypt configurado para ${domain}"
        log_info "Recordá renovar: sudo certbot renew"
        log_info "Agregá a cron: 0 3 * * * sudo certbot renew --quiet && docker compose restart api"
    else
        log_error "Falló la obtención del certificado Let's Encrypt"
        log_info "Probá manualmente: sudo certbot certonly --standalone -d ${domain}"
        exit 1
    fi
}

# ─── BYO (Bring Your Own) ───────────────────────────────────────────────────

setup_byo() {
    local cert_file="$1"
    local key_file="$2"

    if ! validate_cert "$cert_file" "$key_file"; then
        exit 1
    fi

    write_env_tls "$cert_file" "$key_file"
    remove_disable_tls

    log_ok "Certificados BYO configurados"
    log_info "Reiniciá la API para aplicar los cambios:"
    log_info "  docker compose restart api"
}

# ─── Self-signed ────────────────────────────────────────────────────────────

setup_generate() {
    if [ -f "${KURO_HOME}/scripts/generate-tls-certs.sh" ]; then
        bash "${KURO_HOME}/scripts/generate-tls-certs.sh"
    else
        log_error "generate-tls-certs.sh no encontrado"
        exit 1
    fi

    write_env_tls "${CERT_DIR}/server.crt" "${CERT_DIR}/server.key"
    remove_disable_tls

    log_ok "Certificados self-signed generados en ${CERT_DIR}"
    log_info "IMPORTANTE: Los curls a la API necesitarán -k o --cacert ${CERT_DIR}/ca.crt"
    log_info "Reiniciá: docker compose restart api"
}

# ─── Disable TLS ────────────────────────────────────────────────────────────

setup_disable() {
    # Limpiar variables TLS de .env
    sed -i '/^TLS_CERT_FILE=/d' "${ENV_FILE}" 2>/dev/null || true
    sed -i '/^TLS_KEY_FILE=/d' "${ENV_FILE}" 2>/dev/null || true
    sed -i '/^# TLS Configuration/d' "${ENV_FILE}" 2>/dev/null || true

    restore_disable_tls

    log_ok "TLS deshabilitado — API volverá a HTTP"
    log_info "Reiniciá: docker compose restart api"
}

# ─── Status ─────────────────────────────────────────────────────────────────

show_status() {
    echo ""
    echo "╔═══════════════════════════════════════════════╗"
    echo "║          Kuro — TLS Status            ║"
    echo "╚═══════════════════════════════════════════════╝"
    echo ""

    # Protocolo actual
    local proto
    proto=$(detect_proto)
    echo "  Protocolo:      ${proto}"

    # Variables de entorno
    local cert_file key_file
    cert_file=$(grep -E "^TLS_CERT_FILE=" "${ENV_FILE}" 2>/dev/null | cut -d= -f2 || echo "(no configurado)")
    key_file=$(grep -E "^TLS_KEY_FILE=" "${ENV_FILE}" 2>/dev/null | cut -d= -f2 || echo "(no configurado)")
    echo "  TLS_CERT_FILE:  ${cert_file}"
    echo "  TLS_KEY_FILE:   ${key_file}"

    # KURO_DISABLE_TLS
    local disable_tls
    disable_tls=$(grep -E "KURO_DISABLE_TLS" "${COMPOSE_FILE}" 2>/dev/null | grep -v "^#" | head -1 | sed 's/.*"\(.*\)"/\1/' || echo "no seteado")
    echo "  KURO_DISABLE_TLS: ${disable_tls}"

    # Self-signed cert check
    local self_signed_cert="${CERT_DIR}/server.crt"
    if [ -f "$self_signed_cert" ]; then
        local days_left
        days_left=$(openssl x509 -in "$self_signed_cert" -noout -enddate 2>/dev/null | cut -d= -f2 | xargs -I{} date -d "{}" +%s 2>/dev/null | awk -v now="$(date +%s)" '{printf "%.0f", ($1 - now) / 86400}')
        echo "  Self-signed cert: ${self_signed_cert} (${days_left} días)"
    else
        echo "  Self-signed cert: no generado"
    fi

    # Verificar endpoint con curl
    echo ""
    if curl -sf http://localhost:8080/health >/dev/null 2>&1; then
        echo "  ⚡ API responde en http://localhost:8080/health"
    elif curl -sfk https://localhost:8080/health >/dev/null 2>&1; then
        echo "  ⚡ API responde en https://localhost:8080/health (self-signed)"
    else
        echo "  ⚡ API no está respondiendo (verificar con docker compose ps)"
    fi
    echo ""
}

# ─── Main ───────────────────────────────────────────────────────────────────

case "${1:-}" in
    --letsencrypt)
        setup_letsencrypt "${2:-}"
        ;;
    --byo)
        if [ $# -lt 3 ]; then
            log_error "Uso: $0 --byo <cert> <key>"
            exit 1
        fi
        setup_byo "$2" "$3"
        ;;
    --generate)
        setup_generate
        ;;
    --status)
        show_status
        ;;
    --disable)
        setup_disable
        ;;
    -h|--help)
        usage
        ;;
    "")
        interactive
        ;;
    *)
        log_error "Opción desconocida: $1"
        usage
        ;;
esac
