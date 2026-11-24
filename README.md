# Monitor Me

**Self-hosted infrastructure monitoring for VPS, cloud VMs, VPC-connected machines and dedicated Linux servers.**

Monitor Me gives infrastructure operators one place to view server health, host metrics, system logs and threshold alerts. Run one control plane on infrastructure you own, enrol the machines you want to monitor, and inspect their telemetry from a browser.

<p align="center">
  <img src="https://ik.imagekit.io/xvqovhmcyr/monitor_me/Monitor%20Me%20-%20Google%20Chrome%2011_24_2026%206_40_07%20PM.png" alt="Monitor Me monitoring interface" width="92%" />
</p>

<p align="center">
  <img src="https://ik.imagekit.io/xvqovhmcyr/monitor_me/Docker%20Compose%20%E0%A6%B8%E0%A6%AE%E0%A6%B8%E0%A7%8D%E0%A6%AF%E0%A6%BE%20%282%29%20-%20Google%20Chrome%2011_24_2026%206_40_21%20PM.png" alt="Monitor Me deployment view" width="32%" />
  <img src="https://ik.imagekit.io/xvqovhmcyr/monitor_me/Docker%20Compose%20%E0%A6%B8%E0%A6%AE%E0%A6%B8%E0%A7%8D%E0%A6%AF%E0%A6%BE%20%282%29%20-%20Google%20Chrome%2011_24_2026%206_40_25%20PM.png" alt="Monitor Me service view" width="32%" />
  <img src="https://ik.imagekit.io/xvqovhmcyr/monitor_me/Docker%20Compose%20%E0%A6%B8%E0%A6%AE%E0%A6%B8%E0%A7%8D%E0%A6%AF%E0%A6%BE%20%282%29%20-%20Google%20Chrome%2011_24_2026%206_40_32%20PM.png" alt="Monitor Me container view" width="32%" />
</p>

## What Monitor Me does

A Monitor Me installation has two parts:

- **Control plane:** the web interface, API, database, log store and alert engine. It runs on one supported Linux server that you control.
- **Monitoring agent:** a small native service installed on each Linux machine. It reads operating-system counters and supported system logs, then securely sends telemetry to the control plane.

The agent observes the host. Monitoring is read-only: Monitor Me does not open remote shells, execute commands or start, stop or restart host processes and services.

## Features

| Area | Capabilities |
|---|---|
| **Monitoring** | Fleet overview, connected-first server cards, health state and heartbeat. Each server opens a detailed view with CPU, per-core CPU, memory, swap, mounted storage, disk I/O, network, process count and recent host logs. |
| **Servers** | Add machines, create one-time enrolment tokens, view host identity and agent connectivity, and inspect machine metrics and process/network inventory. |
| **Logs** | Search host system-journal events by time range, severity and source. Inspect timestamps, event source and message body. |
| **Dashboards** | Create and save installation-local dashboards with metric and log panels. Visualisations include time series, area, bar, stat, gauge, heatmap and table. |
| **Alerts** | Define metric-threshold rules, edit or pause them, and review firing and recovery events alongside the related metrics. |

## Host measurements

The Linux agent collects measurements from the enrolled machine:

- **CPU:** total utilisation, per-core utilisation, user/system time and I/O wait.
- **Memory:** physical memory, available memory, cache and swap.
- **Storage:** mounted filesystem capacity by mountpoint, device and filesystem, including total, used, available and utilisation.
- **Disk:** read/write throughput and read/write operations per second.
- **Network:** receive/transmit throughput, packets and errors per second, separated by interface.
- **Host state:** uptime, process count, selected process memory information, TCP connections and listening ports.
- **System logs:** systemd journal events when the agent has permission to read the host journal.

Monitoring charts and volume details are based on telemetry collected from each enrolled host. Storage cards show individual mounted volumes and their capacity history.

## Install the control plane

The installer is designed for a clean **Ubuntu or Debian Linux x86-64 (amd64)** server with root access, systemd and internet connectivity.

After the project is uploaded to the **thesadiklaskar/monitor_me** repository, install it with:

~~~bash
git clone https://github.com/thesadiklaskar/monitor_me.git
cd monitor_me
sudo ./install.sh
~~~

The guided installer prepares Docker Engine and Compose v2, Nginx, HTTPS certificates, firewall rules, private service credentials, application containers and database migrations. It then checks that the API and web interface are ready.

### Before installation

Prepare the following:

1. An Ubuntu/Debian amd64 VPS, VM or dedicated Linux server with root/sudo access.
2. A public IPv4 address or a correctly routed public endpoint for the control plane.
3. A DNS hostname with an A record pointing to that endpoint.
4. Inbound TCP 80, 443 and 8443 allowed in the provider firewall, plus SSH restricted to the administrator IP.
5. The administrator public IP/CIDR that should be allowed to open the web interface.
6. An email address for HTTPS certificate registration and renewal notices.

