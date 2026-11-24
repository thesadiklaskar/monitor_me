# Monitor Me

A self-hosted infrastructure monitoring product for a VPS, VM, cloud VM, VPC-hosted Linux machine, dedicated server or a small fleet. Every installation runs independently on infrastructure controlled by its owner. Monitoring data stays with that installation.

Monitor Me is an original monitoring product with an included Grafana OSS companion UI. Grafana is provisioned against Monitor Me's PostgreSQL telemetry using a dedicated read-only database role. Prometheus, Zabbix and Netdata are not required.

> **Release status:** this is the development `main` branch, not a certified public release. On November 24, 2026, the local API, database, migration runner, agent-artifact builder and native web dashboard started successfully in one Windows Docker Desktop environment. The new Grafana/PostgreSQL/ClickHouse integration has not yet passed that local acceptance run. Clean-host production install, multiple Linux distributions, ARM hardware, backup/restore and load limits remain unverified. GitHub Actions is not used as the acceptance gate.

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
| Metric dashboards | Native saved dashboards plus a full Grafana OSS companion at `/grafana/`; provisioned PostgreSQL datasource for metrics and ClickHouse datasource for logs; Grafana provides SQL/query builder, Explore, variables, annotations, transformations, rich visualizations and alerting | Grafana is upstream Grafana OSS, not a Monitor Me reimplementation. Its datasources use dedicated read-only DB roles; the integrated provisioning and dashboards still require local acceptance after pull |
| Live metrics | Authenticated agent stream, bounded WebSocket updates and historical catch-up | Live freshness depends on agent sampling, storage and network; no zero-latency guarantee |
| Logs | Linux journald ingestion through bounded local spool and ClickHouse search; native UI filters and paging; Grafana ClickHouse datasource with Logs query builder, Explore and a provisioned Host Logs panel | The native UI's 10-second auto-refresh is HTTP polling, not push-based live tail. General file/syslog tail and rotation support are not delivered. Windows Event Log source code needs runtime acceptance |
| Alerts | Metric threshold rules, firing/recovered events and in-app notifications | External email/chat notification providers are not implemented |
| Deployment | Linux production Compose stack, health checks, migration runner, backup/restore for PostgreSQL and rollback of application images | ClickHouse backup/restore, automated upgrades and clean-host acceptance remain release work |

Grafana itself supplies the full upstream OSS dashboard/query/Explore experience; the native Monitor Me dashboard remains a smaller, purpose-built operator UI. The Grafana provisioning, permissions and query paths must pass the local checks below before production rollout.

## Install the control plane

### Intended one-command setup

After the final public repository transfer, the public install flow will be:

```bash
git clone https://github.com/thesadiklaskar/monitor_me.git
cd monitor_me
sudo ./install.sh
```

The development repository is currently `WorkRCS/monitor_me`; the public target is `thesadiklaskar/monitor_me`. Do not use the public clone URL until the final code and release have actually been transferred there.

The root `install.sh` is a guided Ubuntu/Debian Linux x86-64 bootstrap. It installs Docker/Compose, Nginx, Certbot and UFW when no host firewall is active (or uses an already-active firewalld); generates database/cache/Grafana secrets; requests a Let's Encrypt certificate; enables automatic certificate renewal (including Nginx reload and gRPC API restart); applies an administrator CIDR boundary; builds the stack; runs migrations; provisions dedicated read-only Grafana users in PostgreSQL and ClickHouse; and checks API/web/Grafana readiness. The development `main` branch stores `install.sh` as an executable file (`100755`), so the intended command is `sudo ./install.sh`. Preserve this executable mode when transferring the final source to `thesadiklaskar/monitor_me`.

The installer asks for:
1. A DNS hostname whose A record routes to the control plane or its public HTTPS load balancer.
2. The administrator public IP/CIDR (it suggests the SSH client IP when available).
3. An email address for certificate expiry notices.
4. Confirmation before enabling UFW if the host firewall is inactive.

Before starting, the VPS/provider firewall must allow TCP 80, 443 and 8443, plus SSH from the administrator network. The script cannot edit cloud security groups/VPC ACLs without the provider's IAM credentials. The Monitor Me management UI has no application login; do not expose it publicly. Nginx and the host firewall restrict it to `ADMIN_CIDR`. Grafana is served at `https://YOUR_HOST/grafana/`, requires its own admin login and is also restricted by the same Nginx CIDR allowlist. Its generated admin password is stored in the private `.env.production` file.

