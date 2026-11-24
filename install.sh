#!/usr/bin/env bash
set -Eeuo pipefail
umask 077
ROOT="$(cd -- "$(dirname -- "$0")" && pwd)"
PROD="$ROOT/deploy/production"
ENV_FILE="$ROOT/.env.production"
CERT_DIR="/var/lib/monitor-me/certs"
NGINX_SITE="/etc/nginx/conf.d/monitor-me.conf"
NGINX_ACCESS="/etc/nginx/snippets/monitor-me-admin-allowlist.conf"
log() { printf '\n[monitor-me] %s\n' "$*"; }
fail() { printf '\n[monitor-me] ERROR: %s\n' "$*" >&2; exit 1; }
prompt() {
  local label="$1" default="$2" answer
  if [[ -n "$default" ]]; then
    read -r -p "$label [$default]: " answer
    if [[ -z "$answer" ]]; then answer="$default"; fi
  else
    read -r -p "$label: " answer
  fi
  printf '%s' "$answer"
}
valid_domain() { [[ "$1" =~ ^([A-Za-z0-9]([A-Za-z0-9-]{0,61}[A-Za-z0-9])?\.)+[A-Za-z]{2,63}$ ]]; }
valid_cidr() {
  python3 - "$1" <<'PYTHON'
import ipaddress, sys
try:
    net = ipaddress.ip_network(sys.argv[1], strict=False)
    if net.prefixlen == 0:
        raise ValueError("unrestricted network")
except ValueError as error:
    print(error, file=sys.stderr)
    raise SystemExit(1)
PYTHON
}
[[ "$EUID" -eq 0 ]] || fail "Run with sudo: sudo ./install.sh"
[[ -f "$PROD/compose.yml" && -f "$PROD/deploy.sh" ]] || fail "Run from the root of a complete Monitor Me checkout."
[[ -f /etc/os-release ]] || fail "Cannot identify this Linux distribution."
. /etc/os-release
case "$ID" in ubuntu|debian) ;; *) fail "Automated control-plane installation currently supports Ubuntu/Debian only." ;; esac
[[ "$(uname -m)" == "x86_64" ]] || fail "Production control plane currently targets Linux x86-64."

log "Installing Docker Engine/Compose when missing, plus Nginx, Certbot, UFW and utilities."
export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y ca-certificates curl openssl python3 nginx certbot ufw gnupg
if ! command -v docker >/dev/null 2>&1; then
  install -m 0755 -d /etc/apt/keyrings
  case "$ID" in
    ubuntu)
      DOCKER_REPO_OS=ubuntu
      DOCKER_SUITE="${UBUNTU_CODENAME:-$VERSION_CODENAME}"
      ;;
    debian)
      DOCKER_REPO_OS=debian
      DOCKER_SUITE="$VERSION_CODENAME"
      ;;
  esac
  curl -fsSL "https://download.docker.com/linux/$DOCKER_REPO_OS/gpg" -o /etc/apt/keyrings/docker.asc
  chmod a+r /etc/apt/keyrings/docker.asc
  printf 'Types: deb\nURIs: https://download.docker.com/linux/%s\nSuites: %s\nComponents: stable\nArchitectures: %s\nSigned-By: /etc/apt/keyrings/docker.asc\n' "$DOCKER_REPO_OS" "$DOCKER_SUITE" "$(dpkg --print-architecture)" > /etc/apt/sources.list.d/docker.sources
  apt-get update
  apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
fi
if ! docker compose version >/dev/null 2>&1; then
  if ! apt-get install -y docker-compose-v2 2>/dev/null; then
    if ! grep -q '^URIs: https://download.docker.com/' /etc/apt/sources.list.d/docker.sources 2>/dev/null; then
      install -m 0755 -d /etc/apt/keyrings
      case "$ID" in
        ubuntu)
          DOCKER_REPO_OS=ubuntu
          DOCKER_SUITE="${UBUNTU_CODENAME:-$VERSION_CODENAME}"
          ;;
        debian)
          DOCKER_REPO_OS=debian
          DOCKER_SUITE="$VERSION_CODENAME"
          ;;
      esac
      curl -fsSL "https://download.docker.com/linux/$DOCKER_REPO_OS/gpg" -o /etc/apt/keyrings/docker.asc
      chmod a+r /etc/apt/keyrings/docker.asc
      printf 'Types: deb\nURIs: https://download.docker.com/linux/%s\nSuites: %s\nComponents: stable\nArchitectures: %s\nSigned-By: /etc/apt/keyrings/docker.asc\n' "$DOCKER_REPO_OS" "$DOCKER_SUITE" "$(dpkg --print-architecture)" > /etc/apt/sources.list.d/docker.sources
      apt-get update
    fi
    apt-get install -y docker-compose-plugin
  fi