The installer asks for the DNS hostname, administrator CIDR, optional agent source CIDR and certificate email. It can suggest the IP address used by the current SSH session for the administrator CIDR.

### Required network access

| Port | Direction | Allowed source | Purpose |
|---|---|---|---|
| TCP 22 | Inbound | Administrator IP/CIDR only | SSH administration |
| TCP 80 | Inbound | Public internet | HTTPS certificate validation and HTTP-to-HTTPS redirect |
| TCP 443 | Inbound | Public internet | HTTPS interface and agent installation, CA and binary downloads |
| TCP 8443 | Inbound | Monitored host networks | Agent telemetry over TLS gRPC |

The operator interface and its live updates are restricted by the Nginx ADMIN_CIDR allowlist. Agent bootstrap, certificate and binary-download routes are available over HTTPS so new machines can enrol. The installer configures the host firewall when a supported active firewall manager is available. Provider security groups, VPC firewall policies and network ACLs must be configured in your cloud account.

PostgreSQL, Redis, ClickHouse and internal application ports remain on the private Compose network. Do not publish database ports 5432, 6379, 8123 or 9000, or the private API/web ports 18080 and 13000.

### VPS, VM, VPC and dedicated-server deployments

The hosting environment can vary; the control-plane operating system and network path must meet the requirements above.

| Environment | How to deploy |
|---|---|
| **VPS** | Use an Ubuntu/Debian amd64 VPS with a public IPv4 endpoint, DNS hostname and the required inbound ports. Run the installer over SSH. |
| **Cloud VM** | Create an Ubuntu/Debian amd64 instance, attach or route a public endpoint, configure provider firewall rules, then run the same installer. |
| **VPC-connected VM** | Place the control plane and monitored machines in routable subnets. Permit administrator HTTPS access to the control plane and agent TCP 8443 traffic from monitored subnets. |
| **Dedicated server / bare metal** | Install on Ubuntu/Debian amd64, point the DNS A record to the server and allow the same network ports at the perimeter firewall. |
| **Private-only VPC** | The current installer requires public HTTP certificate validation on TCP 80 and public HTTPS bootstrap routes. A VPC with no public ingress path requires a different certificate/bootstrap design and is not a supported one-command installation target. |

**AWS example:** create an x86-64 Ubuntu/Debian VM in your VPC, assign a public IPv4 address or a routed public endpoint, create the DNS A record, and configure its security group. Allow SSH only from the administrator CIDR, TCP 80/443 for public HTTPS setup and TCP 8443 from the monitored-machine networks.

**Google Cloud example:** create an x86-64 Ubuntu/Debian VM in the selected VPC, configure its external IPv4 endpoint and DNS A record, and add targeted VPC firewall ingress rules for SSH, TCP 80/443 and TCP 8443. Keep database and internal application ports private.

These are guest-OS installation steps. Monitor Me does not create cloud VMs, configure IAM, reserve addresses, edit provider security groups or manage VPC routing.

A load balancer must use TCP pass-through for ports 80, 443 and 8443. Do not terminate TLS before the Monitor Me Nginx or gRPC endpoint. Preserve the administrator source IP so the ADMIN_CIDR allowlist can enforce access.

### Check host prerequisites

The optional read-only preflight checks the control-plane or agent host without installing packages or changing firewall rules:

~~~bash
sudo bash ./scripts/linux-preflight.sh --control-plane
sudo bash ./scripts/linux-preflight.sh --agent
~~~

The control-plane installer supports Ubuntu/Debian amd64. The production Linux agent target is systemd-based Linux amd64 with readable procfs counters, curl and sha256sum.

## Enrol monitored machines

1. Open the HTTPS address printed by the installer from the administrator network.
2. Go to **Servers** and select **Add machine**.
3. Select the target operating system and copy the generated installation command.
4. Run that command with root privileges on the machine to monitor.
5. Open **Monitoring** and select the machine to view its health, metrics, storage volumes and recent logs.

The Linux agent is installed as a native systemd service. Docker is not required on monitored machines. The installer downloads the matching agent and CA over HTTPS, verifies the binary checksum, enrols with a one-time token and sends telemetry to TCP 8443. Monitored machines need outbound HTTPS and outbound TLS gRPC to the control plane; they do not need an inbound Monitor Me port.

Check an enrolled agent:

~~~bash
sudo systemctl status monitor-me-agent
sudo journalctl -u monitor-me-agent -n 100 --no-pager
sudo journalctl -u monitor-me-agent -f
~~~

## How the data flows

