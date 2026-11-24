# Production Linux Firewall Requirements

**Status:** Phase 5 Linux-MVP helper implemented; production validation pending. The repository includes a dry-run-first UFW/firewalld helper for additive control-plane rules and an agent-only no-inbound profile. It does not enforce the complete least-privilege policy or replace host/cloud review.

## 1. Deployment roles

Monitor Me has two different network roles. Do not apply the control-plane rules to every monitored host.

- **Control plane:** the VPS/VM that runs the web application, HTTP API, TLS gRPC agent endpoint, PostgreSQL, and Redis.
- **Monitored host:** a Linux VPS, VM, bare-metal server, or local machine running only the native Monitor Me agent. It initiates connections to the control plane and does not accept inbound Monitor Me connections.

A local development machine that runs both roles can use loopback. A LAN-hosted control plane can expose the agent endpoint only to the trusted LAN/VPN subnet.

## 2. Required network policy

The default policy is deny unsolicited inbound traffic and allow established/related return traffic. Keep outbound DNS, time synchronization, operating-system updates, and required application egress working.

| Host / service | Direction | Port | Policy |
| --- | --- | --- | --- |
| Control-plane HTTPS reverse proxy | Inbound | TCP 443 | Public TLS listener for the native installer, public CA and checksum-verified agent artifacts. Nginx restricts the dashboard, its normal API and live WebSocket to the administrator CIDR/VPN. |
| Agent TLS gRPC endpoint | Inbound to control plane | TCP 8443 by default | Allow agent connections; restrict to a VPN/private subnet when available. The port can be changed with deployment configuration |
| ACME HTTP-01 challenge | Inbound to control plane | TCP 80 | Optional; open only when the chosen certificate workflow requires it, then redirect HTTP to HTTPS |
| Administrator SSH | Inbound to control plane and hosts | TCP 22 by default | Allow only from the administrator's fixed IP/CIDR, VPN, or bastion; use the configured SSH port if different |
| Agent to control plane | Outbound from monitored host | TCP 8443 by default | Required. The agent establishes TLS gRPC; responses return over the established connection |
| DNS resolution | Outbound from hosts | UDP/TCP 53 | Allow to the configured resolver when egress is restricted |
| Time synchronization | Outbound from hosts | UDP 123 | Allow to the configured NTP service when egress is restricted |
| OS/package and certificate downloads | Outbound from hosts | TCP 443; TCP 80 only where required | Allow to approved repositories and certificate endpoints |
| PostgreSQL | Inbound | TCP 5432 | Never expose publicly; keep on loopback or a private container/network interface |
| Redis | Inbound | TCP 6379 | Never expose publicly; keep on loopback or a private container/network interface |
| Internal API | Inbound | TCP 8080 | Never expose publicly; reverse-proxy to it over loopback/private networking |
| Next.js application | Inbound | TCP 3000 | Never expose publicly; reverse-proxy to it over loopback/private networking |

The product's agent path uses TLS gRPC on TCP 8443 by default. If a deployment changes the gRPC listen port, firewall rules and the agent's `MONITOR_GRPC_ADDRESS` must use the same port. No unsolicited inbound port is required on an agent-only monitored host. The product does not require an inbound UDP port.

Do not blanket-drop all ICMP/ICMPv6. Preserve the ICMP/ICMPv6 messages required for diagnostics and Path MTU Discovery, and apply an equivalent policy to IPv4 and IPv6. Do not leave an unintended IPv6 path open when configuring only IPv4 rules.

## 3. Control-plane VPS/VM

Apply both layers when available:

1. The cloud provider's security group / network firewall.
2. The Linux host firewall.

