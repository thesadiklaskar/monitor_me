# Monitor Me

A self-hosted infrastructure monitoring product for a VPS, VM, cloud VM, VPC-hosted Linux machine, dedicated server or a small fleet. Every installation runs independently on infrastructure controlled by its owner. Monitoring data stays with that installation.

Monitor Me is an original monitoring product. It does not embed Grafana, Prometheus, Zabbix or Netdata.

> **Release status:** this is the development `main` branch, not a certified public release. The latest frontend changes and `install.sh` have not yet passed a fresh green CI run, browser review or a clean Ubuntu/Debian VPS installation. Treat the platform matrix and feature limits below as the current truth; do not assume zero-lag, unlimited scale or full Grafana parity.

## Product principles

- One installation, one operator boundary and one local data store.
- No browser login/signup, customer profile, organization, workspace, project or project mapping.
- No remote shell, stop/restart or host-control operations. This is a monitoring tool.
- Four clear UI areas: **Servers**, **Logs**, **Dashboards** and **Alerts**.
- Real host telemetry only. Missing or unsupported metrics are shown as unavailable, never as fabricated zeroes.
- Native Go agent on monitored hosts; Docker is needed for the Linux control plane, not on monitored machines.
- Keep the product easy to install, understand, operate and modify.

## Current capabilities

| Area | Implemented path | Known limits |
|---|---|---|
| Machine inventory | Enrolment tokens, machine status, agent heartbeat and credential rotation | The owner must keep the dashboard protected by the configured network allowlist |
| Host metrics | CPU, memory, swap, disk, network, uptime, process count and selected process/socket metadata | Metric availability differs by OS; no eBPF or HTTP request tracing |
| Metric dashboards | Saved installation-wide dashboards, time-series/stat/table panels, historical averages and min/max ranges, persisted up/down ordering | No drag/resize grid, template variables, annotations, log panels or advanced query editor yet |
| Live metrics | Authenticated agent stream, bounded WebSocket updates and historical catch-up | Live freshness depends on agent sampling, storage and network; no zero-latency guarantee |
| Logs | Linux journald ingestion through bounded local spool and ClickHouse search; time/severity/source filters and paged results | The UI's 10-second auto-refresh is HTTP polling, not push-based live tail. General file/syslog tail and rotation support are not delivered. Windows Event Log source code needs runtime acceptance |
| Alerts | Metric threshold rules, firing/recovered events and in-app notifications | External email/chat notification providers are not implemented |
| Deployment | Linux production Compose stack, health checks, migration runner, backup/restore for PostgreSQL and rollback of application images | ClickHouse backup/restore, automated upgrades and clean-host acceptance remain release work |

Do not describe this as a complete Grafana replacement until the remaining query/panel/log features and release gates are tested.

## Install the control plane

### Intended one-command setup

After the final public repository transfer, the public install flow will be:

```bash
git clone https://github.com/thesadiklaskar/monitor_me.git
cd monitor_me
sudo ./install.sh
```

The development repository is currently `WorkRCS/monitor_me`; the public target is `thesadiklaskar/monitor_me`. Do not use the public clone URL until the final code and release have actually been transferred there.

The root `install.sh` is a guided Ubuntu/Debian Linux x86-64 bootstrap. It installs Docker/Compose, Nginx, Certbot and UFW when no host firewall is active (or uses an already-active firewalld); generates database/cache secrets; requests a Let's Encrypt certificate; enables automatic certificate renewal (including Nginx reload and gRPC API restart); applies an administrator CIDR boundary; builds the stack; runs migrations; and checks API/web readiness. The development `main` branch now stores `install.sh` as an executable file (`100755`), so the intended command is `sudo ./install.sh`. Preserve this executable mode when transferring the final source to `thesadiklaskar/monitor_me`.

The installer asks for:
1. A DNS hostname whose A record routes to the control plane or its public HTTPS load balancer.
2. The administrator public IP/CIDR (it suggests the SSH client IP when available).
3. An email address for certificate expiry notices.
4. Confirmation before enabling UFW if the host firewall is inactive.

Before starting, the VPS/provider firewall must allow TCP 80, 443 and 8443, plus SSH from the administrator network. The script cannot edit cloud security groups/VPC ACLs without the provider's IAM credentials. The management UI has no application login; do not expose it publicly. Nginx and the host firewall restrict it to `ADMIN_CIDR`.

### Add monitored machines

After the dashboard opens, choose **Add machine** and copy the generated Linux or Windows command to the target host. The installer detects the supported OS/architecture, downloads the matching native agent and CA, verifies the artifact checksum, installs the service and enrols the machine using a short-lived token.

A monitored host needs outbound HTTPS for the installer/artifact and outbound TLS gRPC to the control plane (default TCP 8443). It does not need a public inbound Monitor Me port.

## Supported platform targets