~~~mermaid
flowchart LR
  subgraph HOSTS["YOUR MONITORED MACHINES"]
    direction TB
    VPS["VPS / Linux VM"]
    VPC["Cloud VM / VPC host"]
    BARE["Dedicated server / bare metal"]
  end

  subgraph CONTROL["ONE MONITOR ME CONTROL PLANE"]
    direction TB
    EDGE["Nginx edge<br/>HTTPS · TLS · administrator IP allowlist"]
    subgraph APP["APPLICATION SERVICES"]
      direction LR
      WEB["Web interface<br/>Next.js · React · TypeScript"]
      API["Go API<br/>HTTP · TLS gRPC :8443"]
    end
    subgraph DATA["DATA SERVICES"]
      direction LR
      PG[("PostgreSQL<br/>metrics · machines · alerts")]
      REDIS[("Redis<br/>latest-value fan-out")]
      CH[("ClickHouse<br/>host log events")]
    end
    EDGE --> WEB
    WEB <-->|Private HTTP API| API
    API --> PG
    API --> REDIS
    API --> CH
  end

  OPERATOR["Administrator browser<br/>trusted IP or VPN"]
  OPERATOR -->|HTTPS :443| EDGE
  VPS -->|TLS gRPC :8443| API
  VPC -->|TLS gRPC :8443| API
  BARE -->|TLS gRPC :8443| API
~~~

The agent reads host counters and journal events, keeps unacknowledged telemetry in a bounded local write-ahead spool, and sends authenticated batches over TLS gRPC. The API validates each machine credential and stores accepted telemetry. PostgreSQL stores machine state and metric history, Redis provides ephemeral latest-value fan-out, and ClickHouse stores structured host logs. The web interface reads metric ranges and log pages through the private API.

[Open the editable system diagram](docs/monitor-me-architecture.excalidraw) in [Excalidraw](https://excalidraw.com/) to inspect or modify the host, network, application and data-service flow.

## Technology

| Layer | Technology | Role |
|---|---|---|
| Web interface | Next.js 16, React 19, TypeScript | Responsive monitoring UI, API proxy routes, charts and dashboards |
| Backend | Go 1.27, HTTP, gRPC and Protocol Buffers | Enrolment, telemetry ingestion, queries, alert rules and migrations |
| Host agent | Go 1.27 | Native system service for Linux metrics, journald and telemetry delivery |
| Metrics and application state | PostgreSQL 18 | Machine inventory, credentials, metric history, dashboards and alerts |
| Latest-value delivery | Redis 8 | Ephemeral cache and live fan-out |
| Host log storage | ClickHouse 26 | Structured event storage and time-range log queries |
| Service runtime | Docker Compose v2 | Isolated application services and persistent volumes |
| Network edge | Nginx and TLS 1.2/1.3 | HTTPS routing and administrator IP allowlisting |

Go is used for the API and agent so the same language and protocol definitions can be used for reliable telemetry collection and ingestion. TypeScript and React power the browser interface. PostgreSQL stores durable application and metric data, Redis carries live updates, and ClickHouse is used for structured host logs.

## Security and access

- The browser interface does not have an application login. Nginx restricts operator routes to the configured ADMIN_CIDR. Use a trusted static address or VPN and never open the operator interface to the whole internet.
- Agent telemetry uses TLS gRPC and machine-scoped credentials. First enrolment uses a short-lived, single-use token.
- Downloaded agent binaries are checked against their published SHA-256 checksums.
- Database and cache passwords are generated during installation and stored in the root-only .env.production file with mode 0600.
- PostgreSQL, Redis and ClickHouse are private Compose services and are not published on public interfaces.
- Host monitoring is read-only; remote command execution and process/service control are not part of the product.

## Logs, health and backups

View control-plane services and logs on the control-plane host:

~~~bash
sudo docker compose --env-file .env.production -f deploy/production/compose.yml ps
sudo docker compose --env-file .env.production -f deploy/production/compose.yml logs --tail=100 api web migrate postgres redis clickhouse
curl -fsS http://127.0.0.1:18080/health/ready
curl -fsS http://127.0.0.1:13000/api/config
~~~

Create a PostgreSQL backup:

~~~bash
sudo ./deploy/production/backup.sh
~~~

Restore a PostgreSQL backup after reviewing the target file:

~~~bash
sudo ./deploy/production/restore.sh --confirm /absolute/path/to/backup.dump
~~~

The restore script first creates a pre-restore PostgreSQL snapshot. The ClickHouse log store uses its own persistent volume and should be included in the host's data-volume backup and recovery plan.

## Current deployment scope

- **Control plane:** Ubuntu or Debian, Linux amd64, systemd, Docker Engine and Compose v2.
- **Linux agent:** systemd-based Linux amd64 with readable procfs counters.
- **Networking:** public DNS, public TCP 80 for certificate validation, HTTPS 443 for operator and agent bootstrap routes, and TLS gRPC 8443 for agent telemetry.
- **Operator access:** network allowlist through ADMIN_CIDR; there is no application-level sign-in.
- **Host logs:** systemd journal collection. Arbitrary /var/log file tailing and generic syslog-file ingestion are not included.
- **Load balancers:** TCP pass-through is required; TLS termination in front of the shipped Nginx and gRPC endpoints is not supported.
