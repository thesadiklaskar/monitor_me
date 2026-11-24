# Monitor Me — Release Readiness Audit

**Assessment date:** 2026-11-24  
**Purpose:** Honest product-readiness estimate for an international, self-hosted observability product.  
**Status:** Not production-ready; engineering implementation is in progress.

## Executive estimate

- **Production release readiness: approximately 35%.**
- **Feature/engineering implementation progress: approximately 50%.**
- **Recruiter-facing technical substance: promising, but not yet a reliable live demo.**

These are engineering estimates, not test measurements or a certification. Production readiness is weighted heavily toward repeatable builds, clean-install tests, platform verification, safe upgrades and real-world operations. Implemented source code does not receive full credit until the relevant test or host acceptance has passed.

A successful compile alone will not make this release-ready. The application must also start, migrate, survive restarts, collect real host data, handle outages, restore backups, and work on the OS/architecture combinations it claims to support.

## Scorecard

| Area | Estimate | Evidence and remaining blockers |
|---|---:|---|
| Core metrics and agent pipeline | 55% | Native Go agent, host metrics, durable telemetry spool, TLS gRPC and backend persistence paths exist. End-to-end clean-host and sustained-load acceptance remain open. |
| Logs and log exploration | 30% | Linux journald → bounded spool → TLS gRPC → ClickHouse ingestion and bounded search exist. Windows Application/System Event Log reader source and Windows artifact target exist, but runtime acceptance is pending; file/syslog tail and push/replay are missing. The UI now shows structured labels and an explicit 10-second auto-refresh (HTTP polling, not live push). |
| Dashboard and UX | 38% | Source now uses dark slate with a muted teal accent, readable full-word Servers/Logs/Dashboards/Alerts navigation, larger menu/table typography, whole-host charts and structured log labels. Project Mapping and remote-operation browser routes are removed. The frontend has not passed a fresh build or browser visual acceptance. A full Grafana-equivalent panel/query editor, variables, annotations and true push-based log tail remain outstanding. |
| Single-install architecture | 35% | Browser login/signup and customer project/workspace UI/API proxies have been removed. SQL tables, foreign keys, internal Go routes and query paths still use legacy users, organizations, memberships, sessions, organization_id and project_id. Remote-operation browser proxies are also removed; internal Go handlers remain. Safe data-preserving migrations/retirement are not implemented. |
| Installation and operations | 30% | Local Compose and Linux production Compose/deploy, firewall, TLS, backup/restore and rollback scripts exist. Production still needs manual host prerequisites, DNS, Nginx/Certbot and endpoint configuration; the clean one-command control-plane bootstrap is not complete. |
| Cross-platform support | 30% | The artifact builder targets Linux amd64, Linux arm64 and Windows amd64. Linux installer requires systemd; Windows x64 installer requires an elevated PowerShell session. Windows service, metrics and Application/System Event Log code has not passed real runtime acceptance. macOS and Windows control-plane hosting are unsupported. Full matrix: [PLATFORM_SUPPORT_MATRIX.md](PLATFORM_SUPPORT_MATRIX.md). |
| Build reproducibility and CI | 20% | The previous unused import, missing UI component, Windows service constant and `slugPattern` compile source issue have fixes in main; a regression test was added for `slugPattern`. No fresh complete build has run after the latest UI refactor. Backend go.sum and web package-lock.json are missing. Recent Actions jobs had runner_id 0 and zero steps, so they ran no tests. |
| Repository portability | 70% | Go imports use the owner-independent internal module namespace monitor-me.internal; source and installer docs have been audited to remove current owner/repository URLs and sample deployment hosts. Dependency lockfiles, release publishing and signed artifacts still need completion. |

## What is implemented in source

- Go API and migration runner, PostgreSQL persistence, Redis live notifications/cache and ClickHouse log storage.
- Native agent with host metrics, durable local spool, TLS gRPC transport, enrollment/session credential handling and Linux/Windows platform-specific code.
- Linux journald collection with durable cursors and bounded log ingestion/search.
- Initial dashboards, server inventory, alerting/remote-operation foundations and an initial WebSocket live channel.
- Local Windows Docker Desktop startup path and Linux production Compose/deployment scripts.
- No-login installation shell, network allowlist model and operator-configurable deployment endpoints.

