#!/usr/bin/env bash
set -Eeuo pipefail

GRPC_ADDRESS="localhost:8443"
CA_FILE=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --grpc-address) GRPC_ADDRESS="${2:?Missing gRPC address}"; shift 2 ;;
    --ca-file) CA_FILE="${2:?Missing CA file path}"; shift 2 ;;
    -h|--help)
      echo "Usage: sudo ./scripts/setup-linux-agent.sh [--grpc-address host:8443] [--ca-file /path/to/server.crt]"
      exit 0
      ;;
    *) echo "Unknown option: $1" >&2; exit 2 ;;
  esac
done

if [[ "$(id -u)" -ne 0 ]]; then
  echo "Run this installer with sudo so it can install the systemd service." >&2
  exit 1
fi
if ! command -v docker >/dev/null 2>&1; then
  echo "Docker is required to build the native agent from this source checkout." >&2
  exit 1
fi
if ! command -v systemctl >/dev/null 2>&1; then
  echo "This installer currently supports systemd-based Linux distributions." >&2
  exit 1
fi

ROOT_DIR="$(cd -- "$(dirname -- "$0")/.." && pwd)"
BINARY="$ROOT_DIR/.data/linux-agent/monitor-agent"
if [[ -z "$CA_FILE" ]]; then
  CA_FILE="$ROOT_DIR/.data/certs/server.crt"
fi
if [[ ! -s "$CA_FILE" ]]; then
  echo "TLS CA certificate not found: $CA_FILE" >&2
  echo "Pass the control-plane CA with --ca-file." >&2
  exit 1
fi

docker run --rm --entrypoint /bin/sh \
  --mount "type=bind,source=$ROOT_DIR,target=/src" \
  --workdir /src golang:1.27.4-alpine3.23 /src/scripts/build-linux-agent.sh
if [[ ! -s "$BINARY" ]]; then
  echo "The Linux agent build did not produce $BINARY." >&2
  exit 1
fi

install -d -m 0755 /usr/local/bin /etc/monitor-me
install -m 0755 "$BINARY" /usr/local/bin/monitor-agent
install -m 0644 "$ROOT_DIR/agent/deploy/systemd/monitor-me-agent.service" /etc/systemd/system/monitor-me-agent.service
install -m 0644 "$CA_FILE" /etc/monitor-me/server-ca.crt

STATE_FILE="/var/lib/monitor-me/agent-state.json"
ENV_FILE="/etc/monitor-me/agent.env"
TOKEN=""
if [[ ! -s "$STATE_FILE" ]]; then
  read -r -s -p "Paste the one-time Monitor Me enrollment token: " TOKEN
  echo
  if [[ -z "$TOKEN" ]]; then
    echo "An enrollment token is required for first enrollment." >&2
    exit 1
  fi
fi

umask 077
{
  printf 'MONITOR_GRPC_ADDRESS=%s\n' "$GRPC_ADDRESS"
  printf 'MONITOR_TLS_CA_FILE=%s\n' "/etc/monitor-me/server-ca.crt"
  printf 'MONITOR_COLLECTION_INTERVAL=%s\n' "5s"
  printf 'MONITOR_AGENT_STATE_FILE=%s\n' "$STATE_FILE"
  if [[ -n "$TOKEN" ]]; then
    printf 'MONITOR_ENROLLMENT_TOKEN=%s\n' "$TOKEN"
  fi
} > "$ENV_FILE"
unset TOKEN

systemctl daemon-reload
systemctl enable --now monitor-me-agent

if [[ ! -s "$STATE_FILE" ]]; then
  for _ in $(seq 1 30); do
    [[ -s "$STATE_FILE" ]] && break
    sleep 1
  done
fi
if [[ ! -s "$STATE_FILE" ]]; then
  echo "Agent session was not persisted. Recent service logs:" >&2
  journalctl -u monitor-me-agent -n 30 --no-pager >&2 || true
  exit 1
fi

# Remove the one-time token from the service environment after the session is saved.
tmp_env="$(mktemp)"
grep -v '^MONITOR_ENROLLMENT_TOKEN=' "$ENV_FILE" > "$tmp_env" || true
install -m 0600 "$tmp_env" "$ENV_FILE"
rm -f "$tmp_env"
systemctl restart monitor-me-agent

echo "Monitor Me Linux agent installed and running as a systemd service."
echo "Check status with: systemctl status monitor-me-agent"
echo "Follow logs with: journalctl -u monitor-me-agent -f"
