# Monitor Me — Platform and Operating-System Support Matrix

**Assessment date:** 2026-11-24  
**Status:** Source and installer-path audit only. No new OS runtime certification is claimed by this document.

## Read this first

VPS, VM, cloud VM, VPC and dedicated bare-metal are hosting environments, not operating systems. Compatibility depends on the guest/host OS, CPU architecture, service manager, container runtime and network policy.

The current product has two separate roles:

1. **Control plane:** the API, dashboard, PostgreSQL, Redis and ClickHouse services that run together as one self-hosted installation.
2. **Monitored agent:** a native process installed on each Linux or Windows machine that sends telemetry to that control plane.

A Linux control plane can monitor remote Linux and Windows hosts when the nodes can reach the published HTTPS/gRPC endpoints. It does not require all monitored hosts to run the same OS.

## Control-plane compatibility

| Host environment | Current assessment | Requirements / limitations |
|---|---|---|
| Ubuntu/Debian Linux VM or VPS, x86-64 | Primary intended production target | Docker Engine, Docker Compose v2, Nginx, Certbot, DNS/TLS, persistent storage, firewall and an explicit ADMIN_CIDR. A complete automated installer and clean-host certification are still open. |
| Other systemd Linux distributions, x86-64 | Intended, not distro-certified | Must support Docker Engine/Compose v2 and the documented Nginx/Certbot/firewall deployment. No exhaustive distribution/version matrix has passed. |
| Linux cloud VM or VPC, x86-64 | Intended, not provider-certified | Same Linux prerequisites. Cloud security groups, VPC ACLs and provider firewall policies require owner/provider configuration; the installer does not assume cloud IAM access. |
| Dedicated bare-metal Linux, x86-64 | Intended | Same Docker, systemd host, storage, TLS and network requirements as a Linux VPS. Hardware sizing and bare-metal acceptance are not yet benchmark-certified. |
| Linux arm64 VM/VPS/cloud/bare metal | Not certified for the control plane | The agent bundle includes a Linux arm64 build target; the full multi-container control-plane arm64 deployment has not been certified. |
| Windows workstation with Docker Desktop and Linux containers | Local development/evaluation path | This is a Linux-container development environment, not native Windows production hosting. The current owner Windows Docker Desktop build still needs a successful acceptance run. |
| Native Windows Server as control plane | Not supported/certified | Use a supported Linux VM/VPS for the control plane. Windows can be a monitored node. |
| macOS as control plane | Not supported | No production Compose target or installer is certified. |

## Monitored-agent compatibility

| Operating system / architecture | Source and installer evidence | Current support status |
|---|---|---|
| Linux x86-64 (amd64) | Linux artifact builder and downloaded shell installer; checks uname, verifies SHA-256, requires systemd, curl, sha256sum, root and installs a systemd service. | Intended primary agent target. Clean-host runtime acceptance is still required. |
| Linux ARM 64-bit (arm64/aarch64) | Artifact builder emits linux-arm64; Linux installer detects aarch64/arm64 and selects that artifact. | Build/installer target exists. Real hardware and service acceptance are pending. |
| Windows x86-64 (amd64) | Artifact builder emits windows-amd64; PowerShell installer requires Administrator, detects x64, verifies SHA-256 and installs a restart-enabled Windows Service. | Source and artifact target exist. Runtime installation, service recovery, upgrade and event-log ingestion acceptance remain pending. |
| Windows ARM64 | No artifact or installer target in the current bundle. | Unsupported. |
| macOS Intel / Apple silicon | Unsupported-platform log collector stub; no macOS artifact or launchd installer target. | Unsupported. |
| Other BSD/Unix systems | Unsupported-platform stub; no release artifact/service installer. | Unsupported. |

### Log-source support

- **Linux:** native journalctl reader with cursor-based collection, bounded local spool and retry/acknowledgement path. Requires a working systemd journal and permissions to read the intended journal entries.
- **Windows:** source code for bounded Application and System Event Log reads through PowerShell; Security log is intentionally excluded. The runtime and ingestion path still need Windows acceptance.
- **macOS:** no unified-log collector.
- **Arbitrary files / syslog:** no general file-tail, rotation-aware or remote syslog collector is currently delivered.

The Log Explorer now presents structured event labels and the supported Linux/Windows source intent. Its “Auto-refresh · 10s” control performs HTTP polling; it is not a native push-based live log stream. Push, replay and gap-free live-tail acceptance remain open.

## Virtualization, cloud and network requirements

A supported Linux guest can be hosted on a VPS, hypervisor VM, public cloud VM, private cloud/VPC or dedicated bare-metal machine. The platform layer does not make an unsupported guest OS supported. No claim is made that every VPS size, hypervisor, cloud provider or hardened image will work without configuration.

For a remote agent to enroll and send telemetry:

- It must be able to download its agent and public CA over HTTPS from the control plane.
- It must be able to reach the configured control-plane gRPC/TLS endpoint (default port 8443 unless changed).
- The operator must configure the provider security group/VPC ACL and host firewall to permit the chosen traffic.
- The no-login dashboard/API/WebSocket must remain restricted to an administrator/VPN CIDR. Do not expose the management UI to the public internet.

An agent-only monitored host does not need an inbound Monitor Me firewall rule; it initiates outbound connections. A control-plane host needs persistent storage for PostgreSQL and ClickHouse and must satisfy the production deployment's Nginx/TLS/firewall prerequisites.

## What would be required before claiming “supported”

1. Clean-install and upgrade tests on named Linux distro versions and x86-64/arm64 hardware.
2. Windows x64 service lifecycle and end-to-end Application/System Event Log tests.
3. Public-cloud/VPC and bare-metal tests covering DNS, TLS, security groups, host firewalls, gRPC enrollment and reconnect.
4. A tested control-plane sizing profile, resource limits, disk-growth and backup/restore evidence.
5. Published releases with checksums, provenance and Windows publisher signing.

Until those pass, describe Linux x86-64 as the primary intended deployment path, Linux arm64 and Windows x64 as agent build targets pending acceptance, and macOS/Windows control-plane hosting as unsupported.
