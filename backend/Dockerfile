FROM golang:1.27.4-alpine3.23 AS build

WORKDIR /src

RUN apk add --no-cache protobuf

COPY backend/go.mod ./
COPY backend/ ./
COPY proto/ ./proto/

RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.12 && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.2

ENV PATH="/go/bin:${PATH}"

RUN protoc -I proto \
    --go_out=. --go_opt=module=monitor-me.internal/backend \
    --go-grpc_out=. --go-grpc_opt=module=monitor-me.internal/backend \
    proto/monitor/v1/agent.proto

# Resolve only the packages required by the shipped binaries. Do not run
# "go mod tidy" in an image build: tidy also loads tests and every build-tagged
# package, and mutates module metadata as a side effect.
RUN mkdir -p /out && \
    CGO_ENABLED=0 GOOS=linux \
    go build -mod=mod -trimpath -ldflags="-s -w" -o /out/monitor-me-api ./cmd/api && \
    CGO_ENABLED=0 GOOS=linux \
    go build -mod=mod -trimpath -ldflags="-s -w" -o /out/monitor-me-migrate ./cmd/migrate

FROM alpine:3.23 AS runtime

RUN addgroup -S -g 10001 monitor && \
    adduser -S -D -H -u 10001 -G monitor monitor && \
    apk add --no-cache ca-certificates

COPY --from=build /out/monitor-me-api /usr/local/bin/monitor-me-api
COPY --from=build /out/monitor-me-migrate /usr/local/bin/monitor-me-migrate

USER 10001:10001

EXPOSE 8080 8443

HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=5 \
    CMD wget -q -O /dev/null http://127.0.0.1:8080/health/ready || exit 1

ENTRYPOINT ["/usr/local/bin/monitor-me-api"]
