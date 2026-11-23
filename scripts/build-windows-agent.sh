#!/bin/sh
set -eu

apk add --no-cache protobuf

export PATH="$(go env GOPATH)/bin:$PATH"
go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.12
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.2

sh scripts/generate-proto.sh

cd agent
go mod tidy
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build \
  -trimpath \
  -ldflags="-s -w" \
  -o /src/.data/windows-agent/monitor-agent.exe \
  ./cmd/monitor-agent
