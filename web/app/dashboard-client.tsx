"use client";

import { FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import { apiRequest } from "@/lib/client-api";
import { ServerDetail } from "./server-detail";
import ProjectWorkspace from "./project-workspace";
import AlertsWorkspace from "./alerts-workspace";
import RemoteOperationsWorkspace from "./remote-operations-workspace";

type User = {
  id?: string;
  user_id?: string;
  email: string;
  display_name: string;
  organization_id: string;
  organization_name?: string;
  organization?: string;
  role: string;
};

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

type AuthResponse = { user: User; expires_at: string };
type InventoryResponse = { servers: Server[] };
type Project = { id: string; name: string; slug: string; created_at: string };
type ProjectListResponse = { projects: Project[] };
type ConfigResponse = { agent_grpc_address: string | null; public_api_url: string | null };

type ViewState = "checking" | "signed-out" | "signed-in";
type AuthMode = "login" | "register";

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
    case "offline": return "Disconnected";
    default: return status.replaceAll("_", " ");
  }
}

function statusClass(status: string): string {
  switch (status.toLowerCase()) {
    case "online": return "border-emerald-400/25 bg-emerald-400/10 text-emerald-300";
    case "pending": return "border-amber-400/25 bg-amber-400/10 text-amber-200";
    case "disabled":
    case "offline": return "border-slate-500/30 bg-slate-500/10 text-slate-300";
    default: return "border-[var(--border)] bg-[var(--surface-raised)] text-[var(--muted)]";
  }
}

function canManageServers(role: string): boolean {
  return ["owner", "admin", "operator"].includes(role);
}

function canAdminister(role: string): boolean {
  return ["owner", "admin"].includes(role);
}

function slugFromName(value: string): string {
  return value.toLowerCase().trim().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "").slice(0, 64);
}

