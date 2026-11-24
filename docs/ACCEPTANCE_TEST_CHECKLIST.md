# Monitor Me — Owner Acceptance Checklist

This checklist verifies the current single-install, no-login local experience and the implemented first Linux journald → ClickHouse / Log Explorer slice. It does not claim production readiness, Windows/file log coverage or sustained-volume capacity.

## 1. Prepare the local stack

1. Start Docker Desktop / Docker Engine.
2. From the repository root, run:

   ```powershell
   .\scripts\start-local.ps1
   ```

   On Linux, use `./scripts/start-local.sh`.

3. The helper creates `.env` and a localhost-only development TLS certificate if absent, builds the Compose services and native agent artifacts, starts the services and probes the dashboard.
4. Confirm PostgreSQL, Redis, ClickHouse, API, agent-artifacts and web are healthy/completed as appropriate. Open `http://localhost:3000`.
5. Confirm the dashboard opens without signup, login or organization creation. Local no-login access must remain bound to loopback; do not expose this profile to LAN/public networks.

Do not delete existing database volumes or run `docker compose down -v` during ordinary upgrades or acceptance.

## 2. Add a machine without OS selection

1. In the dashboard choose **Add Machine**, provide a display label and create the one-time token.
2. Confirm the enrollment dialog has no Windows/Linux selector and presents both shell commands; run only the command matching the terminal available on the target host.
3. Confirm the token is never included in the command, URL or shell history and is requested through a hidden prompt on the target.
4. Confirm a newly created machine appears as pending until its native agent reports.

## 3. Enroll a native Linux or Windows agent

For a local Windows PC using the same control plane:

```powershell
& ([scriptblock]::Create((Invoke-RestMethod -Uri 'http://localhost:3000/install/agent-windows.ps1'))) -ServerUrl 'http://localhost:3000' -GrpcAddress 'localhost:8443'
```

Run PowerShell as Administrator. The installer should detect Windows x64, download the artifact and public CA, verify SHA-256, install/start the MonitorMeAgent Windows Service, prompt for the token securely, wait for the persisted session and remove the token from its config. Verify with `Get-Service MonitorMeAgent`; inspect `%ProgramData%\MonitorMe\agent.log` when troubleshooting.

For a Linux systemd host on the same machine as the local stack:

```sh
curl -fsSL 'http://localhost:3000/install/agent-linux.sh' | sudo bash -s -- --server-url 'http://localhost:3000' --grpc-address 'localhost:8443'
```

The installer should detect Linux amd64/arm64, verify the downloaded binary SHA-256, securely prompt for the one-time token, install and enable the systemd service, wait for session persistence and remove the token from its EnvironmentFile. Verify with `systemctl status monitor-me-agent` and `journalctl -u monitor-me-agent -f`.

For a remote host, use the control plane HTTPS URL and the reachable gRPC endpoint from the Add Machine dialog. A remote host must trust the public control-plane certificate chain. The monitored node needs outbound TLS gRPC only; do not open an inbound Monitor Me firewall port on the agent-only host.

The target machine must not require Docker, Go, Node.js or a repository checkout for these download-based installers. The current Windows artifact is x64; Linux artifacts are amd64/arm64. macOS and non-systemd Linux are not supported by these installers yet.

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
2. Open Operations in the local single-install workspace (no browser login is required).
3. Queue a restart and confirm the browser confirmation is shown.
4. Verify the operation moves through queued/dispatched to its reported result and an audit entry identifies the actor/action/time.
5. Verify the backend's allowlist rejects any service unit not explicitly approved; do not treat the no-login UI as a reason to widen this allowlist.
6. Verify an unavailable agent causes a clear failure/expiry and that arbitrary commands cannot be submitted.

Never use system-critical units such as ssh, networking, database, container runtime or the Monitor Me API as the test target. The UI is not a substitute for backend authorization.

## 7. Failure and recovery

- Restart the API and reload the browser; the implicit installation workspace should reinitialize without a login or browser session cookie.
- Stop/restart Redis and PostgreSQL independently using the recovery procedures in the repository's Compose CI/runbook; verify readiness fails/recoveries behave as documented.
- Restart the Linux agent and confirm its persisted session/WAL recover.
- Confirm alert/operation history survives API restart.

## 8. Initial Linux Log Explorer acceptance

1. Enroll a Linux systemd machine with `journalctl`; verify its capabilities include `logs.journald`.
2. Generate a synthetic journal message on that host and open **Logs** in the dashboard. Choose a time range containing the message.
3. Verify timestamp, source/systemd unit, severity, message body and event JSON details.
4. Apply a source, severity and text filter and confirm the results update.
5. When more than 200 events match, use **Load older events**. Confirm the returned page contains older events, does not duplicate IDs, and never expands one page beyond 200 visible events.
6. Turn on the polling live-tail mode, generate another synthetic event, verify it appears after a poll, then pause/resume the polling.
7. Stop ClickHouse on a disposable environment while journald continues to produce events; verify the agent queue grows only to its configured quota and resumes delivery after ClickHouse returns.
8. Record the actual event count, time range and query latency. This is functional acceptance, not evidence of 10,000 events/second or a 100-million-event search benchmark.

The current log reader supports Linux journald only. Windows Event Log, file-tail/rotation, full-text indexes, native push streaming, context-around-event, saved queries, ClickHouse backup/restore and sustained-load acceptance remain outstanding.

## 9. Local health checks and reset

From the repository root, verify the local services:

    curl -fsS http://localhost:8080/health/live
    curl -fsS http://localhost:8080/health/ready
    docker compose ps --all

For troubleshooting:

    docker compose logs --tail=150 postgres redis clickhouse migrate api web

Stop the stack and keep test data:

    docker compose down

To erase all local test data and recreate the owner/server records from scratch:

    docker compose down -v
    docker compose up --build -d

The second command sequence permanently deletes the local PostgreSQL and Redis volumes. Do not use it when you need to keep your test records.

Remote Operations stays read-only/disabled for command execution by default because REMOTE_ALLOWED_SYSTEMD_UNITS is empty in .env.example. To test it, use only a disposable Linux VM with a harmless test systemd service, set that exact unit in .env, restart the API container, and follow section 6. Do not enable service stop/restart for ssh, networking, Docker, PostgreSQL, Redis, or the Monitor Me API.

No command in this local checklist applies the production firewall helper or touches your computer's host firewall.

## 10. Record results

Record date, commit, host/OS, browser, agent version and result for each section. File a defect with the exact dashboard page, server ID (not its enrollment token), timestamp, expected behavior and observed behavior. Never include passwords, access tokens, private keys or enrollment tokens.
