FROM golang:1.27.4-alpine3.23

WORKDIR /src

RUN apk add --no-cache protobuf && \
    go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.12 && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.2

ENV PATH="/go/bin:${PATH}"
ENV MONITOR_AGENT_ARTIFACT_DIR=/out

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
    chmod 0755 /usr/local/bin/build-agent-artifacts

ENTRYPOINT ["/usr/local/bin/build-agent-artifacts"]