function AuthScreen({
  onAuthenticated,
}: {
  onAuthenticated: (user: User) => void;
}) {
  const [mode, setMode] = useState<AuthMode>("login");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [organizationName, setOrganizationName] = useState("");
  const [organizationSlug, setOrganizationSlug] = useState("");

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setBusy(true);
    setError("");
    try {
      const result = await apiRequest<AuthResponse>(
        mode === "login" ? "/api/auth/login" : "/api/auth/register",
        {
          method: "POST",
          body: JSON.stringify(
            mode === "login"
              ? { email, password }
              : {
                  email,
                  password,
                  display_name: displayName,
                  organization_name: organizationName,
                  organization_slug: organizationSlug || slugFromName(organizationName),
                },
          ),
        },
      );
      onAuthenticated(result.user);
    } catch (submitError) {
      setError(displayError(submitError));
    } finally {
      setBusy(false);
    }
  }

  return (
    <main className="auth-shell">
      <div className="auth-brand">
        <div className="brand-mark">M</div>
        <div>
          <p className="brand-name">MONITOR ME</p>
          <p className="brand-subtitle">Infrastructure control plane</p>
        </div>
      </div>

      <section className="auth-card">
        <div className="eyebrow">Secure server operations</div>
        <h1>{mode === "login" ? "Welcome back." : "Create your workspace."}</h1>
        <p className="auth-intro">
          {mode === "login"
            ? "Sign in to manage your organization and enrolled infrastructure."
            : "This setup creates the first organization owner. Registration closes automatically after the first account is created."}
        </p>

        <form onSubmit={submit} className="form-stack">
          {mode === "register" && (
            <>
              <label className="field">
                <span>Your name</span>
                <input value={displayName} onChange={(event) => setDisplayName(event.target.value)} required minLength={1} maxLength={120} autoComplete="name" />
              </label>
              <label className="field">
                <span>Organization name</span>
                <input
                  value={organizationName}
                  onChange={(event) => {
                    setOrganizationName(event.target.value);
                    setOrganizationSlug(slugFromName(event.target.value));
                  }}
                  required minLength={2} maxLength={120}
                  autoComplete="organization"
                />
              </label>
              <label className="field">
                <span>Organization slug</span>
                <input value={organizationSlug} onChange={(event) => setOrganizationSlug(slugFromName(event.target.value))} required minLength={3} maxLength={64} />
              </label>
            </>
          )}
          <label className="field">
            <span>Email address</span>
            <input value={email} onChange={(event) => setEmail(event.target.value)} type="email" required maxLength={254} autoComplete="email" />
          </label>
          <label className="field">
            <span>Password</span>
            <input value={password} onChange={(event) => setPassword(event.target.value)} type="password" required minLength={mode === "register" ? 12 : 1} maxLength={128} autoComplete={mode === "login" ? "current-password" : "new-password"} />
            {mode === "register" && <small>Use at least 12 characters.</small>}
          </label>

          {error && <div className="notice notice-error" role="alert">{error}</div>}
          <button className="button button-primary button-wide" type="submit" disabled={busy}>
            {busy ? "Please wait…" : mode === "login" ? "Sign in" : "Create owner account"}
          </button>
        </form>

        <div className="auth-switch">
          {mode === "login" ? (
            <>
              <span>First time setting up Monitor Me?</span>
              <button type="button" onClick={() => { setMode("register"); setError(""); }}>Set up the first owner</button>
            </>
          ) : (
            <>
              <span>Already have an account?</span>
              <button type="button" onClick={() => { setMode("login"); setError(""); }}>Return to sign in</button>
            </>
          )}
        </div>
      </section>
      <p className="auth-footnote">Sessions are held in an HttpOnly cookie. The bearer credential is not exposed to browser JavaScript.</p>
    </main>
  );
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
          {copied ? "Copied to clipboard" : "Copy one-time token"}
        </button>

        <div className="install-instructions">
          <h3>Linux agent setup</h3>
          {grpcAddress ? (
            <p>Configure the agent to connect to <code>{grpcAddress}</code>.</p>
          ) : (
            <p className="text-warning">The public agent gRPC address has not been configured. Set MONITOR_ME_AGENT_GRPC_ADDRESS in the web service environment before using this installation guidance.</p>
          )}
          <ol>
            <li>Install the Monitor Me agent binary and systemd unit on the Linux host.</li>
            <li>Copy the control-plane TLS CA certificate to <code>/etc/monitor-me/server-ca.crt</code>.</li>
            <li>Edit <code>/etc/monitor-me/agent.env</code> with the configured gRPC address, CA path and this token.</li>
            <li>Run <code>sudo chmod 0600 /etc/monitor-me/agent.env</code>, then <code>sudo systemctl enable --now monitor-me-agent</code>.</li>
            <li>After the private agent state file exists, remove the enrollment token from the environment file and restart the service.</li>
          </ol>
          <p className="install-note">Never paste the token into a URL, shell command, shared ticket, or log. The host does not need Docker.</p>
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
            <div className="eyebrow">New host registration</div>
            <h2 id="add-server-title">Add a server</h2>
          </div>
          <button className="icon-button" type="button" onClick={onClose} aria-label="Close">×</button>
        </div>
        <p className="modal-copy">Create an organization-scoped, single-use enrollment credential for a Linux host.</p>
        <form onSubmit={submit} className="form-stack">
          <label className="field">
            <span>Server display name</span>
            <input autoFocus required minLength={2} maxLength={120} value={name} onChange={(event) => setName(event.target.value)} placeholder="Production API" />
          </label>
          <label className="field">
            <span>Hostname (optional)</span>
            <input maxLength={253} value={hostname} onChange={(event) => setHostname(event.target.value)} placeholder="api-01.example.com" />
          </label>
          {error && <div className="notice notice-error" role="alert">{error}</div>}
          <div className="modal-actions">
            <button className="button button-secondary" type="button" onClick={onClose}>Cancel</button>
            <button className="button button-primary" type="submit" disabled={busy}>{busy ? "Creating…" : "Create server and token"}</button>
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
  const [user, setUser] = useState<User | null>(null);
  const [servers, setServers] = useState<Server[]>([]);
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
  const [showAlerts, setShowAlerts] = useState(false);
  const [showOperations, setShowOperations] = useState(false);
  const [logoutBusy, setLogoutBusy] = useState(false);

  const loadServers = useCallback(async () => {
    setInventoryBusy(true);
    setInventoryError("");
    try {
      const result = await apiRequest<InventoryResponse>("/api/servers");
      setServers(Array.isArray(result.servers) ? result.servers : []);
    } catch (loadError) {
      const message = displayError(loadError);
      setInventoryError(message);
      if (message.toLowerCase().includes("sign in")) setView("signed-out");
    } finally {
      setInventoryBusy(false);
    }
  }, []);

  useEffect(() => {
    let active = true;
    void Promise.all([
      apiRequest<{ user: User }>("/api/auth/me"),
      apiRequest<ConfigResponse>("/api/config"),
      apiRequest<ProjectListResponse>("/api/projects"),
    ]).then(async ([session, config, projectResult]) => {
      if (!active) return;
      setUser(session.user);
      setView("signed-in");
      setGrpcAddress(config.agent_grpc_address);
      setPublicAPIURL(config.public_api_url);
      setProjects(Array.isArray(projectResult.projects) ? projectResult.projects : []);
      await loadServers();
    }).catch(() => {
      if (!active) return;
      setView("signed-out");
    });
    return () => { active = false; };
  }, [loadServers]);

  const onlineCount = useMemo(() => servers.filter((server) => server.status.toLowerCase() === "online").length, [servers]);
  const pendingCount = useMemo(() => servers.filter((server) => server.status.toLowerCase() === "pending").length, [servers]);
  const canCreate = Boolean(user && canManageServers(user.role));

  const updateProjects = useCallback((nextProjects: Project[]) => {
    setProjects(nextProjects);
    setSelectedProjectId((current) => nextProjects.some((project) => project.id === current) ? current : "");
  }, []);

  async function logout() {
    setLogoutBusy(true);
    setNotice("");
    try {
      await apiRequest<null>("/api/auth/logout", { method: "POST", body: "{}" });
      setUser(null);
      setServers([]);
      setView("signed-out");
    } catch (logoutError) {
      setNotice(displayError(logoutError));
      setUser(null);
      setServers([]);
      setView("signed-out");
    } finally {
      setLogoutBusy(false);
    }
  }

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

  function onAuthenticated(nextUser: User) {
    setUser(nextUser);
    setView("signed-in");
    void loadServers();
  }

  if (view === "checking") {
    return <main className="loading-screen"><div className="loading-mark">M</div><p>Checking your secure session…</p></main>;
  }

  if (view === "signed-out" || !user) {
    return <AuthScreen onAuthenticated={onAuthenticated} />;
  }

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div className="sidebar-brand">
          <div className="brand-mark">M</div>
          <div><p className="brand-name">MONITOR ME</p><p className="brand-subtitle">Control plane</p></div>
        </div>
        <div className="workspace-switcher">
          <span className="workspace-icon">{(user.organization_name || user.organization || "W").charAt(0).toUpperCase()}</span>
          <div><strong>{user.organization_name || user.organization || "Workspace"}</strong><span>Current workspace</span></div>
          <span className="chevron">⌄</span>
        </div>
        <nav className="side-nav" aria-label="Main navigation">
          <span className="nav-section-label">Workspace</span>
          <button className={!showProjects && !showAlerts && !showOperations ? "nav-item nav-item-active" : "nav-item"} type="button" onClick={() => { setShowProjects(false); setShowAlerts(false); setShowOperations(false); setSelectedServer(null); }}><span>▦</span>Servers</button>
          <button className={showProjects ? "nav-item nav-item-active" : "nav-item"} type="button" onClick={() => { setShowAlerts(false); setShowOperations(false); setSelectedServer(null); setShowProjects(true); }}><span>◫</span>Projects</button>
          <span className="nav-section-label nav-section-spaced">Manage</span>
          <button className={showAlerts ? "nav-item nav-item-active" : "nav-item"} type="button" onClick={() => { setShowProjects(false); setShowOperations(false); setSelectedServer(null); setShowAlerts(true); }}><span>◉</span>Alerts</button>
          <button className={showOperations ? "nav-item nav-item-active" : "nav-item"} type="button" onClick={() => { setShowProjects(false); setShowAlerts(false); setSelectedServer(null); setShowOperations(true); }}><span>↻</span>Operations</button>
          <span className="nav-item nav-item-disabled" aria-disabled="true"><span>⚙</span>Settings <small>Later</small></span>
        </nav>
        <div className="sidebar-bottom">
          <div className="sidebar-status"><span className="status-dot status-dot-online" /> API session active</div>
          <div className="user-card">
            <div className="avatar">{(user.display_name || user.email).charAt(0).toUpperCase()}</div>
            <div className="user-meta"><strong>{user.display_name}</strong><span>{user.role}</span></div>
            <button className="icon-button logout-button" type="button" onClick={logout} disabled={logoutBusy} aria-label="Sign out">↗</button>
          </div>
        </div>
      </aside>

      <main className="dashboard-main" id="servers">
        {showOperations ? (
          <RemoteOperationsWorkspace servers={servers} role={user.role} onBack={() => setShowOperations(false)} />
        ) : showAlerts ? (
          <AlertsWorkspace servers={servers} projects={projects} role={user.role} onBack={() => setShowAlerts(false)} />
        ) : showProjects ? (
          <ProjectWorkspace servers={servers} projects={projects} role={user.role} onProjectsChanged={updateProjects} onBack={() => setShowProjects(false)} />
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
            <span className="live-label"><span className="status-dot status-dot-online" /> API connected</span>
            <label className="project-scope-select"><span>Project scope</span>
              <select value={selectedProjectId} onChange={(event) => setSelectedProjectId(event.target.value)}>
                <option value="">All projects</option>
                {projects.map((project) => <option key={project.id} value={project.id}>{project.name}</option>)}
              </select>
            </label>
            <button className="button button-secondary" type="button" onClick={() => void loadServers()} disabled={inventoryBusy}>{inventoryBusy ? "Refreshing…" : "Refresh"}</button>
            {canCreate && <button className="button button-primary" type="button" onClick={() => setShowAdd(true)}>+ Add server</button>}
          </div>
        </header>

        {notice && <div className="notice notice-error" role="alert">{notice}</div>}
        {inventoryError && <div className="notice notice-error" role="alert">{inventoryError}</div>}

        <section className="welcome-strip">
          <div><div className="eyebrow">Infrastructure at a glance</div><p>Manage enrolled hosts and secure agent credentials for {user.organization_name || user.organization || "your workspace"}.</p></div>
          <div className="phase-label"><span className="status-dot status-dot-online" /> Phase 4 · Dashboard rollout</div>
        </section>

        <section className="summary-grid" aria-label="Server totals">
          <article className="summary-card">
            <span>Total servers</span><strong>{servers.length}</strong><small>Organization inventory</small>
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
            <div><h2>Servers</h2><p>Server records are scoped to your authenticated organization.</p></div>
            {canCreate && <button className="button button-secondary" type="button" onClick={() => setShowAdd(true)}>Register a host</button>}
          </div>
          {inventoryBusy && servers.length === 0 ? (
            <div className="empty-state"><div className="loading-mark small">M</div><h3>Loading server inventory</h3><p>Retrieving the servers visible to your organization.</p></div>
          ) : servers.length === 0 ? (
            <div className="empty-state">
              <div className="empty-icon">+</div><h3>No servers enrolled yet</h3>
              <p>Register your first Linux host. Monitor Me will create a short-lived, single-use credential for the agent.</p>
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
        <footer className="dashboard-footer"><span>Monitor Me · Custom infrastructure monitoring</span><span>Linux agent · Telemetry API · Organization-scoped access</span></footer>
        </>}
      </main>

      {showAdd && <AddServerDialog onClose={() => setShowAdd(false)} onCreated={(created) => { setShowAdd(false); setIssued(created); void loadServers(); }} />}
      {issued && (
        <EnrollmentDialog
          issued={issued}
          grpcAddress={grpcAddress}
          onClose={() => setIssued(null)}
          onRevoked={() => { setIssued(null); setNotice("The one-time enrollment token was revoked."); }}
          canRevoke={canAdminister(user.role)}
        />
      )}
    </div>
  );
}
