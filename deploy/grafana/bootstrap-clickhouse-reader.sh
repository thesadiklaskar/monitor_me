#!/bin/sh
set -eu

: "${CLICKHOUSE_USER:?CLICKHOUSE_USER is required}"
: "${CLICKHOUSE_PASSWORD:?CLICKHOUSE_PASSWORD is required}"
: "${GRAFANA_CLICKHOUSE_PASSWORD:?GRAFANA_CLICKHOUSE_PASSWORD is required}"
: "${CLICKHOUSE_DB:?CLICKHOUSE_DB is required}"

case "$GRAFANA_CLICKHOUSE_PASSWORD" in
  ''|*[!A-Fa-f0-9]*)
    echo "GRAFANA_CLICKHOUSE_PASSWORD must contain only hexadecimal characters." >&2
    exit 2
    ;;
esac
if [ "${#GRAFANA_CLICKHOUSE_PASSWORD}" -lt 32 ]; then
  echo "GRAFANA_CLICKHOUSE_PASSWORD must be at least 32 hexadecimal characters." >&2
  exit 2
fi

sql="
CREATE SETTINGS PROFILE IF NOT EXISTS grafana_reader_profile
    SETTINGS readonly = 1 CHANGEABLE_IN_READONLY MIN 1 MAX 1,
             max_execution_time = 60 CHANGEABLE_IN_READONLY;
CREATE ROLE IF NOT EXISTS grafana_reader_role
    SETTINGS PROFILE grafana_reader_profile;
CREATE USER IF NOT EXISTS grafana_reader
    IDENTIFIED WITH sha256_password BY '$GRAFANA_CLICKHOUSE_PASSWORD'
    DEFAULT ROLE grafana_reader_role;
ALTER USER grafana_reader
    IDENTIFIED WITH sha256_password BY '$GRAFANA_CLICKHOUSE_PASSWORD'
    DEFAULT ROLE grafana_reader_role;
GRANT SELECT ON $CLICKHOUSE_DB.logs TO grafana_reader_role;
"

clickhouse-client \
  --host clickhouse \
  --user "$CLICKHOUSE_USER" \
  --password "$CLICKHOUSE_PASSWORD" \
  --multiquery \
  --query "$sql"

echo "Grafana ClickHouse reader is ready (read-only on $CLICKHOUSE_DB.logs)."
