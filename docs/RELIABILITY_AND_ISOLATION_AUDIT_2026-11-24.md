# Monitor Me — Reliability, isolation and Grafana-parity review

**Review date:** 2026-11-24  
**Branch:** `main`  
**Review scope:** Critical-path source review across the 187-file repository tree: identity/authentication, tenant scoping, projects, server inventory/liveness, agent telemetry ingestion and WAL acknowledgement, alert evaluation, notifications, remote operations, builds and platform installers, and dashboard workspaces. This is not a line-by-line proof of every file and does not replace execution tests.

## Executive assessment

Monitor Me is a real metrics-monitoring foundation, not yet a complete Grafana replacement. The most important risks found in the review were corrected in source and regression coverage was added. Those tests have **not** been executed for these new commits because the latest GitHub Actions run ended with an empty job list. Treat the changes as implemented but pending runtime verification.

For strict data ownership today, deploy one self-hosted installation per customer. Each installation owns its PostgreSQL data and Compose volumes. The current multi-organization mode provides application-level logical isolation in one database; it is not yet a physically separate database per organization, and RLS is not enabled. Do not advertise a shared multi-tenant SaaS as production-ready until the remaining isolation gates below pass.

## Corrective changes committed

1. **Remote operations and audit atomicity.** Enqueueing a systemd operation and writing its audit record now happen in one database transaction. If the audit insert fails, the command cannot remain queued without an audit record. An integration test injects an audit failure and checks rollback.
2. **Heartbeat truth.** Server inventory derives state from authenticated agent heartbeat timestamps: stale after 45 seconds and offline after 90 seconds. Pending, disabled and revoked states remain distinct. The dashboard refreshes inventory and its selected-server state every 15 seconds and no longer displays a permanently green API badge.
3. **Alert delivery reliability.** Rule evaluation, alert-event transitions and in-app notifications are now in the same transaction as the telemetry batch. A database error returns gRPC Unavailable and rolls the batch back, so the agent can replay its persisted WAL item. Alert rules are fetched once per batch, scoped by organization/server/metric, and matched by project. An integration test injects notification failure and asserts that batch, samples and firing event all roll back before the same batch ID is retried.
4. **Organization ambiguity.** A user with memberships in multiple organizations is no longer silently placed into the oldest organization. Login fails closed with HTTP 409 until organization switching is implemented. Use a distinct user per organization meanwhile.
5. **Repository portability.** Go modules, source imports, generated Protobuf package metadata, Docker build flags and Protobuf scripts use stable internal module namespaces rather than the previous GitHub owner/repository. Moving the Git remote does not change those build imports.

## What currently exists

- Persistent organization-owned projects with ID/name/slug, plus organization-scoped server/workload mappings. Projects are database entities, not mock cards.
- Linux-first native host metrics, an agent-side bounded durable spool, authenticated TLS gRPC ingestion, telemetry-batch idempotency, PostgreSQL history/retention and Redis-only live notification fan-out.
- Input validation rejects NaN and infinite metric values; batch acknowledgements occur only after the PostgreSQL transaction commits.
- Organization-scoped server, project, metric, alert, notification and remote-operation queries; composite foreign keys tie project/server references to the same organization.
- Basic threshold alert rules (`gt/gte/lt/lte`) and in-app firing/recovery notifications.
- Constrained remote operations for allowlisted systemd service restart/stop, with owner/admin authorization and audit records.
- Initial server charts and workspaces for Servers, Projects, Alerts and Operations.

## Major gaps — do not advertise as finished

- **Logs: 0%.** There is no raw log agent pipeline, no ClickHouse deployment/schema, no indexed search, no Log Explorer and no live tail.
- **Traces/APM: 0%.** No trace ingestion or service-map/request instrumentation.
- **Alerting is basic.** No `for` duration, severity model, routing tree, webhook/email/SMS integrations, escalation, silences/maintenance windows or multi-condition expressions. The current notification channel is in-app only.
- **Projects are not full workspaces yet.** They persist and map workloads, but do not have project-specific metric dashboards, saved queries or project-level overview charts.
- **Settings and team administration are incomplete.** No finished member invitation/role management or organization switcher. Multi-membership login deliberately fails closed.
- **UI is not Grafana-equivalent.** There is no dashboard builder, panel library, variables, templating, annotation system, broad data-source/plugin ecosystem or Log Explorer. The app uses a custom UI and Lucide icons; shadcn/ui has not yet been adopted.
- **Platform coverage is limited.** Linux systemd is the primary implemented collector; Windows x64 service code exists but owner runtime acceptance is pending. macOS/launchd and non-systemd Linux are not supported acceptance targets.
- **Production and scale gates remain.** Code signing, upgrade/rollback/uninstall acceptance, host IPv4/IPv6 firewall checks, cloud security-group integration, backups/restore, multi-node/high-volume load tests and millions-of-events benchmarks remain open.