fi
systemctl enable --now docker nginx
docker compose version >/dev/null 2>&1 || fail "Docker Compose v2 is unavailable."

PUBLIC_IPV4="$(curl -4 --connect-timeout 5 --max-time 12 -fsS https://api.ipify.org 2>/dev/null || true)"
SSH_CLIENT_VALUE="$(printenv SSH_CLIENT 2>/dev/null || true)"
SSH_CLIENT_IP="$(printf '%s\n' "$SSH_CLIENT_VALUE" | awk '{print $1}')"
DEFAULT_ADMIN=""
if [[ -n "$SSH_CLIENT_IP" ]]; then
  if [[ "$SSH_CLIENT_IP" == *:* ]]; then DEFAULT_ADMIN="$SSH_CLIENT_IP/128"; else DEFAULT_ADMIN="$SSH_CLIENT_IP/32"; fi
fi

if [[ -f "$ENV_FILE" ]]; then
  log "Existing .env.production found; preserving its secrets."
  PUBLIC_HOSTNAME="$(sed -n 's/^PUBLIC_HOSTNAME=//p' "$ENV_FILE" | tail -n 1 | tr -d '"' | tr -d '\r')"
  ADMIN_CIDR="$(sed -n 's/^ADMIN_CIDR=//p' "$ENV_FILE" | tail -n 1 | tr -d '"' | tr -d '\r')"
  LE_EMAIL="$(sed -n 's/^LE_EMAIL=//p' "$ENV_FILE" | tail -n 1 | tr -d '"' | tr -d '\r')"
  [[ -n "$PUBLIC_HOSTNAME" && -n "$ADMIN_CIDR" ]] || fail "Existing .env.production must define PUBLIC_HOSTNAME and ADMIN_CIDR."
else
  printf '\nA DNS hostname with an A record pointing to this VPS is required for trusted HTTPS.\n'
  printf 'The dashboard has no login and will be restricted to the administrator CIDR.\n\n'
  PUBLIC_HOSTNAME="$(prompt 'Public DNS hostname (example monitor.example.com)' '')"
  valid_domain "$PUBLIC_HOSTNAME" || fail "Enter a valid fully-qualified DNS hostname."
  ADMIN_CIDR="$(prompt 'Administrator public IP/CIDR' "$DEFAULT_ADMIN")"
  valid_cidr "$ADMIN_CIDR" || fail "ADMIN_CIDR must be a specific IPv4/IPv6 CIDR, not a default route."
  LE_EMAIL="$(prompt 'Email for TLS certificate expiry notices' '')"
  [[ "$LE_EMAIL" == *@*.* ]] || fail "Enter a valid email address."
  if [[ -n "$PUBLIC_IPV4" ]]; then
    RESOLVED_IPV4="$(getent ahostsv4 "$PUBLIC_HOSTNAME" | awk 'NR==1 {print $1}' || true)"
    [[ -n "$RESOLVED_IPV4" ]] || fail "DNS for $PUBLIC_HOSTNAME has no IPv4 A record. Point it to this control plane or its HTTPS load balancer."
  fi
  POSTGRES_PASSWORD="$(openssl rand -hex 32)"
  REDIS_PASSWORD="$(openssl rand -hex 32)"
  CLICKHOUSE_PASSWORD="$(openssl rand -hex 32)"
  CERT_GROUP_ID="$(getent group monitor-me-cert | cut -d: -f3 || true)"
  if [[ -z "$CERT_GROUP_ID" ]]; then
    CERT_GROUP_ID=15001
    while getent group "$CERT_GROUP_ID" >/dev/null 2>&1; do CERT_GROUP_ID=$((CERT_GROUP_ID + 1)); done
  fi
  cat > "$ENV_FILE" <<ENV
MONITOR_ME_RELEASE=0.1.0
PUBLIC_HOSTNAME=$PUBLIC_HOSTNAME
LE_EMAIL=$LE_EMAIL
AGENT_GRPC_PUBLIC_ADDRESS=$PUBLIC_HOSTNAME:8443
ADMIN_CIDR=$ADMIN_CIDR
AGENT_GRPC_BIND_ADDRESS=0.0.0.0
AGENT_GRPC_PORT=8443
API_HTTP_BIND_PORT=18080
WEB_BIND_PORT=13000
AGENT_TLS_CERT_DIR=$CERT_DIR
CERT_GROUP_ID=$CERT_GROUP_ID
POSTGRES_DB=monitor_me
POSTGRES_USER=monitor_me
POSTGRES_PASSWORD=$POSTGRES_PASSWORD
REDIS_PASSWORD=$REDIS_PASSWORD
CLICKHOUSE_PASSWORD=$CLICKHOUSE_PASSWORD
CLICKHOUSE_IMAGE=clickhouse/clickhouse-server:26.8.44.3
DATABASE_URL=postgres://monitor_me:$POSTGRES_PASSWORD@postgres:5432/monitor_me?sslmode=disable
REMOTE_ALLOWED_SYSTEMD_UNITS=
ENV
  chmod 600 "$ENV_FILE"
