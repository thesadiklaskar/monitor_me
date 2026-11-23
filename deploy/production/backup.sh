#!/usr/bin/env bash
set -Eeuo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
ENV_FILE="$ROOT/.env.production"
COMPOSE_FILE="$ROOT/deploy/production/compose.yml"
if [[ ! -f "$ENV_FILE" ]]; then echo "Missing $ENV_FILE." >&2; exit 2; fi
destination="${1:-$ROOT/backups/monitor-me-$(date -u +%Y%m%dT%H%M%SZ).dump}"
backup_dir="$(dirname "$destination")"
mkdir -p "$backup_dir"
if [[ -e "$destination" ]]; then echo "Refusing to overwrite existing backup: $destination" >&2; exit 2; fi
umask 077
tmp="$(mktemp "$destination.partial.XXXXXX")"
trap 'rm -f "$tmp"' EXIT
docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" exec -T postgres \
  sh -c 'exec pg_dump -Fc -U "$POSTGRES_USER" -d "$POSTGRES_DB"' > "$tmp"
if [[ ! -s "$tmp" ]]; then echo "Database backup is empty." >&2; exit 1; fi
# A same-filesystem hard link atomically refuses an existing destination.
ln "$tmp" "$destination"
rm -f "$tmp"
trap - EXIT
chmod 600 "$destination"
echo "$destination"
