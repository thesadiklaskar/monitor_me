"use client";

import { useCallback, useEffect, useMemo, useState, type FormEvent } from "react";
import { Activity, ArrowDown, ArrowUp, GripVertical, LayoutDashboard, Plus, RefreshCw, Save, Trash2, X } from "lucide-react";
import { apiRequest } from "@/lib/client-api";
import { formatByteRate } from "@/lib/format";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";

type Server = { id: string; name: string; status: string };
type Visualization = "timeseries" | "stat" | "table" | "bar" | "area" | "gauge" | "heatmap" | "logs";
type TimeRange = "1h" | "6h" | "24h" | "7d" | "30d";
type GridWidth = 4 | 6 | 8 | 12;
type DashboardPanel = {
  id: string;
  dashboard_id: string;
  title: string;
  visualization: Visualization;
  server_id: string;
  metric_name: string;
  time_range: TimeRange;
  position: number;
  grid_width: GridWidth;
  label_key: string;
  label_value: string;
  created_at: string;
  updated_at: string;
};
type Dashboard = {
  id: string;
  name: string;
  description: string;
  is_default: boolean;
  default_server_id?: string | null;
  default_time_range: "panel" | TimeRange;
  created_at: string;
  updated_at: string;
  panels: DashboardPanel[];
};
type SeriesPoint = { bucket: string; average: number; minimum: number; maximum: number; sample_count: number };
type SeriesResponse = { points: SeriesPoint[] };
type DashboardLogEvent = {
  event_id: string;
  event_time_unix_nanos: number;
  source: string;
  severity: string;
  body: string;
  attributes: Record<string, string>;
};
type LogResponse = { events: DashboardLogEvent[]; count: number; has_more: boolean };
type PanelSeries = { points?: SeriesPoint[]; logs?: DashboardLogEvent[]; error?: string };
type MetricDefinition = { value: string; label: string; unit: string; category: string };
type DashboardAlertEvent = {
  id: string;
  rule_name: string;
  server_id: string;
  metric_name: string;
  status: "firing" | "resolved";
  triggered_at: string;
  resolved_at?: string | null;
};

const metricCatalog: MetricDefinition[] = [
  { value: "host.cpu.utilization", label: "CPU utilization", unit: "%", category: "Compute" },
  { value: "host.memory.used", label: "Memory used", unit: "bytes", category: "Memory" },
  { value: "host.memory.available", label: "Memory available", unit: "bytes", category: "Memory" },
  { value: "host.memory.swap.used", label: "Swap used", unit: "bytes", category: "Memory" },
  { value: "host.disk.used.percent", label: "Disk usage", unit: "%", category: "Storage" },
  { value: "host.disk.read.bytes_per_second", label: "Disk read throughput", unit: "bytes/s", category: "Storage" },
  { value: "host.disk.write.bytes_per_second", label: "Disk write throughput", unit: "bytes/s", category: "Storage" },
  { value: "host.network.receive.bytes_per_second", label: "Network receive throughput", unit: "bytes/s", category: "Network" },
  { value: "host.network.transmit.bytes_per_second", label: "Network transmit throughput", unit: "bytes/s", category: "Network" },
  { value: "host.network.receive.packets_per_second", label: "Network receive packets", unit: "packets/s", category: "Network" },
  { value: "host.network.transmit.packets_per_second", label: "Network transmit packets", unit: "packets/s", category: "Network" },
  { value: "host.process.count", label: "Process count", unit: "processes", category: "Processes" },
  { value: "process.memory.rss", label: "Process RSS", unit: "bytes", category: "Processes" },
  { value: "agent.cpu.utilization", label: "Agent CPU utilization", unit: "%", category: "Agent" },
];

const rangeMilliseconds: Record<TimeRange, number> = {
  "1h": 60 * 60 * 1000,
  "6h": 6 * 60 * 60 * 1000,
  "24h": 24 * 60 * 60 * 1000,
  "7d": 7 * 24 * 60 * 60 * 1000,
  "30d": 30 * 24 * 60 * 60 * 1000,
};
const rangeSteps: Record<TimeRange, string> = {
  "1h": "30s",
  "6h": "3m",
  "24h": "12m",
  "7d": "84m",
  "30d": "6h",
};
const rangeLabels: Record<TimeRange, string> = {
  "1h": "Last hour",
  "6h": "Last 6 hours",
  "24h": "Last 24 hours",
  "7d": "Last 7 days",
  "30d": "Last 30 days",
};

function formatMetric(value: number, unit: string): string {
  if (!Number.isFinite(value)) return "Unavailable";
  if (unit === "bytes") {
    const amount = value / (1024 ** 3);
    return amount.toFixed(2) + " GiB";
  }
  if (unit === "bytes/s") return formatByteRate(value);
  if (unit === "%") return value.toFixed(1) + "%";
  return value.toLocaleString(undefined, { maximumFractionDigits: 2 }) + (unit ? " " + unit : "");
}

function formatCompact(value: number): string {
  return Intl.NumberFormat(undefined, { notation: "compact", maximumFractionDigits: 1 }).format(value);
}