Keep the rules consistent. For the no-login single-install profile, TCP 443 must reach Nginx so remote monitored hosts can download the public installer, CA and agent artifact. Nginx must source-restrict the dashboard, normal API and live WebSocket to the administrator CIDR/VPN while leaving only the dedicated installer/CA/artifact paths public. TCP 8443 is the TLS gRPC enrollment/data channel; restrict it to known agent source ranges when possible, or expose it publicly when agents roam. Expose TCP 80 only for HTTP-01 certificate validation. Permit SSH only from an administrator CIDR, VPN or bastion. Apply the same port policy to the cloud security group; the Nginx allowlist remains the application-layer admin boundary. Database, cache, API, and web application ports remain private.

### Conservative automation helper

Run deploy/production/firewall.sh first without --apply to review its dry-run plan. Control-plane mode requires one or more explicit --admin-cidr values; optional --zone must identify the zone attached to the public interface. The helper adds source-restricted SSH, public HTTPS (the reverse proxy enforces which paths are public), gRPC rules (source-restricted when --grpc-cidr is supplied, otherwise public for roaming agents), and optional HTTP-01 allow rules. It detects active UFW/firewalld, refuses to enable an inactive firewall, and records rules it added for --rollback. Agent-only mode deliberately makes no inbound changes.

This implementation is additive by design: it does not set default policies or remove existing rules. The no-login deploy gate also verifies that Nginx includes the generated admin-only allowlist in both dashboard and live-WebSocket locations, while installer, CA and artifact paths remain public. This check does not audit every other Nginx vhost, IPv6 path, Docker forwarding rule or cloud security group. Review those separately, keep a second SSH session/provider console open and perform external IPv4/IPv6 acceptance before production use.

### Ubuntu/Debian with UFW

Confirm the administrator CIDR and allow SSH before enabling UFW. Keep the current SSH session open and verify a second SSH session before closing it. Replace the example CIDR; do not copy it literally.

```sh
sudo ufw default deny incoming
sudo ufw default allow outgoing
sudo ufw allow from <ADMIN_CIDR> to any port 22 proto tcp
# TCP 443 must reach Nginx so remote agents can download installers and binaries.
# Nginx applies the ADMIN_CIDR allowlist to dashboard/API/WebSocket paths.
sudo ufw allow 443/tcp
sudo ufw allow 8443/tcp

# Optional: only when using ACME HTTP-01 validation.
sudo ufw allow 80/tcp

sudo ufw status numbered
sudo ufw enable
sudo ufw status verbose
```

If SSH uses a non-default port, allow that configured port instead of TCP 22. If the machine uses IPv6, verify UFW IPv6 support is enabled and that the same intended exposure policy applies to IPv6.

### RHEL/Rocky/Alma/Fedora with firewalld

First inspect the active zone and its existing services. The example below assumes the public zone is the Internet-facing zone and narrows SSH to the administrator CIDR.

```sh
sudo firewall-cmd --get-active-zones
sudo firewall-cmd --permanent --zone=public \
  --add-rich-rule='rule family="ipv4" source address="<ADMIN_CIDR>" service name="ssh" accept'
sudo firewall-cmd --permanent --zone=public --remove-service=ssh
# TCP 443 reaches Nginx for public installer/CA/artifact paths;
# Nginx restricts the dashboard/API/WebSocket to the administrator CIDR/VPN.
sudo firewall-cmd --permanent --zone=public --add-port=443/tcp
sudo firewall-cmd --permanent --zone=public --add-port=8443/tcp

# Optional: only when using ACME HTTP-01 validation.
sudo firewall-cmd --permanent --zone=public --add-service=http

sudo firewall-cmd --check-config
sudo firewall-cmd --reload
sudo firewall-cmd --zone=public --list-all
```

Add a corresponding IPv6 administrator rule when SSH administration uses IPv6. Adapt the zone to the interface actually connected to the Internet; do not assume every server uses the public zone.

### nftables

Use a host's existing nftables ruleset and its established management process. Do not paste a generic flush-and-replace ruleset onto a production host. On Docker hosts, firewall forwarding and container NAT are managed in coordination with Docker; test the effective ruleset and forwarding path before changing it.

## 4. Monitored Linux host (agent only)

The agent opens an outbound TLS gRPC connection to the configured control-plane address. It does not listen for inbound connections. Therefore:

