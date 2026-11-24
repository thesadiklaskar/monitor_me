#!/bin/sh
set -eu

: "${POSTGRES_DB:?POSTGRES_DB is required}"
: "${POSTGRES_USER:?POSTGRES_USER is required}"
: "${POSTGRES_PASSWORD:?POSTGRES_PASSWORD is required}"
: "${GRAFANA_DB_PASSWORD:?GRAFANA_DB_PASSWORD is required}"

case "$GRAFANA_DB_PASSWORD" in
  ''|*[!A-Fa-f0-9]*)
    echo "GRAFANA_DB_PASSWORD must contain only hexadecimal characters." >&2
    exit 2
    ;;
esac
if [ "${#GRAFANA_DB_PASSWORD}" -lt 32 ]; then
  echo "GRAFANA_DB_PASSWORD must be at least 32 hexadecimal characters." >&2
  exit 2
fi

export PGPASSWORD="$POSTGRES_PASSWORD"
psql -X -h postgres -U "$POSTGRES_USER" -d "$POSTGRES_DB" \
  -v ON_ERROR_STOP=1 \
  -v grafana_password="$GRAFANA_DB_PASSWORD" \
  -v database="$POSTGRES_DB" \
  -v owner="$POSTGRES_USER" <<'SQL'
SELECT format(
  CASE
    WHEN EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname = 'grafana_reader')
      THEN 'ALTER ROLE grafana_reader WITH LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION PASSWORD %L'
    ELSE 'CREATE ROLE grafana_reader WITH LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION PASSWORD %L'
  END,
  :'grafana_password'
) \gexec

GRANT CONNECT ON DATABASE :"database" TO grafana_reader;
GRANT USAGE ON SCHEMA public TO grafana_reader;
REVOKE CREATE ON SCHEMA public FROM grafana_reader;
REVOKE CREATE, TEMPORARY ON DATABASE :"database" FROM grafana_reader;
REVOKE ALL PRIVILEGES ON ALL TABLES IN SCHEMA public FROM grafana_reader;
GRANT SELECT ON TABLE public.servers, public.telemetry_samples TO grafana_reader;
SQL

unset PGPASSWORD
echo "Grafana PostgreSQL reader is ready (read-only)."
