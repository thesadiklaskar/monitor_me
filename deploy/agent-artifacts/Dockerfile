FROM golang:1.27.4-alpine3.23

WORKDIR /src
COPY agent ./agent
COPY scripts/build-agent-artifacts.sh /usr/local/bin/build-agent-artifacts
RUN chmod 0755 /usr/local/bin/build-agent-artifacts

ENV MONITOR_AGENT_ARTIFACT_DIR=/out
ENTRYPOINT ["/usr/local/bin/build-agent-artifacts"]
