# Monitor Me — Product and Engineering Backlog

**Last reviewed:** 2026-11-24  
**Delivery rule:** Work directly on `main`. Do not start feature upgrades until the current source builds cleanly and the complete local stack passes its acceptance gate.

## Product scope lock — 2026-11-24

The intended product is a simple independent monitoring installation, not a hosted team workspace. The main operator navigation is limited to **Servers**, **Logs**, **Dashboards** and **Alerts**. There are no browser user accounts, organizations, customer projects, workspace selectors or Project Mapping screens. Technical workload context (process, systemd unit, container, cgroup, volume and interface) stays attached to the host as metadata. The UI source has been simplified accordingly, but no fresh frontend build or browser acceptance has run yet.

Do not treat removing a menu item as completing the backend migration. Existing project and organization tables, keys and filters still exist in the database/API and must be removed through forward migrations that preserve machine, metric, log, alert and dashboard data. Do not delete project/organization tables or reset volumes as a shortcut.

The root-level `install.sh` does not yet exist. The current `deploy/production/deploy.sh` requires an operator-prepared environment file, Docker, Nginx, Certbot, a valid TLS certificate, hostname, gRPC address and `ADMIN_CIDR`. The requested `git clone ... && sudo ./install.sh` workflow is a release-blocking deliverable, not an implemented command.

## 0. Release-blocking stability gate — first

- [ ] Pull the exact latest `main` revision and verify the local source hash before building. The Windows and POSIX startup scripts now reject a stale/diverged checkout, validate Compose configuration, build every image first, and only then start services with `--no-build`.
- [ ] Generate and commit `backend/go.sum` from a trusted successful Go module resolution; do not disable `GOSUMDB` or checksum validation. The backend currently has `go.mod` but no committed `go.sum`.
- [ ] Generate and commit `web/package-lock.json`, then change the web Dockerfile and CI from `npm install` to `npm ci` so clean builds use a deterministic dependency tree.
- [ ] Confirm Go module resolution and the Linux/Windows agent cross-build on a clean network path. The backend image no longer runs `go mod tidy`; it builds only the shipped API/migration packages with `-mod=mod`, HTTP/1.1 module transport, and the standard checksum database still enabled.
- [ ] Pass backend and migration Go builds, Go unit/race tests, vet and formatting checks.
- [ ] Pass Linux amd64/arm64 and Windows amd64 agent artifact builds; confirm the artifact bundle and manifest checksums.
- [ ] Pass frontend lint, TypeScript typecheck and production Next.js build.
- [ ] Pass Compose integration: PostgreSQL, Redis and ClickHouse healthy; migrations current; API and dashboard ready; no-login installation endpoint works; removed login routes return 404; agent enrollment succeeds.
- [ ] Validate the same path on the owner's Windows Docker Desktop, then on a clean supported Linux host. Record the exact commit and command output. No green build/test/acceptance claim until the relevant command actually passes.

**Current verification caveat:** Recent GitHub Actions attempts created jobs but assigned no runner and executed zero steps. That is not a passing or failing code test. The reason for runner allocation failure is not established. The available assistant execution environment also does not have the required Go/Node versions or Docker, so owner-machine/runtime acceptance remains necessary.

## 1. Remove legacy customer, profile and organization logic

- [ ] Make the data model genuinely single-install: remove interactive users, profiles, organizations, memberships, sessions, customer projects, project mappings and tenant-based authorization from backend APIs, queries, tests and migrations.
- [ ] Remove `organization_id` and `project_id` scoping, project tables/mappings and user actor foreign keys from server inventory, enrollment, telemetry, logs, dashboards, alerts, operations, audit and live-stream records. Preserve useful workload context as machine/process/service/container labels. Installation-local ownership replaces tenant ownership.
- [ ] Add a carefully ordered, numbered forward migration for existing databases. Preserve servers, agent/session identity, metric/log history, dashboards, alert history and audit data. Detect ambiguous or corrupt legacy state and stop with an actionable message; never pick an arbitrary organization or drop data.
- [ ] Keep agent enrollment tokens and device/session credentials: these authenticate monitored machines, not customer accounts or browser users.
- [ ] Ensure one installation maps to one private datastore/volume set. No hosted SaaS, shared customer tenancy or central account service.

## 2. One-command self-hosted installation and upgrades

- [ ] Ship a tested installer for supported control-plane Linux distributions and architectures: prerequisite checks, secure secret/TLS generation, private environment file, storage, migrations, Compose startup, readiness, diagnostics and dashboard URL.
- [ ] Make install, reconfigure, migrate, upgrade, rollback, backup and restore repeatable and idempotent. Preserve operator data and secrets; never run `docker compose down -v` in upgrade paths.
- [ ] Make remote dashboard access opt-in, HTTPS-only and restricted to an explicit VPN/admin CIDR. Keep databases and internal service ports private. Agent-only machines need outbound connectivity only.
- [ ] Clearly distinguish automatic host firewall changes from cloud security groups/VPC ACLs that require the owner's cloud IAM permissions and explicit approval.

## 3. Native agent installation and real host discovery

- [ ] Publish signed, versioned, checksum-verified Linux amd64/arm64 and Windows amd64 artifacts from an operator-configurable release origin.
- [ ] Target-side launcher detects OS, architecture, distribution, system/service manager, VM/bare-metal/container hints and cloud metadata locally; no manual OS selector, Docker, Go, Node or repository checkout on the monitored host.
- [ ] Validate one-time enrollment, credential rotation/revocation, service restart/recovery, upgrade, rollback and uninstall on clean Linux and Windows machines.
- [ ] Report real CPU, memory, swap, disk, filesystem, network, uptime, process and supported service metrics with explicit stale/unsupported/permission-denied states. Do not fabricate zeroes or unsupported platform capabilities.

