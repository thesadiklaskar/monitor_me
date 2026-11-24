#!/usr/bin/env sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
PATH="$(go env GOPATH)/bin:$PATH"
export PATH

protoc -I "$ROOT_DIR/proto" \
  --go_out="$ROOT_DIR/backend" \
  --go_opt=module=example.com/monitor-me/backend \
  --go-grpc_out="$ROOT_DIR/backend" \
  --go-grpc_opt=module=example.com/monitor-me/backend \
  "$ROOT_DIR/proto/monitor/v1/agent.proto"

mkdir -p "$ROOT_DIR/agent/gen"
protoc -I "$ROOT_DIR/proto" \
  --go_out="$ROOT_DIR/agent" \
  --go_opt=module=example.com/monitor-me/agent \
  "--go_opt=Mmonitor/v1/agent.proto=example.com/monitor-me/agent/gen/monitor/v1;monitorv1" \
  --go-grpc_out="$ROOT_DIR/agent" \
  --go-grpc_opt=module=example.com/monitor-me/agent \
  "--go-grpc_opt=Mmonitor/v1/agent.proto=example.com/monitor-me/agent/gen/monitor/v1;monitorv1" \
  "$ROOT_DIR/proto/monitor/v1/agent.proto"
