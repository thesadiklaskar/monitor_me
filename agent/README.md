# Native Agent Workspace

This directory is the ownership boundary for the native Go monitoring agent.

The Phase 2 control plane is ready to enroll a native agent over the versioned,
TLS-protected `monitor.v1` gRPC contract. The executable, operating-system
collectors, local credential store, persistent spool, and service lifecycle are
implemented in Phase 3; no placeholder collector or non-functional agent
binary is shipped in this phase.

## Planned Phase 3 Layout

- `cmd/monitor-agent`: native agent entry point and lifecycle.
- `internal/config`: validated configuration and secure credential paths.
- `internal/enrollment`: TLS gRPC enrollment and session renewal.
- `internal/collectors`: operating-system-specific collectors.
- `internal/spool`: bounded persistent telemetry spool.
- `internal/transport`: authenticated telemetry delivery and reconnect handling.

The agent will run directly on monitored Linux, Windows, and macOS hosts. Docker
is not a prerequisite for installing or running the native agent. Phase 3 will
introduce this directory's Go module and CI checks with the first executable
agent slice.
