"use client";
import { useCallback, useEffect, useMemo, useState, type FormEvent } from "react";
import { apiRequest } from "@/lib/client-api";

type Server = { id: string; name: string; status: string };
type OperationType = "restart_service" | "stop_service";
type OperationConfig = { allowed_systemd_units: string[]; operations: OperationType[] };
type RemoteOperation = {
  id: string; server_id: string; server_name: string; operation: string; target: string;
  status: string; result?: string; created_at: string; expires_at: string;
  dispatched_at?: string | null; completed_at?: string | null;
};
type AuditEntry = { id: string; operation_id: string; action: string; actor: string; details: string; created_at: string };
const operationLabels: Record<OperationType, string> = {
  restart_service: "Restart service",
  stop_service: "Stop service",
};
function formatTime(value?: string | null): string {
  if (!value) return "Not recorded";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "Unknown time" : date.toLocaleString();
}
function statusClass(status: string): string {
  switch (status) {
    case "succeeded": return "operation-state operation-state-success";
    case "failed":
    case "expired": return "operation-state operation-state-failed";
    case "dispatched": return "operation-state operation-state-active";
    default: return "operation-state";
  }
}
function operationLabel(value: string): string {
  if (value === "restart_service") return operationLabels.restart_service;
  if (value === "stop_service") return operationLabels.stop_service;
  return value.replaceAll("_", " ");
}

