#!/bin/bash
# Script para configurar reglas de firewall para kuro-internal
# Ejecutar con: sudo ./scripts/setup-firewall.sh

if [ "$EUID" -ne 0 ]; then
  echo "❌ Error: Por favor ejecutá este script como root (sudo)."
  exit 1
fi

echo "🔍 Detectando interfaz física de la red Docker 'kuro-internal'..."
NET_ID=$(docker network inspect kuro-internal --format '{{.Id}}' 2>/dev/null)

if [ -z "$NET_ID" ]; then
  echo "❌ Error: No se encontró la red 'kuro-internal'. ¿Corriste 'docker compose up' primero?"
  exit 1
fi

IFACE="br-${NET_ID:0:12}"
echo "✅ Interfaz física detectada: $IFACE"

echo "⚙️ Configurando reglas de iptables..."
iptables -A FORWARD -i "$IFACE" -j ACCEPT
iptables -A FORWARD -o "$IFACE" -j ACCEPT

# Si usan UFW (muy común)
if command -v ufw >/dev/null 2>&1; then
  echo "🛡️ Detectado UFW. Configurando reglas de UFW..."
  ufw route allow in on "$IFACE"
  ufw route allow out on "$IFACE"
fi

echo "🎉 Reglas de firewall aplicadas exitosamente para la red kuro-internal!"
echo "Ahora los contenedores bridge pueden comunicarse libremente con el Host y entre sí."
