#!/usr/bin/env bash
set -Eeuo pipefail

usage() {
  cat <<'USAGE'
Usage:
  sudo bash ./scripts/linux-preflight.sh --control-plane
  sudo bash ./scripts/linux-preflight.sh --agent

Read-only checks. No packages, services, firewall rules, or data are changed.
USAGE
}
fail() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }
pass() { printf 'PASS: %s\n' "$*"; }
info() { printf 'INFO: %s\n' "$*"; }
[[ $# -eq 1 ]] || { usage >&2; exit 2; }
MODE="$1"
[[ "$MODE" == "--control-plane" || "$MODE" == "--agent" ]] || { usage >&2; exit 2; }
[[ "$(uname -s)" == "Linux" ]] || fail "Linux is required."
[[ -r /etc/os-release ]] || fail "/etc/os-release is unavailable."
# shellcheck disable=SC1091
. /etc/os-release
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64) NORMALIZED_ARCH=amd64 ;;
  aarch64|arm64) NORMALIZED_ARCH=arm64 ;;
  *) fail "Unsupported architecture: $ARCH (native Linux agent targets: amd64, arm64)." ;;
esac
info "Distribution: ${PRETTY_NAME:-$ID}"
info "Kernel: $(uname -r)"
info "Architecture: $ARCH ($NORMALIZED_ARCH)"
info "CPU cores: $(getconf _NPROCESSORS_ONLN 2>/dev/null || echo unknown)"
if [[ -r /proc/meminfo ]]; then awk '/^MemTotal:/ {printf "INFO: Memory: %.1f GiB\n", $2 / 1048576}' /proc/meminfo; fi
[[ "$EUID" -eq 0 ]] || fail "Run this preflight with sudo/root privileges."
if [[ "$MODE" == "--agent" ]]; then
  command -v systemctl >/dev/null 2>&1 || fail "systemd/systemctl is required by the current Linux agent installer."
  command -v curl >/dev/null 2>&1 || fail "curl is required for artifact download."
  command -v sha256sum >/dev/null 2>&1 || fail "sha256sum is required for artifact verification."
  for counter in /proc/stat /proc/meminfo /proc/diskstats /proc/net/dev; do [[ -r "$counter" ]] || fail "Required host counter is missing or unreadable: $counter"; done
  pass "systemd and native procfs host counters are available."
  if command -v journalctl >/dev/null 2>&1; then
    if journalctl --no-pager -n 0 >/dev/null 2>&1; then pass "The system journal is readable."; else info "journalctl exists but the current context cannot read the journal; host logs may be unavailable."; fi
  else
    info "journalctl is absent; journald ingestion is unavailable on this host."
  fi
  if [[ -r /sys/fs/cgroup/cgroup.controllers ]]; then pass "cgroup v2 is available for workload metadata."; else info "cgroup v2 is not detected; workload attribution may be limited."; fi
  info "This agent target still requires clean-install and real hardware/runtime acceptance on the detected distribution."
  exit 0
fi
[[ "$ID" == "ubuntu" || "$ID" == "debian" ]] || fail "Automated control-plane installation currently supports Ubuntu/Debian only (detected: $ID)."
[[ "$NORMALIZED_ARCH" == "amd64" ]] || fail "Automated control-plane installation currently targets amd64 only."
command -v apt-get >/dev/null 2>&1 || fail "apt-get is required."
command -v systemctl >/dev/null 2>&1 || fail "systemd/systemctl is required."
command -v getent >/dev/null 2>&1 || fail "getent is required for DNS checks."
pass "Control-plane OS and architecture match the current installer target."
if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then pass "Docker Engine and Compose v2 are installed."; else info "Docker Engine/Compose v2 must be installed by the guided installer."; fi
if command -v nginx >/dev/null 2>&1; then pass "Nginx is installed."; else info "Nginx will be installed by the guided installer."; fi
if command -v certbot >/dev/null 2>&1; then pass "Certbot is installed."; else info "Certbot will be installed by the guided installer."; fi
info "Configure public DNS and provider/VPC firewall rules separately; this preflight cannot inspect cloud security groups."
info "A preflight pass is not a clean-host production acceptance certificate."