| Role | Platform | Status |
|---|---|---|
| Control plane | Ubuntu/Debian Linux x86-64 | Primary intended target; clean-host/release certification remains pending |
| Agent | Linux amd64 | Primary agent target; clean-host runtime acceptance required |
| Agent | Linux arm64/aarch64 | Build/installer target; real hardware runtime acceptance required |
| Agent | Windows x64 | Service and metrics/Event Log code exists; Windows runtime acceptance required |
| Control plane | Native Windows Server | Unsupported |
| Agent/control plane | macOS, Windows ARM64, BSD | Not currently supported as released targets |

VPS, VM, cloud VM, VPC and bare metal are hosting types, not operating systems. Compatibility depends on the guest OS, CPU architecture, service manager, storage, DNS, TLS and network rules.

## Real-data demo and README screenshots

Use a Linux test VM or non-production host that has already enrolled in Monitor Me. Do not run load tests on a critical production service.

Install the bounded workload generator:

```bash
sudo apt-get update
sudo apt-get install -y stress-ng
```

Generate real journald events:

```bash
logger -p user.info -t monitor-me-demo "Monitor Me demo: application started"
logger -p user.warning -t monitor-me-demo "Monitor Me demo: cache latency above normal"
```

Generate bounded real CPU, memory and disk activity for two minutes:

```bash
bash ./scripts/demo-load.sh --duration 120
```

The script asks for confirmation, runs `stress-ng` with bounded defaults and exits after the requested duration. Open **Servers** and inspect the machine, then open its metric charts. In **Logs**, search for `monitor-me-demo`. In **Dashboards**, create a dashboard and add CPU, memory, disk and network panels. In **Alerts**, create a temporary CPU threshold rule if you want to demonstrate a firing/recovery cycle; remove the rule after the demo.

Capture screenshots from the browser after actual data has arrived:
1. Servers inventory showing the enrolled machine and online status.
2. Machine detail showing real CPU, memory, storage and network charts.
3. A saved dashboard with several metric panels and clear time ranges.
4. Logs with the `monitor-me-demo` journald events and event details.
5. Alerts showing the test rule and its actual event state.

On Windows use `Win+Shift+S`; on Linux use the desktop screenshot shortcut/tool. Save reviewed PNGs in `screenshots/` with descriptive names and add them to this README only after confirming machine names, timestamps and values are real. Do not put API tokens, IP addresses, secrets or private log content in public screenshots. No fake/demo telemetry is bundled in the product.

## Local development

Toolchain versions are pinned by the project manifests:
- Go 1.27.4.
- Node.js 26.13.x.
- Docker Engine with Docker Compose v2.

Windows developers can use Docker Desktop with Linux containers for local development; that does not make Windows a certified production control-plane host.

PowerShell:

```powershell
git pull --ff-only origin main
.\scripts\start-local.ps1
```

Linux/macOS development shell (the control plane still targets Linux containers):

```bash
git pull --ff-only origin main
./scripts/start-local.sh
```

The startup scripts validate source freshness/working tree, Compose configuration, build before replacing services and check readiness. Never run `docker compose down -v` as a routine troubleshooting step; it deletes persistent data.

## Architecture

```text
Monitored Linux / Windows hosts
  native Go agent
  ├─ CPU, memory, disk, network, process and host metadata
  ├─ journald / supported Windows Event Log source
  ├─ bounded local WAL and retry
  └─ TLS gRPC
          |
          v
Self-hosted Linux control plane (Docker Compose)
  Go API + migration runner
  ├─ PostgreSQL: durable metrics and operator state
  ├─ Redis: ephemeral latest-value fan-out
  └─ ClickHouse: structured log events
          |
          v
Next.js dashboard
  Servers | Logs | Dashboards | Alerts
```

The backend is the source of truth for telemetry; Redis messages are notifications only. The agent removes a queued batch only after durable backend acknowledgement.

## Configuration, data and security

- Production settings and secrets live in `.env.production` (mode 0600); never commit it.
- Dashboard/API/WebSocket access is network-restricted by Nginx to `ADMIN_CIDR`; agents use their own short-lived credentials.
- PostgreSQL, Redis and ClickHouse are private to the Compose network. Do not publish ports 5432, 6379, 8080, 8123 or 9000.
- PostgreSQL backups are provided by `deploy/production/backup.sh` and `restore.sh`. ClickHouse backup/restore must be designed and tested before a production retention commitment.
- The current schema still contains legacy organization/user/session compatibility plumbing used internally for installation-local ownership and live tickets. It is not a customer account system and is not exposed in the UI; removing it fully requires a separate data-preserving schema refactor.
- `scripts/demo-load.sh` creates load only when explicitly confirmed. It does not write fabricated telemetry.

## Repository map

- `web/README.md` — frontend architecture, UI rules and npm commands.
- `backend/README.md` — Go API, storage boundaries, migrations and tests.
- `agent/README.md` — native agent capabilities, OS support and configuration.

Keep product documentation to these four Markdown entry points. Add source comments near the code for implementation details; do not create another layer of stale roadmaps, audit notes or historical phase documents.
