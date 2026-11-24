# ADR 0004: Single-install, no-login operator experience with network isolation

- **Status:** Accepted as the target product direction; production implementation and security verification pending
- **Date:** 2026-11-24

## Context

The product owner wants a Grafana-like, self-hosted install that opens directly into a useful monitoring console. Default operation must not ask for signup, login, organization creation or role administration. Each monitored node receives a short-lived, single-use enrollment token.

The repository already has a local-only anonymous Compose mode, but the production Compose topology still defaults to normal authentication. The backend anonymous middleware creates an internal owner principal and allows protected routes to execute as that principal. Enabling that mode on a public dashboard would expose inventory, alert, remote-operation and other owner-level APIs.

## Decision

1. Treat one deployment as one installation and one implicit workspace. Keep internal organization identifiers only as an implementation/migration boundary; do not expose organization creation or membership management in the default UI.
2. Remove interactive signup/login/profile/role workflows from the default single-install operator UX. A generated installation identity is created at first boot.
3. Keep local development no-login access bound to loopback.
4. A remote no-login installation must be protected by HTTPS and an Nginx layer-7 operator boundary (administrator CIDR, VPN or equivalent). TCP 443 remains reachable for dedicated public node bootstrap/CA/artifact routes; Nginx applies the allowlist to the dashboard, normal API and live WebSocket. Host firewall rules protect SSH and intended service ports, while remote agent downloads remain possible.
5. Keep public agent gRPC separate from operator HTTP access. Enrollment accepts only short-lived single-use tokens and exchanges them for persistent agent credentials. Do not use node enrollment tokens as browser access credentials.
6. Agent-only hosts open no inbound Monitor Me port. They establish outbound TLS gRPC to the control plane.
7. Existing broad HTTPS/SSH rules, cloud security groups, IPv6 and Docker forwarding are outside the helper's additive change set and require explicit verification. The helper does not claim to revoke those pre-existing exposures.

## Consequences

- The default local experience is simple and no-login.
- A public no-login dashboard is not an acceptable production configuration. The operator UI must be inaccessible to untrusted networks.
- The current production Compose profile, deployment script and API configuration still need to enforce this target. Do not enable anonymous owner-level API access on a public bind without the network restriction gate.
- Cloud security groups/network ACLs cannot be updated automatically unless the installer has provider credentials with least-privilege access. The installer must state this limitation rather than guess or silently skip a required layer.
- Authenticated multi-tenant deployment can remain a later optional profile; it is not the default single-install product workflow.

## Verification required

- [ ] Default local Compose access works without login and remains bound to loopback.
- [ ] Production no-login mode is blocked from starting until HTTPS operator CIDRs/VPN access policy is configured and applied.
- [ ] Firewall dry-run/apply/rollback tests prove public HTTPS reaches only the intended Nginx routes, dashboard/API/WebSocket paths enforce the admin allowlist, and SSH remains source-restricted.
- [ ] External IPv4/IPv6 tests confirm the dashboard is unavailable outside the admin network while agent enrollment is reachable on the intended gRPC network.
- [ ] Existing broad firewall and cloud security-group rules are detected or explicitly reported as operator review items.
- [ ] Enrollment tokens are single-use, expire, are not logged and cannot authenticate the dashboard.
