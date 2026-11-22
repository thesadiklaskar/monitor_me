# Agent Protocol

The protocol is defined in \`monitor/v1/agent.proto\`.

- The package name is versioned as \`monitor.v1\`.
- The agent uses gRPC and Protocol Buffers for control and telemetry delivery.
- Enrollment tokens are short-lived and must be transmitted only over authenticated TLS.
- The backend stores a token hash rather than the original enrollment token.
- Telemetry batches carry stable batch IDs so the receiver can implement idempotent acknowledgement.
- The backend must validate every agent identity, session, label, size limit, and timestamp before accepting data.
- Remote commands are deliberately represented as typed command messages; the protocol must never become an unrestricted shell tunnel.

Generated Go and client code is not checked in yet. Code generation, compatibility checks, message size limits, mTLS/session establishment, and command authorization are Phase 2 implementation tasks.
