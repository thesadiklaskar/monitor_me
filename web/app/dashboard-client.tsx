"use client";

import { FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import { Activity, Bell, Copy, FileText, Folder, LayoutDashboard, PanelLeftClose, PanelLeftOpen, Plus, RefreshCw, Server as ServerIcon } from "lucide-react";
import { apiRequest } from "@/lib/client-api";
import { ServerDetail } from "./server-detail";
import ProjectWorkspace from "./project-workspace";
import AlertsWorkspace from "./alerts-workspace";
import RemoteOperationsWorkspace from "./remote-operations-workspace";
import DashboardsWorkspace from "./dashboards-workspace";
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
type Project = { id: string; name: string; slug: string; created_at: string };
type ProjectListResponse = { projects: Project[] };
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
            <div className="eyebrow">New node enrollment</div>
            <h2 id="add-server-title">Add a machine</h2>
          </div>
          <button className="icon-button" type="button" onClick={onClose} aria-label="Close">×</button>
        </div>
        <p className="modal-copy">Create a one-time enrollment token. No operating system needs to be selected here; host discovery happens on the target machine.</p>
        <form onSubmit={submit} className="form-stack">
          <label className="field">
            <span>Node label</span>
            <input autoFocus required minLength={2} maxLength={120} value={name} onChange={(event) => setName(event.target.value)} placeholder="production-api-01" />
            <small>A display label only. The agent reports the actual hostname and operating system.</small>
          </label>
          <label className="field">
            <span>Expected hostname (optional)</span>
            <input maxLength={253} value={hostname} onChange={(event) => setHostname(event.target.value)} placeholder="Leave empty to discover automatically" />
          </label>
          {error && <div className="notice notice-error" role="alert">{error}</div>}
          <div className="modal-actions">
            <button className="button button-secondary" type="button" onClick={onClose}>Cancel</button>
            <button className="button button-primary" type="submit" disabled={busy}>{busy ? "Creating…" : "Create node token"}</button>
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
  const [projects, setProjects] = useState<Project[]>([]);
  const [selectedProjectId, setSelectedProjectId] = useState("");
  const [showProjects, setShowProjects] = useState(false);
  const [showDashboards, setShowDashboards] = useState(false);
  const [showLogs, setShowLogs] = useState(false);
  const [showAlerts, setShowAlerts] = useState(false);
  const [showOperations, setShowOperations] = useState(false);
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);

  useEffect(() => {
    try { setSidebarCollapsed(window.localStorage.getItem("monitor-me-sidebar-collapsed") === "true"); } catch { /* browser storage may be unavailable */ }
  }, []);

  function toggleSidebar() {
    setSidebarCollapsed((current) => {
      const next = !current;
      try { window.localStorage.setItem("monitor-me-sidebar-collapsed", String(next)); } catch { /* preserve the in-memory preference */ }
      return next;
    });
  }

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

  const loadWorkspaceMetadata = useCallback(async () => {
    const [configResult, projectResult] = await Promise.allSettled([
      apiRequest<ConfigResponse>("/api/config"),
      apiRequest<ProjectListResponse>("/api/projects"),
    ]);

    if (configResult.status === "fulfilled") {
      setGrpcAddress(configResult.value.agent_grpc_address);
      setPublicAPIURL(configResult.value.public_api_url);
    } else {
      setNotice(displayError(configResult.reason));
    }

    if (projectResult.status === "fulfilled") {
      setProjects(Array.isArray(projectResult.value.projects) ? projectResult.value.projects : []);
    } else {
      setNotice(displayError(projectResult.reason));
    }
  }, []);

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
        await Promise.all([loadWorkspaceMetadata(), loadServers()]);
      } catch {
        if (!active) return;
        setView("unavailable");
      }
    }

    void initializeInstallation();
    return () => { active = false; };
  }, [loadServers, loadWorkspaceMetadata]);

  const onlineCount = useMemo(() => servers.filter((server) => server.status.toLowerCase() === "online").length, [servers]);
  const pendingCount = useMemo(() => servers.filter((server) => server.status.toLowerCase() === "pending").length, [servers]);
  const canCreate = true;

  const updateProjects = useCallback((nextProjects: Project[]) => {
    setProjects(nextProjects);
    setSelectedProjectId((current) => nextProjects.some((project) => project.id === current) ? current : "");
  }, []);

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
    return <main className="loading-screen"><div className="loading-mark">M</div><p>Connecting to your Monitor Me workspace…</p></main>;
  }

  if (view === "unavailable") {
    return (
      <main className="loading-screen">
        <div className="loading-mark">M</div>
        <h1>Local workspace unavailable</h1>
        <p>Monitor Me could not initialize the local workspace. Check that the API, database, and Docker services are healthy.</p>
        <button className="button button-primary" type="button" onClick={() => window.location.reload()}>Retry connection</button>
      </main>
    );
  }

  return (
    <div className={sidebarCollapsed ? "app-shell app-shell-sidebar-collapsed" : "app-shell"}>
      <aside className={sidebarCollapsed ? "sidebar sidebar-collapsed" : "sidebar"}>
        <div className="sidebar-brand">
          <div className="brand-mark">M</div>
          <div className="sidebar-brand-copy"><p className="brand-name">MONITOR ME</p><p className="brand-subtitle">Observability</p></div>
          <button className="sidebar-toggle" type="button" onClick={toggleSidebar} aria-label={sidebarCollapsed ? "Expand sidebar" : "Collapse sidebar"} title={sidebarCollapsed ? "Expand sidebar" : "Collapse sidebar"}>{sidebarCollapsed ? <PanelLeftOpen size={18} strokeWidth={1.8} /> : <PanelLeftClose size={18} strokeWidth={1.8} />}</button>
        </div>
        <div className="workspace-switcher" title="Single installation">
          <span className="workspace-icon"><Activity size={15} strokeWidth={1.8} /></span>
          <div className="workspace-switcher-copy"><strong>Infrastructure</strong><span>Single installation</span></div>
        </div>
        <nav className="side-nav" aria-label="Main navigation">
          <span className="nav-section-label">Monitor</span>
          <button title="Servers" aria-label="Servers" className={!showProjects && !showDashboards && !showLogs && !showAlerts && !showOperations ? "nav-item nav-item-active" : "nav-item"} type="button" onClick={() => { setShowProjects(false); setShowDashboards(false); setShowLogs(false); setShowAlerts(false); setShowOperations(false); setSelectedServer(null); }}><span className="nav-item-icon"><ServerIcon size={18} strokeWidth={1.8} /></span><span className="nav-item-label">Servers</span></button>
          <button title="Logs" aria-label="Logs" className={showLogs ? "nav-item nav-item-active" : "nav-item"} type="button" onClick={() => { setShowProjects(false); setShowDashboards(false); setShowAlerts(false); setShowOperations(false); setSelectedServer(null); setShowLogs(true); }}><span className="nav-item-icon"><FileText size={18} strokeWidth={1.8} /></span><span className="nav-item-label">Logs</span></button>
          <button title="Projects" aria-label="Projects" className={showProjects ? "nav-item nav-item-active" : "nav-item"} type="button" onClick={() => { setShowDashboards(false); setShowLogs(false); setShowAlerts(false); setShowOperations(false); setSelectedServer(null); setShowProjects(true); }}><span className="nav-item-icon"><Folder size={18} strokeWidth={1.8} /></span><span className="nav-item-label">Projects</span></button>
          <button title="Dashboards" aria-label="Dashboards" className={showDashboards ? "nav-item nav-item-active" : "nav-item"} type="button" onClick={() => { setShowProjects(false); setShowLogs(false); setShowAlerts(false); setShowOperations(false); setSelectedServer(null); setShowDashboards(true); }}><span className="nav-item-icon"><LayoutDashboard size={18} strokeWidth={1.8} /></span><span className="nav-item-label">Dashboards</span></button>
          <span className="nav-section-label nav-section-spaced">Manage</span>
          <button title="Alerts" aria-label="Alerts" className={showAlerts ? "nav-item nav-item-active" : "nav-item"} type="button" onClick={() => { setShowProjects(false); setShowDashboards(false); setShowLogs(false); setShowOperations(false); setSelectedServer(null); setShowAlerts(true); }}><span className="nav-item-icon"><Bell size={18} strokeWidth={1.8} /></span><span className="nav-item-label">Alerts</span></button>
          <button title="Operations" aria-label="Operations" className={showOperations ? "nav-item nav-item-active" : "nav-item"} type="button" onClick={() => { setShowProjects(false); setShowDashboards(false); setShowLogs(false); setShowAlerts(false); setSelectedServer(null); setShowOperations(true); }}><span className="nav-item-icon"><Activity size={18} strokeWidth={1.8} /></span><span className="nav-item-label">Operations</span></button>
        </nav>
        <div className="sidebar-bottom">
          <div className="sidebar-status" title="Self-hosted single installation"><span className="status-dot status-dot-online" /><span className="sidebar-status-label">Self-hosted · Single install</span></div>
        </div>
      </aside>

      <main className="dashboard-main" id="servers">
        {showLogs ? (
          <LogExplorer servers={servers} projects={projects} onBack={() => setShowLogs(false)} />
        ) : showOperations ? (
          <RemoteOperationsWorkspace servers={servers} role="owner" onBack={() => setShowOperations(false)} />
        ) : showAlerts ? (
          <AlertsWorkspace servers={servers} projects={projects} role="owner" onBack={() => setShowAlerts(false)} />
        ) : showDashboards ? (
          <DashboardsWorkspace servers={servers} projects={projects} role="owner" onBack={() => setShowDashboards(false)} />
        ) : showProjects ? (
          <ProjectWorkspace servers={servers} projects={projects} role="owner" onProjectsChanged={updateProjects} onBack={() => setShowProjects(false)} />
        ) : selectedServer ? (
          <div className="server-detail-shell">
            <label className="project-scope-select detail-project-scope">
              <span>Project scope</span>
              <select value={selectedProjectId} onChange={(event) => setSelectedProjectId(event.target.value)}>
                <option value="">All projects</option>
                {projects.map((project) => <option key={project.id} value={project.id}>{project.name}</option>)}
              </select>
            </label>
            <ServerDetail server={selectedServer} apiPublicURL={publicAPIURL} projectId={selectedProjectId || null} onBack={() => setSelectedServer(null)} />
          </div>
        ) : <>
        <header className="topbar">
          <div><span className="breadcrumbs">Workspace / Servers</span><h1>Server overview</h1></div>
          <div className="topbar-actions">
            <span className="live-label">
              <span className={`status-dot ${apiStatus === "online" ? "status-dot-online" : apiStatus === "offline" ? "status-dot-unavailable" : "status-dot-pending"}`} />
              {apiStatus === "online" ? "API connected" : apiStatus === "offline" ? "API unavailable" : "Checking API"}
            </span>
            <label className="project-scope-select"><span>Project scope</span>
              <select value={selectedProjectId} onChange={(event) => setSelectedProjectId(event.target.value)}>
                <option value="">All projects</option>
                {projects.map((project) => <option key={project.id} value={project.id}>{project.name}</option>)}
              </select>
            </label>
            <button className="button button-secondary" type="button" onClick={() => void loadServers()} disabled={inventoryBusy}><RefreshCw size={15} /> {inventoryBusy ? "Refreshing…" : "Refresh"}</button>
            {canCreate && <button className="button button-primary" type="button" onClick={() => setShowAdd(true)}><Plus size={16} /> Add machine</button>}
          </div>
        </header>

        {notice && <div className="notice notice-error" role="alert">{notice}</div>}
        {inventoryError && <div className="notice notice-error" role="alert">{inventoryError}</div>}

        <section className="welcome-strip">
          <div><div className="eyebrow">Fleet overview</div><p>Monitor the health and performance of every enrolled machine from one place.</p></div>
          <div className="phase-label"><span className="status-dot status-dot-online" /> {onlineCount} online · {pendingCount} pending</div>
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
            <div><h2>Servers</h2><p>Server records are scoped to your local workspace.</p></div>
            {canCreate && <button className="button button-secondary" type="button" onClick={() => setShowAdd(true)}>Register a host</button>}
          </div>
          {inventoryBusy && servers.length === 0 ? (
            <div className="empty-state"><div className="loading-mark small">M</div><h3>Loading server inventory</h3><p>Retrieving the servers in this installation.</p></div>
          ) : servers.length === 0 ? (
            <div className="empty-state">
              <div className="empty-icon">+</div><h3>No servers enrolled yet</h3>
              <p>Add a machine to receive a short-lived, single-use enrollment token. Its native agent reports the host operating system and available environment metadata automatically.</p>
              {canCreate && <button className="button button-primary" type="button" onClick={() => setShowAdd(true)}>Add your first server</button>}
            </div>
          ) : (
            <div className="table-wrap">
              <table>
                <thead><tr><th>Server</th><th>Status</th><th>Last heartbeat</th><th>Added</th><th aria-label="Actions" /></tr></thead>
                <tbody>{servers.map((server) => <ServerRow key={server.id} server={server} canRotate={canCreate} onRotate={rotateToken} onOpen={setSelectedServer} />)}</tbody>
              </table>
            </div>
          )}
        </section>
        <footer className="dashboard-footer"><span>Monitor Me · Custom infrastructure monitoring</span><span>Linux agent · Telemetry API · Single-install access</span></footer>
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