## 4. Grafana-style real-time logs

- [x] Improve the Log Explorer table to show timestamp, severity, source, structured labels and an event-detail JSON view; make the 10-second polling control explicitly say auto-refresh rather than implying a push stream. Source change is committed; browser acceptance is pending.
- [ ] Complete native sources: Linux journald plus configured syslog/file tail (inode/offset checkpoints and rotation); Windows Application/System Event Logs plus configurable file tail. Security Event Log requires a separately documented permission/privacy policy. Current Windows Application/System reader source exists but has not passed runtime acceptance; file tail is still missing.
- [ ] Preserve source event time and observed/ingest time separately. Keep source precision where available; expose UTC and selected local timezone clearly. Do not claim microsecond precision when the source only provides milliseconds.
- [ ] Replace the Log Explorer's current 10-second polling toggle with an authenticated server-push stream (WebSocket or SSE) with reconnect, cursor replay, bounded queues, back-pressure, pause/resume and explicit dropped-event counters. The current log UI is bounded/keyset-paginated, but logs are not yet pushed natively in real time.
- [ ] Add time range, severity, node/source, text and structured-field filters; event JSON/detail, context around an event, saved queries, export, virtualized rendering and clear no-data/permission/stale states.
- [ ] Keep agent spool and ingestion bounded and durable; batch writes; use idempotent IDs; cap query time/range/rows; apply retention and tested ClickHouse backup/restore.

## 5. Grafana-style metrics and dashboards

- [x] Improve saved-dashboard time-series panels with timestamp-scaled x-axis, average line, min/max band, y-axis labels, visible gaps and latest-point tooltip. Expand the panel metric catalog to include currently defined host metrics. Browser/runtime acceptance is pending.
- [x] Replace icon-only navigation with full words and simplify the primary navigation to Servers, Logs, Dashboards and Alerts. Remove Project Mapping and Remote Operations from the main menu; remove project filters from logs, dashboards, alerts and host metric detail. Refresh the dark slate UI with a calmer teal accent and keyboard focus indicators. Source changes are committed; frontend build/browser acceptance is pending.
- [ ] Deliver a global node selector, time range, timezone, live/pause state and explicit refresh interval.
- [ ] Add low-latency metric push/update without manual page refresh, reconnect and stale-data indicators. Choose a default refresh cadence only after CPU/network/storage profiling.
- [ ] Add a dashboard/panel editor with resize/reorder, stat, time-series, table and logs panels; query editor, units, legends, thresholds, variables/templates, saved dashboards and annotations.
- [ ] Support per-node and installation-wide views without introducing organizations, projects, workspaces or user accounts.

## 6. Alerting and operations maturity

- [ ] Add alert duration/for windows, severity, grouping/deduplication, maintenance silences, multi-condition rules and recovery semantics.
- [ ] Add configurable notification routing (webhook/email and other explicitly configured channels), retry/delivery history and escalation policy.
- [ ] Keep remote operations least-privilege, allowlisted, audited, idempotent and disabled where the platform cannot safely enforce them.

## 7. Reliability, security and production acceptance

- [ ] Add failure/restart tests for API, database, Redis, ClickHouse, network, agent spool and live-stream reconnect/replay.
- [ ] Test backup/restore, disk-full behavior, bounded memory, retention, upgrade/rollback, IPv4/IPv6 and VPN/firewall boundaries.
- [ ] Benchmark realistic node counts and log distributions; publish hardware, versions, events/sec, bytes/sec, p50/p95/p99 latency, CPU/RSS, disk growth and recovery behavior. Treat 10,000 events/sec for 60 minutes and 100-million-event search as proposed acceptance targets, not current guarantees.
- [ ] Add dependency/image scanning, secret scanning, signed release provenance and update integrity checks.

## 8. Later scope

- [ ] OpenTelemetry traces/APM, service maps and request correlation after metrics/log identity and correlation paths are stable.
- [ ] macOS/launchd and additional non-systemd platforms only after native collectors and lifecycle tests exist.

---

## Current implementation snapshot (not a release claim)

- The monitoring UI source now uses a dark slate/teal palette and a simple Servers / Logs / Dashboards / Alerts navigation. Project filters and Project Mapping navigation have been removed from the product-facing UI. Time-aware average/min/max charts and structured log labels remain. This has not yet passed owner browser acceptance or a frontend build.
- The new [platform support matrix](PLATFORM_SUPPORT_MATRIX.md) separates Linux control-plane hosting from Linux/Windows agent build targets and records the pending runtime acceptance gates.
- No-login single-install dashboard/API shell is present; local Compose dashboard/API/database ports are loopback-bound.
- Host metric agent, durable telemetry spool, TLS gRPC delivery and the initial metrics/alerts/dashboard/operations slices exist.
- Linux journald → bounded log spool → TLS gRPC → ClickHouse ingestion, bounded 200-row keyset search and initial Log Explorer exist.
- Windows Application/System Event Log collection source and redaction tests exist in source, but Windows runtime acceptance is pending.
- Log Explorer live mode currently polls every 10 seconds; native push, file tail, saved searches, context search and production ClickHouse recovery/scale acceptance are incomplete.
- Legacy user/organization IDs, organization-based schema/query paths, project tables and project_id query paths remain in the database. The product-facing UI hides project concepts, but a numbered, data-preserving migration and backend/API cleanup are still release blockers.
- A source-portability scan found no dependency on the current GitHub owner/repository URL in Go imports. Stable internal Go module paths are intentional; release/download origins must remain configurable.
