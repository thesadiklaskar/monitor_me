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

admin_cidr="$(sed -n 's/^ADMIN_CIDR=//p' "$ENV_FILE" | tail -n 1 | tr -d '"' | tr -d '\r')"
public_hostname="$(sed -n 's/^PUBLIC_HOSTNAME=//p' "$ENV_FILE" | tail -n 1 | tr -d '"' | tr -d '\r')"
agent_grpc_public_address="$(sed -n 's/^AGENT_GRPC_PUBLIC_ADDRESS=//p' "$ENV_FILE" | tail -n 1 | tr -d '"' | tr -d '\r')"
access_file="/etc/nginx/snippets/monitor-me-admin-allowlist.conf"
access_directory="$(dirname "$access_file")"
access_tmp="$(mktemp)"
access_backup=""

if [[ -z "$public_hostname" || "$public_hostname" == *REPLACE* ]]; then
  echo "Set PUBLIC_HOSTNAME to the actual DNS name or IP used by the HTTPS certificate." >&2
  exit 2
fi
if [[ -z "$agent_grpc_public_address" || "$agent_grpc_public_address" == *REPLACE* ]]; then
  echo "Set AGENT_GRPC_PUBLIC_ADDRESS to the reachable control-plane host and gRPC port." >&2
  exit 2
fi
if [[ -z "$admin_cidr" || "$admin_cidr" == *REPLACE* ]]; then
  echo "Refusing no-login production deployment: set ADMIN_CIDR to the trusted administrator/VPN source range." >&2
  exit 2
fi
if [[ "$EUID" -ne 0 ]]; then
  echo "No-login production deployment requires root to install the Nginx allowlist and host firewall rules." >&2
  exit 2
fi
python3 -c 'import ipaddress,sys; n=ipaddress.ip_network(sys.argv[1], strict=False); sys.exit(1 if n.prefixlen == 0 else 0)' "$admin_cidr" || {
  echo "ADMIN_CIDR must be a specific IPv4/IPv6 source range, not a default route." >&2
  exit 2
}
mkdir -p "$access_directory"
printf 'allow %s;\ndeny all;\n' "$admin_cidr" > "$access_tmp"
chmod 0644 "$access_tmp"
if [[ -f "$access_file" ]]; then
  access_backup="$(mktemp)"
  cp -p "$access_file" "$access_backup"
fi
install -m 0644 "$access_tmp" "$access_file"
rm -f "$access_tmp"

if ! nginx -t; then
  echo "Nginx validation failed; restoring the previous administrator access file." >&2
  if [[ -n "$access_backup" ]]; then
    install -m 0644 "$access_backup" "$access_file"
  else
    rm -f "$access_file"
  fi
  rm -f "$access_backup"
  exit 2
fi
rm -f "$access_backup"
systemctl reload nginx

firewall_args=(--role control-plane --admin-cidr "$admin_cidr" --verify-admin-ui-allowlist)
# The dashboard has no user login: install and verify its network boundary
# before exposing HTTPS routes used by remote agent bootstrap.
"$ROOT/deploy/production/firewall.sh" "${firewall_args[@]}"
"$ROOT/deploy/production/firewall.sh" "${firewall_args[@]}" --apply

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
compose up -d --no-build api web grafana
api_port="$(sed -n 's/^API_HTTP_BIND_PORT=//p' "$ENV_FILE" | tail -n 1)"
web_port="$(sed -n 's/^WEB_BIND_PORT=//p' "$ENV_FILE" | tail -n 1)"
grafana_port="$(sed -n 's/^GRAFANA_BIND_PORT=//p' "$ENV_FILE" | tail -n 1)"
api_port="${api_port:-18080}"
web_port="${web_port:-13000}"
grafana_port="${grafana_port:-13001}"
ready=0
for attempt in $(seq 1 45); do
  if curl --connect-timeout 2 --max-time 5 -fsS "http://127.0.0.1:$api_port/health/ready" >/dev/null &&
     curl --connect-timeout 2 --max-time 5 -fsS "http://127.0.0.1:$web_port/api/config" >/dev/null &&
     curl --connect-timeout 2 --max-time 5 -fsS "http://127.0.0.1:$grafana_port/api/health" >/dev/null; then
    ready=1
    break
  fi
  sleep 2
done
compose ps --all
if [[ "$ready" != 1 ]]; then
  compose logs --no-color --tail=150 api web grafana grafana-db-setup
  echo "Deployment health verification failed. Inspect API/web/Grafana logs and schema compatibility before rollback." >&2
  exit 1
fi
echo "Application and Grafana containers are healthy."
echo "Grafana: https://$public_hostname/grafana/ (admin login; password stored in .env.production)."
echo "Verify Nginx, external HTTPS, WebSocket and gRPC from another network."
