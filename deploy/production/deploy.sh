#!/usr/bin/env bash
set -Eeuo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
ENV_FILE="$ROOT/.env.production"
COMPOSE_FILE="$ROOT/deploy/production/compose.yml"
if [[ ! -f "$ENV_FILE" ]]; then
  echo "Missing $ENV_FILE. Copy deploy/production/.env.production.example and configure it first." >&2
  exit 2
fi
command -v docker >/dev/null || { echo "Docker is required." >&2; exit 2; }
command -v curl >/dev/null || { echo "curl is required." >&2; exit 2; }
compose() { docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" "$@"; }

compose config --quiet
compose build api web agent-artifacts
compose up -d postgres redis
ready=0
for attempt in $(seq 1 60); do
  if compose exec -T postgres pg_isready >/dev/null 2>&1 &&
     compose exec -T redis sh -c 'redis-cli --no-auth-warning -a "$REDIS_PASSWORD" ping' 2>/dev/null | grep -qx PONG; then
    ready=1
    break
  fi
  sleep 2
done
if [[ "$ready" != 1 ]]; then
  compose ps --all
  compose logs --no-color --tail=100 postgres redis
  echo "PostgreSQL or Redis did not become healthy; application containers were not upgraded." >&2
  exit 1
fi

# Keep the one-shot migration container so Compose can verify its exit status
# when it evaluates service_completed_successfully for the API dependency.
compose up -d migrate
migration_container="$(compose ps --all -q migrate)"
[[ -n "$migration_container" ]] || { echo "Migration container was not created." >&2; exit 1; }
migration_complete=0
for attempt in $(seq 1 60); do
  migration_state="$(docker inspect --format '{{.State.Status}} {{.State.ExitCode}}' "$migration_container")"
  if [[ "$migration_state" == "exited 0" ]]; then
    migration_complete=1
    break
  fi
  if [[ "$migration_state" == exited\ * ]]; then
    compose logs --no-color --tail=150 migrate
    echo "Database migration failed; existing API/web containers were not upgraded." >&2
    exit 1
  fi
  sleep 2
done
if [[ "$migration_complete" != 1 ]]; then
  compose logs --no-color --tail=150 migrate
  echo "Database migration did not finish within the readiness window." >&2
  exit 1
fi
compose up -d --no-build api web
api_port="$(sed -n 's/^API_HTTP_BIND_PORT=//p' "$ENV_FILE" | tail -n 1)"
web_port="$(sed -n 's/^WEB_BIND_PORT=//p' "$ENV_FILE" | tail -n 1)"
api_port="${api_port:-18080}"
web_port="${web_port:-13000}"
ready=0
for attempt in $(seq 1 45); do
  if curl --connect-timeout 2 --max-time 5 -fsS "http://127.0.0.1:$api_port/health/ready" >/dev/null &&
     curl --connect-timeout 2 --max-time 5 -fsS "http://127.0.0.1:$web_port/api/config" >/dev/null; then
    ready=1
    break
  fi
  sleep 2
done
compose ps --all
if [[ "$ready" != 1 ]]; then
  compose logs --no-color --tail=150 api web
  echo "Deployment health verification failed. Inspect logs and schema compatibility before rollback." >&2
  exit 1
fi
echo "Application containers are healthy. Verify Nginx, external HTTPS, WebSocket and gRPC from another network."