## Tenant isolation model

| Boundary | Current assessment |
|---|---|
| Server and project ownership | Organization-scoped query paths and composite foreign keys exist |
| Telemetry history | Query and ingestion paths carry organization/server/agent identity |
| Alerts and notifications | Organization-scoped rule/event/notification persistence |
| Live events | Organization-specific channels/tickets |
| Operations and audit | Organization-scoped; enqueue and audit now atomic |
| Multiple memberships per login | Fails closed; organization chooser is not implemented |
| Database-level RLS | Not implemented |
| Physical data store per organization in a shared deployment | Not implemented |
| Million-tenant isolation/load proof | Not tested |

A customer's self-hosted installation can keep its database and volumes on that customer's own VPS/server. If many customers share one deployment, their rows still live in one PostgreSQL cluster and depend on application-level organization predicates. The latter must not be described as physically private per-customer storage.

## Approximate Grafana-style feature coverage

These percentages are **subjective feature-scope estimates**, not benchmark scores, parity certification or a comparison of performance.

| Area | Estimated coverage today | Why |
|---|---:|---|
| Basic host metrics collection/history | 45–55% | Linux host collection, persisted metric history, bounded queries and basic charts exist; source coverage and dashboard customisation are limited |
| Server and project operations | 40–50% | Inventory and real project/mapping entities exist; no project dashboards, broader orchestration or team settings |
| Alerting | 15–20% | Basic threshold rules and in-app firing/recovery only |
| Logs | 0% | No end-to-end log data plane |
| Traces/APM | 0% | Not implemented |
| Dashboard/UI breadth | 20–25% | A few monitoring workspaces exist; no general dashboard builder, panel ecosystem or Log Explorer |
| Plug-and-play lifecycle | 45–55% | One-use enrollment, checksum-verified native artifacts and initial service installers exist; control-plane bootstrap, signing and runtime acceptance remain |
| Application-level tenant boundary | 55–65% | Organization scoping and relational constraints exist; RLS, org switching, memberships and scale proof remain |

**Overall Grafana-style observability scope: approximately 20–25%.** Monitor Me is not more advanced than Grafana today. A high-confidence percentage must wait until metrics, alerts, logs, traces, dashboards, access control and scale tests are delivered against an agreed feature checklist.

## Verification status

The new regression tests are committed but unexecuted. The latest workflow run, #45336425843, is marked completed/failure and returned no jobs. This means no Go, frontend, agent or Compose test result exists for these changes. Do not infer that source compiles or runtime behavior passes from the commit alone.

Before release, run:
- Go formatting, race-enabled unit tests, vet and backend integration tests;
- agent unit/race tests and Linux/Windows service installation tests on supported hosts;
- frontend lint, TypeScript and production build;
- clean Compose startup, migration, live telemetry, alert-failure replay, tenant-isolation and backup/restore tests;
- IPv4/IPv6 and external firewall checks on an actual production host;
- 10k log events/second sustained and 100-million-event search/retention benchmarks only after the log pipeline exists.

## Recommended next implementation order

1. Run all committed tests and repair every failure before feature expansion.
2. Finish authenticated organization selection, membership invitations and admin settings—or keep one user per customer in the self-hosted product.
3. Build the durable Linux journald/file log collector and Windows Event Log collector, ClickHouse schema/ingest/search/retention, bounded paging and live tail.
4. Expand alerting with duration, severity, routing, notification destinations, silences and delivery retry/outbox.
5. Add project dashboards, dashboard builder, panel library, variables and saved queries.
6. Adopt shadcn/ui primitives incrementally for consistent accessible dialogs, forms, menus, tables and tabs after the current UI tests are green.
7. Run documented tenant-isolation, recovery, load and production acceptance gates before calling the product Grafana-replacement ready.
