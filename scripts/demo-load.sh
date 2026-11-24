#!/usr/bin/env bash
set -Eeuo pipefail

DURATION=120
CPU_LOAD=55
MEMORY=128M
DISK=128M
CPU_WORKERS=1

while (($#)); do
  case "$1" in
    --duration) DURATION="${2:?Missing seconds}"; shift 2 ;;
    --cpu-workers) CPU_WORKERS="${2:?Missing worker count}"; shift 2 ;;
    --cpu-load) CPU_LOAD="${2:?Missing CPU percentage}"; shift 2 ;;
    --memory) MEMORY="${2:?Missing memory size, for example 128M}"; shift 2 ;;
    --disk) DISK="${2:?Missing temporary disk size, for example 128M}"; shift 2 ;;
    -h|--help)
      cat <<'HELP'
Usage: ./scripts/demo-load.sh [options]
  --duration SECONDS       Demo duration (default: 120, maximum: 300)
  --cpu-workers COUNT      CPU workers (default: 1, maximum: 2)
  --cpu-load PERCENT       CPU load per worker (default: 55, maximum: 75)
  --memory SIZE            Memory stress (default: 128M)
  --disk SIZE              Temporary disk I/O budget (default: 128M)

Runs real, bounded CPU/memory/disk activity on this Linux host.
Run only on a test or non-production host. Requires stress-ng.
HELP
      exit 0
      ;;
    *) printf 'Unknown option: %s\n' "$1" >&2; exit 2 ;;
  esac
done

[[ "$(uname -s)" == "Linux" ]] || { echo "This demo is for Linux hosts." >&2; exit 2; }
command -v stress-ng >/dev/null 2>&1 || {
  echo "stress-ng is required. Install it with: sudo apt-get update && sudo apt-get install -y stress-ng" >&2
  exit 2
}
[[ "$DURATION" =~ ^[0-9]+$ ]] && (( DURATION >= 15 && DURATION <= 300 )) || { echo "Duration must be between 15 and 300 seconds." >&2; exit 2; }
[[ "$CPU_WORKERS" =~ ^[1-2]$ ]] || { echo "CPU workers must be 1 or 2." >&2; exit 2; }
[[ "$CPU_LOAD" =~ ^[0-9]+$ ]] && (( CPU_LOAD >= 10 && CPU_LOAD <= 75 )) || { echo "CPU load must be between 10 and 75 percent." >&2; exit 2; }
[[ "$MEMORY" =~ ^[0-9]+[KMG]$ ]] || { echo "Memory must be a bounded size such as 128M." >&2; exit 2; }
[[ "$DISK" =~ ^[0-9]+[KMG]$ ]] || { echo "Disk must be a bounded size such as 128M." >&2; exit 2; }

available_kb="$(awk 'NR==2 {print $4}' < <(df -Pk /tmp))"
(( available_kb >= 1048576 )) || { echo "At least 1 GiB free in /tmp is required for the demo." >&2; exit 2; }

cat <<INFO
Monitor Me real-data demo
  duration:    $DURATION seconds
  CPU workers: $CPU_WORKERS at up to $CPU_LOAD percent each
  memory:      $MEMORY
  temporary I/O budget: $DISK

This runs actual load on this machine. Do not run it on a critical production workload.
INFO
read -r -p "Start the bounded demo load? [y/N]: " answer
[[ "$answer" == "y" || "$answer" == "Y" ]] || { echo "Cancelled."; exit 0; }

exec stress-ng \
  --cpu "$CPU_WORKERS" --cpu-load "$CPU_LOAD" \
  --vm 1 --vm-bytes "$MEMORY" --vm-keep \
  --hdd 1 --hdd-bytes "$DISK" \
  --timeout "${DURATION}s" --metrics-brief