### Add monitored machines

After the dashboard opens, choose **Add machine** and copy the generated Linux or Windows command to the target host. The installer detects the supported OS/architecture, downloads the matching native agent and CA, verifies the artifact checksum, installs the service and enrols the machine using a short-lived token.

A monitored host needs outbound HTTPS for the installer/artifact and outbound TLS gRPC to the control plane (default TCP 8443). It does not need a public inbound Monitor Me port.

## Supported platform targets

| Role | Platform | Status |
|---|---|---|
| Control plane | Ubuntu/Debian Linux amd64 | Automated installer target; clean-host/release certification remains pending |
| Agent | systemd Linux amd64 | Native agent and installer target; clean-host runtime acceptance required |
| Agent | systemd Linux arm64/aarch64 | Native agent and installer target; real ARM hardware runtime acceptance required |
| Agent | Windows x64 | Service and metrics/Event Log code exists; Windows runtime acceptance required |
| Control plane | Native Windows Server | Unsupported |
| Agent/control plane | macOS, Windows ARM64, BSD | Not currently supported as released targets |

VPS, VM, cloud VM, VPC and bare metal are hosting types, not operating systems. An agent can run on systemd-based Linux distributions with amd64 or arm64 CPUs when procfs is mounted and readable. Journald collection additionally requires `journalctl` and readable systemd journal access. Alpine/OpenRC and non-systemd Linux are not accepted agent targets yet. The control-plane installer is narrower: Ubuntu/Debian amd64 only; ARM64 control-plane deployment is not accepted because the current stack pins an x86-64-only ClickHouse Docker server image. Provider security groups/VPC ACLs must be configured separately.

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

Grafana OSS 13.3.1 is included. Its ClickHouse datasource plugin is pinned to 4.23.5; Grafana installs that plugin synchronously before starting, so first startup needs outbound access to the Grafana plugin catalog.

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

The startup scripts validate source freshness/working tree, Compose configuration, build before replacing services and check API, web and Grafana readiness. Local Grafana opens from the sidebar at `http://localhost:3000/grafana/` (direct loopback port `3001` also redirects there). Local credentials are `admin` / `localgrafanaadminpasswordchange`; change them before exposing any non-local environment.

Run the read-only Linux preflight on a candidate host before installing:

```bash
sudo ./scripts/linux-preflight.sh --control-plane
# On each monitored Linux host:
sudo ./scripts/linux-preflight.sh --agent
```

The preflight does not install packages or change services; a pass is not a production acceptance certificate. Never run `docker compose down -v` as a routine troubleshooting step; it deletes persistent data.

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
          |                       |
          v                       v
Next.js dashboard             Grafana OSS companion
  Servers | Logs | Dashboards | Alerts
                              ├─ PostgreSQL reader: metrics
                              └─ ClickHouse reader: host logs
                              SQL / query builder / Explore / alerting
```

The backend is the source of truth for telemetry; Redis messages are notifications only. The agent removes a queued batch only after durable backend acknowledgement.

## Configuration, data and security

- Production settings and secrets live in `.env.production` (mode 0600); never commit it.
- Dashboard/API/WebSocket/Grafana access is network-restricted by Nginx to `ADMIN_CIDR`; Grafana additionally uses its own administrator login. PostgreSQL metrics use `grafana_reader` (SELECT-only), while ClickHouse logs use a separate `grafana_reader` user/role limited to `monitor_me.logs` with `readonly=1`; agents use their own short-lived credentials.
- PostgreSQL, Redis and ClickHouse are private to the Compose network. Do not publish ports 5432, 6379, 8080, 8123 or 9000.
- PostgreSQL backups are provided by `deploy/production/backup.sh` and `restore.sh`. ClickHouse backup/restore must be designed and tested before a production retention commitment.
- The current schema still contains legacy organization/user/session compatibility plumbing used internally for installation-local ownership and live tickets. It is not a customer account system and is not exposed in the UI; removing it fully requires a separate data-preserving schema refactor.
- `scripts/demo-load.sh` creates load only when explicitly confirmed. It does not write fabricated telemetry.

## Repository map

- `web/README.md` — frontend architecture, UI rules and npm commands.
- `backend/README.md` — Go API, storage boundaries, migrations and tests.
- `agent/README.md` — native agent capabilities, OS support and configuration.

Keep product documentation to these four Markdown entry points. Add source comments near the code for implementation details; do not create another layer of stale roadmaps, audit notes or historical phase documents.