- Do not open TCP 8443 inbound on monitored hosts.
- Do not create inbound rules for the agent.
- If outbound traffic is restricted, allow TCP 8443 only to the configured control-plane IP/FQDN (including its resolved addresses).
- Preserve the host's normal DNS, NTP, and package-update egress.
- Keep SSH limited to the administrator network; the agent does not need SSH access.

For a strict UFW egress policy, the Monitor Me-specific rule is:

```sh
sudo ufw allow out to <CONTROL_PLANE_IP> port 8443 proto tcp
```

This is an outbound rule, not an inbound port opening. Add only the DNS/NTP/repository egress rules required by the host's existing network policy.

## 5. Local machine and LAN deployments

- **Agent and control plane on the same machine:** use `127.0.0.1:8443` for the agent endpoint where supported. No public firewall opening is needed for that loopback connection.
- **Agent on another LAN machine:** permit TCP 8443 to the control-plane host from the specific LAN/VPN subnet only. Do not expose it to the public Internet just to monitor a local machine.
- **Local development:** the current Compose file binds PostgreSQL 5432, Redis 6379, API 8080, gRPC 8443, and web 3000 to `127.0.0.1`. That is intentional; same-host development does not require public inbound firewall rules.

## 6. Docker and firewall interaction

The current Compose configuration binds published service ports to loopback. Preserve that property for production behind a host reverse proxy or private network. Do not change database or Redis bindings to `0.0.0.0`.

Docker installs its own packet-filtering/NAT rules for bridge networking. Published container ports can bypass the normal UFW input path, so a UFW allow/deny rule alone is not a sufficient control if a container port is published on a public address. Do not disable Docker's firewall management. If direct external container publishing is unavoidable, add and test restrictions through the firewall backend supported by that Docker installation (for the iptables backend, Docker documents the `DOCKER-USER` chain; native nftables uses a different integration). Prefer loopback-bound containers plus a deliberately exposed reverse proxy.

## 7. Remaining firewall automation and verification requirements

The full production firewall workflow must eventually:

- Detect the active firewall manager and its active interface/zone before proposing changes.
- Support the documented UFW and firewalld profiles without running competing firewall managers.
- Make only the role-appropriate changes: control-plane ingress rules, or no inbound rule changes for an agent-only host.
- Show the exact ports, source ranges, and planned changes before applying them; require explicit approval.
- Preserve existing administrator access, IPv4/IPv6 policy, Docker networking, and unrelated rules.
- Be idempotent, record what it changed, verify the resulting listeners/rules, and provide a safe rollback path.
- Never flush a ruleset, disable the host firewall, expose PostgreSQL/Redis, or silently open SSH to the world.
- Report when a cloud-provider security group must also be updated; host-level automation cannot assume it can modify the provider firewall.

## 8. Verification checklist

- [ ] Confirm the active firewall manager and cloud security-group rules.
- [ ] Confirm public ingress contains only intended HTTPS, gRPC, optional ACME HTTP, and source-restricted SSH.
- [ ] Confirm PostgreSQL 5432, Redis 6379, API 8080, and web 3000 are not reachable from the public network.
- [ ] Confirm an agent-only host has no Monitor Me inbound listener/rule and can make outbound TLS gRPC to the control plane.
- [ ] Confirm local/loopback and intended LAN/VPN use cases work.
- [ ] Test both IPv4 and IPv6 from an external network.
- [ ] Reboot/reload and verify firewall rules persist without interrupting service.
- [ ] Verify Docker port publishing cannot unintentionally bypass the intended exposure policy.

## References

- [Docker packet filtering and firewalls](https://docs.docker.com/engine/network/packet-filtering-firewalls/)
- [Docker with iptables](https://docs.docker.com/engine/network/firewall-iptables/)
- [Ubuntu UFW firewall guide](https://ubuntu.com/server/docs/security-firewall/)
- [firewalld: Open a port or service](https://firewalld.org/documentation/howto/open-a-port-or-service.html)
