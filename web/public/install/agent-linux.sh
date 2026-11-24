#!/bin/sh
set -eu

SERVER_URL=""
GRPC_ADDRESS=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --server-url) SERVER_URL="\${2:?Missing --server-url value}"; shift 2 ;;
    --grpc-address) GRPC_ADDRESS="\${2:?Missing --grpc-address value}"; shift 2 ;;
    -h|--help)
      echo "Monitor Me native Linux installer"
      echo "Usage: curl -fsSL https://monitor.example/install/agent-linux.sh | sudo bash -s -- --server-url https://monitor.example --grpc-address monitor.example:8443"
      exit 0
      ;;
    *) echo "Unknown option: $1" >&2; exit 2 ;;
  esac
done

if [ "$(id -u)" -ne 0 ]; then
  echo "Run this installer with sudo." >&2
  exit 1
fi
if [ -z "$SERVER_URL" ]; then
  echo "--server-url is required." >&2
  exit 2
fi
case "$SERVER_URL" in
  https://*) ;;
  http://localhost*|http://127.0.0.1*) ;;
  *) echo "Use HTTPS for a remote control plane. HTTP is allowed only for local testing." >&2; exit 2 ;;
esac
case "$(uname -s)" in
  Linux) ;;
  *) echo "This installer is for Linux. Run the PowerShell installer on Windows." >&2; exit 2 ;;
esac
case "$(uname -m)" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported Linux architecture: $(uname -m)" >&2; exit 2 ;;
esac
command -v systemctl >/dev/null 2>&1 || { echo "This installer requires systemd." >&2; exit 2; }
command -v curl >/dev/null 2>&1 || { echo "curl is required." >&2; exit 2; }
command -v sha256sum >/dev/null 2>&1 || { echo "sha256sum is required." >&2; exit 2; }

SERVER_URL="\${SERVER_URL%/}"
if [ -z "$GRPC_ADDRESS" ]; then
  authority="$(printf '%s' "$SERVER_URL" | sed 's#^[A-Za-z]*://##' | cut -d/ -f1)"
  host="\${authority%%:*}"
  GRPC_ADDRESS="$host:8443"
fi

DATA_DIR="/var/lib/monitor-me"
CONFIG_DIR="/etc/monitor-me"
BINARY="/usr/local/bin/monitor-agent"
STATE_FILE="$DATA_DIR/agent-state.json"
CONFIG_FILE="$CONFIG_DIR/agent.env"
TEMP_DIR="$(mktemp -d)"
old_tty=""
cleanup() {
  if [ -n "$old_tty" ] && [ -r /dev/tty ]; then stty "$old_tty" < /dev/tty 2>/dev/null || true; fi
  rm -rf "$TEMP_DIR"
}
trap cleanup EXIT HUP INT TERM

headers="$TEMP_DIR/agent.headers"
download="$TEMP_DIR/monitor-agent"
ca="$TEMP_DIR/agent-ca.pem"
curl --fail --silent --show-error --location "$SERVER_URL/api/agent-artifacts/linux/$ARCH" -D "$headers" -o "$download"
expected="$(tr -d '\r' < "$headers" | sed -n 's/^[Xx]-[Mm]onitor[Mm]e-[Ss][Hh][Aa]256:[[:space:]]*//p' | tail -n 1)"
version="$(tr -d '\r' < "$headers" | sed -n 's/^[Xx]-[Mm]onitor[Mm]e-[Aa]gent-[Vv]ersion:[[:space:]]*//p' | tail -n 1)"
actual="$(sha256sum "$download" | awk '{print $1}')"
if [ -z "$expected" ] || [ "$actual" != "$expected" ] || [ -z "$version" ]; then
  echo "Native agent download failed SHA-256 or release-version verification." >&2
  exit 1
fi
curl --fail --silent --show-error --location "$SERVER_URL/api/agent-ca" -o "$ca"
grep -q -- '-----BEGIN CERTIFICATE-----' "$ca" || {
  echo "The control plane did not provide a valid PEM certificate." >&2
  exit 1
}

