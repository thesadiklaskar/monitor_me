# Monitor Me

**Monitor Me is a custom infrastructure monitoring platform** designed to make server observability easier to deploy, easier to understand, and safer to operate across infrastructure environments.

The product is being built around its own native host agent, telemetry ingestion pipeline, metric model, project attribution, alert engine, and operator dashboard. It is intended for infrastructure engineers, platform teams, and organizations that need clear visibility into servers, workloads, and applications.

> **Project setup:** **Historical baseline completed; current release gate open.** The original Phase 1–3 CI baseline passed in historical GitHub Actions runs, but recent single-install/startup changes do not yet have a fresh green CI or owner-machine acceptance. The no-login browser shell, native metrics path and first ClickHouse log path exist; this is not yet a released Grafana replacement. The three previously reported compile blockers are corrected in current `main` source, but the user's Windows build still needs rerunning from the exact latest `main`. The PowerShell/POSIX startup scripts now verify source freshness where Git remotes are available, validate Compose, build all images before replacing/running services, and only then start using `--no-build` with API/dashboard readiness checks. GitHub Actions attempts have repeatedly ended before any step with no runner assigned; no tests are claimed from those runs. The final product is one independent self-hosted installation per owner (VM, VPS, VPC, cloud or bare metal), with no hosted control plane, login, profile or organization workflow. Legacy user/organization database and query logic still exists and requires a safe data-preserving migration. Linux journald → ClickHouse ingestion and a bounded Log Explorer exist; Windows Application/System Event Log source code is present but not runtime-accepted; file tail and push-based live logs remain incomplete. The current Log Explorer live toggle polls every 10 seconds. Signed installs, one-command production bootstrap, real-time log push, advanced dashboard editing, backups/restore and high-volume benchmarks remain open. See [the prioritized backlog](docs/PRODUCT_BACKLOG.md) and [current project state](docs/PROJECT_STATE.md).

---

## Table of Contents

