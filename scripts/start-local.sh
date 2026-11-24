#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "$0")/.." && pwd)"
cd "$ROOT_DIR"

assert_current_source() {
  if [[ "${MONITOR_ME_SKIP_SOURCE_CHECK:-0}" == "1" ]]; then
    echo "WARNING: Git source freshness check was explicitly skipped." >&2
    return
  fi
  if ! command -v git >/dev/null 2>&1 || ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    echo "WARNING: Git checkout unavailable; source freshness cannot be verified." >&2
    return
  fi

  local branch local_head origin_url remote_head
  branch="$(git branch --show-current)"
  if [[ "$branch" != "main" ]]; then
    echo "Expected branch 'main'; current branch is '$branch'. No Docker services were changed." >&2
    exit 1
  fi

  local_head="$(git rev-parse --verify HEAD)"
  if ! origin_url="$(git remote get-url origin 2>/dev/null)"; then
    echo "WARNING: No origin remote is configured; source freshness cannot be checked." >&2
    echo "Local source revision: $local_head"
    return
  fi

  if ! git fetch --quiet --no-tags origin main; then
    echo "Could not verify origin/main. Check GitHub access and run 'git pull --ff-only origin main'." >&2
    echo "No Docker services were changed. Set MONITOR_ME_SKIP_SOURCE_CHECK=1 only for an intentional offline snapshot." >&2
    exit 1
  fi

  remote_head="$(git rev-parse --verify FETCH_HEAD)"
  echo "Local source revision:  $local_head"
  echo "Remote main revision:   $remote_head"
  if [[ "$local_head" != "$remote_head" ]]; then
    if git merge-base --is-ancestor HEAD FETCH_HEAD; then
      echo "Local main is behind origin/main. Run 'git pull --ff-only origin main', inspect local changes, then retry." >&2
      exit 1
    fi
    if ! git merge-base --is-ancestor FETCH_HEAD HEAD; then
      echo "Local main and origin/main have diverged. Reconcile history without discarding work, then retry." >&2
      exit 1
    fi
    echo "WARNING: Local main contains commits ahead of origin/main; building the local checkout." >&2
  fi

  if [[ -n "$(git status --short)" ]]; then
    if [[ "${MONITOR_ME_ALLOW_DIRTY_SOURCE:-0}" != "1" ]]; then
      echo "Working tree has local changes. Review and preserve them, then commit/stash before building." >&2
      echo "Set MONITOR_ME_ALLOW_DIRTY_SOURCE=1 only when intentionally building modified source. No Docker services were changed." >&2
      exit 1
    fi
    echo "WARNING: Explicit dirty-source override is enabled; building local modifications." >&2
  fi
}

assert_current_source

if ! docker info >/dev/null 2>&1; then
  echo "Docker Engine is not running. Start Docker and retry." >&2
  exit 1
fi

if [[ ! -f .env && -f .env.example ]]; then
  cp .env.example .env
  echo "Created .env from .env.example."
fi

mkdir -p .data/certs
if [[ ! -s .data/certs/server.crt || ! -s .data/certs/server.key ]]; then
  echo "Generating a local-only development TLS certificate with Docker..."
  docker run --rm --entrypoint /bin/sh \
    --mount "type=bind,source=$ROOT_DIR/.data/certs,target=/certs" \
    alpine:3.23 -c "apk add --no-cache openssl >/dev/null && openssl req -x509 -newkey rsa:2048 -nodes -keyout /certs/server.key -out /certs/server.crt -days 365 -subj '/CN=localhost' -addext 'subjectAltName=DNS:localhost,IP:127.0.0.1' && chmod 644 /certs/server.crt && chmod 600 /certs/server.key"
fi

echo "Validating Docker Compose configuration..."
if ! docker compose config --quiet; then
  echo "Compose configuration is invalid. No services were started or replaced." >&2
  exit 1
fi

echo "Building every application image before changing running services..."
if ! docker compose build; then
  echo "Image build failed. Docker Compose was not asked to start or replace any service." >&2
  echo "Existing container state (may belong to an older source revision):" >&2
  docker compose ps --all >&2 || true
  exit 1
fi

echo "All images built successfully. Starting the stack without rebuilding..."
if ! docker compose up -d --no-build; then
  echo "Docker Compose could not start the freshly built stack." >&2
  docker compose ps --all >&2 || true
  docker compose logs --no-color --since=5m --tail=100 >&2 || true
  exit 1
fi

echo "Waiting for API and dashboard readiness..."
for _ in $(seq 1 40); do
  if curl -fsS --connect-timeout 2 --max-time 3 http://localhost:8080/health/ready >/dev/null \
    && curl -fsS --connect-timeout 2 --max-time 3 http://localhost:3000/api/config >/dev/null \
    && curl -fsS --connect-timeout 2 --max-time 3 http://localhost:3000/grafana/api/health >/dev/null; then
    echo "Monitor Me is ready at http://localhost:3000"
    echo "Grafana is ready at http://localhost:3000/grafana/ (also available at http://localhost:3001)."
    exit 0
  fi
  sleep 2
done

echo "The stack did not become healthy. Recent service state:" >&2
docker compose ps --all >&2 || true
docker compose logs --no-color --since=5m --tail=100 >&2 || true
exit 1
