#!/bin/sh
set -eu

apk add --no-cache protobuf

export PATH="$(go env GOPATH)/bin:$PATH"
go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.12
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.2

sh scripts/generate-proto.sh