- [Product and Engineering Backlog](docs/PRODUCT_BACKLOG.md)
- [Product Vision](#product-vision)
- [Current Project Status](#current-project-status)
- [What the Product Will Provide](#what-the-product-will-provide)
- [Technology Stack](#technology-stack)
- [Architecture at a Glance](#architecture-at-a-glance)
- [Docker and Deployment Model](#docker-and-deployment-model)
- [Telemetry and Data Reliability](#telemetry-and-data-reliability)
- [Security Principles](#security-principles)
- [Production Firewall Requirements](#production-firewall-requirements)
- [Five-Phase Delivery Roadmap](#five-phase-delivery-roadmap)
- [Documentation Map](#documentation-map)
- [Engineering and Contribution Standards](#engineering-and-contribution-standards)
- [Performance and Reliability Policy](#performance-and-reliability-policy)
- [Final System Design](#final-system-design)

## Product Vision

Monitor Me aims to provide a guided, low-friction monitoring experience without requiring operators to assemble a monitoring stack manually.

The target workflow is a single-install, no-login monitoring console:

1. An operator runs one control-plane installation command. Docker Compose, storage, TLS, migrations, health checks and safe network defaults are configured automatically.
2. The dashboard opens directly. No signup, login or organization creation is required in the default single-install experience.
3. The operator selects **Add machine** and receives a short-lived, single-use enrollment token.
4. A platform-native launcher is run on the target host. It detects the target's OS, architecture, service manager and available virtualization/cloud metadata locally, then installs and starts the matching native agent.
5. The agent enrolls and reports supported metrics and host identity. Each server remains a separately enrolled machine.
6. The operator investigates fleet health, metrics, alerts and—once delivered—the initial searchable Linux journald log pipeline.

The complete product contract, architecture boundaries and acceptance targets are documented in [PRODUCT_VISION_PLUG_AND_PLAY.md](docs/PRODUCT_VISION_PLUG_AND_PLAY.md). The no-login production network boundary and ClickHouse log-store proposal are recorded in [ADR 0004](docs/adr/0004-single-install-no-login-network-boundary.md) and [ADR 0005](docs/adr/0005-clickhouse-high-volume-logs.md).

## Current Project Status

| Area | Status | Details |
| --- | --- | --- |
| Project setup baseline | **Completed** | Repository layout, documented toolchain, Docker Compose local stack, database migrations, and the four-job Phase 1–3 CI baseline are established and verified |
| Product requirements | **Completed** | Functional requirements, MVP scope, and acceptance criteria documented |
| Metric definitions | **Completed** | Names, units, calculations, timestamps, and platform limitations documented |
| High-level architecture | **Completed** | Component boundaries, data flow, storage responsibilities, and security boundaries documented |
| Feature acceptance matrix | **Completed** | Planned features mapped to implementation tests and CI evidence |
| Backend implementation | **Single-install refactor incomplete** | Browser login routes and UI are removed; one implicit installation context exists. Legacy user/organization schema, foreign keys and organization-scoped backend queries still need removal through a tested forward migration that preserves existing data |
| Frontend implementation | **Phase 4 in progress** | Monitoring detail refresh, no-login local shell, OS-agnostic Add Machine enrollment, two shell launcher options and collapsible icon-led sidebar are implemented; full console redesign, operator acceptance and production network-boundary verification remain |
| Local platform stack | **Build/start acceptance pending** | Startup scripts now verify the `main` source revision, validate Compose, build before start and check API/dashboard readiness. The updated scripts have not yet passed owner Windows Docker Desktop acceptance |
| Database and cache integration | **Historical baseline verified; no-login bootstrap pending** | PostgreSQL remains durable; Redis and PostgreSQL dependency outage/readiness recovery were tested before the current installation-context changes |
| Agent protocol and ingestion | **Historical Phase 3 CI baseline** | TLS enrollment, persisted agent session, heartbeat, reconnect/backoff and durable telemetry ingestion existed in the previously verified baseline; the newest source changes still need fresh Go/Compose acceptance |
| Native host agent | **Linux baseline; Windows acceptance pending** | Linux collector path exists; Windows metrics and Application/System Event Log reader source exist but need actual Windows runtime/service tests. File-tail logs, signed releases and broader platform parity remain |
| Telemetry history and live transport | **Phase 3 verified** | Bounded PostgreSQL range query, 30-day raw retention, 90-day idempotency retention, one-use session-bound WebSocket tickets, cross-tenant isolation, and bounded subscriber queues pass CI |
| Project/workload attribution | **Phase 3 verified (Linux mapping path)** | cgroup/Systemd/container selectors, exact server/project mappings, and explicit host-total/unclassified/shared attribution pass database integration tests; full Docker Engine discovery remains later work |
| Monitoring dashboard | **In Progress** | Saved dashboards/basic panels, metric refresh and bounded log pages exist; log live mode currently polls every 10 seconds. Native log push/replay, flexible panel editor, variables, richer visualizations and owner acceptance remain |
| Production firewall setup | **In Progress** | UFW/firewalld helper opens HTTPS for public node bootstrap while Nginx restricts dashboard/API/WebSocket paths to ADMIN_CIDR; deployment validates this allowlist before enabling remote no-login. Real-host IPv4/IPv6 and cloud security-group checks remain |
| Production release | Not started | Planned for Phase 5 |

**Status vocabulary:** Completed means implemented and verified for the documented environment. Specified means requirements are documented but implementation is not complete. Planned means work has not started. A design document or mock screen does not count as a working feature.

## Verified Phase 3 Implementation

The current Linux-first implementation provides a real telemetry path rather than a collector mock:

- **Native Linux agent:** aggregate and per-core CPU, memory available/used/cache/swap, disk byte and operation rates, network byte/packet/error rates, host/agent uptime, process count, top-128 readable process RSS, TCP state counts, and listening-port metadata. It also emits process CPU utilization, collector errors per interval, durable WAL bytes and pending batch depth, and the last successful batch-ACK timestamp.
- **Initial Windows collector:** aggregate/per-core CPU, memory/swap, mounted-volume capacity and usage, disk throughput/IOPS, network throughput/packet/error rates, uptime, OS/kernel metadata, process count and top-128 readable process RSS. Windows Service installation code is present but runtime acceptance is pending; TCP/listening-port metrics, per-process CPU and workload attribution remain unsupported in this first slice.
- **Correct rate semantics:** counter deltas use actual monotonic elapsed time. Initial samples have no fabricated rate; resets and invalid intervals produce missing values rather than negative rates.
- **Workload metadata:** process RSS samples include cgroup path and workload selector. Systemd units and container cgroup IDs are identified when present. Unmapped and shared resource labels remain explicit.
- **Agent durability:** append-only JSONL local WAL, fsync per batch, 0600 file/0700 directory, configurable disk quota, restart recovery for a truncated trailing record, and atomic compaction after accepted acknowledgements.
- **Secure delivery:** TLS gRPC enrollment with a short-lived one-use server token; session secret is persisted locally with private permissions. The agent reports heartbeats, retries with bounded backoff, and removes a revoked local session when re-enrollment credentials are available. A hardened Linux systemd unit is provided with a protected environment file, private state directory, restrictive umask and filesystem/kernel protections. It runs as root for host-wide process metadata; a monitored Linux host does not need Docker.
- **Durable ingestion:** the backend validates authenticated batches, limits message/sample/label sizes, stores batch hashes and telemetry samples in PostgreSQL in one transaction, and sends ACK only after commit. Same-payload retries are idempotent; changed content under the same batch ID is rejected.
- **Historical query:** authenticated server-scoped range queries aggregate with PostgreSQL `date_bin`. The query enforces a 100ms minimum step, a 30-day raw range, a 5,000-point maximum and rejects a bucket grid that would be silently truncated.
- **Live delivery:** post-commit latest-value events publish to organization-specific Redis Pub/Sub channels. Each event contains at most 128 metric points. Browsers exchange their bearer session for a 30-second, single-use ticket; only the ticket hash is stored. Tickets are invalidated when the parent user session is revoked. WebSocket output queues are bounded to 32 events; slow clients are disconnected and should reconnect and fetch durable history to catch up. Cross-organization delivery is covered by integration tests.
- **Retention:** default raw samples are kept for 30 days; batch idempotency records for 90 days; expired live tickets are removed in bounded cleanup batches. Redis is notification/cache infrastructure, never the durable telemetry store.

The Phase 3 Linux implementation did **not** originally include Windows/macOS collection, per-process CPU, eBPF, complete Docker Engine API discovery, HTTP request latency/error instrumentation, persistent materialized rollups, or a dashboard UI. An initial Windows collector has now been added, but its Windows runtime acceptance and installer lifecycle are still pending; macOS collection remains unimplemented. The protobuf AgentHealth envelope remains reserved; its health fields are emitted as ordinary telemetry samples. The last-successful-send timestamp is process-local and absent after restart until a matching ACK arrives. The dashboard and operator workflows are Phase 4; cross-platform collectors, signed installers and production scale are Phase 5. Historical queries aggregate with PostgreSQL `date_bin`; persistent rollup tables are deferred until scale measurements justify them. A monitored Linux host does not need Docker installed.

The latest agent-health and systemd verification is GitHub Actions run #45045054465 (historical Actions run). Its benchmark snapshot on a GitHub-hosted Ubuntu runner was:

| Benchmark | Result | Workload |
| --- | ---: | --- |
| Linux proc collector | 5.21 ms/op; 1.04 MB/op; 5,192 allocs/op | Read host counters, processes, workload metadata and agent CPU |
| Counter-rate calculation | 2.75 ns/op; 0 B/op | One counter delta and monotonic elapsed-time calculation |
| Agent spool append | 393 μs/op; 984 B/op; 14 allocs/op | Append and fsync a one-sample batch to the local WAL |
| Live event preparation | 137 μs/op; 165.7 KB/op; 897 allocs/op | Collapse and encode a representative 96-sample event using an in-memory publisher |
| TLS gRPC to PostgreSQL ingestion | 4.27 ms per 64-sample batch; 14,980 samples/sec | Authenticated TLS gRPC stream, PostgreSQL transaction, durable ACK, and Redis publication |

These figures describe one hosted CI runner and this benchmark workload only. They are not production capacity guarantees or evidence of million-host scale. Hardware, filesystem durability, database placement, network latency, metric cardinality, and write concurrency materially affect results. Phase 5 will add staged multi-agent capacity testing.

## What the Product Will Provide

The following are target capabilities. They are not represented as implemented until their acceptance tests pass.

### Server Enrollment and Host Monitoring

- Guided **Add Server** and one-command installation flow.
- Secure, expiring, revocable agent enrollment.
- Overall server view for CPU, per-core CPU, memory, swap, disk, IOPS, and network.
- Agent health, server uptime, and last-seen status.
- Native monitoring agent for supported Linux, Windows, and macOS versions.

### Project and Workload Monitoring

- Header-level **All Projects / Project Selector**.
- Dashboard updates without a full page reload when the project changes.
- Project-specific CPU, memory, disk, network, process, and supported application metrics.
- Docker container and Systemd service discovery where supported and authorized.
- Linux cgroups-based resource attribution, with explicit labels for inferred or shared resources.

### Process and Application Visibility

- Process inventory and per-process CPU/RSS information.
- Listening ports and connection metadata.
- Application request latency and HTTP status/error rates through supported application instrumentation, reverse-proxy integrations, or tracing.
- Clear distinction between socket visibility and actual HTTP request telemetry.

### Dashboards, Alerts, and Operations

- Custom live and historical charts.
- Configurable metric sampling and time ranges.
- Threshold and condition-based alerts.
- Alert history, recovery events, and notifications.
- Allowlisted, audited remote restart/stop actions for supported targets.
- Historical retention and metric-appropriate downsampling.

## Technology Stack

| Layer | Technology | Responsibility |
| --- | --- | --- |
| Frontend | Next.js, TypeScript | Operator interface and dashboard |
| UI styling | Tailwind CSS | Consistent and responsive UI |
| Backend | Go (Golang) | APIs, enrollment, ingestion, processing, alerts, and action orchestration |
| Host agent | Native Go binary | OS-level metrics and workload collection |
| Agent transport | gRPC, Protocol Buffers | Authenticated, versioned binary communication |
| Live browser updates | WebSocket | Authorized live metric and event streaming |
| Primary database | PostgreSQL | Durable application state and historical telemetry |
| Live cache | Redis | Ephemeral cache and short-lived coordination |
| Raw log storage | ClickHouse MergeTree | Structured logs, bounded search, idempotent ingestion and 30-day TTL |
| Local orchestration | Docker Compose | Repeatable local control-plane environment |
| Visualization | Custom Canvas/WebGL where justified | Efficient live and historical metric rendering |
| System design | Excalidraw | Final as-built architecture diagrams |

The monitoring product itself is custom-built. The project will not embed or depend on ready-made monitoring platforms such as Grafana, Prometheus, Zabbix, or Netdata. General-purpose development and infrastructure technologies listed above are allowed.

## Local Development

### Native agent setup (run on each monitored host)

The Compose stack is the control plane. Every monitored machine runs its own native agent; the installer detects the target OS and architecture there—not in the browser and not from the control-plane container.

After choosing **Add Machine**, copy the command for the terminal available on the target host:

**Windows · PowerShell (Run as Administrator)**

```powershell
& ([scriptblock]::Create((Invoke-RestMethod -Uri 'https://monitor.example/install/agent-windows.ps1'))) -ServerUrl 'https://monitor.example' -GrpcAddress 'monitor.example:8443'
```

The installer detects Windows x64, downloads the matching artifact, verifies its SHA-256, installs a restart-enabled Windows Service and securely prompts for the one-time token. The session is stored under ProgramData and the token is removed after enrollment.

**Linux · Shell (systemd, run as root)**

```sh
curl -fsSL 'https://monitor.example/install/agent-linux.sh' | sudo bash -s -- --server-url 'https://monitor.example' --grpc-address 'monitor.example:8443'
```

The installer detects Linux amd64/arm64, downloads and checksum-verifies the native agent, installs/enables the systemd service, securely prompts for the one-time token and removes it after session persistence.

These launchers do not need Docker, Go, Node.js or a repository checkout on the monitored host. They require the target to reach the control-plane HTTPS installer and the advertised TLS gRPC endpoint. `localhost:8443` only works when the agent and control plane share a host. The binaries are checksum-verified but are not yet publisher-code-signed; signing and OS publisher verification are release gates.

Host OS, architecture and available VM/hypervisor metadata come from the native agent. Provider detection is best-effort; where a provider or virtualization signal is unavailable, Monitor Me must report unknown rather than guess.


See [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) for prerequisites and current limitations.

For a one-command local control-plane setup, start Docker Desktop / Docker Engine, clone this repository, then run the matching helper from the repository root:

```powershell
.\scripts\start-local.ps1
```

```sh
./scripts/start-local.sh
```

The helper creates `.env` from `.env.example` when needed, generates a localhost-only development TLS certificate, builds the Compose services, waits for the dashboard probe, and opens (PowerShell) or prints (shell) `http://localhost:3000`. It does not erase existing database volumes. The helper is for local development only; do not reuse its self-signed certificate or loopback-only no-login mode on a public deployment.

The API exposes `/health/live` and `/health/ready`. Liveness reports process health independently; readiness checks PostgreSQL, Redis and ClickHouse and returns HTTP 503 when a required dependency is unavailable. Docker Compose runs a one-shot migration service before the API starts. The dashboard includes server/project views, historical/live charts, alert and notification workflows, and allowlisted remote operations.

Build the native agent and its tests:

```sh
cd agent
go mod tidy
go test -race ./...
go vet ./...
go build -trimpath ./cmd/monitor-agent
```

To run a Linux agent against a local control plane, create a server from the dashboard (no browser account is required), copy its one-time enrollment token, then start the agent with the local development CA:

```sh
MONITOR_GRPC_ADDRESS=localhost:8443 \
MONITOR_TLS_CA_FILE=../.data/certs/server.crt \
MONITOR_ENROLLMENT_TOKEN="$ENROLLMENT_TOKEN" \
MONITOR_SPOOL_DIR=./.monitor-me/spool \
MONITOR_AGENT_STATE_FILE=./.monitor-me/agent-state.json \
./monitor-agent
```

The enrollment token is single-use and should not be committed to shell history or source control. The agent stores the returned session in a private local state file; after the initial enrollment, a restart can reuse that session without the enrollment token while it remains valid.

History endpoint:

```text
GET /v1/servers/{serverID}/metrics?metric=host.cpu.utilization&from=2026-11-22T10:00:00Z&to=2026-11-22T10:05:00Z&step=1s&limit=300
```

Browser live connections first call `POST /v1/live/ticket` with the bearer session, then connect to `GET /v1/live/ws?ticket={single-use-ticket}`. The ticket is valid for 30 seconds and cannot be reused.

## Architecture at a Glance

```text
 Monitored Hosts
 Bare Metal / VPS / VM / Cloud / Local Machines
 Linux / Windows / macOS
                 |
                 v
       Native Go Monitoring Agent
       - OS-specific collectors
       - Process and workload discovery
       - Bounded queue and persistent spool
       - Agent health and capabilities
                 |
                 | Authenticated gRPC + Protocol Buffers
                 v
          Custom Go Backend
       - Single-install context and agent enrollment
       - Telemetry validation and ingestion
       - Project and workload attribution
       - Historical query and alert processing
       - Authorized remote-action orchestration
             /             \
            v               v
      PostgreSQL           Redis
      Durable data         Live cache and
      and history          ephemeral state
            \               /
             v             v
          Authorized WebSocket Updates
                 |
                 v
         Next.js Dashboard
       - Overall server view
       - All Projects / project view
       - Live and historical charts
       - Alerts, events, and operations
```

The control plane opens as one self-hosted installation with no user login or organization-creation workflow. A private compatibility identity remains inside the database schema for existing foreign keys and resource scoping; it is not a customer account or selectable organization. Monitored nodes still enroll through short-lived, single-use credentials and TLS-protected gRPC. Phase 3 provides a native Linux collector, durable spool, idempotent PostgreSQL ingestion, range queries, retention, and live delivery. The diagram shows the intended end-state data flow, not a claim that every component is already implemented.

## Docker and Deployment Model

Docker is part of the project toolchain and has two distinct roles:

1. **Control-plane development and deployment:** Docker Compose will provide a repeatable local environment for the Go backend, Next.js frontend, PostgreSQL, and Redis. A containerized single-node deployment may also be supported.
2. **Monitored workload discovery:** Where explicitly enabled and authorized, the native host agent may collect Docker container metadata and use supported cgroup information for workload attribution.

The monitoring agent is intended to run as a **native Go service/binary on the monitored host**. Docker is not required on every monitored machine. A Bare Metal server or VPS can be monitored without running the control plane in Docker.

## Telemetry and Data Reliability

The system separates live delivery from durable historical storage:

- **PostgreSQL** is the durable store for application state and historical telemetry.
- **Redis** is used for defined ephemeral workloads such as live cache and short-lived coordination. It is not the sole source of truth.
- **Agent-side persistent spool/WAL** retains unsent telemetry across temporary network interruptions and agent restarts, within configured storage limits.
- **Bounded queues, backpressure, retry limits, idempotency, and graceful shutdown** are required across the telemetry pipeline.
- **Downsampling and retention** must preserve documented resolution and must not silently replace raw data before the configured raw-retention period expires.

The initial collection target is configurable in the 100ms–500ms range where host overhead and collector capabilities allow it. Sampling interval, ingestion latency, processing latency, and dashboard freshness are separate measurements. A 100ms collection interval does not imply 1ms end-to-end latency.

## Security Principles

Every installation is no-login and single-install. Local Compose publishes the dashboard/API only on loopback. Production deployment refuses to proceed until a specific `ADMIN_CIDR` is supplied and the Nginx allowlist plus host firewall boundary are installed for the dashboard/API/WebSocket. Public installer/CA/artifact routes remain reachable over HTTPS for node bootstrap. There is no environment switch to restore password login. See ADR 0004.

- TLS-protected agent communication with short-lived, single-use, revocable enrollment credentials.
- Single-install operation has no signup, login, user management or organization-creation workflow. A non-interactive compatibility scope remains internal to the database for legacy foreign keys.
- Local access is loopback-bound. Production management access always relies on HTTPS plus an Nginx administrator CIDR/VPN allowlist for the dashboard/API/WebSocket; only dedicated node bootstrap, CA and artifact paths are public.
- Keep operator dashboard access separate from agent gRPC enrollment. A node token never authenticates a browser session.
- Keep PostgreSQL, Redis, ClickHouse (when implemented), API HTTP and internal web ports private.
- Internal installation/server isolation remains mandatory even when the operator UI is single-tenant.
- Least-privilege host collection and Docker access.
- Explicit allowlists and audit records for remote actions.
- No unrestricted remote shell feature.
- Installer artifacts are SHA-256 verified before installation. Publisher code-signing and signature enforcement are still release requirements; checksum integrity alone is not publisher identity.
- Secrets are never committed to the repository or written to logs.
- Production deployment uses a least-privilege host firewall: expose only intended HTTPS/gRPC entry points, restrict SSH to administrators, and keep database/cache ports private.

## Production Firewall Requirements

The production firewall policy is defined for two roles: a control-plane VPS/VM and agent-only monitored Linux hosts. Agent-only hosts require no inbound Monitor Me ports; they send outbound TLS gRPC to the control plane. The no-login control plane exposes TCP 443 to Nginx for public installer/CA/artifact routes; Nginx always restricts the dashboard/API/WebSocket to the administrator CIDR/VPN. TCP 8443 is the TLS agent endpoint (public or source-restricted), optional TCP 80 is for ACME HTTP-01, and SSH is limited to a trusted administrator CIDR. PostgreSQL 5432, Redis 6379, API 8080, and web 3000 must remain private. See [Production Firewall Requirements](docs/PRODUCTION_FIREWALL.md) for UFW/firewalld examples, Docker firewall caveats, and local/LAN deployment rules. An additive UFW/firewalld helper and a mandatory production no-login Nginx access gate are implemented; host/provider verification and fully automatic firewall provisioning remain Phase 5 acceptance work.

## Five-Phase Delivery Roadmap

| Phase | Name | Status | Main outcome |
| --- | --- | --- | --- |
| 1 | Product Specification and Technical Blueprint | **Completed** | Requirements, metric catalog, acceptance matrix, and architecture baseline |
| 2 | Platform Foundation and Secure Connectivity | **Completed** | Go/Next.js foundation, PostgreSQL/Redis, migrations, internal installation scope, TLS gRPC enrollment, heartbeat, reconnect, and revocation pass the historical CI baseline |
| 3 | Monitoring Agent and Telemetry Data Engine | **Completed** | Linux host collection, durable spool, TLS gRPC ingestion, PostgreSQL history/retention, authorized live delivery, workload attribution, and CI benchmarks all verified |
| 4 | Dashboard and Operational Features | **In Progress** | Server/project dashboards, custom charts, application visibility, alerts, notifications, one-click installation workflow, and secure remote actions |
| 5 | Cross-Platform Production and Scale | In Progress | Linux amd64/arm64 and Windows x64 checksummed agent bundles, target-side service installers, Nginx no-login access gate, and UFW/firewalld helper are implemented in code. Windows runtime acceptance, publisher signing, macOS, raw logs, fleet tests and real-host verification remain |

A phase is marked Completed only after implementation, acceptance tests, relevant security/failure checks, documentation, and Git commit requirements pass.

## MVP Readiness

The historical Phase 1–3 baseline passed its CI gates. The latest single-install refactor still needs a fresh CI pass. This does **not** mean the first usable product MVP is released. The current single-install implementation removes browser login/signup, account login methods and organization creation from the operator workflow; private legacy database relationship rows remain pending a dedicated schema migration. Host metrics, server inventory, historical/live charts, projects, alerts and operations exist in varying implementation states; raw logs, publisher-signed installers and the complete one-command production control-plane bootstrap remain outstanding. The API, Linux agent, durable ingestion, history, and live transport are verified foundations for the next server-detail/history/live views.

## Documentation Map

| [Product and Engineering Backlog](docs/PRODUCT_BACKLOG.md) | Prioritized stability gate, strict single-install migration, real-time logs, Grafana-style dashboards and production acceptance |
| Document | Purpose |
| --- | --- |
| [Product Requirements](docs/PRODUCT_REQUIREMENTS.md) | Product requirements and current functional acceptance baseline |
| [Plug-and-Play Product Contract](docs/PRODUCT_VISION_PLUG_AND_PLAY.md) | Single-install no-login workflow, target-side discovery, firewall boundary, log scale and delivery sequence |
| [ADR 0004](docs/adr/0004-single-install-no-login-network-boundary.md) | No-login single-install model and required network security boundary |
| [ADR 0005](docs/adr/0005-clickhouse-high-volume-logs.md) | Proposed durable ClickHouse raw-log data plane |
| [Metric Catalog](docs/METRIC_CATALOG.md) | Metric names, units, sources, calculations, and platform limitations |
| [Feature Acceptance Matrix](docs/FEATURE_ACCEPTANCE_MATRIX.md) | Feature-by-feature verification criteria and evidence status |
| [Architecture Overview](docs/ARCHITECTURE_OVERVIEW.md) | Components, data flow, storage, Docker boundaries, and security boundaries |
| [Production Firewall Requirements](docs/PRODUCTION_FIREWALL.md) | Least-privilege firewall profiles for the control plane, agent hosts, Docker, and local/LAN deployments |
| [Phase 4 Execution Plan](docs/PHASE_4_EXECUTION_PLAN.md) | Dashboard implementation order, verified API baseline, acceptance gates, and agreed release/test sequence |
| [Zero-to-Production Roadmap](docs/ROADMAP.md) | Five delivery phases and completion gates |
| [Project State](docs/PROJECT_STATE.md) | Exact current phase, completed work, commit references, and next action |
| [Repository Instructions](AGENTS.md) | Continuation, code quality, testing, security, and commit rules |

## Engineering and Contribution Standards

- All source code, comments, documentation, UI strings, logs, tests, and commit messages must be written in English.
- Comments explain intent, invariants, security boundaries, and non-obvious logic. Comments are not added mechanically to every trivial line.
- Every feature must have relevant unit/integration tests and documented acceptance evidence.
- Use descriptive Conventional Commits, such as `feat(agent): collect host memory metrics`.
- Keep the default branch buildable; complete one small, reviewable unit at a time.
- Update the feature acceptance matrix when implementation status changes.
- Update this README when a feature is verified and whenever a phase is completed. The README must always distinguish implemented capabilities from planned capabilities.
- Update `docs/PROJECT_STATE.md` after every work session so development can resume from one exact next action.
- Never mark an item Completed based solely on a stub, mock, or design document.

## Performance and Reliability Policy

Monitor Me targets low-latency telemetry, responsive visualization, bounded resource use, and predictable recovery. No software is assumed to be perfectly crash-free or loss-free. Reliability and scale claims must be supported by reproducible tests and benchmarks that record the environment, build, workload, duration, and results.

## Final System Design

After the implementation is complete, the final Excalidraw system design will be created from the actual source code and deployment configuration. It will cover:

- Components and service boundaries.
- Agent enrollment and communication.
- Telemetry and event data flows.
- Database schema and retention paths.
- Project/workload attribution.
- Authentication, authorization, and trust boundaries.
- Failure handling, recovery, and deployment topology.
- Measured scaling and performance characteristics.

The final diagram must describe the system that was actually built, not an aspirational architecture.

**Raw logs are not implemented yet.** The current telemetry path stores host metrics in PostgreSQL and uses Redis for ephemeral live updates. ClickHouse is the proposed high-volume raw-log store; do not present log search, live tail or million-event reliability as shipped until ADR 0005 is implemented and load-tested.

---

**Current resume point:** Do not start feature upgrades until the full startup/acceptance gate passes. Pull exact latest `main`, run `scripts/start-local.ps1`, and capture fresh BuildKit output only if an image build fails. Recent GitHub Actions attempts assigned no runners and executed no tests; no fresh CI pass is claimed. Read [docs/PROJECT_STATE.md](docs/PROJECT_STATE.md) and [docs/PRODUCT_BACKLOG.md](docs/PRODUCT_BACKLOG.md) for the authoritative order.

## Local single-install mode

The default Docker Compose stack opens the monitoring dashboard directly. There is no signup, login, logout, user management or organization-creation screen. On first use, the API initializes one private installation scope for existing database relationships; this is not a customer account and is not exposed in the UI.

The dashboard, API, PostgreSQL and Redis host ports are loopback-bound by default. Only the machine running the Compose stack can reach the local management interface. Do not change those bindings to a public interface without a production reverse proxy and network policy.

Production deployment is also no-login. The deployment script refuses to continue until a specific trusted administrator/VPN ADMIN_CIDR is configured and the Nginx allowlist and host firewall checks are applied. Public installer, CA and checksum-verified agent artifact routes remain available for monitored nodes; the dashboard, normal API and live WebSocket remain restricted to the administrator network. There is no configuration flag to turn password login back on.
