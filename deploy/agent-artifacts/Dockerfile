FROM golang:1.27.4-alpine3.23 AS build

WORKDIR /src

RUN apk add --no-cache protobuf && \
    go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.12 && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.2

ENV PATH="/go/bin:${PATH}"

ARG MONITOR_AGENT_VERSION=0.1.0-dev
ENV MONITOR_AGENT_VERSION=${MONITOR_AGENT_VERSION}
ENV MONITOR_AGENT_ARTIFACT_DIR=/bundle

COPY agent ./agent
COPY proto ./proto
COPY scripts/build-agent-artifacts.sh /usr/local/bin/build-agent-artifacts

RUN protoc -I proto \
    --go_out=agent \
    --go_opt=module=example.com/monitor-me/agent \
    "--go_opt=Mmonitor/v1/agent.proto=example.com/monitor-me/agent/gen/monitor/v1;monitorv1" \
    --go-grpc_out=agent \
    --go-grpc_opt=module=example.com/monitor-me/agent \
    "--go-grpc_opt=Mmonitor/v1/agent.proto=example.com/monitor-me/agent/gen/monitor/v1;monitorv1" \
    proto/monitor/v1/agent.proto && \
    chmod 0755 /usr/local/bin/build-agent-artifacts && \
    /usr/local/bin/build-agent-artifacts

FROM alpine:3.23 AS runtime

RUN apk add --no-cache ca-certificates

COPY --from=build /bundle /bundle
COPY scripts/publish-agent-artifacts.sh /usr/local/bin/publish-agent-artifacts

RUN chmod 0755 /usr/local/bin/publish-agent-artifacts

ENV MONITOR_AGENT_ARTIFACT_DIR=/out

ENTRYPOINT ["/usr/local/bin/publish-agent-artifacts"]
