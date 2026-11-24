"use client";

import { useCallback, useEffect, useMemo, useState, type FormEvent } from "react";
import { Activity, LayoutDashboard, Plus, RefreshCw, Trash2 } from "lucide-react";
import { apiRequest } from "@/lib/client-api";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";

type Server = { id: string; name: string; status: string };
type Project = { id: string; name: string; slug: string };
type DashboardPanel = {
  id: string;
  dashboard_id: string;
  title: string;
  visualization: "timeseries" | "stat";
  server_id: string;
  project_id?: string | null;
  metric_name: string;
  time_range: "1h" | "6h" | "24h" | "7d" | "30d";
  position: number;
  created_at: string;
  updated_at: string;
};
type Dashboard = {
  id: string;
  name: string;
  description: string;
  project_id?: string | null;
  is_default: boolean;
  created_at: string;
  updated_at: string;
  panels: DashboardPanel[];
};
type SeriesPoint = { bucket: string; average: number; minimum: number; maximum: number; sample_count: number };
type SeriesResponse = { points: SeriesPoint[] };

const metricCatalog = [
  { value: "host.cpu.utilization", label: "CPU utilization", unit: "%" },
  { value: "host.memory.used", label: "Memory used", unit: "bytes" },
  { value: "host.memory.swap.used", label: "Swap used", unit: "bytes" },
  { value: "host.disk.read.bytes_per_second", label: "Disk read", unit: "bytes/s" },
  { value: "host.network.receive.bytes_per_second", label: "Network receive", unit: "bytes/s" },
  { value: "host.process.count", label: "Process count", unit: "processes" },
  { value: "process.memory.rss", label: "Process RSS", unit: "bytes" },
] as const;

const rangeMilliseconds: Record<DashboardPanel["time_range"], number> = {
  "1h": 60 * 60 * 1000,
  "6h": 6 * 60 * 60 * 1000,
  "24h": 24 * 60 * 60 * 1000,
  "7d": 7 * 24 * 60 * 60 * 1000,
  "30d": 30 * 24 * 60 * 60 * 1000,
};
const rangeSteps: Record<DashboardPanel["time_range"], string> = {
  "1h": "30s",
  "6h": "3m",
  "24h": "12m",
  "7d": "84m",
  "30d": "6h",
};

function formatMetric(value: number, unit: string): string {
  if (!Number.isFinite(value)) return "Unavailable";
  if (unit === "bytes") {
    const amount = value / (1024 ** 3);
    return amount.toFixed(2) + " GiB";
  }
  if (unit === "bytes/s") return (value / (1024 ** 2)).toFixed(2) + " MiB/s";
  if (unit === "%") return value.toFixed(1) + "%";
  return value.toLocaleString(undefined, { maximumFractionDigits: 2 }) + (unit ? " " + unit : "");
}

function chartPath(points: SeriesPoint[]): string {
  const values = points.map((point) => point.average).filter(Number.isFinite);
  if (values.length < 2) return "";
  const low = Math.min(...values);
  const high = Math.max(...values);
  const spread = high - low || Math.max(Math.abs(high) * 0.05, 1);
  return values.map((value, index) => {
    const x = (index / (values.length - 1)) * 1000;
    const y = 210 - ((value - low) / spread) * 180;
    return (index === 0 ? "M" : "L") + x.toFixed(2) + " " + y.toFixed(2);
  }).join(" ");
}