The above is a source review, not a statement that all these paths passed a clean installation or production test.

## Platform and operating-system support

See the detailed [Platform and Operating-System Support Matrix](PLATFORM_SUPPORT_MATRIX.md) for the distinction between control-plane hosting and monitored-agent targets, including VPS/VM/cloud/bare-metal prerequisites and current certification gaps.

| Environment | Intended role | Current support assessment |
|---|---|---|
| Ubuntu/Debian and other systemd Linux distributions, amd64 | Control plane and monitored agent | Primary intended target. Linux agent installer requires systemd. Clean-host deployment and distro matrix are not yet certified. |
| Linux arm64 | Monitored agent | Artifact target exists in the build; hardware/runtime acceptance is pending. Control-plane arm64 support is not yet certified. |
| Windows 10/11/Server, amd64 | Monitored agent | Windows Service, metrics and Application/System Event Log source exists. Real Windows install, service restart, upgrade and log-ingestion acceptance are still required. |
| Windows 10/11 development workstation with Docker Desktop using Linux containers | Local control-plane development/test | Intended local development path. This is not the same as a native Windows production control plane. Current build acceptance is pending. |
| Windows Server as the control-plane host | Production control plane | Not a certified target. Use a supported Linux host/VM for the control plane; Windows can be a monitored node. |
| macOS | Agent/control plane | Not supported as a released target. Native log collector currently returns an explicit unsupported-platform response. |
| Dedicated bare-metal Linux server | Control plane or monitored node | Intended, subject to the same Linux prerequisites, network/storage sizing and host acceptance as a VPS. |
| Linux VM, VPS or cloud VM | Control plane or monitored node | Intended. Cloud security groups/VPC ACLs still require owner/provider configuration; the project does not automatically gain cloud IAM privileges. |

Do not advertise “all operating systems” or “any VPS size.” Only the combinations that pass a recorded acceptance matrix should be called supported.

## Sizing and performance

No minimum production hardware profile or maximum supported node/event rate has been demonstrated by the current benchmark evidence. The stack contains PostgreSQL, Redis, ClickHouse, API and web containers, so a tiny 1-vCPU/1-GB VPS must not be promised to work.

For an initial evaluation, a sensible **provisional test profile** is 4 vCPU, 8 GB RAM and SSD storage with at least 100 GB free, then measure actual growth and CPU/RAM under the customer's node count, metric interval, log events/sec and retention. This is a planning starting point, not a verified minimum or capacity guarantee. A small lab may work with less, but that must be measured.

The release must publish benchmark evidence for node count, events/sec, bytes/sec, event size, retention, query mix, p50/p95/p99 latency, CPU/RSS, disk growth, back-pressure, outage recovery and restore time. Proposed 10,000 events/sec and 100-million-event search targets are not current guarantees.

## Realistic deployment experience and timing

The following are planning estimates only. They are not current service-level commitments.

### Local Windows evaluation

1. Install/launch Docker Desktop with Linux containers and clone the repository: typically 5–15 minutes if prerequisites are already present.
2. Configure the local environment file and run the PowerShell startup script: first image build may take 5–20 minutes depending on download speed and cache.
3. Wait for database migrations, API and dashboard readiness: typically another 1–3 minutes.
4. Enroll a Windows or Linux test node: typically 2–5 minutes once the dashboard is healthy.

**Current caveat:** This path has not passed a clean Windows Docker Desktop acceptance. The latest owner build reached Go compilation and failed on an undefined `slugPattern`; a source fix and regression test have since been committed, but the owner has not yet run the latest main revision. Do not present the local stack as ready until it builds and passes readiness.

### Linux VPS/VM/cloud control plane