function formatLogTimestamp(unixNanos: number): string {
  const value = new Date(unixNanos / 1_000_000);
  return Number.isNaN(value.getTime()) ? "Invalid event time" : value.toLocaleString();
}

function severityTone(severity: string): string {
  if (["emergency", "alert", "critical", "error"].includes(severity)) return "dashboard-log-severity dashboard-log-severity-error";
  if (severity === "warning") return "dashboard-log-severity dashboard-log-severity-warning";
  if (severity === "info") return "dashboard-log-severity dashboard-log-severity-info";
  return "dashboard-log-severity";
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
  points: ChartPoint[];
  line: string;
  area: string;
  ticks: { y: number; value: number }[];
  minimum: number;
  maximum: number;
  latest: ChartPoint;
  totalSamples: number;
  startLabel: string;
  endLabel: string;
};

function chartGeometry(points: SeriesPoint[], durationMs: number): ChartGeometry | null {
  const end = Date.now();
  const start = end - durationMs;
  const values = points.map((point) => ({
    time: new Date(point.bucket).getTime(),
    average: point.average,
    minimum: point.minimum,
    maximum: point.maximum,
    sampleCount: point.sample_count,
  })).filter((point) =>
    Number.isFinite(point.time) && point.time >= start && point.time <= end &&
    Number.isFinite(point.average) && Number.isFinite(point.minimum) && Number.isFinite(point.maximum),
  ).sort((left, right) => left.time - right.time);
  if (values.length === 0) return null;

  const minimum = Math.min(...values.map((point) => point.minimum));
  const maximum = Math.max(...values.map((point) => point.maximum));
  const spread = maximum - minimum;
  const padding = spread > 0 ? spread * 0.12 : Math.max(Math.abs(maximum) * 0.08, 1);
  const floor = minimum - padding;
  const ceiling = maximum + padding;
  const yFor = (value: number) => 18 + ((ceiling - value) / (ceiling - floor || 1)) * 164;
  const xFor = (time: number) => 42 + ((time - start) / durationMs) * 920;
  const mapped: ChartPoint[] = values.map((point) => ({
    ...point,
    x: xFor(point.time),
    yAverage: yFor(point.average),
    yMinimum: yFor(point.minimum),
    yMaximum: yFor(point.maximum),
  }));
  const line = mapped.map((point, index) =>
    (index === 0 ? "M " : "L ") + point.x.toFixed(1) + " " + point.yAverage.toFixed(1),
  ).join(" ");
  const area = line + " L " + mapped[mapped.length - 1].x.toFixed(1) + " 190 L " + mapped[0].x.toFixed(1) + " 190 Z";
  const ticks = Array.from({ length: 5 }, (_, index) => ({
    y: 18 + index * 41,
    value: ceiling - ((ceiling - floor) * index) / 4,
  }));
  return {
    points: mapped,
    line,
    area,
    ticks,
    minimum,
    maximum,
    latest: mapped[mapped.length - 1],
    totalSamples: values.reduce((sum, point) => sum + Math.max(0, point.sampleCount || 0), 0),
    startLabel: new Date(start).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" }),
    endLabel: new Date(end).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" }),
  };
}

