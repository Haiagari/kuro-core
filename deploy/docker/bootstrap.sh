#!/bin/sh
# SigNoz bootstrap: creates clickhouse user and fixes service unit
# Runs once at boot via kuro-bootstrap.service

# Create sysusers config and run it
mkdir -p /etc/sysusers.d
cp /data/clickhouse-sysuser.conf /etc/sysusers.d/clickhouse.conf 2>/dev/null
systemd-sysusers 2>/dev/null || true

# Fix ClickHouse service: remove RuntimeDirectory
cp /etc/foundry/pours/deployment/signoz-telemetrystore-clickhouse-0-0.service /etc/systemd/system/signoz-telemetrystore-clickhouse-0-0.service 2>/dev/null
sed -i '/RuntimeDirectory=/d' /etc/systemd/system/signoz-telemetrystore-clickhouse-0-0.service 2>/dev/null

# Create runtime dir manually
mkdir -p /run/clickhouse
chown clickhouse:clickhouse /run/clickhouse 2>/dev/null || true

# Reload systemd
systemctl daemon-reload 2>/dev/null || true

# Self-disable so it only runs once
systemctl disable kuro-bootstrap.service 2>/dev/null || true
