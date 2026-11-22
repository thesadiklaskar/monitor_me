# Agent Protocol

The versioned contract is defined in `monitor/v1/agent.proto`.

## Phase 2 Control Plane

- The package name is versioned as `monitor.v1`.
- Agent RPC traffic is served over TLS 1.2 or later.
- `Enroll` consumes a scoped, short-lived enrollment token exactly once.
- The backend stores SHA-256 digests of enrollment and agent-session secrets, never the raw credentials.
- The enrollment response returns an opaque session token once; the agent must persist it securely.
- `ReportHeartbeat` validates the session token, agent identity, version, capability list, and timestamp before refreshing liveness.
- User-driven agent revocation invalidates all active agent sessions.
- Agent session credentials expire after 30 days and can be rotated by re-enrolling with a newly issued token.
- Capability lists are bounded and validated before persistence.

## Phase 3 Telemetry Contract

- `Stream` is defined as a bidirectional telemetry/control channel but returns `UNIMPLEMENTED` until durable telemetry ingestion is delivered in Phase 3.
- Telemetry batches carry stable batch IDs so the receiver can implement idempotent acknowledgement.
- The backend must validate every agent identity, session, label, size limit, and timestamp before accepting data.
- Remote commands are deliberately represented as typed command messages; the protocol must never become an unrestricted shell tunnel.

## Code Generation

Install `protoc` and the pinned Go plugins, then run:

```sh
go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.12
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.2
sh scripts/generate-proto.sh
```

The Docker build and GitHub Actions workflow regenerate the sources from the versioned schema. Generated code is not hand-edited.
