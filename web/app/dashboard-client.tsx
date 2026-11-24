"use client";

import { FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import { Bell, Copy, FileText, LayoutDashboard, PanelLeftClose, PanelLeftOpen, Plus, RefreshCw, Server as ServerIcon } from "lucide-react";
import { apiRequest } from "@/lib/client-api";
import { ServerDetail } from "./server-detail";
import AlertsView from "./alerts";
import DashboardsView from "./dashboards";
import LogExplorer from "./log-explorer";

type Server = {
  id: string;
  name: string;
  hostname?: string;
  status: string;
  agent_id?: string | null;
  agent_last_seen_at?: string | null;
  created_at: string;
};

type Enrollment = {
  token_id: string;
  token: string;
  expires_at: string;
  protocol: string;
};

type IssuedServer = {
  server: Server;
  enrollment: Enrollment;
};

type InventoryResponse = { servers: Server[] };
type ConfigResponse = { agent_grpc_address: string | null; public_api_url: string | null };

type ViewState = "checking" | "active" | "unavailable";

function displayError(error: unknown): string {
  return error instanceof Error ? error.message : "An unexpected error occurred.";
}

function formatDate(value?: string | null): string {
  if (!value) return "Never";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "Unknown";
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(date);
}

function timeSince(value?: string | null): string {
  if (!value) return "No heartbeat received";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "Last-seen time unavailable";
  const seconds = Math.max(0, Math.floor((Date.now() - date.getTime()) / 1000));
  if (seconds < 60) return `Seen ${seconds}s ago`;
  if (seconds < 3600) return `Seen ${Math.floor(seconds / 60)}m ago`;
  if (seconds < 86400) return `Seen ${Math.floor(seconds / 3600)}h ago`;
  return `Seen ${Math.floor(seconds / 86400)}d ago`;
}

function statusLabel(status: string): string {
  switch (status.toLowerCase()) {
    case "online": return "Connected";
    case "pending": return "Awaiting agent";
    case "disabled": return "Disabled";
    case "stale": return "Delayed heartbeat";
    case "offline": return "Disconnected";
    default: return status.replaceAll("_", " ");
  }
}

function statusClass(status: string): string {
  switch (status.toLowerCase()) {
    case "online": return "border-emerald-400/25 bg-emerald-400/10 text-emerald-300";
    case "pending": return "border-amber-400/25 bg-amber-400/10 text-amber-200";
    case "stale": return "border-amber-400/25 bg-amber-400/10 text-amber-200";
    case "disabled":
    case "offline": return "border-slate-500/30 bg-slate-500/10 text-slate-300";
    default: return "border-[var(--border)] bg-[var(--surface-raised)] text-[var(--muted)]";
  }
}







function EnrollmentDialog({
  issued,
  grpcAddress,
  onClose,
  onRevoked,
  canRevoke,
}: {
  issued: IssuedServer;
  grpcAddress: string | null;
  onClose: () => void;
  onRevoked: () => void;
  canRevoke: boolean;
}) {
  const [copied, setCopied] = useState(false);
  const [commandCopied, setCommandCopied] = useState<"windows" | "linux" | null>(null);
  const [revoking, setRevoking] = useState(false);
  const [error, setError] = useState("");
  const expires = formatDate(issued.enrollment.expires_at);

  async function copyToken() {
    try {
      await navigator.clipboard.writeText(issued.enrollment.token);
      setCopied(true);
    } catch {
      setError("Clipboard access was blocked. Select and copy the token manually.");
    }
  }

  async function copySetupCommand(platform: "windows" | "linux") {
    const baseUrl = window.location.origin;
    const endpoint = grpcAddress || "localhost:8443";
    const quotePowerShell = (value: string) => "'" + value.replace(/'/g, "''") + "'";
    const quoteShell = (value: string) => "'" + value.replace(/'/g, "'\\''") + "'";
    const command = platform === "windows"
      ? "& ([scriptblock]::Create((Invoke-RestMethod -Uri " + quotePowerShell(baseUrl + "/install/agent-windows.ps1") + "))) -ServerUrl " + quotePowerShell(baseUrl) + " -GrpcAddress " + quotePowerShell(endpoint)
      : "curl -fsSL " + quoteShell(baseUrl + "/install/agent-linux.sh") + " | sudo bash -s -- --server-url " + quoteShell(baseUrl) + " --grpc-address " + quoteShell(endpoint);
    try {
      await navigator.clipboard.writeText(command);
      setCommandCopied(platform);
    } catch {
      setError("Clipboard access was blocked. Select and copy the setup command manually.");
    }
  }

  async function revokeToken() {
    setRevoking(true);
    setError("");
    try {
      await apiRequest<null>(`/api/enrollment-tokens/${issued.enrollment.token_id}/revoke`, { method: "POST", body: "{}" });
      onRevoked();
    } catch (revokeError) {
      setError(displayError(revokeError));
    } finally {
      setRevoking(false);
    }
  }

  return (
    <div className="modal-backdrop" role="presentation">
      <section className="modal-card enrollment-card" role="dialog" aria-modal="true" aria-labelledby="enrollment-title">
        <div className="modal-heading">
          <div>
            <div className="eyebrow">One-time credential</div>
            <h2 id="enrollment-title">Enroll {issued.server.name}</h2>
          </div>
          <button className="icon-button" type="button" onClick={onClose} aria-label="Close enrollment details">×</button>
        </div>

        <div className="notice notice-warning">
          This token is shown only in this response. Copy it now. It expires on {expires} and can be used once.
        </div>

        <label className="field token-field">
          <span>Enrollment token</span>
          <textarea readOnly value={issued.enrollment.token} rows={3} onFocus={(event) => event.currentTarget.select()} />
        </label>
        <button className="button button-secondary button-wide" type="button" onClick={copyToken}>
          <Copy size={15} /> {copied ? "Copied to clipboard" : "Copy one-time token"}
        </button>

        <div className="install-instructions">
          <div className="install-heading-row">
            <div>
              <h3>Install a native agent</h3>
              <p className="install-platform-note">No OS setting is required. Run the command in the terminal native to the target machine; the agent identifies the host platform.</p>
            </div>
            <span className="install-platform-badge">AUTO DISCOVERY</span>
          </div>
          <p>Agent endpoint: <code>{grpcAddress || "localhost:8443"}</code></p>
          <div className="install-platform-block">
            <strong>Windows · PowerShell (Administrator)</strong>
            <div className="install-command-wrap">
              <div className="install-command-row">
                <code className="install-command">PowerShell bootstrap · detects Windows x64, verifies SHA-256, installs and starts the Windows Service</code>
                <button className="button button-table" type="button" onClick={() => void copySetupCommand("windows")}>{commandCopied === "windows" ? "Copied" : "Copy one command"}</button>
              </div>
            </div>
            <p>Run in an elevated PowerShell session on the Windows target. It detects the host, downloads the matching native agent, verifies SHA-256, installs a restart-enabled Windows Service and securely prompts for the one-time token.</p>
          </div>
          <div className="install-platform-block">
            <strong>Linux · Shell</strong>
            <div className="install-command-wrap">
              <div className="install-command-row">
                <code className="install-command">Shell bootstrap · detects Linux architecture, verifies SHA-256, installs and starts systemd</code>
                <button className="button button-table" type="button" onClick={() => void copySetupCommand("linux")}>{commandCopied === "linux" ? "Copied" : "Copy one command"}</button>
              </div>
            </div>
            <p>Run in a root shell on a systemd Linux target. It detects the host architecture, downloads the matching native agent, verifies SHA-256, installs and enables the systemd service, then removes the one-time token after session persistence.</p>
          </div>
          <p className="install-note">Use the command for the shell available on the target machine; the installer itself detects that machine's OS and architecture. It downloads binaries and the public CA over the control-plane endpoint. Remote hosts must reach the advertised gRPC address; <code>localhost</code> only works when the agent and control plane share a host. The downloaded binaries are checksum-verified, but are not yet code-signed; OS publisher-signature verification remains a release gate.</p>
        </div>

        {error && <div className="notice notice-error" role="alert">{error}</div>}
        <div className="modal-actions">
          {canRevoke && (
            <button className="button button-danger" type="button" onClick={revokeToken} disabled={revoking}>
              {revoking ? "Revoking…" : "Revoke this token"}
            </button>
          )}
          <button className="button button-primary" type="button" onClick={onClose}>Done</button>
        </div>
      </section>
    </div>
  );
}

function AddServerDialog({
  onClose,
  onCreated,
}: {
  onClose: () => void;
  onCreated: (issued: IssuedServer) => void;
}) {
  const [name, setName] = useState("");
  const [hostname, setHostname] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setBusy(true);
    setError("");
    try {
      const issued = await apiRequest<IssuedServer>("/api/servers", {
        method: "POST",
        body: JSON.stringify({ name, hostname }),
      });
      onCreated(issued);
    } catch (submitError) {
      setError(displayError(submitError));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="modal-backdrop" role="presentation">
      <section className="modal-card" role="dialog" aria-modal="true" aria-labelledby="add-server-title">
        <div className="modal-heading">
          <div>
            <div className="eyebrow">Machine enrollment</div>
            <h2 id="add-server-title">Add a machine</h2>
          </div>
          <button className="icon-button" type="button" onClick={onClose} aria-label="Close">×</button>
        </div>
        <p className="modal-copy">Create a one-time enrollment token. No operating system needs to be selected here; host discovery happens on the target machine.</p>
        <form onSubmit={submit} className="form-stack">
          <label className="field">
            <span>Machine name</span>
            <input autoFocus required minLength={2} maxLength={120} value={name} onChange={(event) => setName(event.target.value)} placeholder="production-api-01" />
            <small>Choose a recognizable name. The agent reports the actual hostname and operating system.</small>
          </label>
          <label className="field">
            <span>Expected hostname (optional)</span>
            <input maxLength={253} value={hostname} onChange={(event) => setHostname(event.target.value)} placeholder="Leave empty to discover automatically" />
          </label>
          {error && <div className="notice notice-error" role="alert">{error}</div>}
          <div className="modal-actions">
            <button className="button button-secondary" type="button" onClick={onClose}>Cancel</button>
            <button className="button button-primary" type="submit" disabled={busy}>{busy ? "Creating…" : "Create enrollment token"}</button>
          </div>
        </form>
      </section>
    </div>
  );
}

function ServerRow({
  server,
  canRotate,
  onRotate,
  onOpen,
}: {
  server: Server;
  canRotate: boolean;
  onRotate: (server: Server) => void;
  onOpen: (server: Server) => void;
}) {
  return (
    <tr>
      <td>
        <div className="server-name-cell">
          <span className={`status-dot status-dot-${server.status.toLowerCase()}`} />
          <div>
            <button className="server-open-link" type="button" onClick={() => onOpen(server)}>{server.name}</button>
            <span>{server.hostname || "Hostname not provided"}</span>
          </div>
        </div>
      </td>
      <td><span className={`status-pill ${statusClass(server.status)}`}>{statusLabel(server.status)}</span></td>
      <td>
        <div className="last-seen-cell">
          <strong>{timeSince(server.agent_last_seen_at)}</strong>
          <span>{formatDate(server.agent_last_seen_at)}</span>
        </div>
      </td>
      <td>{formatDate(server.created_at)}</td>
      <td className="actions-cell">
        {canRotate && (
          <button className="button button-table" type="button" onClick={() => onRotate(server)}>
            Issue new token
          </button>
        )}
      </td>
    </tr>
  );
}

export default function DashboardClient() {
  const [view, setView] = useState<ViewState>("checking");
  const [servers, setServers] = useState<Server[]>([]);
  const [apiStatus, setApiStatus] = useState<"checking" | "online" | "offline">("checking");
  const [inventoryBusy, setInventoryBusy] = useState(false);
  const [inventoryError, setInventoryError] = useState("");
  const [notice, setNotice] = useState("");
  const [showAdd, setShowAdd] = useState(false);
  const [issued, setIssued] = useState<IssuedServer | null>(null);
  const [grpcAddress, setGrpcAddress] = useState<string | null>(null);
  const [publicAPIURL, setPublicAPIURL] = useState<string | null>(null);
  const [selectedServer, setSelectedServer] = useState<Server | null>(null);
  const [showDashboards, setShowDashboards] = useState(false);
  const [showLogs, setShowLogs] = useState(false);
  const [showAlerts, setShowAlerts] = useState(false);
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);
  const loadServers = useCallback(async () => {
    setInventoryBusy(true);
    setInventoryError("");
    try {
      const result = await apiRequest<InventoryResponse>("/api/servers");
      const nextServers = Array.isArray(result.servers) ? result.servers : [];
      setServers(nextServers);
      setSelectedServer((current) => current
        ? nextServers.find((server) => server.id === current.id) ?? null
        : null);
      setApiStatus("online");
    } catch (loadError) {
      const message = displayError(loadError);
      setInventoryError(message);
      setApiStatus("offline");
    } finally {
      setInventoryBusy(false);
    }
  }, []);

  useEffect(() => {
    if (view !== "active") return;
    const refresh = window.setInterval(() => { void loadServers(); }, 15000);
    return () => window.clearInterval(refresh);
  }, [loadServers, view]);

  useEffect(() => {
    let active = true;

    async function initializeInstallation() {
      try {
        const config = await apiRequest<ConfigResponse>("/api/config");
        if (!active) return;
        setGrpcAddress(config.agent_grpc_address);
        setPublicAPIURL(config.public_api_url);

        await apiRequest<{ installation: { name: string; mode: string } }>("/api/installation");
        if (!active) return;

        setView("active");
        await loadServers();
      } catch {
        if (!active) return;
        setView("unavailable");
      }
    }

    void initializeInstallation();
    return () => { active = false; };
  }, [loadServers]);

  const onlineCount = useMemo(() => servers.filter((server) => server.status.toLowerCase() === "online").length, [servers]);
  const pendingCount = useMemo(() => servers.filter((server) => server.status.toLowerCase() === "pending").length, [servers]);
  const fleetHealthStatus = onlineCount > 0 ? "online" : pendingCount > 0 ? "pending" : servers.length > 0 ? "offline" : "disabled";
  const canCreate = true;

  async function rotateToken(server: Server) {
    setNotice("");
    try {
      const response = await apiRequest<IssuedServer>(
        `/api/servers/${server.id}/enrollment-token`,
        { method: "POST", body: "{}" },
      );
      setIssued(response);
    } catch (rotateError) {
      setNotice(displayError(rotateError));
    }
  }

  if (view === "checking") {
    return <main className="loading-screen"><span className="loading-wordmark">MONITOR ME</span><span className="loading-indicator" aria-hidden="true" /><p>Connecting to monitoring services…</p></main>;
  }

  if (view === "unavailable") {
    return (
      <main className="loading-screen">
        <span className="loading-wordmark">MONITOR ME</span>
        <h1>Monitoring service unavailable</h1>
        <p>Monitor Me could not connect to its local services. Check that the API, database, and Docker services are healthy.</p>
        <button className="button button-primary" type="button" onClick={() => window.location.reload()}>Retry connection</button>
      </main>
    );
  }

  return (
    <div className="app-shell">
      <aside className={sidebarCollapsed ? "sidebar sidebar-collapsed" : "sidebar"}>
        <div className="sidebar-brand">
          <span className="brand-name">MONITOR ME</span>
          <button className="sidebar-toggle" type="button" onClick={() => setSidebarCollapsed((value) => !value)} aria-label={sidebarCollapsed ? "Expand sidebar" : "Collapse sidebar"} aria-pressed={sidebarCollapsed} title={sidebarCollapsed ? "Expand menu" : "Collapse menu"}>
            {sidebarCollapsed ? <PanelLeftOpen size={18} /> : <PanelLeftClose size={18} />}
          </button>
        </div>
        <nav className="side-nav" aria-label="Main navigation">
          <span className="nav-section-label">Monitoring</span>
          <button title="Servers" aria-label="Servers" className={!showDashboards && !showLogs && !showAlerts ? "nav-item nav-item-active" : "nav-item"} type="button" onClick={() => {  setShowDashboards(false); setShowLogs(false); setShowAlerts(false);  setSelectedServer(null); }}><span className="nav-item-icon"><ServerIcon size={18} strokeWidth={1.8} /></span><span className="nav-item-label">Servers</span></button>
          <button title="Logs" aria-label="Logs" className={showLogs ? "nav-item nav-item-active" : "nav-item"} type="button" onClick={() => {  setShowDashboards(false); setShowAlerts(false);  setSelectedServer(null); setShowLogs(true); }}><span className="nav-item-icon"><FileText size={18} strokeWidth={1.8} /></span><span className="nav-item-label">Logs</span></button>
          <button title="Dashboards" aria-label="Dashboards" className={showDashboards ? "nav-item nav-item-active" : "nav-item"} type="button" onClick={() => {  setShowLogs(false); setShowAlerts(false);  setSelectedServer(null); setShowDashboards(true); }}><span className="nav-item-icon"><LayoutDashboard size={18} strokeWidth={1.8} /></span><span className="nav-item-label">Dashboards</span></button>
                    <button title="Alerts" aria-label="Alerts" className={showAlerts ? "nav-item nav-item-active" : "nav-item"} type="button" onClick={() => {  setShowDashboards(false); setShowLogs(false);  setSelectedServer(null); setShowAlerts(true); }}><span className="nav-item-icon"><Bell size={18} strokeWidth={1.8} /></span><span className="nav-item-label">Alerts</span></button>
        </nav>
      </aside>

      <main className="dashboard-main" id="servers">
        {showLogs ? (
          <LogExplorer servers={servers} />
        ) : showAlerts ? (
          <AlertsView servers={servers} role="owner" />
        ) : showDashboards ? (
          <DashboardsView servers={servers} role="owner" />
        ) : selectedServer ? (
          <ServerDetail server={selectedServer} apiPublicURL={publicAPIURL} onBack={() => setSelectedServer(null)} />
        ) : <>
        <header className="topbar">
          <div><span className="breadcrumbs">Infrastructure / Servers</span><h1>Servers</h1><p className="page-subtitle">Monitor machine health, resource usage and agent connectivity.</p></div>
          <div className="topbar-actions">
            <span className="live-label" title={apiStatus === "online" ? "Monitoring API is connected" : "Monitoring API connection status"}>
              <span className={`status-dot ${apiStatus === "online" ? "status-dot-online" : apiStatus === "offline" ? "status-dot-unavailable" : "status-dot-pending"}`} />
              {apiStatus === "online" ? "Connected" : apiStatus === "offline" ? "Service unavailable" : "Connecting"}
            </span>
            <button className="button button-secondary" type="button" onClick={() => void loadServers()} disabled={inventoryBusy}><RefreshCw size={15} /> {inventoryBusy ? "Refreshing…" : "Refresh"}</button>
            {canCreate && <button className="button button-primary" type="button" onClick={() => setShowAdd(true)}><Plus size={16} /> Add machine</button>}
          </div>
        </header>

        {notice && <div className="notice notice-error" role="alert">{notice}</div>}
        {inventoryError && <div className="notice notice-error" role="alert">{inventoryError}</div>}

        <section className="welcome-strip">
          <div><div className="eyebrow">Current health</div><p>Live status for the machines connected to this installation.</p></div>
          <div className="phase-label"><span className={`status-dot status-dot-${fleetHealthStatus}`} /> {onlineCount} online · {pendingCount} pending</div>
        </section>

        <section className="summary-grid" aria-label="Server totals">
          <article className="summary-card">
            <span>Total servers</span><strong>{servers.length}</strong><small>Local server inventory</small>
          </article>
          <article className="summary-card">
            <span>Connected</span><strong>{onlineCount}</strong><small>Backend-reported online status</small>
          </article>
          <article className="summary-card">
            <span>Awaiting enrollment</span><strong>{pendingCount}</strong><small>Agent has not checked in yet</small>
          </article>
        </section>

        <section className="inventory-panel">
          <div className="panel-heading">
            <div><h2>Machine inventory</h2><p>Every enrolled VPS, VM, cloud instance and dedicated server connected to this installation.</p></div>
            {canCreate && <button className="button button-secondary" type="button" onClick={() => setShowAdd(true)}><Plus size={15} /> Add machine</button>}
          </div>
          {inventoryBusy && servers.length === 0 ? (
            <div className="empty-state"><span className="loading-indicator" aria-hidden="true" /><h3>Loading server inventory</h3><p>Retrieving connected machines.</p></div>
          ) : servers.length === 0 ? (
            <div className="empty-state empty-state-guide">
              <div className="empty-icon"><ServerIcon size={21} /></div>
              <h3>Start monitoring your first machine</h3>
              <p>Connect a Linux VPS, VM, cloud instance or dedicated server. Monitor Me installs a lightweight native agent; the machine sends its own real metrics and supported logs to this installation.</p>
              <ol className="setup-steps">
                <li><span>1</span><div><strong>Add a machine</strong><small>Create a short-lived, one-time enrolment token.</small></div></li>
                <li><span>2</span><div><strong>Run the install command</strong><small>Paste it into a root or administrator terminal on the machine.</small></div></li>
                <li><span>3</span><div><strong>Watch real telemetry</strong><small>Open the machine to view CPU, memory, storage, network and available logs.</small></div></li>
              </ol>
              {canCreate && <button className="button button-primary" type="button" onClick={() => setShowAdd(true)}><Plus size={16} /> Add your first machine</button>}
            </div>
          ) : (
            <div className="table-wrap">
              <table>
                <thead><tr><th>Machine</th><th>Health</th><th>Last seen</th><th>Enrolled</th><th aria-label="Actions" /></tr></thead>
                <tbody>{servers.map((server) => <ServerRow key={server.id} server={server} canRotate={canCreate} onRotate={rotateToken} onOpen={setSelectedServer} />)}</tbody>
              </table>
            </div>
          )}
        </section>
        </>}
      </main>

      {showAdd && <AddServerDialog onClose={() => setShowAdd(false)} onCreated={(created) => { setShowAdd(false); setIssued(created); void loadServers(); }} />}
      {issued && (
        <EnrollmentDialog
          issued={issued}
          grpcAddress={grpcAddress}
          onClose={() => setIssued(null)}
          onRevoked={() => { setIssued(null); setNotice("The one-time enrollment token was revoked."); }}
          canRevoke={true}
        />
      )}
    </div>
  );
}
