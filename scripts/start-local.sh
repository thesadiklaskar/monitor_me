#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "$0")/.." && pwd)"
cd "$ROOT_DIR"

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

if docker compose up --build -d; then
  compose_exit_code=0
else
  compose_exit_code=$?
fi
if [[ "$compose_exit_code" -ne 0 ]]; then
  echo "Docker Compose failed. Recent service state:" >&2
  docker compose ps --all >&2 || true
  echo "Agent artifact builder output:" >&2
  docker compose logs --no-color --tail=200 agent-artifacts >&2 || true
  echo "Recent Compose output:" >&2
  docker compose logs --no-color --tail=100 >&2 || true
  exit "$compose_exit_code"
fi
echo "Waiting for the dashboard..."
for _ in $(seq 1 40); do
  if curl -fsS http://localhost:3000/api/config >/dev/null; then
    echo "Monitor Me is ready at http://localhost:3000"
    exit 0
  fi
  sleep 2
done

echo "Dashboard did not become ready. Inspect: docker compose logs --tail=100" >&2
exit 1