1. Provision a supported systemd Linux VM/VPS, DNS and persistent SSD storage: 5–15 minutes if already provisioned; cloud provisioning varies.
2. Install Docker Engine/Compose, Nginx, Certbot and firewall prerequisites; configure the real hostname, gRPC address, strong secrets and ADMIN_CIDR: approximately 15–40 minutes for a prepared operator.
3. Obtain/install the HTTPS certificate, validate Nginx and run the production deployment script: allow 10–30 minutes for image builds, database startup, migration and health checks, depending on network and hardware.
4. Enroll a node and confirm real metrics/logs: approximately 2–5 minutes after the control plane is healthy.

The current production deployment is not a true zero-input one-click installer: DNS, certificate, Nginx, host packages, firewall review and cloud networking still require operator action. A later installer should automate supported host-local work but must ask for/validate the real domain, trusted admin/VPN CIDR and secrets. It must not silently open a no-login dashboard to the internet.

## Dependency and build reliability gaps

- backend/go.mod exists, but backend/go.sum is absent. A clean build therefore needs the Go checksum service to authenticate missing module hashes.
- web/package.json exists, but web/package-lock.json is absent. npm install can resolve a changed transitive dependency tree on a later build.
- The backend image no longer runs go mod tidy during an image build. It resolves only the shipped API/migration build graph with -mod=mod, creates /out explicitly and keeps Go checksum validation enabled. Go downloads use HTTP/1.1 to avoid the exact HTTP/2 stream failure reported in the local build. This mitigation is not a guarantee against future proxy/network outages.
- CI no longer runs go mod tidy as a broad prerequisite; Go tests/builds resolve the packages actually used. Go module checksums must still be authenticated; do not set GOSUMDB=off.
- The required next dependency-hardening step is to generate/commit backend/go.sum and web/package-lock.json, then use npm ci in CI and Docker builds.

## Release-blocking acceptance checklist

- [ ] Pull the exact current main and confirm the source revision matches origin/main.
- [ ] Build API, migration runner, web and all Linux/Windows agent artifacts from a clean cache and a warm cache.
- [ ] Pass Go unit tests, race tests where supported, vet, formatting and frontend lint/typecheck/production build.
- [ ] Start the complete Windows Docker Desktop local stack; verify PostgreSQL, Redis and ClickHouse health, migrations, API readiness and dashboard readiness.
- [ ] Install agents on clean Ubuntu/Debian amd64 and arm64 hosts; verify service enable/restart/upgrade/uninstall and real metric freshness.
- [ ] Install and exercise the Windows amd64 service; verify CPU/memory/disk/network and Application/System log collection end-to-end.
- [ ] Verify offline buffering, reconnect, duplicate suppression, bounded disk use, disk-full handling and clock skew.
- [ ] Verify live log push/replay, filters, query cancellation, retention, backups and ClickHouse restore.
- [ ] Migrate a populated legacy database to the strict single-install schema without data loss; verify ambiguous data fails closed.
- [ ] Test production deployment, allowed/denied admin networks, public agent bootstrap routes, TLS renewal, WebSocket and gRPC from an external network.
- [ ] Publish tested sizing profiles, a reproducible install guide, a demo environment, screenshots, architecture diagram and load-test report.
- [ ] Generate and commit backend Go checksums and frontend npm lockfile; require green CI with actual runner steps.

## International/recruiter presentation bar

Before calling this a finished portfolio-grade product, provide:
- A one-page architecture diagram showing agent → durable spool → TLS gRPC → API → PostgreSQL/Redis/ClickHouse → dashboard.
- A short recorded demo on real Linux and Windows nodes, with the dashboard showing actual machine identity, timestamped metrics and logs.
- A successful clean-clone installation from a repository transferred to a different owner/name.
- Green CI with actual executed steps, release version/checksums, dependency scanning and a reproducible build.
- Published OS/architecture matrix, minimum sizing evidence, limitations, backup/restore instructions and a real benchmark report.

Until those gates pass, describe Monitor Me as an in-progress self-hosted observability platform, not a production-ready Grafana replacement.
