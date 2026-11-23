#!/usr/bin/env bash
set -Eeuo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
ENV_FILE="$ROOT/.env.production"
COMPOSE_FILE="$ROOT/deploy/production/compose.yml"
if [[ "${1:-}" != "--confirm" || -z "${2:-}" ]]; then
  echo "Usage: $0 --confirm /absolute/path/to/backup.dump" >&2
  echo "This replaces the active database. A pre-restore snapshot is created first." >&2
  exit 2
fi
backup="$(realpath "$2")"
if [[ ! -s "$backup" ]]; then echo "Backup file is missing or empty: $backup" >&2; exit 2; fi
if [[ ! -f "$ENV_FILE" ]]; then echo "Missing $ENV_FILE." >&2; exit 2; fi
compose() { docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" "$@"; }

pre_restore="$ROOT/backups/pre-restore-$(date -u +%Y%m%dT%H%M%SZ).dump"
"$ROOT/deploy/production/backup.sh" "$pre_restore"

services_stopped=false
restart_application() {
  if [[ "$services_stopped" == true ]]; then
    compose up -d api web || echo "WARNING: application services did not restart automatically; inspect Compose logs." >&2
  fi
}
trap restart_application EXIT

# Stop application writers before replacing the active schema/data.
services_stopped=true
compose stop api web
compose exec -T postgres \
  sh -c 'exec pg_restore --single-transaction --clean --if-exists --no-owner --exit-on-error -U "$POSTGRES_USER" -d "$POSTGRES_DB" -' < "$backup"
compose up -d api web
services_stopped=false
trap - EXIT
echo "Restore completed from $backup."
echo "Pre-restore snapshot: $pre_restore"
echo "Verify application health and sample data before returning traffic."
