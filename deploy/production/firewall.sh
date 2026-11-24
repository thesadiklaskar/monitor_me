#!/usr/bin/env bash
set -Eeuo pipefail
ROLE=""
MANAGER="auto"
ZONE=""
SSH_PORT=22
GRPC_PORT=8443
ALLOW_HTTP=false
APPLY=false
ROLLBACK=false
VERIFY_NO_PUBLIC_HTTPS=false
STATE_DIR="${MONITOR_ME_FIREWALL_STATE_DIR:-/var/lib/monitor-me/firewall}"
ADMIN_CIDRS=()
GRPC_CIDRS=()
usage() {
  cat <<'USAGE'
Usage:
  firewall.sh --role control-plane --admin-cidr CIDR [--admin-cidr CIDR] [--grpc-cidr CIDR] [--manager auto|ufw|firewalld] [--zone ZONE] [--ssh-port PORT] [--grpc-port PORT] [--http-01] [--apply] [--verify-no-public-https]
  firewall.sh --role agent-only
  firewall.sh --rollback
Dry-run is the default. This helper adds allow rules only; it never changes default policies or removes existing rules.
USAGE
}
valid_port() { [[ "$1" =~ ^[0-9]+$ ]] && (( 10#$1 >= 1 && 10#$1 <= 65535 )); }
while (($#)); do
  case "$1" in
    --role) ROLE="${2:?}"; shift 2 ;;
    --manager) MANAGER="${2:?}"; shift 2 ;;
    --zone) ZONE="${2:?}"; shift 2 ;;
    --admin-cidr) ADMIN_CIDRS+=("${2:?}"); shift 2 ;;
    --grpc-cidr) GRPC_CIDRS+=("${2:?}"); shift 2 ;;
    --ssh-port) SSH_PORT="${2:?}"; shift 2 ;;
    --grpc-port) GRPC_PORT="${2:?}"; shift 2 ;;
    --http-01) ALLOW_HTTP=true; shift ;;
    --apply) APPLY=true; shift ;;
    --rollback) ROLLBACK=true; shift ;;
    --verify-no-public-https) VERIFY_NO_PUBLIC_HTTPS=true; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown argument: $1" >&2; usage; exit 2 ;;
  esac
done
if [[ "$ROLLBACK" == true ]]; then
  [[ "$EUID" -eq 0 ]] || { echo "Rollback requires root." >&2; exit 2; }
  state_file="$STATE_DIR/changes.tsv"
  [[ -s "$state_file" ]] || { echo "No recorded firewall changes to roll back." >&2; exit 1; }
  mapfile -t changes < "$state_file"
  firewalld_changed=false
  for ((index=${#changes[@]}-1; index>=0; index--)); do
    IFS='|' read -r manager kind first second third <<< "${changes[$index]}"
    case "$manager:$kind" in
      ufw:source) ufw --force delete allow from "$first" to any port "$second" proto tcp ;;
      ufw:port) ufw --force delete allow "$second/tcp" ;;
      firewalld:source)
        family=ipv4; [[ "$first" == *:* ]] && family=ipv6
        rule="rule family=\"$family\" source address=\"$first\" port port=\"$third\" protocol=\"tcp\" accept"
        firewall-cmd --permanent --zone="$second" --remove-rich-rule="$rule"
        firewalld_changed=true
        ;;
      firewalld:port)
        firewall-cmd --permanent --zone="$first" --remove-port="$second"
        firewalld_changed=true
        ;;
      *) echo "Unrecognized rollback record: ${changes[$index]}" >&2; exit 1 ;;
    esac
  done
  if [[ "$firewalld_changed" == true ]]; then firewall-cmd --reload; fi
  mv "$state_file" "$state_file.rolled-back.$(date -u +%Y%m%dT%H%M%SZ)"
  echo "Recorded Monitor Me additions removed. Existing rules and default policies were left untouched."
  exit 0
fi
[[ "$ROLE" == "control-plane" || "$ROLE" == "agent-only" ]] || { usage; exit 2; }
valid_port "$SSH_PORT" || { echo "Invalid SSH port." >&2; exit 2; }
valid_port "$GRPC_PORT" || { echo "Invalid gRPC port." >&2; exit 2; }
if [[ "$ROLE" == "agent-only" ]]; then
  echo "Agent-only host: no inbound Monitor Me firewall rules are required."
  echo "No changes made. When outbound traffic is restricted, allow TCP $GRPC_PORT only to the control-plane address."
  exit 0