function MetricVisualization({ panel, series, durationMs, annotations = [] }: {
  panel: DashboardPanel;
  series: PanelSeries | undefined;
  durationMs: number;
  annotations?: DashboardAlertEvent[];
}) {
  const metric = metricCatalog.find((item) => item.value === panel.metric_name);
  if (panel.visualization === "logs") {
    if (series === undefined) return <div className="dashboard-chart-loading"><Skeleton className="h-36 w-full" /></div>;
    if (series.error) return <div className="dashboard-chart-error" role="alert">{series.error}</div>;
    const events = series.logs ?? [];
    return events.length === 0 ? (
      <div className="dashboard-chart-empty">No host log events in this time range. Check the machine's log collection capability and agent status.</div>
    ) : (
      <div className="dashboard-log-panel">
        <div className="dashboard-log-panel-meta">{events.length} events · newest first · latest 50 shown</div>
        <div className="dashboard-log-panel-list">
          {events.slice(0, 50).map((event) => (
            <article className="dashboard-log-row" key={event.event_id}>
              <time>{formatLogTimestamp(event.event_time_unix_nanos)}</time>
              <span className={severityTone(event.severity)}>{event.severity}</span>
              <strong>{event.source}</strong>
              <p>{event.body}</p>
            </article>
          ))}
        </div>
      </div>
    );
  }
  const points = series?.points;
  const latest = points?.[points.length - 1];
  const chart = points ? chartGeometry(points, durationMs) : null;
  const unit = metric?.unit ?? "";

  if (points === undefined) {
    return <div className="dashboard-chart-loading"><Skeleton className="h-36 w-full" /></div>;
  }
  if (series?.error) {
    return <div className="dashboard-chart-error" role="alert">{series.error}</div>;
  }
  if (panel.visualization === "stat") {
    return (
      <div className="dashboard-stat-value dashboard-stat-v2">
        <strong>{formatMetric(latest?.average ?? Number.NaN, unit)}</strong>
        <span>Latest aggregated bucket · {latest ? new Date(latest.bucket).toLocaleString() : "Unavailable"}</span>
        <div className="dashboard-stat-range"><span>Min {formatMetric(latest?.minimum ?? Number.NaN, unit)}</span><span>Max {formatMetric(latest?.maximum ?? Number.NaN, unit)}</span></div>
      </div>
    );
  }
  if (panel.visualization === "table") {
    return !points.length ? <div className="dashboard-chart-empty">No metric samples in this time range.</div> : (
      <div className="dashboard-data-table-wrap">
        <table className="dashboard-data-table">
          <thead><tr><th>Time</th><th>Average</th><th>Minimum</th><th>Maximum</th><th>Samples</th></tr></thead>
          <tbody>{[...points].slice(-20).reverse().map((point) => (
            <tr key={point.bucket}>
              <td>{new Date(point.bucket).toLocaleString()}</td>
              <td>{formatMetric(point.average, unit)}</td>
              <td>{formatMetric(point.minimum, unit)}</td>
              <td>{formatMetric(point.maximum, unit)}</td>
              <td>{point.sample_count.toLocaleString()}</td>
            </tr>
          ))}</tbody>
        </table>
      </div>
    );
  }
  if (!chart) {
    return <div className="dashboard-chart-empty">No valid metric samples in this time range. Missing intervals are not drawn as zero.</div>;
  }
  if (panel.visualization === "gauge") {
    const raw = chart.latest.average;
    const normalized = unit === "%" ? Math.max(0, Math.min(100, raw)) : chart.maximum > 0 ? Math.max(0, Math.min(100, (raw / chart.maximum) * 100)) : 0;
    return (
      <div className="dashboard-gauge-wrap">
        <div className="dashboard-gauge-track"><div className="dashboard-gauge-fill" style={{ width: normalized + "%" }} /></div>
        <strong>{formatMetric(raw, unit)}</strong>
        <span>{unit === "%" ? "of 100%" : "relative to the visible range"} · {new Date(chart.latest.time).toLocaleTimeString()}</span>
      </div>
    );
  }
  if (panel.visualization === "heatmap") {
    const low = Math.min(...chart.points.map((point) => point.average));
    const high = Math.max(...chart.points.map((point) => point.average));
    return (
      <div className="dashboard-heatmap-wrap">
        <div className="dashboard-heatmap" role="img" aria-label={panel.title + " intensity over time"}>
          {chart.points.map((point) => {
            const intensity = high === low ? 0.55 : 0.12 + ((point.average - low) / (high - low)) * 0.82;
            return <span key={point.time} style={{ opacity: intensity }} title={new Date(point.time).toLocaleString() + " · " + formatMetric(point.average, unit)} />;
          })}
        </div>
        <div className="dashboard-chart-footer"><span>{chart.startLabel}</span><span>Lower to higher intensity</span><span>{chart.endLabel}</span></div>
      </div>
    );
  }
  return (
    <div className="dashboard-chart-wrap dashboard-chart-v2">
      <svg viewBox="0 0 1000 220" role="img" aria-label={panel.title + " metric history"} preserveAspectRatio="none">
        <title>{panel.title + " · " + metric?.label}</title>
        {chart.ticks.map((tick, index) => (
          <g key={index}>
            <path d={"M42 " + tick.y.toFixed(1) + " H962"} className="dashboard-chart-grid" />
            <text x="34" y={tick.y + 4} textAnchor="end" className="dashboard-chart-axis-label">{formatCompact(tick.value)}</text>
          </g>
        ))}
        {panel.visualization === "area" && <path d={chart.area} className="dashboard-chart-area" />}
        {panel.visualization === "bar" ? chart.points.map((point) => {
          const baseline = 190;
          const top = Math.min(point.yAverage, baseline);
          const width = Math.max(2, 880 / chart.points.length * 0.66);
          return <rect key={point.time} x={point.x - width / 2} y={top} width={width} height={Math.max(1, baseline - top)} rx="2" className="dashboard-chart-bar"><title>{new Date(point.time).toLocaleString()} · {formatMetric(point.average, unit)}</title></rect>;
        }) : <path d={chart.line} className="dashboard-chart-line" />}
        {annotations.filter((event) => event.metric_name === panel.metric_name).map((event) => {
          const eventTime = new Date(event.triggered_at).getTime();
          const now = Date.now();
          const chartStart = now - durationMs;
          if (!Number.isFinite(eventTime) || eventTime < chartStart || eventTime > now) return null;
          const x = 42 + ((eventTime - chartStart) / durationMs) * 920;
          return (
            <g key={event.id}>
              <line x1={x} x2={x} y1="18" y2="190" className={event.status === "firing" ? "dashboard-alert-marker dashboard-alert-marker-firing" : "dashboard-alert-marker"} />
              <circle cx={x} cy="19" r="5" className={event.status === "firing" ? "dashboard-alert-dot dashboard-alert-dot-firing" : "dashboard-alert-dot"}>
                <title>{event.rule_name} · {event.status} · {new Date(event.triggered_at).toLocaleString()}</title>
              </circle>
            </g>
          );
        })}
        <line x1={chart.latest.x} x2={chart.latest.x} y1="18" y2="190" className="dashboard-chart-latest-guide" />
        <circle cx={chart.latest.x} cy={chart.latest.yAverage} r="4.5" className="dashboard-chart-latest-dot">
          <title>{new Date(chart.latest.time).toLocaleString()} · average {formatMetric(chart.latest.average, unit)} · min {formatMetric(chart.latest.minimum, unit)} · max {formatMetric(chart.latest.maximum, unit)}</title>
        </circle>
        <text x="42" y="211" textAnchor="start" className="dashboard-chart-axis-label">{chart.startLabel}</text>
        <text x="962" y="211" textAnchor="end" className="dashboard-chart-axis-label">{chart.endLabel}</text>
      </svg>
      <div className="dashboard-chart-footer">
        <span>{chart.totalSamples.toLocaleString()} samples · {points.length} buckets</span>
        <strong>Latest {formatMetric(chart.latest.average, unit)}</strong>
      </div>
    </div>
  );
}

