# Monitor Me

A custom, cross-platform infrastructure monitoring platform designed for clear operations, reliable telemetry, and a simple onboarding experience.

> **Project status:** Foundation initialized; implementation has not started.
>
> **Current phase:** Phase 1 — Requirements and Architecture
>
> **Product principle:** Build our own monitoring product and monitoring engine. Do not embed or depend on ready-made monitoring platforms such as Grafana, Prometheus, Zabbix, or Netdata.

## Product goals

- One-click enrollment and installation for supported hosts.
- Overall host monitoring and project-level resource views.
- Real-time CPU, memory, disk, network, process, and application telemetry.
- Historical metrics, retention, and downsampling.
- Useful alerts and carefully authorized remote actions.
- Windows, Linux, and macOS support, delivered incrementally.
- A production-minded path from a small installation to horizontally scaled infrastructure.

## Technology baseline

| Area | Selected technology |
| --- | --- |
| Frontend | Next.js, TypeScript, Tailwind CSS |
| Backend services | Go |
| Host agent | Go with OS-specific collectors |
| Agent communication | gRPC and Protocol Buffers |
| Live dashboard transport | WebSocket |
| Primary persistent database | PostgreSQL |
| Live cache and ephemeral coordination | Redis |
| Visualization | Custom Canvas/WebGL rendering where justified |
| System design | Excalidraw |

Technology choices may be refined through documented Architecture Decision Records (ADRs). Redis is a cache/coordination component, not the source of truth for durable telemetry. PostgreSQL and the agent's persistent local spool are part of the durability design.

## Build rules

1. All repository content, source code, comments, UI strings, logs, and documentation must be written in English.
2. Build the monitoring agent, telemetry pipeline, data model, alerting, and dashboard as our own product. Do not use an existing monitoring platform as the implementation.
3. Complete one phase at a time. A phase is complete only when its acceptance criteria, tests, and documentation are satisfied.
4. Keep changes small, reviewable, and independently buildable. Do not leave the repository in a knowingly broken intermediate state.
5. Add comments where they explain intent, non-obvious behavior, invariants, safety constraints, or important trade-offs. Do not add comments to every trivial line.
6. Use descriptive Conventional Commits. A Git commit records a change; code comments explain code.
7. Update the project state and roadmap whenever a phase changes status.
8. Do not claim performance, accuracy, reliability, or scale targets until they are measured and the benchmark method and results are recorded.

## Project navigation

- [Project state](docs/PROJECT_STATE.md) — exact resume point, completed work, current work, and next action.
- [Zero-to-production roadmap](docs/ROADMAP.md) — phases, scope, dependencies, and completion gates.
- [AI and contributor instructions](AGENTS.md) — rules for continuing work consistently.

## Quick start

The implementation toolchain and service setup will be documented in the relevant phase after versions and supported environments are verified. No application code is present yet.

## Performance and reliability policy

The project targets low-latency telemetry, smooth visualization, bounded resource use, and graceful recovery. No system is assumed to be crash-free or loss-free by design alone. These properties must be demonstrated with automated tests, failure injection, profiling, and reproducible load tests.

## License

No license has been selected. Do not add or imply an open-source license without an explicit project decision.