install -d -m 0755 /usr/local/bin "$CONFIG_DIR"
install -d -m 0700 "$DATA_DIR"
install -m 0755 "$download" "$TEMP_DIR/monitor-agent.new"
mv -f "$TEMP_DIR/monitor-agent.new" "$BINARY"
install -m 0644 "$ca" "$CONFIG_DIR/server-ca.crt"

TOKEN=""
if [ ! -s "$STATE_FILE" ]; then
  if [ ! -r /dev/tty ]; then
    echo "A one-time enrollment token is required; run this command from an interactive terminal." >&2
    exit 1
  fi
  printf "Paste the one-time Monitor Me enrollment token: " > /dev/tty
  old_tty="$(stty -g < /dev/tty)"
  stty -echo < /dev/tty
  IFS= read -r TOKEN < /dev/tty || TOKEN=""
  stty "$old_tty" < /dev/tty
  old_tty=""
  printf '\n' > /dev/tty
  if [ -z "$TOKEN" ]; then
    echo "An enrollment token is required for first enrollment." >&2
    exit 1
  fi
fi

umask 077
{
  printf 'MONITOR_AGENT_VERSION=%s\n' "$version"
  printf 'MONITOR_GRPC_ADDRESS=%s\n' "$GRPC_ADDRESS"
  printf 'MONITOR_TLS_CA_FILE=%s\n' "$CONFIG_DIR/server-ca.crt"
  printf 'MONITOR_COLLECTION_INTERVAL=5s\n'
  printf 'MONITOR_SPOOL_DIR=%s/spool\n' "$DATA_DIR"
  printf 'MONITOR_AGENT_STATE_FILE=%s\n' "$STATE_FILE"
  if [ -n "$TOKEN" ]; then printf 'MONITOR_ENROLLMENT_TOKEN=%s\n' "$TOKEN"; fi
} > "$TEMP_DIR/agent.env"
unset TOKEN
install -m 0600 "$TEMP_DIR/agent.env" "$CONFIG_FILE"

cat > /etc/systemd/system/monitor-me-agent.service <<'UNIT'
[Unit]
Description=Monitor Me Native Host Agent
Wants=network-online.target
After=network-online.target

[Service]
Type=simple
User=root
Group=root
EnvironmentFile=-/etc/monitor-me/agent.env
Environment=MONITOR_SPOOL_DIR=/var/lib/monitor-me/spool
Environment=MONITOR_AGENT_STATE_FILE=/var/lib/monitor-me/agent-state.json
StateDirectory=monitor-me
StateDirectoryMode=0700
UMask=0077
ExecStart=/usr/local/bin/monitor-agent
Restart=on-failure
RestartSec=5s
KillSignal=SIGTERM
TimeoutStopSec=30s
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=read-only
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
ReadWritePaths=/var/lib/monitor-me

[Install]
WantedBy=multi-user.target
UNIT

systemctl daemon-reload
systemctl enable --now monitor-me-agent
if grep -q '^MONITOR_ENROLLMENT_TOKEN=' "$CONFIG_FILE"; then
  persisted=0
  attempt=0
  while [ "$attempt" -lt 30 ]; do
    if [ -s "$STATE_FILE" ]; then persisted=1; break; fi
    sleep 1
    attempt=$((attempt + 1))
  done
  if [ "$persisted" -ne 1 ]; then
    echo "Agent did not persist its enrollment session. Recent logs:" >&2
    journalctl -u monitor-me-agent -n 40 --no-pager >&2 || true
    exit 1
  fi
  grep -v '^MONITOR_ENROLLMENT_TOKEN=' "$CONFIG_FILE" > "$TEMP_DIR/agent.env"
  install -m 0600 "$TEMP_DIR/agent.env" "$CONFIG_FILE"
  systemctl restart monitor-me-agent
fi

echo "Monitor Me agent installed and running as a systemd service."
echo "Detected: Linux $(uname -m) · agent version $version"
echo "Status: systemctl status monitor-me-agent"
