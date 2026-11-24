"use client";

import { useCallback, useEffect, useMemo, useState, type FormEvent } from "react";
import { Activity, LayoutDashboard, Plus, RefreshCw, Trash2 } from "lucide-react";
import { apiRequest } from "@/lib/client-api";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";

type Server = { id: string; name: string; status: string };
type DashboardPanel = {
  id: string;
  dashboard_id: string;
  title: string;
  visualization: "timeseries" | "stat";
  server_id: string;
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
  is_default: boolean;
  created_at: string;
  updated_at: string;
  panels: DashboardPanel[];
};
type SeriesPoint = { bucket: string; average: number; minimum: number; maximum: number; sample_count: number };
type SeriesResponse = { points: SeriesPoint[] };
type PanelSeries = { points: SeriesPoint[]; error?: string };

const metricCatalog = [
  { value: "host.cpu.utilization", label: "CPU utilization", unit: "%" },
  { value: "host.memory.used", label: "Memory used", unit: "bytes" },
  { value: "host.memory.available", label: "Memory available", unit: "bytes" },
  { value: "host.memory.swap.used", label: "Swap used", unit: "bytes" },
  { value: "host.disk.used.percent", label: "Disk usage", unit: "%" },
  { value: "host.disk.read.bytes_per_second", label: "Disk read throughput", unit: "bytes/s" },
  { value: "host.disk.write.bytes_per_second", label: "Disk write throughput", unit: "bytes/s" },
  { value: "host.network.receive.bytes_per_second", label: "Network receive throughput", unit: "bytes/s" },
  { value: "host.network.transmit.bytes_per_second", label: "Network transmit throughput", unit: "bytes/s" },
  { value: "host.network.receive.packets_per_second", label: "Network receive packets", unit: "packets/s" },
  { value: "host.network.transmit.packets_per_second", label: "Network transmit packets", unit: "packets/s" },
  { value: "host.process.count", label: "Process count", unit: "processes" },
  { value: "process.memory.rss", label: "Process RSS", unit: "bytes" },
  { value: "agent.cpu.utilization", label: "Agent CPU utilization", unit: "%" },
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

type ChartPoint = {
  time: number;
  x: number;
  average: number;
  minimum: number;
  maximum: number;
  yAverage: number;
  yMinimum: number;
  yMaximum: number;
  sampleCount: number;
};

type ChartGeometry = {
  lineSegments: string[];
  rangeBands: string[];
  ticks: { y: number; value: number }[];
  latest: ChartPoint;
  average: number;
  minimum: number;
  maximum: number;
  totalSamples: number;
  startLabel: string;
  endLabel: string;
};

function chartGeometry(points: SeriesPoint[], durationMs: number): ChartGeometry | null {
  const end = Date.now();
  const start = end - durationMs;
  const values = points
    .map((point) => ({
      time: new Date(point.bucket).getTime(),
      average: point.average,
      minimum: point.minimum,
      maximum: point.maximum,
      sampleCount: point.sample_count,
    }))
    .filter((point) =>
      Number.isFinite(point.time) && point.time >= start && point.time <= end &&
      Number.isFinite(point.average) && Number.isFinite(point.minimum) && Number.isFinite(point.maximum),
    )
    .sort((left, right) => left.time - right.time);
  if (values.length === 0) return null;

  const minimum = Math.min(...values.map((point) => point.minimum));
  const maximum = Math.max(...values.map((point) => point.maximum));
  const spread = maximum - minimum;
  const padding = spread > 0 ? spread * 0.1 : Math.max(Math.abs(maximum) * 0.08, 1);
  const floor = minimum - padding;
  const ceiling = maximum + padding;
  const yFor = (value: number) => 22 + ((ceiling - value) / (ceiling - floor || 1)) * 174;
  const xFor = (time: number) => 70 + ((time - start) / durationMs) * 900;

  const mapped: ChartPoint[] = values.map((point) => ({
    ...point,
    x: xFor(point.time),
    yAverage: yFor(point.average),
    yMinimum: yFor(point.minimum),
    yMaximum: yFor(point.maximum),
  }));

  const gapLimit = (durationMs / 120) * 2.2;
  const lineSegments: string[] = [];
  const rangeBands: string[] = [];
  let segment: ChartPoint[] = [];
  const finishSegment = () => {
    if (segment.length === 0) return;
    const line = segment.map((point, index) =>
      (index === 0 ? "M " : "L ") + point.x.toFixed(1) + " " + point.yAverage.toFixed(1),
    ).join(" ");
    lineSegments.push(line);
    if (segment.length > 1) {
      const upper = segment.map((point) => point.x.toFixed(1) + " " + point.yMaximum.toFixed(1));
      const lower = [...segment].reverse().map((point) => point.x.toFixed(1) + " " + point.yMinimum.toFixed(1));
      rangeBands.push("M " + upper.join(" L ") + " L " + lower.join(" L ") + " Z");
    }
    segment = [];
  };

  let previousTime: number | null = null;
  for (const point of mapped) {
    if (previousTime !== null && point.time - previousTime > gapLimit) finishSegment();
    segment.push(point);
    previousTime = point.time;
  }
  finishSegment();

  const latest = mapped[mapped.length - 1];
  const ticks = Array.from({ length: 5 }, (_, index) => ({
    y: 22 + index * 43.5,
    value: ceiling - ((ceiling - floor) * index) / 4,
  }));
  const rangeLabel = (time: number) => durationMs <= 24 * 60 * 60 * 1000
    ? new Date(time).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })
    : new Date(time).toLocaleDateString([], { month: "short", day: "numeric" });

  return {
    lineSegments,
    rangeBands,
    ticks,
    latest,
    average: values.reduce((sum, point) => sum + point.average, 0) / values.length,
    minimum,
    maximum,
    totalSamples: values.reduce((sum, point) => sum + Math.max(0, point.sampleCount || 0), 0),
    startLabel: rangeLabel(start),
    endLabel: rangeLabel(end),
  };
}

