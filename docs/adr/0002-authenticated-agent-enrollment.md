# ADR 0002: User Authentication and Secure Agent Enrollment

- Status: Accepted
- Date: 2026-11-22
- Phase: 2 — Platform Foundation and Secure Connectivity

## Context

The control plane must provide an initial owner bootstrap, authenticated operator APIs, organization-scoped server inventory, and a secure way to bind a native agent to a server without storing reusable credentials in plaintext.

## Decisions

### User authentication

- The first owner account is created through a one-time bootstrap endpoint. A PostgreSQL transaction-scoped advisory lock serializes simultaneous bootstrap attempts.
- Passwords are hashed with Argon2id using a 64 MiB memory setting, three iterations, and two lanes.
- Login returns a cryptographically random 256-bit opaque bearer token with a 24-hour lifetime.
- Only the SHA-256 digest of the bearer token is persisted in PostgreSQL.
- Logout revokes the active session.
- Session lookup binds the user to an organization membership and role on every authenticated request.

### Authorization

- Roles are `owner`, `admin`, `operator`, and `viewer`.
- Owners and administrators can revoke enrollment tokens and agents.
- Owners, administrators, and operators can create servers and issue/reissue enrollment tokens.
- Authenticated organization members can read their organization's server inventory.
- Resource queries are always scoped by the organization ID from the authenticated session, never from an untrusted request parameter.

### Agent enrollment

- Creating a server issues a random 256-bit enrollment secret valid for 15 minutes.
- Only the SHA-256 digest is persisted; the raw token is returned once.
- Enrollment uses TLS gRPC and consumes the token in the same database transaction that binds the agent identity and creates its session.
- The token is single-use. Expired, revoked, unknown, and consumed tokens are rejected.
- The agent receives a random 256-bit session secret valid for 30 days. Only its SHA-256 digest is persisted.
- Re-enrollment for the same server preserves the agent ID and revokes previous agent sessions before creating the replacement session.
- Owners and administrators can revoke agents; all active sessions are revoked in the same transaction.
- The heartbeat endpoint validates the session digest, updates server/agent liveness, and stores a bounded capability list.

### Transport

- The gRPC server requires TLS 1.2 or newer.
- Message send/receive sizes are bounded to 1 MiB and concurrent streams are bounded.
- Production deployments must mount an organization-managed certificate and private key through the secret manager.
- Self-signed certificates are only for isolated local development and CI.
- User HTTP APIs must be exposed over HTTPS by the production ingress or reverse proxy.

### Telemetry stream boundary

The bidirectional stream is defined in the versioned protocol, but returns `UNIMPLEMENTED` until Phase 3 provides durable, idempotent telemetry ingestion. The server must never acknowledge telemetry that it has not durably stored.

## Verification

Phase 2 integration tests cover:
- First-owner registration and rejection of repeat bootstrap.
- Invalid password rejection and successful login.
- Authenticated profile and organization inventory access.
- Viewer read-only authorization.
- Server creation and 15-minute enrollment credential.
- Invalid, expired, consumed, and revoked enrollment credentials.
- TLS-protected gRPC enrollment and heartbeat.
- Stable agent identity across re-enrollment and invalidation of the previous session.
- Agent revocation, session rejection, and user logout.

## Consequences

The control plane can securely register its first operator, provision server identities, enroll agents, and track agent liveness. The monitoring telemetry stream remains intentionally unavailable until the durable Phase 3 ingestion path is implemented and tested.