export default function RemoteOperationsWorkspace({ servers, role, onBack }: {
  servers: Server[]; role: string; onBack: () => void;
}) {
  const [config, setConfig] = useState<OperationConfig>({ allowed_systemd_units: [], operations: [] });
  const [operations, setOperations] = useState<RemoteOperation[]>([]);
  const [audit, setAudit] = useState<AuditEntry[]>([]);
  const [serverId, setServerId] = useState("");
  const [operation, setOperation] = useState<OperationType>("restart_service");
  const [target, setTarget] = useState("");
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");

  const canOperate = role === "owner" || role === "admin";
  const selectedServerId = servers.some((item) => item.id === serverId) ? serverId : servers[0]?.id ?? "";
  const selectedServer = useMemo(() => servers.find((item) => item.id === selectedServerId), [selectedServerId, servers]);
  const allowedOperations = useMemo(
    () => config.operations.filter((item): item is OperationType => item === "restart_service" || item === "stop_service"),
    [config.operations],
  );
  const allowedTargets = config.allowed_systemd_units;
  const selectedOperation = allowedOperations.includes(operation) ? operation : allowedOperations[0] ?? "restart_service";
  const selectedTarget = allowedTargets.includes(target) ? target : allowedTargets[0] ?? "";

  const load = useCallback(async () => {
    try {
      const [configResponse, operationResponse] = await Promise.all([
        apiRequest<OperationConfig>("/api/remote-operations/config"),
        apiRequest<{ operations: RemoteOperation[]; audit: AuditEntry[] }>("/api/remote-operations"),
      ]);
      setConfig({
        allowed_systemd_units: Array.isArray(configResponse.allowed_systemd_units) ? configResponse.allowed_systemd_units : [],
        operations: Array.isArray(configResponse.operations) ? configResponse.operations : [],
      });
      setOperations(Array.isArray(operationResponse.operations) ? operationResponse.operations : []);
      setAudit(Array.isArray(operationResponse.audit) ? operationResponse.audit : []);
      setError("");
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : "Remote operation data could not be loaded.");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    const timer = window.setTimeout(() => { void load(); }, 0);
    const refresh = window.setInterval(() => { void load(); }, 15000);
    return () => { window.clearTimeout(timer); window.clearInterval(refresh); };
  }, [load]);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!canOperate || !selectedServer || !allowedOperations.includes(selectedOperation) || !allowedTargets.includes(selectedTarget)) {
      setError("Choose a server and a target from the configured allowlist.");
      return;
    }
    const confirmed = window.confirm(
      operationLabel(selectedOperation) + " " + selectedTarget + " on " + selectedServer.name +
      "? This action is sent to the enrolled agent and recorded in the audit log.",
    );
    if (!confirmed) return;

    setBusy(true);
    setError("");
    setNotice("");
    try {
      await apiRequest<{ operation: RemoteOperation }>(
        "/api/servers/" + encodeURIComponent(selectedServer.id) + "/remote-operations",
        { method: "POST", body: JSON.stringify({ operation: selectedOperation, target: selectedTarget }) },
      );
      setNotice("The operation was queued. The agent must be online and will report the result.");
      await load();
    } catch (submitError) {
      setError(submitError instanceof Error ? submitError.message : "The remote operation could not be queued.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="operations-workspace">
      <header className="operations-workspace-header">
        <div>
          <button className="detail-back-button" type="button" onClick={onBack}>← Back to servers</button>
          <div className="eyebrow">Controlled systemd actions</div>
          <h1>Remote operations</h1>
          <p>Only explicitly allowlisted systemd services can be restarted or stopped. Arbitrary shell commands are not supported.</p>
        </div>
        <button className="button button-secondary" type="button" onClick={() => { void load(); }} disabled={loading}>Refresh</button>
      </header>
      {error && <div className="notice notice-error" role="alert">{error}</div>}
      {notice && <div className="notice notice-success" role="status">{notice}</div>}

      {canOperate ? (
        <form className="operations-form-card" onSubmit={submit}>
          <h2>Queue an operation</h2>
          <div className="operations-form-grid">
            <label className="field">
              <span>Server</span>
              <select value={selectedServerId} onChange={(event) => setServerId(event.target.value)} required>
                {servers.length === 0 && <option value="">No servers available</option>}
                {servers.map((item) => <option key={item.id} value={item.id}>{item.name} · {item.status}</option>)}
              </select>
            </label>
            <label className="field">
              <span>Operation</span>
              <select value={selectedOperation} onChange={(event) => setOperation(event.target.value as OperationType)} disabled={allowedOperations.length === 0}>
                {allowedOperations.map((item) => <option key={item} value={item}>{operationLabels[item]}</option>)}
              </select>
            </label>
            <label className="field">
              <span>Allowlisted systemd unit</span>
              <select value={selectedTarget} onChange={(event) => setTarget(event.target.value)} disabled={allowedTargets.length === 0}>
                {allowedTargets.map((item) => <option key={item} value={item}>{item}</option>)}
              </select>
            </label>
          </div>
          {allowedTargets.length === 0 ? (
            <p className="operations-help">No services are allowlisted. Configure REMOTE_ALLOWED_SYSTEMD_UNITS on the API service before enabling remote operations.</p>
          ) : (
            <p className="operations-help">The server must be online with a recent authenticated heartbeat. Commands expire after 60 seconds if the agent does not receive them.</p>
          )}
          <div className="operations-form-footer">
            <span>Available only to organization owners and administrators.</span>
            <button className="button button-primary" type="submit" disabled={busy || !selectedServer || allowedTargets.length === 0 || allowedOperations.length === 0}>
              {busy ? "Queueing…" : "Queue operation"}
            </button>
          </div>
        </form>
      ) : <div className="notice notice-warning">Your role has read-only access to remote operation history.</div>}

      <section className="operations-list-card">
        <div className="panel-heading"><div><h2>Recent operations</h2><p>Latest 100 organization-scoped commands</p></div></div>
        {loading ? <div className="project-empty">Loading operation history…</div> : operations.length === 0 ? (
          <div className="project-empty">No remote operations have been queued.</div>
        ) : operations.map((item) => (
          <article className="operation-row" key={item.id}>
            <div className="operation-row-main">
              <strong>{operationLabel(item.operation)}</strong>
              <span>{item.server_name} · {item.target}</span>
              <small>Created {formatTime(item.created_at)} · Expires {formatTime(item.expires_at)}</small>
              {item.result && <p>{item.result}</p>}
            </div>
            <span className={statusClass(item.status)}>{item.status}</span>
          </article>
        ))}
      </section>

      <section className="operations-list-card">
        <div className="panel-heading"><div><h2>Audit history</h2><p>Queue, dispatch, completion and expiry records</p></div></div>
        {loading ? <div className="project-empty">Loading audit history…</div> : audit.length === 0 ? (
          <div className="project-empty">No remote operation audit records.</div>
        ) : audit.map((item) => (
          <article className="operation-row" key={item.id}>
            <div className="operation-row-main">
              <strong>{item.action.replaceAll("_", " ")}</strong>
              <span>{item.actor} · operation {item.operation_id}</span>
              <small>{formatTime(item.created_at)}</small>
              <p>{item.details}</p>
            </div>
          </article>
        ))}
      </section>
    </section>
  );
}
