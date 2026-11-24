#!/usr/bin/env bash
set -Eeuo pipefail

usage() {
  cat <<'USAGE'
Usage:
  sudo ./scripts/linux-preflight.sh --control-plane
  sudo ./scripts/linux-preflight.sh --agent

Read-only checks only. This script does not install packages, open firewall
ports, modify services, or change data.
USAGE
}

fail() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }
pass() { printf 'PASS: %s\n' "$*"; }
note() { printf 'INFO: %s\n' "$*"; }

[[ $# -eq 1 ]] || { usage >&2; exit 2; }
MODE="$1"
[[ "$MODE" == "--control-plane" || "$MODE" == "--agent" ]] || { usage >&2; exit 2; }

[[ "$(uname -s)" == "Linux" ]] || fail "This preflight is for Linux hosts only."
[[ -r /etc/os-release ]] || fail "/etc/os-release is missing."
# shellcheck disable=SC1091
. /etc/os-release

ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64) NORMALIZED_ARCH=amd64 ;;
  aarch64|arm64) NORMALIZED_ARCH=arm64 ;;
  *) fail "Unsupported CPU architecture: $ARCH (supported agent architectures: amd64, arm64)." ;;
esac

note "Distribution: ${PRETTY_NAME:-$ID}"
note "Kernel: $(uname -r)"
note "Architecture: $ARCH ($NORMALIZED_ARCH)"
note "CPU cores: $(getconf _NPROCESSORS_ONLN 2>/dev/null || echo unknown)"
if [[ -r /proc/meminfo ]]; then
  awk '/^MemTotal:/ {printf "INFO: Memory: %.1f GiB\n", $2 / 1048576}' /proc/meminfo
fi

if [[ "$MODE" == "--agent" ]]; then
  [[ "$EUID" -eq 0 ]] || fail "Run with sudo; the installed agent service requires host metrics/log access."
  command -v systemctl >/dev/null 2>&1 || fail "systemd/systemctl is required by the native agent installer."
  command -v curl >/dev/null 2>&1 || fail "curl is required to download the signed-by-checksum agent artifact."
  command -v sha256sum >/dev/null 2>&1 || fail "sha256sum is required to verify the agent artifact."
  for path in /proc/stat /proc/meminfo /proc/diskstats /proc/net/dev; do
    [[ -r "$path" ]] || fail "Required procfs counter is missing or unreadable: $path"
  done
  pass "Native Linux host counters are available through procfs."
  pass "systemd, curl and SHA-256 verification prerequisites are present."
  if command -v journalctl >/dev/null 2>&1; then
    pass "journalctl is available; journald collection can be enabled."
  else
    note "journalctl is unavailable; host metrics can work, but journald ingestion will not."
  fi
  if [[ -r /sys/fs/cgroup/cgroup.controllers ]]; then
    pass "cgroup v2 is available for workload metadata."
  elif [[ -d /sys/fs/cgroup ]]; then
    note "cgroup v2 marker is absent; cgroup/container workload detail may be limited."
  else
    note "cgroup filesystem is unavailable; workload-level metadata will be limited."
  fi
  note "Agent target detected: Linux/$NORMALIZED_ARCH. This is a preflight, not a real-hardware acceptance certificate."
  exit 0
fi

[[ "$EUID" -eq 0 ]] || fail "Run the control-plane preflight with sudo."
[[ "$ID" == "ubuntu" || "$ID" == "debian" ]] || fail "The automated control-plane installer currently supports Ubuntu/Debian only (detected: $ID)."
[[ "$NORMALIZED_ARCH" == "amd64" ]] || fail "The control-plane Compose stack currently targets Linux amd64; ARM64 is not an accepted control-plane target."
command -v apt-get >/dev/null 2>&1 || fail "apt-get is required by the automated control-plane installer."
command -v systemctl >/dev/null 2>&1 || fail "systemd/systemctl is required by the automated control-plane installer."
command -v getent >/dev/null 2>&1 || fail "getent is required for DNS preflight."
command -v python3 >/dev/null 2>&1 || note "python3 is not installed yet; install.sh will install it before validating CIDRs."
pass "Control-plane OS/architecture matches the automated installer target (Ubuntu/Debian amd64)."
if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
  pass "Docker Engine and Compose v2 are already installed."
else
  note "Docker Engine/Compose v2 are missing; install.sh will install them."
fi
if command -v nginx >/dev/null 2>&1; then
  pass "Nginx is installed."
else
  note "Nginx is missing; install.sh will install it."
fi
if command -v certbot >/dev/null 2>&1; then
  pass "Certbot is installed."
else
  note "Certbot is missing; install.sh will install it."
fi
note "Check DNS A/AAAA records and provider/VPC firewall rules separately; this script cannot inspect or change cloud security groups."
note "This is an installer preflight, not a clean-host production acceptance certificate."