fi
(${#ADMIN_CIDRS[@]} > 0) || { echo "Control-plane mode requires --admin-cidr; SSH must never be opened to the world." >&2; exit 2; }
for cidr in "${ADMIN_CIDRS[@]}" "${GRPC_CIDRS[@]}"; do
  python3 -c 'import ipaddress,sys; ipaddress.ip_network(sys.argv[1], strict=False)' "$cidr" || { echo "Invalid administrator CIDR: $cidr" >&2; exit 2; }
done
if [[ "$MANAGER" == "auto" ]]; then
  ufw_active=false; firewalld_active=false
  if command -v ufw >/dev/null 2>&1 && ufw status 2>/dev/null | grep -q '^Status: active'; then ufw_active=true; fi
  if command -v firewall-cmd >/dev/null 2>&1 && firewall-cmd --state >/dev/null 2>&1; then firewalld_active=true; fi
  if [[ "$ufw_active" == true && "$firewalld_active" == true ]]; then echo "Both UFW and firewalld appear active; resolve the competing managers manually." >&2; exit 2
  elif [[ "$ufw_active" == true ]]; then MANAGER=ufw
  elif [[ "$firewalld_active" == true ]]; then MANAGER=firewalld
  else echo "No active UFW or firewalld manager detected; no changes made." >&2; exit 2; fi
fi
if [[ "$MANAGER" == "ufw" ]]; then
  command -v ufw >/dev/null || { echo "UFW is not installed." >&2; exit 2; }
  ufw status 2>/dev/null | grep -q '^Status: active' || { echo "UFW is inactive; this helper will not enable it or set defaults." >&2; exit 2; }
elif [[ "$MANAGER" == "firewalld" ]]; then
  command -v firewall-cmd >/dev/null || { echo "firewalld is not installed." >&2; exit 2; }
  firewall-cmd --state >/dev/null 2>&1 || { echo "firewalld is not running." >&2; exit 2; }
  ZONE="${ZONE:-$(firewall-cmd --get-default-zone)}"
  firewall-cmd --get-active-zones | grep -Fxq "$ZONE" || { echo "Zone $ZONE is not active; inspect active interfaces and pass --zone." >&2; exit 2; }
else
  echo "Unsupported manager: $MANAGER (supported: UFW and firewalld)." >&2; exit 2
fi
if [[ "$APPLY" == true ]]; then
  [[ "$EUID" -eq 0 ]] || { echo "--apply requires root." >&2; exit 2; }
  mkdir -p "$STATE_DIR"; chmod 700 "$STATE_DIR"
  touch "$STATE_DIR/changes.tsv"; chmod 600 "$STATE_DIR/changes.tsv"
fi
add_ufw_source() {
  local cidr="$1" port="$2"
  if ufw status | grep -F "$port/tcp" | grep -Fq "$cidr"; then echo "Already present: TCP $port from $cidr"; return; fi
  echo "Allow TCP $port only from $cidr"
  if [[ "$APPLY" == true ]]; then
    ufw allow from "$cidr" to any port "$port" proto tcp
    printf 'ufw|source|%s|%s|\n' "$cidr" "$port" >> "$STATE_DIR/changes.tsv"
  fi
}
add_ufw_port() {
  local port="$1"
  if ufw status | grep -Fq "$port/tcp"; then echo "Already present: TCP $port"; return; fi
  echo "Allow TCP $port"
  if [[ "$APPLY" == true ]]; then ufw allow "$port/tcp"; printf 'ufw|port||%s|\n' "$port" >> "$STATE_DIR/changes.tsv"; fi
}
add_firewalld_source() {
  local cidr="$1" port="$2" family rule
  family=ipv4; [[ "$cidr" == *:* ]] && family=ipv6
  rule="rule family=\"$family\" source address=\"$cidr\" port port=\"$port\" protocol=\"tcp\" accept"
  if firewall-cmd --permanent --zone="$ZONE" --query-rich-rule="$rule" >/dev/null; then echo "Already present: TCP $port from $cidr in $ZONE"; return; fi
  echo "Allow TCP $port only from $cidr in $ZONE"
  if [[ "$APPLY" == true ]]; then
    firewall-cmd --permanent --zone="$ZONE" --add-rich-rule="$rule"
    printf 'firewalld|source|%s|%s|%s\n' "$cidr" "$ZONE" "$port" >> "$STATE_DIR/changes.tsv"
  fi
}
add_firewalld_port() {
  local port="$1"
  if firewall-cmd --permanent --zone="$ZONE" --query-port="$port/tcp" >/dev/null; then echo "Already present: TCP $port in $ZONE"; return; fi
  echo "Allow TCP $port in $ZONE"
  if [[ "$APPLY" == true ]]; then
    firewall-cmd --permanent --zone="$ZONE" --add-port="$port/tcp"
    printf 'firewalld|port|%s|%s/tcp|\n' "$ZONE" "$port" >> "$STATE_DIR/changes.tsv"
  fi
}
if [[ "$MANAGER" == "ufw" ]]; then
  for cidr in "${ADMIN_CIDRS[@]}"; do add_ufw_source "$cidr" "$SSH_PORT"; done
  for cidr in "${ADMIN_CIDRS[@]}"; do add_ufw_source "$cidr" 443; done
  if ((${#GRPC_CIDRS[@]} > 0)); then
    for cidr in "${GRPC_CIDRS[@]}"; do add_ufw_source "$cidr" "$GRPC_PORT"; done
  else
    add_ufw_port "$GRPC_PORT"
  fi
  if [[ "$ALLOW_HTTP" == true ]]; then add_ufw_port 80; fi
else
  for cidr in "${ADMIN_CIDRS[@]}"; do add_firewalld_source "$cidr" "$SSH_PORT"; done
  for cidr in "${ADMIN_CIDRS[@]}"; do add_firewalld_source "$cidr" 443; done
  if ((${#GRPC_CIDRS[@]} > 0)); then
    for cidr in "${GRPC_CIDRS[@]}"; do add_firewalld_source "$cidr" "$GRPC_PORT"; done
  else
    add_firewalld_port "$GRPC_PORT"
  fi
  if [[ "$ALLOW_HTTP" == true ]]; then add_firewalld_port 80; fi
  if [[ "$APPLY" == true ]]; then firewall-cmd --reload; fi
fi
if [[ "$VERIFY_NO_PUBLIC_HTTPS" == true ]]; then
  if [[ "$MANAGER" == "ufw" ]]; then
    broad_https="$(ufw status | awk '/443\/tcp/ && /Anywhere/ {print} /Nginx Full|Apache Full/ && /Anywhere/ {print}')"
    if [[ -n "$broad_https" ]]; then
      echo "Refusing no-login mode: existing unrestricted UFW HTTPS rules remain:" >&2
      printf '%s\n' "$broad_https" >&2
      echo "Remove or narrow these rules explicitly, then re-run verification." >&2
      exit 3
    fi
  else
    if firewall-cmd --permanent --zone="$ZONE" --query-service=https >/dev/null 2>&1 ||
       firewall-cmd --permanent --zone="$ZONE" --query-port=443/tcp >/dev/null 2>&1; then
      echo "Refusing no-login mode: firewalld still allows HTTPS through a public service/port rule." >&2
      exit 3
    fi
    broad_rich_rules="$(firewall-cmd --permanent --zone="$ZONE" --list-rich-rules | grep -E 'port port="443"' | grep -v 'source address=' || true)"
    if [[ -n "$broad_rich_rules" ]]; then
      echo "Refusing no-login mode: unrestricted firewalld HTTPS rich rules remain:" >&2
      printf '%s\n' "$broad_rich_rules" >&2
      exit 3
    fi
  fi
  echo "Verified: no unrestricted host-firewall HTTPS rule was found in $MANAGER."
fi

if [[ "$APPLY" == true ]]; then
  echo "Rules added. Verify SSH from a second session and test IPv4/IPv6 externally."
  echo "Existing broad rules/default policies were not changed. Inspect them before claiming least-privilege."
  echo "Rollback additions: $STATE_DIR/changes.tsv"
else
  echo "Dry run only. Add --apply after reviewing host role, current rules, SSH access and active zone."
fi