fi

valid_domain "$PUBLIC_HOSTNAME" || fail "PUBLIC_HOSTNAME in .env.production must be a valid FQDN."
valid_cidr "$ADMIN_CIDR" || fail "ADMIN_CIDR in .env.production is invalid or unrestricted."
if [[ -z "$LE_EMAIL" ]]; then
  LE_EMAIL="$(prompt 'Email for TLS certificate expiry notices' '')"
  [[ "$LE_EMAIL" == *@*.* ]] || fail "Enter a valid LE_EMAIL in .env.production."
  printf '\nLE_EMAIL=%s\n' "$LE_EMAIL" >> "$ENV_FILE"
  chmod 600 "$ENV_FILE"
fi
[[ "$LE_EMAIL" == *@*.* ]] || fail "Enter a valid LE_EMAIL in .env.production."
if [[ -n "$PUBLIC_IPV4" ]]; then
  RESOLVED_IPV4="$(getent ahostsv4 "$PUBLIC_HOSTNAME" | awk 'NR==1 {print $1}' || true)"
  [[ -n "$RESOLVED_IPV4" ]] || fail "DNS for $PUBLIC_HOSTNAME has no IPv4 A record. Point it to this control plane or its HTTPS load balancer."
fi

CERT_GROUP_ID="$(sed -n 's/^CERT_GROUP_ID=//p' "$ENV_FILE" | tail -n 1)"
[[ "$CERT_GROUP_ID" =~ ^[0-9]+$ ]] || fail "CERT_GROUP_ID in .env.production must be numeric."
existing_group_name="$(getent group "$CERT_GROUP_ID" | cut -d: -f1 || true)"
existing_monitor_group_id="$(getent group monitor-me-cert | cut -d: -f3 || true)"
if [[ -n "$existing_group_name" && "$existing_group_name" != "monitor-me-cert" ]]; then
  fail "CERT_GROUP_ID $CERT_GROUP_ID is already assigned to the unrelated system group $existing_group_name. Choose an unused ID or repair .env.production."
fi
if [[ -n "$existing_monitor_group_id" && "$existing_monitor_group_id" != "$CERT_GROUP_ID" ]]; then
  fail "The monitor-me-cert group already exists with GID $existing_monitor_group_id, but .env.production requests $CERT_GROUP_ID."
fi
if [[ -z "$existing_group_name" ]]; then groupadd --system --gid "$CERT_GROUP_ID" monitor-me-cert; fi
CERT_GROUP_ID="$(getent group monitor-me-cert | cut -d: -f3)"
install -d -o root -g "$CERT_GROUP_ID" -m 0750 "$CERT_DIR"
install -d -m 0755 /var/www/certbot /etc/nginx/snippets

log "Obtaining trusted HTTPS certificate for $PUBLIC_HOSTNAME."
if [[ ! -s "/etc/letsencrypt/live/$PUBLIC_HOSTNAME/fullchain.pem" ]]; then
  sed "s/REPLACE_WITH_PUBLIC_HOSTNAME/$PUBLIC_HOSTNAME/g" "$PROD/nginx-acme-bootstrap.conf.example" > /etc/nginx/conf.d/monitor-me-acme.conf
  nginx -t
  systemctl reload nginx
  certbot certonly --webroot -w /var/www/certbot -d "$PUBLIC_HOSTNAME" --email "$LE_EMAIL" --agree-tos --non-interactive --keep-until-expiring || fail "Let's Encrypt failed. Verify DNS and public TCP 80, then rerun."
fi

log "Installing restricted certificate copy and renewal hook."
RENEWAL_HOOK="/etc/letsencrypt/renewal-hooks/deploy/monitor-me.sh"
printf '#!/usr/bin/env bash\nset -Eeuo pipefail\nexport CERT_GROUP_ID="%s"\nexport AGENT_TLS_CERT_DIR="%s"\nexport MONITOR_ME_ROOT="%s"\nexec bash "%s/deploy/production/certbot-deploy-hook.sh"\n' "$CERT_GROUP_ID" "$CERT_DIR" "$ROOT" "$ROOT" > "$RENEWAL_HOOK"
chmod 0750 "$RENEWAL_HOOK"
RENEWED_LINEAGE="/etc/letsencrypt/live/$PUBLIC_HOSTNAME" CERT_GROUP_ID="$CERT_GROUP_ID" AGENT_TLS_CERT_DIR="$CERT_DIR" "$RENEWAL_HOOK"

