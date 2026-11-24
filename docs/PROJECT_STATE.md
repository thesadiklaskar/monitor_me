# Project State

This file is the authoritative resume point for Monitor Me. Read it before making changes and update it after every work session.

## Current Snapshot

| Field | Value |
| --- | --- |
| Project | Monitor Me |
| Repository | WorkRCS/monitor_me |
| Default branch | main |
| Working branch | main |
| Branch policy | All work and commits are made directly on main; do not create another branch |
| Repository visibility | Private |
| Completed phases | Phase 1 — Product Specification and Technical Blueprint; Phase 2 — Platform Foundation and Secure Connectivity; Phase 3 — Monitoring Agent and Telemetry Data Engine |
| Current phase | Phase 4 dashboard acceptance and Phase 5 Linux-MVP / Windows local-agent validation |
| Current phase status | In Progress — implementation is present; end-to-end acceptance is pending |
| MVP status | Dashboard workflows are implemented; owner-led local acceptance and production-host verification remain before release |
| Implementation status | The operator product direction is now single-install, no-login and plug-and-play. The dashboard no longer asks for a target OS during Add Machine; it shows PowerShell/POSIX launcher choices, has a collapsible icon-led sidebar and removes org/profile clutter from the local operator shell. Linux/Windows agents report OS/platform/architecture and available virtualization metadata. The local Compose startup helper and HTTPS-downloadable Linux/Windows target installers are committed. The agent bundle is built by Compose; Linux installs systemd and Windows installs a restart-enabled Windows Service. Artifact checksum verification and installer wiring are implemented but not runtime-tested or publisher-code-signed. Production no-login is gated by an Nginx admin allowlist for dashboard/API/WebSocket while public installer/CA/artifact paths remain reachable; host firewall and external IPv4/IPv6 acceptance remain pending. Raw log ingestion/search is not implemented; ClickHouse is proposed in ADR 0005. No-login production must wait for the network access gate and security verification. |
| Last source commit covered by latest successful CI run | 9685e115113123d5f9f8e8a1b222804752f5a283 (Go, frontend and native-agent jobs passed in run #45203706343; Compose integration failed at the web proxy login probe; later commits are not covered) |
| Latest CI run | [#45208235137 — quota blocked](https://github.com/WorkRCS/monitor_me/actions/runs/45208235137); jobs failed before runner assignment/steps because the repository owner reports GitHub Actions free minutes are exhausted |
| Latest implementation commit | [979a737ece372efeeed816381b984d05f74f8fc1](https://github.com/WorkRCS/monitor_me/commit/979a737ece372efeeed816381b984d05f74f8fc1) — collapsible sidebar, icon navigation and agent enrollment UI refinements |
| Next action | Verify the current Next.js dashboard after pulling `main`: run frontend lint/typecheck/build, inspect the OS-agnostic enrollment dialog and collapsed sidebar, then inspect the Nginx administrator allowlist and run `deploy/production/firewall.sh --role control-plane --admin-cidr <ADMIN_CIDR> --verify-admin-ui-allowlist` in dry-run only; confirm public HTTPS is used only for the public bootstrap paths while the dashboard/live stream are admin-restricted. Keep existing data volumes; do not enable production anonymous access or apply firewall changes without a selected host and verified admin CIDR. |




## Single-install plug-and-play product direction — 2026-11-24

**Status:** Target contract recorded; partial UI/security changes committed; major installer/log pipeline/runtime verification remain open.

- [x] Added docs/PRODUCT_VISION_PLUG_AND_PLAY.md with the target operator experience, target-side discovery contract, no-login network boundary, high-volume log data plane, UI and benchmark gates.
- [x] Added ADR 0004 for the internal single-install workspace/no-login model. Local no-login remains loopback-only; production no-login must wait for an enforced HTTPS admin CIDR/VPN boundary. Browser/node enrollment token roles remain separate.
- [x] Added ADR 0005 proposing ClickHouse MergeTree for raw logs with durable batching, idempotent acknowledgements, bounded queues, retention, structured schema, search and replay/load gates. ClickHouse is not deployed, and raw logs are not yet collected or searchable.
- [x] Reworked Add Machine: removed browser-based target-OS selection and shows both native shell launchers after token creation. Added clearer node label and auto-hostname copy. The current launchers still require a source checkout and Docker; this does not yet meet the no-Docker/downloadable-agent target.
- [x] Added a collapsible icon-led sidebar with browser-local preference, vector React icons, and removed workspace-owner/profile clutter from the operator shell. Other pages still need the full professional console redesign and accurate advanced navigation.
- [x] Changed the no-login boundary to Nginx path-level access control so remote agents can still download the public installer, CA and binaries on HTTPS. The firewall helper opens 443 to Nginx, restricts SSH to ADMIN_CIDR, and verifies that Nginx applies the allowlist to both dashboard and live WebSocket locations. Existing broad rules and cloud security groups are not automatically removed/updated; real-host verification is pending.
- [x] Updated README, roadmap and acceptance matrix to reflect the product owner's new direction and distinguish current implementation from target requirements.
- [ ] Remove the remaining production login/signup dependency safely and convert all default provisioning to one implicit installation workspace. Do not enable public anonymous owner-level APIs until network-only access is enforceable and externally verified.
- [ ] Deliver a source-independent signed Linux/Windows installer, target-side OS/architecture detection, Linux systemd and Windows Service lifecycle, one-use token enrollment, upgrades, rollback and uninstall. macOS remains unimplemented.
- [ ] Deliver a single control-plane bootstrap that generates secrets/TLS, provisions supported firewall policy, launches Compose and auto-enrolls the local host. Provider firewalls require scoped provider IAM and must not be guessed.
- [ ] Implement raw log collection and ClickHouse ingestion/search/live-tail/retention/backup. The current product has metrics only.
- [ ] Verify the updated UI, installer and firewall code with local tests, external IPv4/IPv6 checks, cross-platform service acceptance and load/failure benchmarks. GitHub Actions free-minute quota currently prevents fresh runner verification; no new CI pass is claimed.

## Plug-and-play onboarding and host metadata — 2026-11-24

**Status:** Implementation committed to `main`; local runtime acceptance and CI execution remain pending.

- [x] Added `scripts/start-local.ps1` and `scripts/start-local.sh`: create local `.env` from the example when absent, generate a localhost-only development TLS certificate through a temporary Docker container, run `docker compose up --build -d`, and probe the dashboard. Existing database volumes are preserved.
- [x] Removed browser-based target-OS selection from Add Machine. The enrollment dialog now provides both shell launchers (PowerShell/POSIX) and per-command copy actions; the launcher scripts are still interim source-checkout/Docker helpers, not the final automatic release installer.
- [x] Added `scripts/setup-windows-agent.ps1` with Docker-based Windows x64 build, secure one-time token prompt, optional remote gRPC/CA arguments and explicit `-Rebuild`. The current Windows agent remains foreground-only; Windows service installation and signed distribution are not delivered.
- [x] Added `scripts/setup-linux-agent.sh` and a Docker-based Linux build helper. The installer requires systemd and Docker for the source build, installs the service, securely prompts for a one-use token, and removes it from the service environment after session persistence.
- [x] Linux and Windows collectors now emit OS/platform/kernel/architecture and available virtualization metadata. Linux adds best-effort DMI-based provider hints; environment classification distinguishes container, cloud VM hint, virtual machine and physical/unknown. Provider detection is not guaranteed across all VPS vendors.
- [x] Added host-environment display to System information and unit assertions for classification. CI now cross-compiles the Windows x64 agent from Linux runners.
- [x] Refreshed the complete application shell styling: navigation/sidebar, typography, sticky operator header, panels, focus states, mobile navigation, Add Server and install-command surfaces.
- [ ] Build/run the updated Windows executable and accept the new UI against the already-enrolled Windows host. GitHub Actions free-minute quota has blocked runner assignment in recent runs; no fresh CI pass is claimed.
- **Architecture boundary:** Compose starts the control plane, not all monitored machines. Each monitored Windows/Linux host requires a native agent on that host. Remote agents need a reachable gRPC address and a trusted control-plane CA; `localhost` only works for a same-host agent. Do not expose the local anonymous Compose profile outside localhost.

## Dashboard UI refresh — 2026-11-24

**Status:** Visual implementation committed; local owner acceptance and frontend checks are still required.

- [x] Reworked the server detail screen into a monitoring-oriented view with current CPU, memory, busiest-volume and aggregate network KPI cards.
- [x] Added direct CPU / Memory / Disk / Network chart shortcuts, a full metric selector, 1h/6h/24h range controls, and a larger SVG history chart with a value axis, shaded trend area, latest-point marker and visible gaps.
- [x] Kept the existing history API, committed-telemetry WebSocket, project scope, and latest-snapshot/process APIs; no chart dependency or third-party monitoring platform was added.
- [x] Clarified that TCP state and listening-port inventory are not reported by the initial Windows collector.
- [ ] Pull the new main commit, rebuild the web container, and verify the refreshed UI against the enrolled Windows host. GitHub Actions quota is currently blocking fresh runner-based CI, so no new CI pass is claimed.
- **Next action:** Run `git pull origin main` followed by `docker compose up --build -d web`, then open the Windows PC detail page and check current KPI values and the history chart. Keep the agent window running.

## Next Work Session — 2026-11-24

**Target:** Finish owner-led Phase 4 dashboard acceptance, including the refreshed monitoring UI against the enrolled Windows agent, then continue Phase 5 Linux-MVP host validation. Dashboard and deployment/firewall tooling are implemented; current status is In Progress pending runtime test evidence and a real target host.

### Ordered release plan

1. Complete and verify Phase 4 Monitoring Dashboard and operator workflows.
2. Complete Phase 5 firewall setup for the Linux control-plane and agent-only host roles, following docs/PRODUCTION_FIREWALL.md.
3. Complete the controlled production deployment/shift, including TLS, secrets, private PostgreSQL/Redis, migrations, backups/restore, health checks, and rollback.
4. After Phase 4 and Phase 5, the project owner performs the first local end-to-end acceptance test before wider rollout or additional expansion work.

Phase 5 has two primary release deliverables for this Linux-first MVP: **(a) firewall setup** and **(b) production shift**. Windows support has an initial collector slice but still needs runtime verification, service lifecycle and signing; macOS support and fleet-scale work remain broader roadmap items. A Linux production cutover must not be represented as completing them.

### Phase 4 resume point

Read docs/PHASE_4_EXECUTION_PLAN.md. The dashboard now uses real authenticated inventory, history/live telemetry, project mapping, alert/notification and remote-operation APIs. Remote actions are restricted to the API allowlist and require owner/admin authorization plus a browser confirmation. Verify these flows against a live enrolled Linux agent; do not substitute mock data.

### Session boundary

Phase 5 Linux deployment and firewall tooling is prepared, but do not mark it complete until a selected host passes external network, gRPC enrollment, firewall, backup/restore and rollback checks. Keep Phase 4 In Progress until the owner completes the local dashboard/agent acceptance gate.

## Phase 1–3 Audit

The final Phase 1–3 audit passed all four jobs in [GitHub Actions run #45045054465 — success](https://github.com/WorkRCS/monitor_me/actions/runs/45045054465). This run includes strict lowercase batch-ID validation, telemetry integration, and a PostgreSQL outage/recovery canary. The audit also identified individual product requirements that remain In Progress or Planned; see docs/PHASE_1_TO_3_AUDIT.md.

### Explicit unfinished product requirements

- Server-detail metric charts/live updates, project scope/mapping administration, alert rules/in-app notifications and remote systemd actions are implemented in the dashboard; end-to-end owner acceptance remains pending.
- Full Docker Engine API discovery, process lifecycle/per-process CPU and persistent downsampled rollups: In Progress/Planned.
- Linux firewall/deployment helpers and production Compose/Nginx/backup/restore/rollback scripts are prepared; real host application and verification remain outstanding. Windows/macOS agents, signed installers and fleet-scale validation remain expansion work.
- Protobuf event/health/command messages are reserved; only authenticated telemetry batches are accepted by the current stream.

## Completed Phase 1 — Product Specification and Technical Blueprint

**Status: Completed**

- [x] Product requirements and MVP boundary.
- [x] Functional requirements with stable IDs and acceptance criteria.
- [x] Metric catalog with units, sources, calculations, timestamps, and platform limitations.
- [x] Feature acceptance matrix.
- [x] Architecture overview and data flow.
- [x] Five-phase delivery roadmap.
- [x] Professional README and continuous maintenance rules.

Key Phase 1 commits:

- 5c4bf9b3684107bfb3884ebf26181013ed42ea4b — Product requirements.
- 4f982c881823b9cdea6250432f96cdad407af94f — Metric catalog.
- 5bd649196345de671d58fa3d01a3187e6f689608 — Feature acceptance matrix.
- 6d4719e81ab032c9f35f9631275212f78aceef7d — Architecture overview.

## Completed Phase 2 — Platform Foundation and Secure Connectivity

**Status: Completed**

### Delivered implementation

- [x] Repository boundaries for backend, frontend, protocol, deployment, and native agent workspace.
- [x] Go API with environment configuration, JSON structured logging, bounded HTTP timeouts, health/readiness endpoints, and graceful shutdown.
- [x] Next.js, TypeScript, React, and Tailwind application shell.
- [x] Docker Compose services for PostgreSQL, Redis, migration runner, API, and frontend.
- [x] PostgreSQL connection pool and embedded versioned migration runner.
- [x] Redis client with bounded pool settings and explicit timeouts.
- [x] Dependency-aware readiness; liveness remains independent of PostgreSQL/Redis availability.
- [x] First-owner bootstrap, Argon2id password hashing, opaque bearer sessions, login/logout, and authenticated profile.
- [x] Role-based API authorization and organization-scoped server inventory.
- [x] Server creation, one-time enrollment-token issuance/reissue/revocation, and agent revocation.
- [x] Generated versioned Protobuf contract and TLS-protected gRPC server.
- [x] Hash-only agent credentials, 15-minute single-use enrollment tokens, stable agent identity, 30-day agent sessions, heartbeat, capability reporting, reconnect, and re-enrollment/session rotation.
- [x] Bounded gRPC messages (1 MiB) and concurrent streams (128 per connection).
- [x] Telemetry stream explicitly deferred until Phase 3 durable ingestion is implemented.

### Verification evidence

GitHub Actions run [#45030320358](https://github.com/WorkRCS/monitor_me/actions/runs/45030320358) completed successfully:

- [x] Go dependency resolution and verification, Protobuf generation, gofmt, race-enabled tests, go vet, and backend builds.
- [x] Frontend dependency install, ESLint, TypeScript check, and production build.
- [x] Docker Compose configuration validation and full stack startup.
- [x] PostgreSQL migrations and Redis connectivity checks.
- [x] Owner registration/login/logout and role/organization authorization integration tests.
- [x] TLS gRPC enrollment, heartbeat, reconnect, re-enrollment, credential expiry/rejection, and agent revocation.
- [x] PostgreSQL durability and readiness recovery during Redis outage.
- [x] Full stack cleanup after CI.

### Phase 2 boundaries at phase close (historical)

At the moment Phase 2 was closed, the frontend was a foundation shell and the native agent/telemetry stream had not yet been implemented. Phase 3 has since added the Linux agent, durable telemetry path, history, retention, and live delivery. The Compose deployment remains a local development topology, not a production deployment design.

### Key Phase 2 commits

- 1ce85a0b91cf0cce5fa1a29b94967a70c08f87d0 — Authentication, identity, TLS gRPC enrollment, and integration tests.
- 5873f5310b7d7e91a74ef4bcd68ec8bbd12cf142 — Go formatting.
- 17fd0f00c6a094febe768693d9b11101987d806c — Integration/server formatting.
- 9d75c7bcc2027fbd2ee7220ff8e06a8c5a3086a9 — Final integration-test formatting.
- 6b250d32b9e87d3fa8a909594df2284cc0738c8f — Tenant isolation and Redis outage/recovery tests.
- 3afe7bd43715ea7b6bc0c797e2c311f574798d83 — Health endpoint HEAD support and test correction.
- f93f2be2a72abae3056c9610834fad577c4a2d14 — Resolve Protobuf integration-test dependencies in CI.
- 8ba37d96ce10bd5781a02df20f28f0453e6a0be1 — Correct integration TLS certificate path.
- 3f67f317676227f287096c28388acbf90541cf4f — Explicit gRPC reconnect integration test.
- 1ad3e70d6e3a6174da2ebae6b43fdf117117f465 — Bound concurrent gRPC streams and document native agent workspace.
- 694ffccdf8676b03fe9212366c3abaa17b2de985 — Format reconnect heartbeat request; full three-job CI passed.
- 694ffccdf8676b03fe9212366c3abaa17b2de985 — Format the reconnect heartbeat request.

## Completed Phase 3 — Monitoring Agent and Telemetry Data Engine

**Status: Completed — Phase 3 acceptance gate verified. The overall P0 MVP remains incomplete until Phase 4 dashboard workflows are delivered.**

**Verification evidence:** [GitHub Actions run #45043161542 — success](https://github.com/WorkRCS/monitor_me/actions/runs/45043161542). All four jobs passed: Go backend, native Linux agent, Next.js frontend, and Docker Compose integration. The Compose job verified migrations, authenticated TLS enrollment, telemetry/history/retention, Redis outage/recovery, and PostgreSQL canary durability across service restart.

### Delivered implementation

- [x] Native Linux Go agent with validated configuration, JSON logging, graceful shutdown, explicit capability reporting, and a hardened systemd unit validated with systemd-analyze.
- [x] Linux procfs aggregate/per-core CPU, memory/cache/swap, disk throughput/IOPS, network throughput/packets/errors, host/agent uptime, process count and top-128 process RSS; process CPU and agent-health telemetry report collector errors, WAL byte size, pending WAL batches and last successful ACK timestamp.
- [x] TCP connection-state counts, listening-port inventory, cgroup selectors, Systemd unit and container cgroup-ID recognition.
- [x] Counter rates based on monotonic elapsed time; initial samples, resets, and invalid counter intervals never generate fabricated negative rates.
- [x] Private append-only JSONL WAL with fsync, bounded disk quota, truncated-tail recovery, and atomic acknowledgement compaction.
- [x] TLS gRPC enrollment, private persisted agent session, heartbeat, reconnect/backoff, and streaming telemetry batches.
- [x] Bounded authenticated ingestion, batch schema/label validation, idempotency hash, post-commit ACK, and negative/mismatched ACK retention tests.
- [x] PostgreSQL transactionally stores batches and samples; Redis is used for organization-scoped ephemeral live notifications only.
- [x] Authenticated historical range API with exact epoch-aligned `date_bin` buckets, 100ms minimum step, 30-day raw range, and maximum 5,000 points.
- [x] 30-day raw sample and 90-day idempotency retention; expired WebSocket tickets cleaned in bounded batches.
- [x] 30-second single-use live tickets bound to user sessions, logout invalidation, cross-organization isolation, 32-message bounded subscriber queue, slow-client disconnect, and reconnect.
- [x] Database attribution verifies mapped Systemd workload samples and keeps host-total, unclassified, and shared resource categories separate.
- [x] CI benchmarks execute Linux collection, counter-rate calculation, WAL append, live event preparation, and real TLS gRPC-to-PostgreSQL ingestion.
- [x] Redis and PostgreSQL dependency outage/readiness recovery are exercised in Compose CI. The protobuf AgentHealth envelope remains reserved; its health fields are emitted as ordinary telemetry samples. Persistent downsampled rollups remain a later scale optimization.

### Benchmark snapshot (GitHub-hosted Ubuntu runner)

| Benchmark | Result |
| --- | ---: |
| Linux proc collector | 5.33 ms/op; 1.03 MB/op; 5,163 allocs/op |
| Counter rate calculation | 3.68 ns/op; 0 allocs/op |
| One-sample WAL append/fsync | 256 μs/op; 952 B/op; 12 allocs/op |
| Live event preparation (96 input samples) | 128 μs/op; 165.7 KB/op; 897 allocs/op |
| TLS gRPC to PostgreSQL (64 samples/batch) | 4.40 ms/batch; 14,533 samples/sec; 9.2 KB/op; 535 allocs/op |

These measurements are a single CI-runner snapshot, not production capacity guarantees or evidence of million-host scale.

### Phase 3 boundaries

- Implemented host collector platform: Linux. Windows and macOS are Phase 5 work.
- Full Docker Engine API discovery is not implemented; container identification uses cgroup metadata exposed by the host.
- Per-process CPU and eBPF are not implemented.
- Listening ports and TCP state are not HTTP request latency or HTTP error telemetry. Those require application instrumentation, proxy integration, or tracing.
- Historical aggregation is performed on query with PostgreSQL `date_bin`; persistent materialized rollups and table partitioning remain later scale optimizations.
- Dashboard, project selector, alerting, notifications, installer UX, and remote operations remain Phase 4 work.

## Current Phase 4 — Dashboard and Operational Features

**Status: In Progress — started 2026-11-23**

### Phase 4 scope

- [ ] Implement authenticated Next.js login/logout and server session handling.
- [ ] Build server inventory, empty/loading/error states, Add Server and one-time enrollment-token display.
- [ ] Build server detail and overall host dashboard using the verified history/live APIs.
- [ ] Add All Projects / project selector with no-full-reload data scope changes.
- [ ] Add Canvas/WebGL charts after rendering benchmarks.
- [ ] Add process/port/connection/workload views and supported application instrumentation integrations.
- [ ] Add alert rules, deduplication, recovery events and notifications.
- [ ] Add allowlisted, authorized and audited remote restart/stop operations through the agent control channel.

### Delivered slice — 2026-11-23

- [x] First-owner setup, login, current-session lookup and logout use same-origin Next.js route handlers.
- [x] Bearer credentials are held in an HttpOnly, SameSite=Lax cookie; browser JavaScript only receives the safe user profile.
- [x] Organization-scoped server inventory, loading/empty/error states, Add Server, one-time enrollment-token display/copy/expiry, and role-aware revoke/reissue controls are implemented.
- [x] Token reissue revokes all previous unused enrollment credentials in the same PostgreSQL transaction; an integration test covers the replaced-token rejection path.
- [x] Frontend lint/typecheck/production build and full-stack verification passed in [GitHub Actions run #45188259431](https://github.com/WorkRCS/monitor_me/actions/runs/45188259431).

### Exact resume instructions

Continue directly on `main`. Do not create a new branch.

**Next implementation slice:** finish the Phase 4 server-detail experience using the verified historical-series and live-ticket APIs.

1. Add a typed API client for login, current-user, logout and server inventory endpoints.
2. Implement login and session handling without exposing bearer tokens in logs or URLs.
3. Build the server inventory list with loading, empty, error and disconnected states.
4. Add the Add Server flow and show the one-time enrollment token exactly once with copy/install guidance.
5. Add component/API tests and keep all UI strings, comments and documentation in English.
6. Update README, feature acceptance matrix and this state file after the slice passes CI.

Do not start charts, alerts, project switching or remote actions until the authenticated server inventory and Add Server flow are accepted and tested.

## Repository Rules

- English only in source code, comments, documentation, UI strings, logs, tests, and commit messages.
- All work must be committed directly on main; do not create a branch or PR unless the user explicitly changes this instruction.
- Build the monitoring product itself; do not embed Grafana, Prometheus, Zabbix, Netdata, or another ready-made monitoring platform.
- Go, Next.js/TypeScript, PostgreSQL, Redis, Docker Compose, gRPC, Protocol Buffers, and WebSocket are allowed technologies.
- The host monitoring agent will run as a native Go service; Docker is for control-plane development/deployment and optional container discovery.
- Comments explain intent, invariants, security boundaries, and non-obvious logic; do not comment every trivial line mechanically.
- Use descriptive Conventional Commits.
- Update README whenever a feature is verified and at each phase completion.
- Never mark a phase complete based on a partial implementation or a green build alone.

## Session History

Phase 1 and Phase 2 implementation/verification history is recorded above. The final Phase 1–3 audit passed all four jobs in [GitHub Actions run #45043161542](https://github.com/WorkRCS/monitor_me/actions/runs/45043161542).


## Windows Local-Agent Slice — 2026-11-23

- Added a Windows-specific collector using gopsutil for CPU, memory/swap, mounted-volume capacity, disk/network counters, uptime, host identity and readable process RSS.
- Added explicit Windows capabilities; Linux-only TCP/listening-port, cgroup/Systemd/container attribution and systemd remote actions are not advertised or run on Windows.
- Added Windows-specific collector tests, Docker-based x64 cross-build and PowerShell foreground-run helpers. The build helper now invokes a checked-in shell script rather than passing a multiline command through PowerShell native argument parsing.
- Added a Windows-specific atomic spool-file replacement using MoveFileEx so acknowledged batches can be compacted after durable delivery.
- Added a host-health snapshot and System information panel to server detail.
- Made persisted agent session handling compatible with Windows FileMode behavior; the supported local helper stores session/WAL below the current user's LOCALAPPDATA.
- Verification status: GitHub code and docs were inspected, but no Windows runtime or compile test has yet been executed for these new commits. GitHub Actions quota is still blocked. The owner must run the local PowerShell build and agent acceptance helper to provide actual Windows verification.
