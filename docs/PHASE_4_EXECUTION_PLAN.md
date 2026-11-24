# Phase 4 — Monitoring Dashboard Execution Plan

**Prepared:** 2026-11-22  
**Planned work session:** 2026-11-23  
**Target:** Complete Phase 4 — Dashboard and Operational Features before starting Phase 5.

Phase 4 implementation started on 2026-11-23. The former login/register slice has been removed from the product. The current dashboard opens in a single-install context and includes server history/live metrics, project/mapping administration, alert/notification workflows, and allowlisted remote operations; the no-login refactor still requires a fresh green CI run. Current acceptance and CI re-verification remain pending because the repository owner reports the Actions free-minute quota is exhausted. Continue directly on main; do not create a branch.

## 1. Required delivery order

1. Complete Phase 4 dashboard and operator workflows.
2. Complete Phase 5 firewall setup for the Linux control-plane and monitored-host roles.
3. Complete the controlled production deployment/shift, including production configuration, TLS, private data services, backup/restore, health checks, and rollback.
4. After those two phases, the project owner performs the first local end-to-end acceptance test. Do not proceed to wider rollout or additional expansion work until the owner has reviewed the local test results.

Phase 5 has two primary release deliverables for this Linux-first MVP:

- **Firewall setup:** implement and validate the policies documented in docs/PRODUCTION_FIREWALL.md.
- **Production shift:** deploy the verified control plane and agent workflow to the selected production VPS/VM with a repeatable, recoverable procedure.

The existing Windows/macOS and fleet-scale roadmap items remain documented as broader platform/scale work. They must not be silently claimed as complete by the initial Linux production deployment.

## 2. Current implementation baseline

The frontend now opens directly into a single-install dashboard with server inventory/enrollment, server-detail historical charts and live WebSocket updates, project/mapping administration, alert rules and in-app notifications, and a remote-operations workspace. No user login, signup, logout, profile or organization-creation route is part of this product mode. The backend provides these API foundations:

| Capability | Existing endpoint | UI work required |
| --- | --- | --- |
| Single-install bootstrap | GET /v1/installation | Initialize the local installation context without signup, login or organization selection |
| Server inventory | GET /v1/servers | Inventory page, loading/empty/error states, server status/last-seen display |
| Add Server | POST /v1/servers | Form, one-time enrollment-token handling, copyable Linux install/enrollment instructions |
| Reissue/revoke credentials | POST /v1/servers/{serverID}/enrollment-token, POST /v1/enrollment-tokens/{tokenID}/revoke | Authorized credential lifecycle controls and clear expiry/revocation feedback |
| Historical metric series | GET /v1/servers/{serverID}/metrics | Server detail charts, metric selection, time range, units, gaps and stale states |
| Live updates | POST /v1/live/ticket, GET /v1/live/ws | Ticket-based live connection, reconnect handling, and history catch-up |
| Projects and workload mappings | GET/POST/DELETE /v1/projects and project-server mapping routes | Project workspace, mapping administration, and project scope selection |
| Alerts and notifications | /v1/alert-rules, /v1/alerts, /v1/notifications | Rule management, firing/recovery events, in-app notifications/read state |
| Remote operations | /v1/remote-operations and POST /v1/servers/{serverID}/remote-operations | Allowlisted operation selection, explicit confirmation, status and audit history |

The existing historical query is bounded: 100ms minimum aggregation step, 30-day raw range, and at most 5,000 output points. The live ticket is single-use and expires after 30 seconds. The UI must request a fresh ticket after reconnect; it must not store or reuse a consumed ticket.

Project mapping administration, alert evaluation/in-app notifications, and constrained remote systemd operations now have backend/UI paths. Remote execution is available to the trusted installation operator, but still requires a configured REMOTE_ALLOWED_SYSTEMD_UNITS allowlist, a recent online agent, and an end-to-end result/audit acceptance test. External notification-provider delivery, application HTTP request metrics, and one-command agent installation are not complete. The current gRPC stream carries telemetry batches; remote systemd operations use the agent's authenticated device credential and installation-level command path. Do not simulate these features with hard-coded dashboard values.

## 3. Phase 4 implementation order

### Slice 1 — No-login single-install shell

**Implementation committed; fresh CI/runtime acceptance pending:** the browser login/register screens and API endpoints have been removed. The web dashboard requests `/api/installation`, and the backend resolves one implicit local installation context. No browser session cookie or account is required.

- Keep local management ports loopback-bound.
- Keep all normal operator APIs behind the production Nginx administrator/VPN allowlist; do not expose owner-level installation capabilities publicly.
- Test first startup against an empty database and a database created by the previous version.
- Verify loading, unavailable, and recovery states without introducing a login fallback.