function DashboardPanelCard({
  panel,
  points,
  serverName,
  projectName,
  canEdit,
  onDelete,
}: {
  panel: DashboardPanel;
  points: SeriesPoint[] | undefined;
  serverName: string;
  projectName: string;
  canEdit: boolean;
  onDelete: () => void;
}) {
  const metric = metricCatalog.find((item) => item.value === panel.metric_name);
  const latest = points?.[points.length - 1];
  const hasData = !!points && points.length > 0;
  const path = points ? chartPath(points) : "";
  return (
    <Card className="dashboard-panel-card">
      <CardHeader className="dashboard-panel-heading">
        <div className="dashboard-panel-title">
          <CardTitle>{panel.title}</CardTitle>
          <CardDescription>{serverName} · {projectName} · {panel.time_range}</CardDescription>
        </div>
        <div className="dashboard-panel-actions">
          <Badge variant={panel.visualization === "stat" ? "secondary" : "default"}>{panel.visualization === "stat" ? "Stat" : "Time series"}</Badge>
          {canEdit && <Button variant="ghost" size="icon" aria-label={"Remove " + panel.title} title="Remove panel" onClick={onDelete}><Trash2 size={14} /></Button>}
        </div>
      </CardHeader>
      <CardContent className="dashboard-panel-content">
        {points === undefined ? (
          <div className="dashboard-chart-loading"><Skeleton className="h-28 w-full" /></div>
        ) : !hasData ? (
          <div className="dashboard-chart-empty">No metric samples in this time range.</div>
        ) : panel.visualization === "stat" ? (
          <div className="dashboard-stat-value">
            <strong>{formatMetric(latest?.average ?? Number.NaN, metric?.unit ?? "")}</strong>
            <span>Latest aggregated bucket · {latest ? new Date(latest.bucket).toLocaleTimeString() : "Unavailable"}</span>
          </div>
        ) : (
          <div className="dashboard-chart-wrap">
            <svg viewBox="0 0 1000 240" role="img" aria-label={panel.title + " metric history"} preserveAspectRatio="none">
              <path d="M0 30 H1000 M0 120 H1000 M0 210 H1000" className="dashboard-chart-grid" />
              {path && <path d={path} className="dashboard-chart-line" />}
            </svg>
            <div className="dashboard-chart-footer">
              <span>{points.length} buckets</span>
              <strong>{formatMetric(latest?.average ?? Number.NaN, metric?.unit ?? "")}</strong>
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  );
}

export default function DashboardsWorkspace({
  servers,
  projects,
  role,
  onBack,
}: {
  servers: Server[];
  projects: Project[];
  role: string;
  onBack: () => void;
}) {
  const [dashboards, setDashboards] = useState<Dashboard[]>([]);
  const [selectedId, setSelectedId] = useState("");
  const [seriesByPanel, setSeriesByPanel] = useState<Record<string, SeriesPoint[]>>({});
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [showDashboardForm, setShowDashboardForm] = useState(false);
  const [showPanelForm, setShowPanelForm] = useState(false);
  const [dashboardName, setDashboardName] = useState("");
  const [dashboardDescription, setDashboardDescription] = useState("");
  const [dashboardProjectId, setDashboardProjectId] = useState("");
  const [dashboardDefault, setDashboardDefault] = useState(false);
  const [panelTitle, setPanelTitle] = useState("");
  const [panelServerId, setPanelServerId] = useState("");
  const [panelProjectId, setPanelProjectId] = useState("");
  const [panelMetric, setPanelMetric] = useState<string>(metricCatalog[0].value);
  const [panelVisualization, setPanelVisualization] = useState<"timeseries" | "stat">("timeseries");
  const [panelRange, setPanelRange] = useState<DashboardPanel["time_range"]>("1h");

  const canEdit = role === "owner" || role === "admin";
  const selectedDashboard = useMemo(
    () => dashboards.find((dashboard) => dashboard.id === selectedId) ?? null,
    [dashboards, selectedId],
  );

  const loadDashboards = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const result = await apiRequest<{ dashboards: Dashboard[] }>("/api/dashboards");
      const nextDashboards = Array.isArray(result.dashboards) ? result.dashboards : [];
      setDashboards(nextDashboards);
      setSelectedId((current) => {
        if (nextDashboards.some((dashboard) => dashboard.id === current)) return current;
        return nextDashboards.find((dashboard) => dashboard.is_default)?.id ?? nextDashboards[0]?.id ?? "";
      });
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : "Dashboards could not be loaded.");
    } finally {
      setLoading(false);
    }
  }, []);

  const refreshSeries = useCallback(async () => {
    if (!selectedDashboard || selectedDashboard.panels.length === 0) {
      setSeriesByPanel({});
      return;
    }
    const now = Date.now();
    const results = await Promise.all(selectedDashboard.panels.map(async (panel) => {
      const from = new Date(now - rangeMilliseconds[panel.time_range]).toISOString();
      const to = new Date(now).toISOString();
      const query = new URLSearchParams({
        from,
        to,
        step: rangeSteps[panel.time_range],
        limit: "120",
      });
      if (panel.project_id) query.set("project_id", panel.project_id);
      try {
        const response = await apiRequest<SeriesResponse>(
          "/api/servers/" + encodeURIComponent(panel.server_id) + "/metrics?" + query.toString(),
        );
        return [panel.id, Array.isArray(response.points) ? response.points : []] as const;
      } catch {
        return [panel.id, []] as const;
      }
    }));
    setSeriesByPanel(Object.fromEntries(results));
  }, [selectedDashboard]);

  useEffect(() => {
    const timer = window.setTimeout(() => { void loadDashboards(); }, 0);
    return () => window.clearTimeout(timer);
  }, [loadDashboards]);

  useEffect(() => {
    if (!selectedDashboard) return;
    const initial = window.setTimeout(() => { void refreshSeries(); }, 0);
    const interval = window.setInterval(() => { void refreshSeries(); }, 15000);
    return () => { window.clearTimeout(initial); window.clearInterval(interval); };
  }, [selectedDashboard, refreshSeries]);

  async function createDashboard(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setBusy(true);
    setError("");
    setNotice("");
    try {
      const result = await apiRequest<{ dashboard: Dashboard }>("/api/dashboards", {
        method: "POST",
        body: JSON.stringify({
          name: dashboardName,
          description: dashboardDescription,
          project_id: dashboardProjectId || "",
          is_default: dashboardDefault,
        }),
      });
      setDashboardName("");
      setDashboardDescription("");
      setDashboardProjectId("");
      setDashboardDefault(false);
      setShowDashboardForm(false);
      setNotice("Dashboard saved to this installation.");
      await loadDashboards();
      setSelectedId(result.dashboard.id);
    } catch (createError) {
      setError(createError instanceof Error ? createError.message : "Dashboard could not be created.");
    } finally {
      setBusy(false);
    }
  }

  async function createPanel(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!selectedDashboard || !panelServerId) {
      setError("Choose a server before adding a panel.");
      return;
    }
    const metric = metricCatalog.find((item) => item.value === panelMetric);
    setBusy(true);
    setError("");
    setNotice("");
    try {
      await apiRequest<{ panel: DashboardPanel }>(
        "/api/dashboards/" + encodeURIComponent(selectedDashboard.id) + "/panels",
        {
          method: "POST",
          body: JSON.stringify({
            title: panelTitle.trim() || metric?.label || panelMetric,
            visualization: panelVisualization,
            server_id: panelServerId,
            project_id: panelProjectId || selectedDashboard.project_id || "",
            metric_name: panelMetric,
            time_range: panelRange,
            position: selectedDashboard.panels.length,
          }),
        },
      );
      setPanelTitle("");
      setShowPanelForm(false);
      setNotice("Panel configuration saved.");
      await loadDashboards();
    } catch (createError) {
      setError(createError instanceof Error ? createError.message : "Dashboard panel could not be created.");
    } finally {
      setBusy(false);
    }
  }

  async function deleteDashboard() {
    if (!selectedDashboard || !window.confirm("Delete dashboard \"" + selectedDashboard.name + "\" and all of its panels?")) return;
    setBusy(true);
    setError("");
    try {
      await apiRequest<null>("/api/dashboards/" + encodeURIComponent(selectedDashboard.id), { method: "DELETE" });
      setNotice("Dashboard deleted.");
      setSeriesByPanel({});
      await loadDashboards();
    } catch (deleteError) {
      setError(deleteError instanceof Error ? deleteError.message : "Dashboard could not be deleted.");
    } finally {
      setBusy(false);
    }
  }

  async function deletePanel(panel: DashboardPanel) {
    if (!window.confirm("Remove panel \"" + panel.title + "\"?")) return;
    setError("");
    try {
      await apiRequest<null>("/api/dashboard-panels/" + encodeURIComponent(panel.id), { method: "DELETE" });
      setNotice("Panel removed.");
      await loadDashboards();
    } catch (deleteError) {
      setError(deleteError instanceof Error ? deleteError.message : "Panel could not be removed.");
    }
  }

  const selectedServerDefault = servers[0]?.id ?? "";
  const serverLabel = (id: string) => servers.find((server) => server.id === id)?.name ?? "Server unavailable";
  const projectLabel = (id?: string | null) => projects.find((project) => project.id === id)?.name ?? "All projects";

  return (
    <section className="dashboards-workspace">
      <header className="dashboards-workspace-header">
        <div>
          <button className="detail-back-button" type="button" onClick={onBack}>← Back to servers</button>
          <div className="eyebrow">Saved monitoring layouts</div>
          <h1>Dashboards</h1>
          <p>Build persistent metric views from real server history. Panels refresh automatically and respect organization and project scope.</p>
        </div>
        <div className="dashboards-header-actions">
          <Button variant="secondary" onClick={() => { void loadDashboards(); }} disabled={loading}><RefreshCw size={14} /> Refresh</Button>
          {canEdit && <Button onClick={() => setShowDashboardForm((value) => !value)}><Plus size={15} /> New dashboard</Button>}
        </div>
      </header>

      {error && <div className="notice notice-error" role="alert">{error}</div>}
      {notice && <div className="notice notice-success" role="status">{notice}</div>}

      {showDashboardForm && (
        <Card className="dashboard-editor-card">
          <CardHeader><CardTitle>Create dashboard</CardTitle><CardDescription>Saved dashboards belong to this installation and organization.</CardDescription></CardHeader>
          <CardContent>
            <form className="dashboard-builder-grid" onSubmit={createDashboard}>
              <label className="field"><span>Name</span><input value={dashboardName} onChange={(event) => setDashboardName(event.target.value)} required minLength={2} maxLength={120} placeholder="Production overview" /></label>
              <label className="field"><span>Project scope</span>
                <select value={dashboardProjectId} onChange={(event) => setDashboardProjectId(event.target.value)}>
                  <option value="">All projects</option>
                  {projects.map((project) => <option key={project.id} value={project.id}>{project.name}</option>)}
                </select>
              </label>
              <label className="field dashboard-description-field"><span>Description</span><textarea value={dashboardDescription} onChange={(event) => setDashboardDescription(event.target.value)} maxLength={500} rows={2} placeholder="What this dashboard is used for" /></label>
              <label className="dashboard-default-check"><input type="checkbox" checked={dashboardDefault} onChange={(event) => setDashboardDefault(event.target.checked)} /> Make this the default dashboard</label>
              <div className="dashboard-builder-actions">
                <Button variant="secondary" type="button" onClick={() => setShowDashboardForm(false)}>Cancel</Button>
                <Button type="submit" disabled={busy}>{busy ? "Saving…" : "Create dashboard"}</Button>
              </div>
            </form>
          </CardContent>
        </Card>
      )}

      {loading ? (
        <div className="dashboard-panels-grid"><Skeleton className="h-64 w-full" /><Skeleton className="h-64 w-full" /></div>
      ) : dashboards.length === 0 ? (
        <Card className="dashboard-empty-state">
          <CardContent>
            <div className="dashboard-empty-icon"><LayoutDashboard size={24} /></div>
            <h2>No dashboards yet</h2>
            <p>Create a dashboard, then add charts for CPU, memory, disk, network and workload metrics.</p>
            {canEdit && <Button onClick={() => setShowDashboardForm(true)}><Plus size={15} /> Create your first dashboard</Button>}
          </CardContent>
        </Card>
      ) : (
        <div className="dashboard-layout">
          <aside className="dashboard-list-card">
            <div className="dashboard-list-heading"><span>Dashboards</span><Badge variant="secondary">{dashboards.length}</Badge></div>
            {dashboards.map((dashboard) => (
              <button key={dashboard.id} type="button" className={selectedId === dashboard.id ? "dashboard-list-item dashboard-list-item-active" : "dashboard-list-item"} onClick={() => setSelectedId(dashboard.id)}>
                <span><strong>{dashboard.name}</strong><small>{dashboard.panels.length} panels · {projectLabel(dashboard.project_id)}</small></span>
                {dashboard.is_default && <Badge>Default</Badge>}
              </button>
            ))}
          </aside>

          {selectedDashboard && (
            <div className="dashboard-detail">
              <div className="dashboard-detail-heading">
                <div><h2>{selectedDashboard.name}</h2><p>{selectedDashboard.description || projectLabel(selectedDashboard.project_id)}</p></div>
                <div className="dashboard-detail-actions">
                  {canEdit && <Button variant="destructive" size="sm" onClick={() => void deleteDashboard()} disabled={busy}><Trash2 size={13} /> Delete dashboard</Button>}
                  {canEdit && <Button size="sm" onClick={() => setShowPanelForm((value) => !value)} disabled={servers.length === 0}><Plus size={14} /> Add panel</Button>}
                </div>
              </div>

              {showPanelForm && (
                <Card className="dashboard-editor-card">
                  <CardHeader><CardTitle>Add metric panel</CardTitle><CardDescription>Choose a real metric and the server whose history should be queried.</CardDescription></CardHeader>
                  <CardContent>
                    <form className="dashboard-builder-grid dashboard-panel-builder" onSubmit={createPanel}>
                      <label className="field"><span>Panel title</span><input value={panelTitle} onChange={(event) => setPanelTitle(event.target.value)} maxLength={120} placeholder="CPU utilization" /></label>
                      <label className="field"><span>Metric</span><select value={panelMetric} onChange={(event) => setPanelMetric(event.target.value)}>{metricCatalog.map((metric) => <option key={metric.value} value={metric.value}>{metric.label}</option>)}</select></label>
                      <label className="field"><span>Server</span><select value={panelServerId || selectedServerDefault} onChange={(event) => setPanelServerId(event.target.value)} required>{servers.map((server) => <option key={server.id} value={server.id}>{server.name}</option>)}</select></label>
                      <label className="field"><span>Project scope</span><select value={panelProjectId || selectedDashboard.project_id || ""} onChange={(event) => setPanelProjectId(event.target.value)}><option value="">Dashboard / all projects</option>{projects.map((project) => <option key={project.id} value={project.id}>{project.name}</option>)}</select></label>
                      <label className="field"><span>Visualization</span><select value={panelVisualization} onChange={(event) => setPanelVisualization(event.target.value as "timeseries" | "stat")}><option value="timeseries">Time series</option><option value="stat">Stat</option></select></label>
                      <label className="field"><span>Time range</span><select value={panelRange} onChange={(event) => setPanelRange(event.target.value as DashboardPanel["time_range"])}><option value="1h">Last hour</option><option value="6h">Last 6 hours</option><option value="24h">Last 24 hours</option><option value="7d">Last 7 days</option><option value="30d">Last 30 days</option></select></label>
                      <div className="dashboard-builder-actions">
                        <Button variant="secondary" type="button" onClick={() => setShowPanelForm(false)}>Cancel</Button>
                        <Button type="submit" disabled={busy || servers.length === 0}>{busy ? "Saving…" : "Save panel"}</Button>
                      </div>
                    </form>
                  </CardContent>
                </Card>
              )}

              {selectedDashboard.panels.length === 0 ? (
                <Card className="dashboard-empty-state dashboard-no-panels">
                  <CardContent>
                    <Activity size={22} />
                    <h3>No panels in this dashboard</h3>
                    <p>Add a metric panel to start plotting real telemetry for an enrolled machine.</p>
                    {canEdit && <Button onClick={() => setShowPanelForm(true)} disabled={servers.length === 0}><Plus size={14} /> Add first panel</Button>}
                  </CardContent>
                </Card>
              ) : (
                <div className="dashboard-panels-grid">
                  {[...selectedDashboard.panels].sort((a, b) => a.position - b.position).map((panel) => (
                    <DashboardPanelCard
                      key={panel.id}
                      panel={panel}
                      points={seriesByPanel[panel.id]}
                      serverName={serverLabel(panel.server_id)}
                      projectName={projectLabel(panel.project_id)}
                      canEdit={canEdit}
                      onDelete={() => void deletePanel(panel)}
                    />
                  ))}
                </div>
              )}
            </div>
          )}
        </div>
      )}
    </section>
  );
}