function DashboardPanelCard({
  panel,
  series,
  serverName,
  canEdit,
  onDelete,
}: {
  panel: DashboardPanel;
  series: PanelSeries | undefined;
  serverName: string;
  canEdit: boolean;
  onDelete: () => void;
}) {
  const metric = metricCatalog.find((item) => item.value === panel.metric_name);
  const points = series?.points;
  const latest = points?.[points.length - 1];
  const chart = points ? chartGeometry(points, rangeMilliseconds[panel.time_range]) : null;
  return (
    <Card className="dashboard-panel-card">
      <CardHeader className="dashboard-panel-heading">
        <div className="dashboard-panel-title">
          <CardTitle>{panel.title}</CardTitle>
          <CardDescription>{serverName} · {panel.time_range}</CardDescription>
        </div>
        <div className="dashboard-panel-actions">
          <Badge variant={panel.visualization === "stat" ? "secondary" : "default"}>{panel.visualization === "stat" ? "Stat" : "Time series"}</Badge>
          {canEdit && <Button variant="ghost" size="icon" aria-label={"Remove " + panel.title} title="Remove panel" onClick={onDelete}><Trash2 size={14} /></Button>}
        </div>
      </CardHeader>
      <CardContent className="dashboard-panel-content">
        {points === undefined ? (
          <div className="dashboard-chart-loading"><Skeleton className="h-28 w-full" /></div>
        ) : series?.error ? (
          <div className="dashboard-chart-error" role="alert">{series.error}</div>
        ) : panel.visualization === "stat" ? (
          <div className="dashboard-stat-value">
            <strong>{formatMetric(latest?.average ?? Number.NaN, metric?.unit ?? "")}</strong>
            <span>Latest aggregated bucket · {latest ? new Date(latest.bucket).toLocaleString() : "Unavailable"}</span>
          </div>
        ) : !chart ? (
          <div className="dashboard-chart-empty">No valid metric samples in this time range. Missing intervals are not drawn as zero.</div>
        ) : (
          <div className="dashboard-chart-wrap">
            <svg viewBox="0 0 1000 250" role="img" aria-label={panel.title + " metric history with average and min/max range"} preserveAspectRatio="none">
              <title>{panel.title}: average line with min/max range and visible data gaps</title>
              {chart.ticks.map((tick, index) => (
                <g key={index}>
                  <path d={"M70 " + tick.y.toFixed(1) + " H970"} className="dashboard-chart-grid" />
                  <text x="60" y={tick.y + 4} textAnchor="end" className="dashboard-chart-axis-label">{formatMetric(tick.value, metric?.unit ?? "")}</text>
                </g>
              ))}
              {chart.rangeBands.map((band, index) => <path key={"band-" + index} d={band} className="dashboard-chart-range" />)}
              {chart.lineSegments.map((line, index) => <path key={"line-" + index} d={line} className="dashboard-chart-line" />)}
              <line x1={chart.latest.x} x2={chart.latest.x} y1="22" y2="196" className="dashboard-chart-latest-guide" />
              <circle cx={chart.latest.x} cy={chart.latest.yAverage} r="5" className="dashboard-chart-latest-dot">
                <title>{new Date(chart.latest.time).toLocaleString()} · average {formatMetric(chart.latest.average, metric?.unit ?? "")} · range {formatMetric(chart.latest.minimum, metric?.unit ?? "")} to {formatMetric(chart.latest.maximum, metric?.unit ?? "")}</title>
              </circle>
              <text x="70" y="232" textAnchor="start" className="dashboard-chart-axis-label">{chart.startLabel}</text>
              <text x="970" y="232" textAnchor="end" className="dashboard-chart-axis-label">{chart.endLabel}</text>
            </svg>
            <div className="dashboard-chart-footer">
              <span>{chart.totalSamples.toLocaleString()} samples · {points.length} buckets · min/max range shown</span>
              <strong>Latest {formatMetric(chart.latest.average, metric?.unit ?? "")}</strong>
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  );
}

export default function DashboardsView({ servers }: {
  servers: Server[];
}) {
  const [dashboards, setDashboards] = useState<Dashboard[]>([]);
  const [selectedId, setSelectedId] = useState("");
  const [seriesByPanel, setSeriesByPanel] = useState<Record<string, PanelSeries>>({});
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [showDashboardForm, setShowDashboardForm] = useState(false);
  const [showPanelForm, setShowPanelForm] = useState(false);
  const [dashboardName, setDashboardName] = useState("");
  const [dashboardDescription, setDashboardDescription] = useState("");
  const [dashboardDefault, setDashboardDefault] = useState(false);
  const [panelTitle, setPanelTitle] = useState("");
  const [panelServerId, setPanelServerId] = useState("");
  const [panelMetric, setPanelMetric] = useState<string>(metricCatalog[0].value);
  const [panelVisualization, setPanelVisualization] = useState<"timeseries" | "stat">("timeseries");
  const [panelRange, setPanelRange] = useState<DashboardPanel["time_range"]>("1h");

  const canEdit = true;
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
      query.set("metric", panel.metric_name);
      try {
        const response = await apiRequest<SeriesResponse>(
          "/api/servers/" + encodeURIComponent(panel.server_id) + "/metrics?" + query.toString(),
        );
        if (!response || !Array.isArray(response.points)) {
          throw new Error("Metric history returned an invalid response.");
        }
        return [panel.id, { points: response.points }] as const;
      } catch (queryError) {
        return [panel.id, { points: [], error: queryError instanceof Error ? queryError.message : "Metric history could not be loaded." }] as const;
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
          is_default: dashboardDefault,
        }),
      });
      setDashboardName("");
      setDashboardDescription("");
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

  return (
    <section className="dashboards-view">
      <header className="dashboards-view-header">
        <div>
          <div className="eyebrow">VISUALIZE / METRICS</div>
          <h1>Dashboards</h1>
          <p>Create and save reusable views of CPU, memory, storage and network metrics from your connected machines.</p>
        </div>
        <div className="dashboards-header-actions">
          <Button variant="secondary" onClick={() => { void loadDashboards(); void refreshSeries(); }} disabled={loading}><RefreshCw size={14} /> Refresh</Button>
          {canEdit && <Button onClick={() => setShowDashboardForm((value) => !value)}><Plus size={15} /> New dashboard</Button>}
        </div>
      </header>

      {error && <div className="notice notice-error" role="alert">{error}</div>}
      {notice && <div className="notice notice-success" role="status">{notice}</div>}

      {showDashboardForm && (
        <Card className="dashboard-editor-card">
          <CardHeader><CardTitle>Create dashboard</CardTitle><CardDescription>Saved dashboards belong to this installation.</CardDescription></CardHeader>
          <CardContent>
            <form className="dashboard-builder-grid" onSubmit={createDashboard}>
              <label className="field"><span>Name</span><input value={dashboardName} onChange={(event) => setDashboardName(event.target.value)} required minLength={2} maxLength={120} placeholder="Production overview" /></label>
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
                <span><strong>{dashboard.name}</strong><small>{dashboard.panels.length} panels</small></span>
                {dashboard.is_default && <Badge>Default</Badge>}
              </button>
            ))}
          </aside>

          {selectedDashboard && (
            <div className="dashboard-detail">
              <div className="dashboard-detail-heading">
                <div><h2>{selectedDashboard.name}</h2><p>{selectedDashboard.description || "Installation-wide monitoring dashboard."}</p></div>
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
                      series={seriesByPanel[panel.id]}
                      serverName={serverLabel(panel.server_id)}
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
