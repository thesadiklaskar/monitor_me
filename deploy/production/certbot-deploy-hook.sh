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
echo "Renewed gRPC TLS certificate copied to $destination with group-only read access."
