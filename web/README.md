# Web dashboard

This folder contains the Monitor Me operator interface. It is a self-hosted Next.js application, not a hosted SaaS account portal.

## Product surface

The navigation is intentionally limited to five areas:

- **Dashboard** — fleet cards for every enrolled machine; select a machine to view its live health, real CPU/core, memory, storage, disk, network and process graphs together with recent host logs.
- **Servers** — machine inventory, enrollment and the existing per-machine detail view.
- **Logs** — time-bounded search, severity/source filters, event details and paged results.
- **Dashboards** — editable saved views with persisted default machine/time filters, panel editor, responsive grid, drag/drop ordering, metrics visualizations and host-log panels.
- **Alerts** — editable threshold rules, pause/resume, firing/recovered events and in-app notifications.

There is no login/signup, customer profile, organization, project, workspace or remote host-control UI. Do not add those concepts back. Processes, systemd units, containers, cgroups, disks and network interfaces are host metadata, not customer projects.

## Runtime and configuration

- Next.js 16 / React 19 / TypeScript.
- `MONITOR_ME_API_BASE_URL` is the server-side API origin used by Next.js route handlers. In Compose it points to the private API service.
- `MONITOR_ME_AGENT_GRPC_ADDRESS` is shown in the machine enrollment instructions.
- `MONITOR_ME_PUBLIC_API_URL` is used for the public installation endpoint and agent download base.
- `MONITOR_ME_AGENT_ARTIFACT_DIR` points to the read-only agent artifact bundle.
- `MONITOR_ME_AGENT_CA_FILE` points to the public CA certificate used by generated enrollment commands.

The browser should call same-origin `/api/...` handlers; those handlers proxy to the private Go API. Never expose PostgreSQL, Redis, ClickHouse or the private API port directly to the browser.

## Local development

Use Node.js 26.13.x and the repository's root Compose workflow for a full stack. From this directory:

```bash
npm install
npm run dev
```

Quality commands:

```bash
npm run lint
npm run typecheck
npm run build
```

A committed npm lockfile is a release requirement; until it exists, `npm install` can resolve different transitive versions on different days. Do not claim a reproducible frontend build until `package-lock.json` is committed and Docker/CI use `npm ci`.

## UI conventions

- Keep the left navigation collapsible on desktop and full-label on mobile.
- Keep page headings, breadcrumb/eyebrow, descriptions and actions aligned consistently.
- Use the dark slate / muted teal design tokens in `app/globals.css`; do not introduce one-off saturated colours.
- Use explicit units, timestamps and stale/empty/error states. Missing metrics must not be rendered as zero.
- Keep data bounded and paginated. Do not load unbounded log events into the browser.
- Every control must work. Do not add decorative controls, fake charts or sample customer data.
- Charts must label ranges and values clearly. Use host data returned by the API only.
- Saved metric panels support time-series, stat and table views; panel order is saved with the up/down controls. Never add project mapping or user-role concepts.

## Main source files

- `app/dashboard-client.tsx` — application shell, sidebar, machine inventory and enrollment.
- `app/overview-dashboard.tsx` — fleet health cards, per-host metric history and recent logs in one scrollable view.
- `app/server-detail.tsx` — host metrics, historical/live charts and process/socket inventory.
- `app/log-explorer.tsx` — searchable log explorer.
- `app/dashboards.tsx` — dashboard settings, saved filters, panel editor, saved dashboard defaults, metric/log renderers, alert markers and custom visualizations.
- `app/alerts.tsx` — threshold rules and notifications.
- `app/api/` — same-origin API proxies.
- `app/globals.css` — design tokens, layout, responsive and component styles.

Do not keep extra product documentation in this folder; keep this file as the frontend guide.