export default function DashboardsView({ servers }: { servers: Server[] }) {
  const [dashboards, setDashboards] = useState<Dashboard[]>([]);
  const [selectedId, setSelectedId] = useState("");
  const [seriesByPanel, setSeriesByPanel] = useState<Record<string, PanelSeries>>({});
  const [alertEvents, setAlertEvents] = useState<DashboardAlertEvent[]>([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [showDashboardForm, setShowDashboardForm] = useState(false);
  const [showPanelForm, setShowPanelForm] = useState(false);
  const [editingPanel, setEditingPanel] = useState<DashboardPanel | null>(null);
  const [dashboardName, setDashboardName] = useState("");
  const [dashboardDescription, setDashboardDescription] = useState("");
  const [dashboardDefault, setDashboardDefault] = useState(false);
  const [dashboardDefaultServerId, setDashboardDefaultServerId] = useState("");
  const [dashboardDefaultTimeRange, setDashboardDefaultTimeRange] = useState<"panel" | TimeRange>("panel");
  const [editingDashboardId, setEditingDashboardId] = useState("");
  const [panelTitle, setPanelTitle] = useState("");
  const [panelServerId, setPanelServerId] = useState("");
  const [panelMetric, setPanelMetric] = useState<string>(metricCatalog[0].value);
  const [panelVisualization, setPanelVisualization] = useState<Visualization>("timeseries");
  const [panelRange, setPanelRange] = useState<TimeRange>("1h");
  const [panelGridWidth, setPanelGridWidth] = useState<GridWidth>(6);
  const [panelLabelKey, setPanelLabelKey] = useState("");
  const [panelLabelValue, setPanelLabelValue] = useState("");
  const [serverFilter, setServerFilter] = useState("all");
  const [rangeOverride, setRangeOverride] = useState<"panel" | TimeRange>("panel");
  const [draggingPanelId, setDraggingPanelId] = useState("");

  const selectedDashboard = useMemo(
    () => dashboards.find((dashboard) => dashboard.id === selectedId) ?? null,
    [dashboards, selectedId],
  );
  const orderedPanels = useMemo(
    () => selectedDashboard ? [...selectedDashboard.panels].sort((a, b) => a.position - b.position) : [],
    [selectedDashboard],
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

  const loadAlertEvents = useCallback(async () => {
    try {
      const response = await apiRequest<{ events: DashboardAlertEvent[] }>("/api/alerts");
      setAlertEvents(Array.isArray(response.events) ? response.events : []);
    } catch {
      setAlertEvents([]);
    }
  }, []);

  const refreshSeries = useCallback(async () => {
    if (!selectedDashboard || selectedDashboard.panels.length === 0) {
      setSeriesByPanel({});
      return;
    }
    const now = Date.now();
    const results = await Promise.all(selectedDashboard.panels.map(async (panel) => {
      const range = rangeOverride === "panel" ? panel.time_range : rangeOverride;
      const serverId = serverFilter === "all" ? panel.server_id : serverFilter;
      const duration = rangeMilliseconds[range];
      const effectiveDuration = panel.visualization === "logs" ? Math.min(duration, 7 * 24 * 60 * 60 * 1000) : duration;
      const from = new Date(now - effectiveDuration).toISOString();
      const to = new Date(now).toISOString();
      if (panel.visualization === "logs") {
        const query = new URLSearchParams({ from, to, limit: "201" });
        try {
          const response = await apiRequest<LogResponse>(
            "/api/servers/" + encodeURIComponent(serverId) + "/logs?" + query.toString(),
          );
          if (!response || !Array.isArray(response.events)) throw new Error("Host log query returned an invalid response.");
          return [panel.id, { logs: response.events }] as const;
        } catch (queryError) {
          return [panel.id, { logs: [], error: queryError instanceof Error ? queryError.message : "Host logs could not be loaded." }] as const;
        }
      }
      const query = new URLSearchParams({
        from,
        to,
        step: rangeSteps[range],
        // date_bin uses epoch-aligned buckets; include both edge buckets.
        limit: "150",
        metric: panel.metric_name,
        label_key: panel.label_key || "",
        label_value: panel.label_value || "",
      });
      try {
        const response = await apiRequest<SeriesResponse>(
          "/api/servers/" + encodeURIComponent(serverId) + "/metrics?" + query.toString(),
        );
        if (!response || !Array.isArray(response.points)) throw new Error("Metric history returned an invalid response.");
        return [panel.id, { points: response.points }] as const;
      } catch (queryError) {
        return [panel.id, { points: [], error: queryError instanceof Error ? queryError.message : "Metric history could not be loaded." }] as const;
      }
    }));
    setSeriesByPanel(Object.fromEntries(results));
  }, [selectedDashboard, serverFilter, rangeOverride]);

  useEffect(() => {
    const timer = window.setTimeout(() => { void loadDashboards(); }, 0);
    return () => window.clearTimeout(timer);
  }, [loadDashboards]);

  useEffect(() => {
    if (!selectedDashboard) return;
    setServerFilter(selectedDashboard.default_server_id ?? "all");
    setRangeOverride(selectedDashboard.default_time_range || "panel");
  }, [selectedDashboard?.id, selectedDashboard?.default_server_id, selectedDashboard?.default_time_range]);

  useEffect(() => {
    const initial = window.setTimeout(() => { void loadAlertEvents(); }, 0);
    const interval = window.setInterval(() => { void loadAlertEvents(); }, 30000);
    return () => { window.clearTimeout(initial); window.clearInterval(interval); };
  }, [loadAlertEvents]);

  useEffect(() => {
    if (!selectedDashboard) return;
    const initial = window.setTimeout(() => { void refreshSeries(); }, 0);
    const interval = window.setInterval(() => { void refreshSeries(); }, 15000);
    return () => { window.clearTimeout(initial); window.clearInterval(interval); };
  }, [selectedDashboard, refreshSeries]);

  function resetPanelForm(panel?: DashboardPanel) {
    setEditingPanel(panel ?? null);
    setPanelTitle(panel?.title ?? "");
    setPanelServerId(panel?.server_id ?? servers[0]?.id ?? "");
    setPanelMetric(panel?.visualization === "logs" ? "host.logs.events" : panel?.metric_name ?? metricCatalog[0].value);
    setPanelVisualization(panel?.visualization ?? "timeseries");
    setPanelRange(panel?.time_range ?? "1h");
    setPanelGridWidth(panel?.grid_width ?? 6);
    setPanelLabelKey(panel?.label_key ?? "");
    setPanelLabelValue(panel?.label_value ?? "");
    setShowPanelForm(true);
  }

  function openDashboardForm(dashboard?: Dashboard) {
    setEditingDashboardId(dashboard?.id ?? "");
    setDashboardName(dashboard?.name ?? "");
    setDashboardDescription(dashboard?.description ?? "");
    setDashboardDefault(dashboard?.is_default ?? false);
    setDashboardDefaultServerId(dashboard?.default_server_id ?? "");
    setDashboardDefaultTimeRange(dashboard?.default_time_range ?? "panel");
    setShowDashboardForm(true);
  }

  async function saveDashboard(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setBusy(true);
    setError("");
    setNotice("");
    const body = {
      name: dashboardName.trim(),
      description: dashboardDescription.trim(),
      is_default: dashboardDefault,
      default_server_id: dashboardDefaultServerId || null,
      default_time_range: dashboardDefaultTimeRange,
    };
    try {
      let dashboardId = editingDashboardId;
      if (editingDashboardId) {
        await apiRequest<{ dashboard: Dashboard }>("/api/dashboards/" + encodeURIComponent(editingDashboardId), {
          method: "PATCH",
          body: JSON.stringify(body),
        });
        setNotice("Dashboard settings saved.");
      } else {
        const result = await apiRequest<{ dashboard: Dashboard }>("/api/dashboards", {
          method: "POST",
          body: JSON.stringify(body),
        });
        dashboardId = result.dashboard.id;
        setNotice("Dashboard saved to this installation.");
      }
      setShowDashboardForm(false);
      setEditingDashboardId("");
      await loadDashboards();
      setSelectedId(dashboardId);
    } catch (saveError) {
      setError(saveError instanceof Error ? saveError.message : "Dashboard could not be saved.");
    } finally {
      setBusy(false);
    }
  }

  async function saveViewDefaults() {
    if (!selectedDashboard || busy) return;
    setBusy(true);
    setError("");
    setNotice("");
    try {
      await apiRequest<{ dashboard: Dashboard }>("/api/dashboards/" + encodeURIComponent(selectedDashboard.id), {
        method: "PATCH",
        body: JSON.stringify({
          name: selectedDashboard.name,
          description: selectedDashboard.description,
          is_default: selectedDashboard.is_default,
          default_server_id: serverFilter === "all" ? null : serverFilter,
          default_time_range: rangeOverride,
        }),
      });
      await loadDashboards();
      setNotice("Default machine and time filters saved.");
    } catch (saveError) {
      setError(saveError instanceof Error ? saveError.message : "Dashboard defaults could not be saved.");
    } finally {
      setBusy(false);
    }
  }

  async function savePanel(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!selectedDashboard || !panelServerId) {
      setError("Choose a machine before saving the panel.");
      return;
    }
    const metric = metricCatalog.find((item) => item.value === panelMetric);
    const body = {
      title: panelTitle.trim() || (panelVisualization === "logs" ? "Host logs" : metric?.label || panelMetric),
      visualization: panelVisualization,
      server_id: panelServerId,
      metric_name: panelVisualization === "logs" ? "host.logs.events" : panelMetric,
      time_range: panelRange,
      grid_width: panelGridWidth,
      label_key: panelVisualization === "logs" ? "" : panelLabelKey.trim(),
      label_value: panelVisualization === "logs" ? "" : panelLabelValue,
      ...(editingPanel ? {} : { position: selectedDashboard.panels.length }),
    };
    setBusy(true);
    setError("");
    setNotice("");
    try {
      if (editingPanel) {
        await apiRequest<{ panel: DashboardPanel }>("/api/dashboard-panels/" + encodeURIComponent(editingPanel.id), {
          method: "PATCH",
          body: JSON.stringify(body),
        });
        setNotice("Panel changes saved.");
      } else {
        await apiRequest<{ panel: DashboardPanel }>("/api/dashboards/" + encodeURIComponent(selectedDashboard.id) + "/panels", {
          method: "POST",
          body: JSON.stringify(body),
        });
        setNotice("Panel added to the dashboard.");
      }
      setShowPanelForm(false);
      setEditingPanel(null);
      await loadDashboards();
    } catch (saveError) {
      setError(saveError instanceof Error ? saveError.message : "Panel could not be saved.");
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

  async function movePanelToPosition(panelId: string, position: number) {
    if (!selectedDashboard || busy) return;
    const current = orderedPanels.find((panel) => panel.id === panelId);
    if (!current || current.position === position) return;
    setBusy(true);
    setError("");
    setNotice("");
    try {
      await apiRequest<null>("/api/dashboard-panels/" + encodeURIComponent(panelId) + "/position", {
        method: "PATCH",
        body: JSON.stringify({ position }),
      });
      await loadDashboards();
      setNotice("Panel layout saved.");
    } catch (moveError) {
      setError(moveError instanceof Error ? moveError.message : "Panel order could not be saved.");
    } finally {
      setBusy(false);
      setDraggingPanelId("");
    }
  }

  async function deletePanel(panel: DashboardPanel) {
    if (!window.confirm("Remove panel \"" + panel.title + "\"?")) return;
    setBusy(true);
    setError("");
    try {
      await apiRequest<null>("/api/dashboard-panels/" + encodeURIComponent(panel.id), { method: "DELETE" });
      setNotice("Panel removed.");
      await loadDashboards();
    } catch (deleteError) {
      setError(deleteError instanceof Error ? deleteError.message : "Panel could not be removed.");
    } finally {
      setBusy(false);
    }
  }

  const serverLabel = (id: string) => servers.find((server) => server.id === id)?.name ?? "Machine unavailable";
  const activeRangeLabel = rangeOverride === "panel" ? "Panel-specific ranges" : rangeLabels[rangeOverride];

  return (
    <section className="dashboards-view dashboards-native">
      <header className="dashboards-view-header">
        <div>
          <div className="eyebrow">DATA WORKSPACE / VISUALIZATION</div>
          <h1>Dashboards</h1>
          <p>Build reusable views from real machine telemetry, arrange panels, and inspect historical behaviour.</p>
        </div>
        <div className="dashboards-header-actions">
          <Button variant="secondary" onClick={() => { void loadDashboards(); void refreshSeries(); }} disabled={loading || busy}><RefreshCw size={14} /> Refresh</Button>
          <Button onClick={() => openDashboardForm()}><Plus size={15} /> New dashboard</Button>
        </div>
      </header>

      {error && <div className="notice notice-error" role="alert">{error}</div>}
      {notice && <div className="notice notice-success" role="status">{notice}</div>}

      {showDashboardForm && (
        <Card className="dashboard-editor-card">
          <CardHeader><CardTitle>{editingDashboardId ? "Edit dashboard" : "Create dashboard"}</CardTitle><CardDescription>Save dashboard details and default machine/time filters for every session.</CardDescription></CardHeader>
          <CardContent>
            <form className="dashboard-builder-grid" onSubmit={saveDashboard}>
              <label className="field"><span>Name</span><input value={dashboardName} onChange={(event) => setDashboardName(event.target.value)} required minLength={2} maxLength={120} placeholder="Production overview" /></label>
              <label className="field dashboard-description-field"><span>Description</span><textarea value={dashboardDescription} onChange={(event) => setDashboardDescription(event.target.value)} maxLength={500} rows={2} placeholder="Purpose and audience for this view" /></label>
              <label className="dashboard-default-check"><input type="checkbox" checked={dashboardDefault} onChange={(event) => setDashboardDefault(event.target.checked)} /> Set as the default dashboard</label>
              <label className="field"><span>Default machine</span><select value={dashboardDefaultServerId} onChange={(event) => setDashboardDefaultServerId(event.target.value)}><option value="">Use each panel's machine</option>{servers.map((server) => <option key={server.id} value={server.id}>{server.name}</option>)}</select></label>
              <label className="field"><span>Default time range</span><select value={dashboardDefaultTimeRange} onChange={(event) => setDashboardDefaultTimeRange(event.target.value as "panel" | TimeRange)}><option value="panel">Use each panel's range</option>{Object.entries(rangeLabels).map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select></label>
              <div className="dashboard-builder-actions">
                <Button variant="secondary" type="button" onClick={() => setShowDashboardForm(false)}>Cancel</Button>
                <Button type="submit" disabled={busy}>{busy ? "Saving…" : editingDashboardId ? "Save dashboard" : "Create dashboard"}</Button>
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
            <p>Create your first view, then add panels for CPU, memory, storage, network and workload metrics.</p>
            <Button onClick={() => openDashboardForm()}><Plus size={15} /> Create your first dashboard</Button>
          </CardContent>
        </Card>
      ) : (
        <div className="dashboard-layout">
          <aside className="dashboard-list-card">
            <div className="dashboard-list-heading"><span>Saved dashboards</span><Badge variant="secondary">{dashboards.length}</Badge></div>
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
                <div><h2>{selectedDashboard.name}</h2><p>{selectedDashboard.description || "Installation-wide monitoring view."}</p></div>
                <div className="dashboard-detail-actions">
                  <Button variant="secondary" size="sm" onClick={() => openDashboardForm(selectedDashboard)} disabled={busy}><Activity size={13} /> Edit view</Button>
                  <Button variant="destructive" size="sm" onClick={() => void deleteDashboard()} disabled={busy}><Trash2 size={13} /> Delete</Button>
                  <Button size="sm" onClick={() => resetPanelForm()} disabled={servers.length === 0}><Plus size={14} /> Add panel</Button>
                </div>
              </div>

              <div className="dashboard-variable-bar">
                <label className="field"><span>Machine filter</span><select value={serverFilter} onChange={(event) => setServerFilter(event.target.value)}><option value="all">Use each panel's machine</option>{servers.map((server) => <option key={server.id} value={server.id}>{server.name}</option>)}</select></label>
                <label className="field"><span>Time window</span><select value={rangeOverride} onChange={(event) => setRangeOverride(event.target.value as "panel" | TimeRange)}><option value="panel">Use each panel's range</option>{Object.entries(rangeLabels).map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select></label>
                <div className="dashboard-variable-summary"><strong>{orderedPanels.length} panels</strong><span>{activeRangeLabel}</span><Button variant="secondary" size="sm" onClick={() => void saveViewDefaults()} disabled={busy}>Save view defaults</Button></div>
              </div>

              {showPanelForm && (
                <Card className="dashboard-editor-card">
                  <CardHeader>
                    <div className="dashboard-editor-heading"><div><CardTitle>{editingPanel ? "Edit panel" : "Create a panel"}</CardTitle><CardDescription>Choose the data source and how the result should be displayed.</CardDescription></div><button className="icon-button" type="button" aria-label="Close panel editor" onClick={() => { setShowPanelForm(false); setEditingPanel(null); }}><X size={16} /></button></div>
                  </CardHeader>
                  <CardContent>
                    <form className="dashboard-builder-grid dashboard-panel-builder" onSubmit={savePanel}>
                      <label className="field"><span>Panel title</span><input value={panelTitle} onChange={(event) => setPanelTitle(event.target.value)} maxLength={120} placeholder="CPU utilization" /></label>
                      {panelVisualization !== "logs" && <label className="field"><span>Metric</span><select value={panelMetric} onChange={(event) => setPanelMetric(event.target.value)}>{Object.entries(metricCatalog.reduce<Record<string, MetricDefinition[]>>((groups, metric) => { (groups[metric.category] ||= []).push(metric); return groups; }, {})).map(([category, metrics]) => <optgroup key={category} label={category}>{metrics.map((metric) => <option key={metric.value} value={metric.value}>{metric.label}</option>)}</optgroup>)}</select></label>}
                      <label className="field"><span>Machine</span><select value={panelServerId} onChange={(event) => setPanelServerId(event.target.value)} required>{servers.map((server) => <option key={server.id} value={server.id}>{server.name}</option>)}</select></label>
                      <label className="field"><span>Visualization</span><select value={panelVisualization} onChange={(event) => setPanelVisualization(event.target.value as Visualization)}><option value="timeseries">Time series</option><option value="area">Area</option><option value="bar">Bar</option><option value="stat">Stat</option><option value="gauge">Gauge</option><option value="heatmap">Heatmap</option><option value="table">Table</option><option value="logs">Host logs</option></select></label>
                      <label className="field"><span>Default time range</span><select value={panelRange} onChange={(event) => setPanelRange(event.target.value as TimeRange)}>{Object.entries(rangeLabels).map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select></label>
                      <label className="field"><span>Panel width</span><select value={panelGridWidth} onChange={(event) => setPanelGridWidth(Number(event.target.value) as GridWidth)}><option value={4}>Compact · ⅓</option><option value={6}>Half · ½</option><option value={8}>Wide · ⅔</option><option value={12}>Full width</option></select></label>
                      <label className="field"><span>Label key (optional)</span><input value={panelLabelKey} onChange={(event) => setPanelLabelKey(event.target.value)} maxLength={64} placeholder="mountpoint or interface" required={panelLabelValue.trim().length > 0 && panelVisualization !== "logs"} disabled={panelVisualization === "logs"} /></label>
                      <label className="field"><span>Label value (optional)</span><input value={panelLabelValue} onChange={(event) => setPanelLabelValue(event.target.value)} maxLength={256} placeholder="/ or eth0" required={panelLabelKey.trim().length > 0 && panelVisualization !== "logs"} disabled={panelVisualization === "logs"} /></label>

                      <div className="dashboard-builder-actions">
                        <Button variant="secondary" type="button" onClick={() => { setShowPanelForm(false); setEditingPanel(null); }}>Cancel</Button>
                        <Button type="submit" disabled={busy || servers.length === 0}><Save size={14} /> {busy ? "Saving…" : editingPanel ? "Save changes" : "Add panel"}</Button>
                      </div>
                    </form>
                  </CardContent>
                </Card>
              )}

              {orderedPanels.length === 0 ? (
                <Card className="dashboard-empty-state dashboard-no-panels">
                  <CardContent><Activity size={22} /><h3>This dashboard is empty</h3><p>Add a panel to start exploring real machine metrics.</p><Button onClick={() => resetPanelForm()} disabled={servers.length === 0}><Plus size={14} /> Add first panel</Button></CardContent>
                </Card>
              ) : (
                <div className="dashboard-panels-grid dashboard-panels-grid-v2">
                  {orderedPanels.map((panel, index) => {
                    const effectiveServer = serverFilter === "all" ? panel.server_id : serverFilter;
                    const effectiveRange = rangeOverride === "panel" ? panel.time_range : rangeOverride;
                    const duration = rangeMilliseconds[effectiveRange];
                    const selectedServerName = serverLabel(effectiveServer);
                    return (
                      <Card
                        key={panel.id}
                        className={draggingPanelId === panel.id ? "dashboard-panel-card dashboard-panel-card-v2 dashboard-panel-dragging" : "dashboard-panel-card dashboard-panel-card-v2"}
                        style={{ gridColumn: "span " + (panel.grid_width || 6) }}
                        draggable={!busy}
                        onDragStart={(event) => { setDraggingPanelId(panel.id); event.dataTransfer.effectAllowed = "move"; event.dataTransfer.setData("text/plain", panel.id); }}
                        onDragOver={(event) => { if (draggingPanelId) event.preventDefault(); }}
                        onDrop={(event) => { event.preventDefault(); const id = event.dataTransfer.getData("text/plain") || draggingPanelId; if (id && id !== panel.id) void movePanelToPosition(id, index); setDraggingPanelId(""); }}
                        onDragEnd={() => setDraggingPanelId("")}
                      >
                        <CardHeader className="dashboard-panel-heading dashboard-panel-heading-v2">
                          <div className="dashboard-panel-title">
                            <div className="dashboard-panel-title-line"><span className="dashboard-panel-grip" title="Drag to reorder"><GripVertical size={16} /></span><CardTitle>{panel.title}</CardTitle></div>
                            <CardDescription>{selectedServerName} · {panel.visualization === "logs" && effectiveRange === "30d" ? "Last 7 days (log limit)" : rangeLabels[effectiveRange]} · {panel.visualization === "logs" ? "Host log events" : (metricCatalog.find((metric) => metric.value === panel.metric_name)?.label ?? panel.metric_name) + (panel.label_key && panel.label_value ? " · " + panel.label_key + "=" + panel.label_value : "")}</CardDescription>
                          </div>
                          <div className="dashboard-panel-actions dashboard-panel-actions-v2">
                            <Badge variant="secondary">{panel.visualization}</Badge>
                            <button className="icon-button dashboard-panel-action" type="button" aria-label={"Edit " + panel.title} title="Edit panel" onClick={() => resetPanelForm(panel)}><Activity size={14} /></button>
                            <button className="icon-button dashboard-panel-action" type="button" aria-label={"Move " + panel.title + " up"} title="Move up" disabled={index === 0 || busy} onClick={() => void movePanelToPosition(panel.id, index - 1)}><ArrowUp size={14} /></button>
                            <button className="icon-button dashboard-panel-action" type="button" aria-label={"Move " + panel.title + " down"} title="Move down" disabled={index === orderedPanels.length - 1 || busy} onClick={() => void movePanelToPosition(panel.id, index + 1)}><ArrowDown size={14} /></button>
                            <button className="icon-button dashboard-panel-action dashboard-panel-delete" type="button" aria-label={"Remove " + panel.title} title="Remove panel" disabled={busy} onClick={() => void deletePanel(panel)}><Trash2 size={14} /></button>
                          </div>
                        </CardHeader>
                        <CardContent className="dashboard-panel-content dashboard-panel-content-v2">
                          <MetricVisualization
                            panel={panel}
                            series={seriesByPanel[panel.id]}
                            durationMs={duration}
                            annotations={alertEvents.filter((event) => event.server_id === effectiveServer && event.metric_name === panel.metric_name)}
                          />
                        </CardContent>
                      </Card>
                    );
                  })}
                </div>
              )}
            </div>
          )}
        </div>
      )}
    </section>
  );
}
