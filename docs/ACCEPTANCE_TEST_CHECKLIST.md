# Monitor Me — Owner Acceptance Checklist

This checklist is for the owner to test Monitor Me locally on their own computer. No VPS, public DNS, public certificate, or production firewall change is required. A green build alone is not product acceptance. Do not run remote stop/restart against critical services.

## 1. Prepare the local stack

1. Install the toolchain listed in docs/DEVELOPMENT.md: Go, Node/npm and Docker Compose v2.
2. Copy .env.example to .env and replace the local-only PostgreSQL and Redis passwords.
3. From the repository root, run:

       make certs
       docker compose up --build -d
       docker compose ps --all

4. Confirm PostgreSQL, Redis, API and web become healthy. Open http://localhost:3000.

## 2. Owner setup and server enrollment

1. Complete first-owner setup with a unique test email, strong password and organization name.
2. Sign out and sign in again; refresh the page and confirm the session remains active.
3. Add a test Linux server. Copy the one-time enrollment token once and record its expiry; do not paste it into a shared chat, shell history or committed file.
4. Verify the server appears in the organization inventory with its initial pending state.
5. Reissue a token and confirm the older unused token is rejected. Revoke an unused token and confirm it cannot enroll.

## 3. Enroll a real Linux agent

Use a disposable Linux host or Linux VM. Docker Desktop on Windows/macOS can run the control-plane containers, but the host collector is Linux-only. Follow docs/DEVELOPMENT.md and use the generated one-time server token plus the local development CA.

Confirm:
- Agent enrollment succeeds and the server receives an agent identity.
- The heartbeat changes the server state and last-seen timestamp.
- The agent continues reporting after a process restart using its persisted session.
- If the API or network is briefly unavailable, the local WAL retains batches and delivers them after connectivity returns.

Do not grant broad Docker socket access just to run the agent. Docker is not required on a monitored Linux host.

## 4. Dashboard telemetry

Open the test server detail and verify actual collected values, not placeholder values:
- CPU utilization and per-core data where exposed.
- Memory, swap and cache.
- Disk throughput/IOPS and network throughput.
- Uptime, process count, top process RSS, TCP state counts and listening ports.
- Metric units, sample timestamps, selected time range and gaps.
- Historical values remain available after a browser refresh.
- Live updates resume after a temporary WebSocket disconnect; the client obtains a fresh single-use ticket and catches up from history.
- Empty, stale and disconnected states are understandable and do not display invented zero values.

Check All Projects and the test project scope. Add/remove a server/project workload mapping and confirm the project view changes without confusing host totals with project-attributed, shared or unclassified resources.

## 5. Alerts and notifications

1. Create a test rule using a supported metric/operator/threshold and choose a test server.
2. Generate or wait for a real threshold transition on the disposable host.
3. Confirm one firing event and its in-app notification appear.
4. Move the metric back below the recovery threshold and confirm a recovery event is recorded without duplicate firing spam.
5. Mark the notification read and confirm the state persists after refresh.

External email/chat provider delivery is not part of this in-app notification acceptance pass.

## 6. Remote systemd operations — optional and only on a disposable host

Remote operations are disabled unless the API's REMOTE_ALLOWED_SYSTEMD_UNITS environment variable contains explicit unit names. Keep the default empty in the first pass.

Only after creating a disposable test service on the test VM:
1. Add that exact test unit to the allowlist and restart the local API.
2. Sign in as the organization owner/admin and open Operations.
3. Queue a restart and confirm the browser confirmation is shown.
4. Verify the operation moves through queued/dispatched to its reported result and an audit entry identifies the actor/action/time.
5. Try as a read-only role and confirm the action is unavailable/denied.
6. Verify an unavailable agent causes a clear failure/expiry and that arbitrary commands cannot be submitted.

Never use system-critical units such as ssh, networking, database, container runtime or the Monitor Me API as the test target. The UI is not a substitute for backend authorization.

## 7. Failure and recovery

- Restart the API and verify the web session reuses the secure HttpOnly cookie.
- Stop/restart Redis and PostgreSQL independently using the recovery procedures in the repository's Compose CI/runbook; verify readiness fails/recoveries behave as documented.
- Restart the Linux agent and confirm its persisted session/WAL recover.
- Confirm alert/operation history survives API restart.

## 8. Local health checks and reset

From the repository root, verify the local services:

    curl -fsS http://localhost:8080/health/live
    curl -fsS http://localhost:8080/health/ready
    docker compose ps --all

For troubleshooting:

    docker compose logs --tail=150 postgres redis migrate api web

Stop the stack and keep test data:

    docker compose down

To erase all local test data and recreate the owner/server records from scratch:

    docker compose down -v
    docker compose up --build -d

The second command sequence permanently deletes the local PostgreSQL and Redis volumes. Do not use it when you need to keep your test records.

Remote Operations stays read-only/disabled for command execution by default because REMOTE_ALLOWED_SYSTEMD_UNITS is empty in .env.example. To test it, use only a disposable Linux VM with a harmless test systemd service, set that exact unit in .env, restart the API container, and follow section 6. Do not enable service stop/restart for ssh, networking, Docker, PostgreSQL, Redis, or the Monitor Me API.

No command in this local checklist applies the production firewall helper or touches your computer's host firewall.

## 9. Record results

Record date, commit, host/OS, browser, agent version and result for each section. File a defect with the exact dashboard page, server ID (not its enrollment token), timestamp, expected behavior and observed behavior. Never include passwords, access tokens, private keys or enrollment tokens.