### Slice 2 — Server inventory and Add Server

**Historical implementation retained under one implicit installation context:** server inventory, empty/loading/error states, server creation, one-time token display/copy/expiry, token rotation and revoke controls are in place. Reissuing a token revokes prior unused credentials transactionally. Agent deployment remains guided/manual; a fully automated installer is not yet claimed.

- Build single-install inventory with empty, loading, error, connected, stale, and disconnected states based only on available backend fields.
- Implement Add Server using the existing create-server endpoint.
- Display the returned enrollment token only once; support explicit copy and expiry information.
- Generate an installation/enrollment command from the actual configured control-plane address and CA instructions. Never embed a reusable secret in a committed file or log.
- Add safe reissue/revoke flows for the trusted installation operator. The deployment network boundary, not browser accounts, controls access.

### Slice 3 — Server detail and monitoring dashboard

**Implemented; owner acceptance pending:** server detail uses bounded historical-series queries and one-use live tickets, with metric selection/time ranges, timestamps/units, stale/gap states, reconnect using fresh tickets, and history catch-up. Verify these states against an enrolled agent before treating the slice as accepted.

- Build an overall host view for supported Linux CPU, memory, swap, disk, network, uptime, process, and TCP/listening-port information.
- Use real metric API responses only. Show units, timestamps, selected time range, empty periods, gaps, and stale/last-seen indicators.
- Add bounded historical charts and live updates using one-use WebSocket tickets.
- Reconnect with a new ticket and query historical data to catch up after a disconnect.
- Preserve the distinction between host totals, project-attributed, shared, and unclassified resources.

### Slice 4 — Project and workload views

- Project scope selectors and project/workload mapping administration are implemented in the dashboard. Verify that every affected panel/query changes scope without a full-page reload and that host-total/shared/unclassified attribution remains distinct.
- Clearly identify cgroup/Systemd/container-derived attribution and its limitations. Do not claim full Docker Engine discovery.

### Slice 5 — Operational features

- Add supported process/port/connection/workload views.
- Alert rules, recovery events and persisted in-app notifications are implemented; external provider delivery and its retries remain open.
- The remote-operations UI and same-origin proxy routes are implemented over the API/agent allowlist and audit path. Verify queue, dispatch, completion, expiry, failure, and role-denial behavior end to end. No arbitrary remote shell is provided.
- The guided one-command Linux agent installation workflow remains open. User, login, membership and organization administration are out of scope for the self-hosted single-install product.

### Slice 6 — Phase 4 verification and documentation

- Add component/API/integration tests for normal, empty, stale, missing installation dependencies, invalid agent credentials, expired enrollment, reconnect, and failure states.
- Verify dashboard values and attribution against backend fixtures and actual telemetry ingestion.
- Run Go tests/race tests/vet/build, frontend lint/typecheck/production build, and the complete Compose integration pipeline.
- Update README, the feature acceptance matrix, roadmap, and project state from the test evidence.
- Mark Phase 4 Completed only when every Phase 4 completion-gate item is satisfied. Otherwise record the remaining items and keep the phase In Progress.

## 4. Owner's first local acceptance test after Phase 4 and Phase 5

Use a clean local environment and a real Linux agent host. Verify:

- Local control-plane startup into the dashboard with no account-creation or login prompt; verify restart and database recovery.
- Add Server, one-time token display, token expiry/reissue/revocation, and enrollment.
- Agent heartbeat and server last-seen/connection status.
- Live and historical CPU, memory, disk, network, process, and TCP/listening-port views.
- Dashboard time ranges, empty/stale data, disconnect/reconnect, and historical catch-up.
- Project scope and attribution labels, where configured.
- Alert transitions/notifications and authorized remote actions, if included in the completed Phase 4 release.
- Firewall behavior for loopback/LAN and control-plane/agent roles; PostgreSQL, Redis, and internal app ports must not be exposed publicly.
- Agent restart, API restart, database/cache recovery, and the documented backup/restore/rollback path.

Record actual outcomes and defects before wider release activity. A successful build alone is not local product acceptance.

## 5. Rules for the next session

- Read README.md, AGENTS.md, docs/PROJECT_STATE.md, and this document before editing.
- Continue on main; do not create a branch.
- Keep all repository content in English.
- Keep Phase 4 In Progress until owner acceptance is recorded. Linux Phase 5 tooling is now prepared; apply it only after a real target host, DNS/TLS paths, SSH administrator CIDRs and cloud-network policy are identified.
- Inspect and reuse the existing APIs and tests before adding new abstractions or dependencies.
- Do not mark individual features or Phase 4 complete without their acceptance evidence.
