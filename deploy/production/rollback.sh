#!/usr/bin/env bash
set -Eeuo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
ENV_FILE="$ROOT/.env.production"
COMPOSE_FILE="$ROOT/deploy/production/compose.yml"
if [[ ! -f "$ENV_FILE" ]]; then echo "Missing $ENV_FILE." >&2; exit 2; fi
release="$(sed -n 's/^MONITOR_ME_RELEASE=//p' "$ENV_FILE" | tail -n 1)"
if [[ -z "$release" || "$release" == *[!A-Za-z0-9._-]* ]]; then
  echo "Set MONITOR_ME_RELEASE to a valid previously built image tag in .env.production." >&2
  exit 2
fi
docker image inspect "monitor-me-api:$release" >/dev/null
docker image inspect "monitor-me-web:$release" >/dev/null
compose() { docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" "$@"; }
compose config --quiet
# Application image rollback only; this does not reverse SQL migrations.
compose up -d --no-build api web
api_port="$(sed -n 's/^API_HTTP_BIND_PORT=//p' "$ENV_FILE" | tail -n 1)"
web_port="$(sed -n 's/^WEB_BIND_PORT=//p' "$ENV_FILE" | tail -n 1)"
api_port="${api_port:-18080}"
web_port="${web_port:-13000}"
ready=0
for attempt in $(seq 1 45); do
  if curl --connect-timeout 2 --max-time 5 -fsS "http://127.0.0.1:$api_port/health/ready" >/dev/null &&
     curl --connect-timeout 2 --max-time 5 -fsS "http://127.0.0.1:$web_port/api/config" >/dev/null; then ready=1; break; fi
  sleep 2
done
compose ps --all
if [[ "$ready" != 1 ]]; then compose logs --no-color --tail=150 api web; echo "Rollback health verification failed." >&2; exit 1; fi
echo "Application images rolled back to $release. Database schema was not rolled back."
