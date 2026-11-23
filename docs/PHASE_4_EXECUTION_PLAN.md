# Phase 4 — Monitoring Dashboard Execution Plan

**Prepared:** 2026-11-22  
**Planned work session:** 2026-11-23  
**Target:** Complete Phase 4 — Dashboard and Operational Features before starting Phase 5.

Phase 4 implementation started on 2026-11-23. The first authentication, inventory, and Add Server slice is now implemented on main and awaits CI verification. Continue directly on main; do not create a branch.

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

The initial frontend has moved beyond the foundation preview: web/app/page.tsx now enters the authenticated dashboard shell. It currently provides login/first-owner setup, server inventory, Add Server and one-time token management; full monitoring charts and operational workflows are still in progress. The backend provides the following verified API foundations:

| Capability | Existing endpoint | UI work required |
| --- | --- | --- |
| Initial owner bootstrap | POST /v1/auth/register | First-run setup guidance; do not present registration as an open public signup flow |
| Login and current session | POST /v1/auth/login, GET /v1/auth/me, POST /v1/auth/logout | Login form, safe session handling, logout, expired-session behavior |
| Server inventory | GET /v1/servers | Inventory page, loading/empty/error states, server status/last-seen display |
| Add Server | POST /v1/servers | Form, one-time enrollment-token handling, copyable Linux install/enrollment instructions |
| Reissue/revoke credentials | POST /v1/servers/{serverID}/enrollment-token, POST /v1/enrollment-tokens/{tokenID}/revoke | Authorized credential lifecycle controls and clear expiry/revocation feedback |
| Historical metric series | GET /v1/servers/{serverID}/metrics | Server detail charts, metric selection, time range, units, gaps and stale states |
| Live updates | POST /v1/live/ticket, GET /v1/live/ws | Ticket-based live connection, reconnect handling, and history catch-up |

The existing historical query is bounded: 100ms minimum aggregation step, 30-day raw range, and at most 5,000 output points. The live ticket is single-use and expires after 30 seconds. The UI must request a fresh ticket after reconnect; it must not store or reuse a consumed ticket.

Known backend gaps must be implemented and tested where required by the Phase 4 acceptance criteria. In particular, user-facing project-mapping administration, alert/notification APIs, and remote-action execution are not currently available as complete product workflows. The current gRPC stream accepts telemetry batches only; event, health, and command message variants are reserved. Do not simulate these features with hard-coded dashboard values.

## 3. Phase 4 implementation order

### Slice 1 — Authentication and application shell

**Implementation started 2026-11-23:** first-owner setup, login, logout, current-session lookup, and the authenticated application shell are in place. The bearer credential is held in a server-set HttpOnly cookie.

- Build login and initial owner setup guidance around the existing API.
- Implement session handling without putting bearer tokens in URLs, browser logs, or persistent unencrypted storage.
- Add authenticated route handling, logout, expired-session redirects, and loading/error states.
- Keep API errors understandable without exposing secrets or internal details.

### Slice 2 — Server inventory and Add Server

**Implementation started 2026-11-23:** organization-scoped server inventory, empty/loading/error states, server creation, one-time token display/copy/expiry, token rotation and role-aware revoke controls are in place. Agent deployment remains guided/manual; a fully automated installer is not yet claimed.

- Build organization-scoped inventory with empty, loading, error, connected, stale, and disconnected states based only on available backend fields.
- Implement Add Server using the existing create-server endpoint.
- Display the returned enrollment token only once; support explicit copy and expiry information.
- Generate an installation/enrollment command from the actual configured control-plane address and CA instructions. Never embed a reusable secret in a committed file or log.
- Add safe reissue/revoke flows with role-aware UI feedback.

### Slice 3 — Server detail and monitoring dashboard

- Build an overall host view for supported Linux CPU, memory, swap, disk, network, uptime, process, and TCP/listening-port information.
- Use real metric API responses only. Show units, timestamps, selected time range, empty periods, gaps, and stale/last-seen indicators.
- Add bounded historical charts and live updates using one-use WebSocket tickets.
- Reconnect with a new ticket and query historical data to catch up after a disconnect.
- Preserve the distinction between host totals, project-attributed, shared, and unclassified resources.

### Slice 4 — Project and workload views

- Add the All Projects/project selector and consistent scope changes across the page.
- Implement the required project/mapping administration path before presenting project views as configurable.
- Clearly identify cgroup/Systemd/container-derived attribution and its limitations. Do not claim full Docker Engine discovery.

### Slice 5 — Operational features

- Add supported process/port/connection/workload views.
- Implement alert rules, deduplication, recovery transitions, and notification delivery with backend persistence and tests.
- Implement remote restart/stop only through explicit allowlists, authorization, audit records, timeouts, and safe failure behavior. No arbitrary remote shell.
- Complete the guided one-command Linux installation workflow and user/role/audit workflows required by the release scope.

### Slice 6 — Phase 4 verification and documentation

- Add component/API/integration tests for normal, empty, stale, unauthorized, expired, reconnect, and failure states.
- Verify dashboard values and attribution against backend fixtures and actual telemetry ingestion.
- Run Go tests/race tests/vet/build, frontend lint/typecheck/production build, and the complete Compose integration pipeline.
- Update README, the feature acceptance matrix, roadmap, and project state from the test evidence.
- Mark Phase 4 Completed only when every Phase 4 completion-gate item is satisfied. Otherwise record the remaining items and keep the phase In Progress.

## 4. Owner's first local acceptance test after Phase 4 and Phase 5

Use a clean local environment and a real Linux agent host. Verify:

- Local control-plane startup, first-owner setup, login, logout, and session expiry behavior.
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
- Work on Phase 4 only. Do not begin firewall implementation or production deployment during the Phase 4 session.
- Inspect and reuse the existing APIs and tests before adding new abstractions or dependencies.
- Do not mark individual features or Phase 4 complete without their acceptance evidence.
