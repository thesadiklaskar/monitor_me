#!/usr/bin/env bash
set -Eeuo pipefail

# Certbot supplies RENEWED_LINEAGE when this script runs as a deploy hook.
if [[ -z "${RENEWED_LINEAGE:-}" || ! -s "$RENEWED_LINEAGE/fullchain.pem" || ! -s "$RENEWED_LINEAGE/privkey.pem" ]]; then
  echo "Certbot did not provide a readable renewed certificate lineage." >&2
  exit 2
fi

destination="${AGENT_TLS_CERT_DIR:-/var/lib/monitor-me/certs}"
certificate_group="${CERT_GROUP_ID:-15001}"

if [[ ! "$certificate_group" =~ ^[0-9]+$ ]]; then
  echo "CERT_GROUP_ID must be a numeric host group ID." >&2
  exit 2
fi
if [[ "$EUID" -ne 0 ]]; then
  echo "The certificate deploy hook must run as root." >&2
  exit 2
fi

install -d -o root -g "$certificate_group" -m 0750 "$destination"
install -o root -g "$certificate_group" -m 0640 "$RENEWED_LINEAGE/fullchain.pem" "$destination/fullchain.pem"
install -o root -g "$certificate_group" -m 0640 "$RENEWED_LINEAGE/privkey.pem" "$destination/privkey.pem"
chmod 0750 "$destination"

# Reload the HTTPS edge after the certificate files have been refreshed.
nginx -t
systemctl reload nginx

# The Go gRPC server loads its TLS keypair at process startup. Restart it
# only when this hook is running from a configured Monitor Me installation.
if [[ -n "${MONITOR_ME_ROOT:-}" ]]; then
  env_file="$MONITOR_ME_ROOT/.env.production"
  compose_file="$MONITOR_ME_ROOT/deploy/production/compose.yml"
  if [[ -s "$env_file" && -f "$compose_file" ]] && command -v docker >/dev/null 2>&1; then
    if docker compose --env-file "$env_file" -f "$compose_file" ps -q api 2>/dev/null | grep -q .; then
      docker compose --env-file "$env_file" -f "$compose_file" restart api
    fi
  fi
fi

echo "Renewed browser and gRPC TLS certificates; reloaded Nginx and restarted the running API when present."
