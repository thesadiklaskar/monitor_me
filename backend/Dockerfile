FROM golang:1.27.4-alpine3.23 AS build

WORKDIR /src

COPY backend/go.mod ./
COPY backend/ ./

# Resolve and checksum dependencies after package sources are available to go mod tidy.
RUN go mod tidy

RUN CGO_ENABLED=0 GOOS=linux \
    go build -trimpath -ldflags="-s -w" -o /out/monitor-me-api ./cmd/api && \
    go build -trimpath -ldflags="-s -w" -o /out/monitor-me-migrate ./cmd/migrate

FROM alpine:3.23 AS runtime

RUN addgroup -S -g 10001 monitor && \
    adduser -S -D -H -u 10001 -G monitor monitor && \
    apk add --no-cache ca-certificates

COPY --from=build /out/monitor-me-api /usr/local/bin/monitor-me-api
COPY --from=build /out/monitor-me-migrate /usr/local/bin/monitor-me-migrate

USER 10001:10001

EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=5 \
    CMD wget -q -O /dev/null http://127.0.0.1:8080/health/ready || exit 1

ENTRYPOINT ["/usr/local/bin/monitor-me-api"]
