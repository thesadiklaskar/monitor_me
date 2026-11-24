"use client";

import { FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import { Activity, Bell, Copy, Folder, PanelLeftClose, PanelLeftOpen, Plus, RefreshCw, Server as ServerIcon } from "lucide-react";
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
type ConfigResponse = { agent_grpc_address: string | null; public_api_url: string | null; auth_disabled: boolean };

type ViewState = "checking" | "signed-out" | "signed-in" | "unavailable";
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
  const [user, setUser] = useState<User | null>(null);
  const [servers, setServers] = useState<Server[]>([]);
  const [apiStatus, setApiStatus] = useState<"checking" | "online" | "offline">("checking");
  const [inventoryBusy, setInventoryBusy] = useState(false);
  const [inventoryError, setInventoryError] = useState("");
  const [notice, setNotice] = useState("");
  const [showAdd, setShowAdd] = useState(false);
  const [issued, setIssued] = useState<IssuedServer | null>(null);
  const [grpcAddress, setGrpcAddress] = useState<string | null>(null);
  const [publicAPIURL, setPublicAPIURL] = useState<string | null>(null);
  const [authDisabled, setAuthDisabled] = useState(false);
  const [selectedServer, setSelectedServer] = useState<Server | null>(null);
  const [projects, setProjects] = useState<Project[]>([]);
  const [selectedProjectId, setSelectedProjectId] = useState("");
  const [showProjects, setShowProjects] = useState(false);
  const [showAlerts, setShowAlerts] = useState(false);
  const [showOperations, setShowOperations] = useState(false);
  const [logoutBusy, setLogoutBusy] = useState(false);
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
      if (message.toLowerCase().includes("sign in")) setView("signed-out");
    } finally {
      setInventoryBusy(false);
    }
  }, []);

  useEffect(() => {
    if (view !== "signed-in") return;
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
      setAuthDisabled(configResult.value.auth_disabled === true);
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

    async function restoreSession() {
      let anonymousMode = false;
      try {
        // Read the public runtime mode first so local anonymous installs never
        // fall back to a login screen if their workspace API is unavailable.
        const config = await apiRequest<ConfigResponse>("/api/config");
        if (!active) return;
        anonymousMode = config.auth_disabled === true;
        setAuthDisabled(anonymousMode);
        setGrpcAddress(config.agent_grpc_address);
        setPublicAPIURL(config.public_api_url);

        const session = await apiRequest<{ user: User }>("/api/auth/me");
        if (!active) return;

        setUser(session.user);
        setView("signed-in");
        await Promise.all([loadWorkspaceMetadata(), loadServers()]);
      } catch {
        if (!active) return;
        setUser(null);
        setView(anonymousMode ? "unavailable" : "signed-out");
      }
    }

    void restoreSession();
    return () => { active = false; };
  }, [loadServers, loadWorkspaceMetadata]);

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
    void loadWorkspaceMetadata();
    void loadServers();
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

  if (view === "signed-out" || !user) {
    return <AuthScreen onAuthenticated={onAuthenticated} />;
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
          <button title="Servers" aria-label="Servers" className={!showProjects && !showAlerts && !showOperations ? "nav-item nav-item-active" : "nav-item"} type="button" onClick={() => { setShowProjects(false); setShowAlerts(false); setShowOperations(false); setSelectedServer(null); }}><span className="nav-item-icon"><ServerIcon size={18} strokeWidth={1.8} /></span><span className="nav-item-label">Servers</span></button>
          <button title="Projects" aria-label="Projects" className={showProjects ? "nav-item nav-item-active" : "nav-item"} type="button" onClick={() => { setShowAlerts(false); setShowOperations(false); setSelectedServer(null); setShowProjects(true); }}><span className="nav-item-icon"><Folder size={18} strokeWidth={1.8} /></span><span className="nav-item-label">Projects</span></button>
          <span className="nav-section-label nav-section-spaced">Manage</span>
          <button title="Alerts" aria-label="Alerts" className={showAlerts ? "nav-item nav-item-active" : "nav-item"} type="button" onClick={() => { setShowProjects(false); setShowOperations(false); setSelectedServer(null); setShowAlerts(true); }}><span className="nav-item-icon"><Bell size={18} strokeWidth={1.8} /></span><span className="nav-item-label">Alerts</span></button>
          <button title="Operations" aria-label="Operations" className={showOperations ? "nav-item nav-item-active" : "nav-item"} type="button" onClick={() => { setShowProjects(false); setShowAlerts(false); setSelectedServer(null); setShowOperations(true); }}><span className="nav-item-icon"><Activity size={18} strokeWidth={1.8} /></span><span className="nav-item-label">Operations</span></button>
        </nav>
        <div className="sidebar-bottom">
          <div className="sidebar-status" title={authDisabled ? "Local no-login access" : "Authenticated session"}><span className="status-dot status-dot-online" /><span className="sidebar-status-label">{authDisabled ? "No-login · Local" : "Session active"}</span></div>
          {!authDisabled && <button className="icon-button logout-button" type="button" onClick={logout} disabled={logoutBusy} aria-label="Sign out">↗</button>}
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
            <div><h2>Servers</h2><p>Server records are scoped to your local workspace.</p></div>
            {canCreate && <button className="button button-secondary" type="button" onClick={() => setShowAdd(true)}>Register a host</button>}
          </div>
          {inventoryBusy && servers.length === 0 ? (
            <div className="empty-state"><div className="loading-mark small">M</div><h3>Loading server inventory</h3><p>Retrieving the servers visible to your organization.</p></div>
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
