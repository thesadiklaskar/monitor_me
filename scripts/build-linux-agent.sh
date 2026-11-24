#!/bin/sh
set -eu

apk add --no-cache protobuf
export PATH="$(go env GOPATH)/bin:$PATH"
go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.12
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.2

sed -i 's/\r$//' scripts/generate-proto.sh
sh scripts/generate-proto.sh

mkdir -p /src/.data/linux-agent
cd agent
go mod tidy
GOOS=linux GOARCH="$(go env GOARCH)" CGO_ENABLED=0 go build \
  -trimpath \
  -ldflags="-s -w" \
  -o /src/.data/linux-agent/monitor-agent \
  ./cmd/monitor-agent