log "Enabling automatic certificate renewal."
if systemctl list-unit-files --no-legend certbot.timer 2>/dev/null | awk '{print $1}' | grep -qx 'certbot.timer'; then
  systemctl enable --now certbot.timer
else
  cat > /etc/systemd/system/monitor-me-certbot-renew.service <<'UNIT'
[Unit]
Description=Renew Monitor Me HTTPS and gRPC certificates

[Service]
Type=oneshot
ExecStart=/usr/bin/certbot renew -q
UNIT
  cat > /etc/systemd/system/monitor-me-certbot-renew.timer <<'UNIT'
[Unit]
Description=Twice-daily check for Monitor Me certificate renewal

[Timer]
OnCalendar=daily
RandomizedDelaySec=3600
Persistent=true

[Install]
WantedBy=timers.target
UNIT
  systemctl daemon-reload
  systemctl enable --now monitor-me-certbot-renew.timer
fi

log "Installing HTTPS Nginx site and administrator CIDR allowlist."
sed "s/REPLACE_WITH_PUBLIC_HOSTNAME/$PUBLIC_HOSTNAME/g" "$PROD/nginx.conf.example" > "$NGINX_SITE"
printf 'allow %s;\ndeny all;\n' "$ADMIN_CIDR" > "$NGINX_ACCESS"
chmod 0644 "$NGINX_SITE" "$NGINX_ACCESS"
rm -f /etc/nginx/conf.d/monitor-me-acme.conf
nginx -t
systemctl reload nginx

log "Checking the active host firewall without enabling competing managers."
SSH_PORT=22
if command -v sshd >/dev/null 2>&1; then
  detected_ssh_port="$(sshd -T 2>/dev/null | awk '$1 == "port" {print $2; exit}' || true)"
  [[ -z "$detected_ssh_port" ]] || SSH_PORT="$detected_ssh_port"
fi
UFW_ACTIVE=false
FIREWALLD_ACTIVE=false
if ufw status 2>/dev/null | grep -q '^Status: active'; then UFW_ACTIVE=true; fi
if command -v firewall-cmd >/dev/null 2>&1 && firewall-cmd --state >/dev/null 2>&1; then FIREWALLD_ACTIVE=true; fi
if [[ "$UFW_ACTIVE" == true && "$FIREWALLD_ACTIVE" == true ]]; then
  fail "Both UFW and firewalld are active. Resolve the competing managers manually, then rerun the installer."
elif [[ "$UFW_ACTIVE" == true ]]; then
  ufw allow 80/tcp
  log "UFW is active; Monitor Me firewall rules will be added to UFW."
elif [[ "$FIREWALLD_ACTIVE" == true ]]; then
  firewall_zone="$(firewall-cmd --get-default-zone)"
  firewall-cmd --permanent --zone="$firewall_zone" --add-port=80/tcp
  firewall-cmd --reload
  log "firewalld is active; the ACME HTTP challenge rule was added to zone $firewall_zone."
else
  printf 'No active UFW/firewalld manager found. The installer can enable UFW with inbound deny, outbound allow and SSH restricted to %s.\n' "$ADMIN_CIDR"
  read -r -p 'Enable UFW with this policy? [Y/n]: ' ENABLE_FIREWALL
  [[ "$ENABLE_FIREWALL" != "n" && "$ENABLE_FIREWALL" != "N" ]] || fail "A host firewall is required before exposing a no-login dashboard."
  ufw default deny incoming
  ufw default allow outgoing
  ufw allow from "$ADMIN_CIDR" to any port "$SSH_PORT" proto tcp
  ufw allow 80/tcp
  ufw allow 443/tcp
  ufw allow 8443/tcp
  ufw --force enable
fi

log "Building and starting Monitor Me, applying SQL migrations and checking readiness."
cd "$ROOT"
./deploy/production/deploy.sh

cat <<SUMMARY

Monitor Me is installed.

Dashboard: https://$PUBLIC_HOSTNAME
Admin network: $ADMIN_CIDR
Agent gRPC endpoint: $PUBLIC_HOSTNAME:8443
Data: persistent PostgreSQL, Redis and ClickHouse Docker volumes.

The dashboard has no application login. Access is restricted by Nginx/UFW to ADMIN_CIDR.
Open the dashboard, select Add machine, then run the generated install command on each target host.

Also allow TCP 80, 443 and 8443 in your VPS/cloud provider firewall.
Provider security groups cannot be changed without provider credentials.
Keep .env.production private and back it up securely.
SUMMARY
