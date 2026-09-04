#!/bin/sh
# Bootstrap SigNoz: create clickhouse user + fix service

# 1. Create clickhouse user via sysusers
mkdir -p /etc/sysusers.d 2>/dev/null || true
cat > /etc/sysusers.d/clickhouse.conf << 'CONF'
g clickhouse 996
u clickhouse 996 "ClickHouse server" /var/lib/clickhouse /sbin/nologin
CONF
systemd-sysusers 2>/dev/null || true

# 2. Fix ClickHouse service (remove RuntimeDirectory)
cp /etc/foundry/pours/deployment/signoz-telemetrystore-clickhouse-0-0.service /etc/systemd/system/signoz-telemetrystore-clickhouse-0-0.service 2>/dev/null || true
sed -i '/RuntimeDirectory=/d' /etc/systemd/system/signoz-telemetrystore-clickhouse-0-0.service 2>/dev/null || true

# 3. Create runtime dir with proper ownership
mkdir -p /run/clickhouse 2>/dev/null || true
chown clickhouse:clickhouse /run/clickhouse 2>/dev/null || true

exec /sbin/init
