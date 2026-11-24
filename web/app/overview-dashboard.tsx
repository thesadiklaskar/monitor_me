"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Activity, ArrowDownToLine, ArrowLeft, ArrowUpFromLine, Cpu, Database, HardDrive, MemoryStick, Network, RefreshCw, Server as ServerIcon, TerminalSquare } from "lucide-react";
import { apiRequest } from "@/lib/client-api";
import { formatByteRate } from "@/lib/format";

type Server = {
  id: string;
  name: string;
  hostname?: string;
  status: string;
  agent_last_seen_at?: string | null;
  created_at: string;
};

type SeriesPoint = {
  bucket: string;
  average: number;
  minimum: number;
  maximum: number;
  sample_count: number;
};

type SeriesResponse = { points: SeriesPoint[] };

type LatestSample = {
  metric_name: string;
  value: number;
  unit: string;
  labels?: Record<string, string>;
  observed_at: string;
};

type LatestResponse = { samples: LatestSample[] };

type LogEvent = {
  event_id: string;
  event_time_unix_nanos: number;
  source: string;
  severity: string;
  body: string;
};

type LogResponse = { events: LogEvent[]; count: number; has_more: boolean };

type MetricDefinition = {
  key: string;
  title: string;
  metric: string;
  unit: "percent" | "bytes" | "bytes/second" | "count";
  detail: string;
  icon: typeof Cpu;
};

type PanelResult = { points: SeriesPoint[]; error?: string };
type Loaded<T> = { value: T; error?: string };
type DashboardData = {
  latest: LatestSample[];
  panels: Record<string, PanelResult>;
  logs: LogEvent[];
  logsError?: string;
  refreshedAt: number;
};

const timeWindowMs = 60 * 60 * 1000;
const metricPanels: MetricDefinition[] = [
  { key: "cpu", title: "CPU utilization", metric: "host.cpu.utilization", unit: "percent", detail: "Total processor usage", icon: Cpu },
  { key: "cores", title: "Processor cores", metric: "host.cpu.core.utilization", unit: "percent", detail: "Average across reported cores", icon: Activity },
  { key: "memory-used", title: "Memory used", metric: "host.memory.used", unit: "bytes", detail: "Physical memory in use", icon: MemoryStick },
  { key: "memory-free", title: "Memory available", metric: "host.memory.available", unit: "bytes", detail: "Memory available to the host", icon: Database },
  { key: "storage", title: "Storage usage", metric: "host.disk.used.percent", unit: "percent", detail: "Average mounted-volume usage", icon: HardDrive },
  { key: "disk-read", title: "Disk read", metric: "host.disk.read.bytes_per_second", unit: "bytes/second", detail: "Read throughput across devices", icon: ArrowDownToLine },
  { key: "disk-write", title: "Disk write", metric: "host.disk.write.bytes_per_second", unit: "bytes/second", detail: "Write throughput across devices", icon: ArrowUpFromLine },
  { key: "network-in", title: "Network receive", metric: "host.network.receive.bytes_per_second", unit: "bytes/second", detail: "Incoming traffic across interfaces", icon: Network },
  { key: "network-out", title: "Network transmit", metric: "host.network.transmit.bytes_per_second", unit: "bytes/second", detail: "Outgoing traffic across interfaces", icon: Network },
  { key: "processes", title: "Processes", metric: "host.process.count", unit: "count", detail: "Processes in the latest host sample", icon: TerminalSquare },
];

function healthLabel(status: string): string {
  switch (status.toLowerCase()) {
    case "online": return "Connected";
    case "pending": return "Awaiting agent";
    case "stale": return "Delayed heartbeat";
    case "offline": return "Disconnected";
    case "disabled": return "Disabled";
    default: return "Unknown";
  }
}

function healthClass(status: string): string {
  switch (status.toLowerCase()) {
    case "online": return "is-online";
    case "pending": return "is-pending";
    case "stale": return "is-stale";
    default: return "is-offline";
  }
}

function formatWhen(value?: string | null): string {
  if (!value) return "No heartbeat received";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "Time unavailable" : date.toLocaleString();
}

function formatMetric(value: number | undefined, unit: MetricDefinition["unit"]): string {
  if (value === undefined || !Number.isFinite(value)) return "Unavailable";
  if (unit === "percent") return value.toFixed(1) + "%";
  if (unit === "bytes/second") return formatByteRate(value);
  if (unit === "count") return Math.round(value).toLocaleString();
  const absolute = Math.abs(value);
  if (absolute >= 1024 ** 3) return (value / (1024 ** 3)).toFixed(2) + " GiB";
  if (absolute >= 1024 ** 2) return (value / (1024 ** 2)).toFixed(1) + " MiB";
  if (absolute >= 1024) return (value / 1024).toFixed(1) + " KiB";
  return Math.round(value).toLocaleString() + " B";
}

function severityClass(severity: string): string {
  if (["emergency", "alert", "critical", "error"].includes(severity.toLowerCase())) return "overview-severity overview-severity-error";
  if (severity.toLowerCase() === "warning") return "overview-severity overview-severity-warning";
  if (severity.toLowerCase() === "info") return "overview-severity overview-severity-info";
  return "overview-severity";
}

function formatLogTime(unixNanos: number): string {
  const time = new Date(unixNanos / 1_000_000);
  return Number.isNaN(time.getTime()) ? "Invalid timestamp" : time.toLocaleString();
}

function MiniChart({ points, label }: { points: SeriesPoint[]; label: string }) {
  const values = points
    .map((point) => ({ time: new Date(point.bucket).getTime(), value: point.average }))
    .filter((point) => Number.isFinite(point.time) && Number.isFinite(point.value))
    .sort((left, right) => left.time - right.time);

  if (values.length === 0) {
    return <div className="overview-chart-empty">No real samples in this time range</div>;
  }

  const min = Math.min(...values.map((point) => point.value));
  const max = Math.max(...values.map((point) => point.value));
  const spread = max - min || Math.max(Math.abs(max) * 0.08, 1);
  const coordinates = values.map((point, index) => {
    const x = values.length === 1 ? 50 : 4 + (index / (values.length - 1)) * 92;
    const y = 35 - ((point.value - min) / spread) * 29;
    return { x, y };
  });
  const line = coordinates.map((point) => point.x.toFixed(1) + "," + point.y.toFixed(1)).join(" ");
  const last = coordinates[coordinates.length - 1];
  return (
    <div className="overview-chart">
      <svg viewBox="0 0 100 42" role="img" aria-label={label + " history for the last hour"}>
        {coordinates.length > 1 && <polyline points={line} className="overview-chart-line" />}
        <circle cx={last.x} cy={last.y} r="2.7" className="overview-chart-dot" />
      </svg>
      <div className="overview-chart-range">
        <span>{new Date(values[0].time).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}</span>
        <span>{new Date(values[values.length - 1].time).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}</span>
      </div>
    </div>
  );
}

function MetricCard({
  definition,
  result,
  latest,
  loading,
}: {
  definition: MetricDefinition;
  result?: PanelResult;
  latest?: LatestSample;
  loading: boolean;
}) {
  const points = result?.points ?? [];
  const mostRecentPoint = points[points.length - 1];
  const value = latest?.value ?? mostRecentPoint?.average;
  const Icon = definition.icon;
  return (
    <article className="overview-metric-card">
      <div className="overview-metric-heading">
        <span className="overview-metric-icon"><Icon size={15} /></span>
        <div><h3>{definition.title}</h3><p>{definition.detail}</p></div>
      </div>
      {result?.error ? (
        <div className="overview-chart-empty">{result.error}</div>
      ) : loading && result === undefined ? (
        <div className="overview-chart-empty">Loading real host samples…</div>
      ) : (
        <>
          <strong className="overview-metric-value">{formatMetric(value, definition.unit)}</strong>
          <MiniChart points={points} label={definition.title} />
          <div className="overview-metric-footer">
            <span>{points.reduce((sum, point) => sum + Math.max(0, point.sample_count), 0).toLocaleString()} samples</span>
            <span>Last hour</span>
          </div>
        </>
      )}
    </article>
  );
}

async function fetchMetric(serverId: string, definition: MetricDefinition, from: string, to: string): Promise<PanelResult> {
  const query = new URLSearchParams({
    metric: definition.metric,
    from,
    to,
    step: "1m",
    limit: "150",
  });
  const response = await apiRequest<SeriesResponse>(
    "/api/servers/" + encodeURIComponent(serverId) + "/metrics?" + query.toString(),
  );
  if (!response || !Array.isArray(response.points)) {
    throw new Error("Metric history returned an invalid response.");
  }
  return { points: response.points };
}

export default function OverviewDashboard({ servers }: { servers: Server[] }) {
  const [selectedServerId, setSelectedServerId] = useState("");
  const [data, setData] = useState<DashboardData | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const requestSequence = useRef(0);

  const selectedServer = useMemo(
    () => servers.find((server) => server.id === selectedServerId) ?? null,
    [servers, selectedServerId],
  );
  const connectedCount = useMemo(
    () => servers.filter((server) => server.status.toLowerCase() === "online").length,
    [servers],
  );
  const otherCount = servers.length - connectedCount;

  const loadServerDashboard = useCallback(async () => {
    if (!selectedServerId) return;
    const requestId = ++requestSequence.current;
    const now = Date.now();
    const from = new Date(now - timeWindowMs).toISOString();
    const to = new Date(now).toISOString();
    setLoading(true);
    setError("");

    const [latestResult, logsResult, panelEntries] = await Promise.all([
      apiRequest<LatestResponse>(
        "/api/servers/" + encodeURIComponent(selectedServerId) + "/metrics/latest",
      ).then((response): Loaded<LatestSample[]> => ({ value: Array.isArray(response.samples) ? response.samples : [] }))
        .catch((loadError: unknown): Loaded<LatestSample[]> => ({ error: loadError instanceof Error ? loadError.message : "Latest samples could not be loaded.", value: [] })),
      apiRequest<LogResponse>(
        "/api/servers/" + encodeURIComponent(selectedServerId) + "/logs?" +
        new URLSearchParams({ from, to, limit: "21" }).toString(),
      ).then((response): Loaded<LogEvent[]> => ({ value: Array.isArray(response.events) ? response.events : [] }))
        .catch((loadError: unknown): Loaded<LogEvent[]> => ({ error: loadError instanceof Error ? loadError.message : "Host logs could not be loaded.", value: [] })),
      Promise.all(metricPanels.map(async (definition) => {
        try {
          return [definition.key, await fetchMetric(selectedServerId, definition, from, to)] as const;
        } catch (loadError) {
          return [definition.key, { points: [], error: loadError instanceof Error ? loadError.message : "Metric history could not be loaded." }] as const;
        }
      })),
    ]);

    if (requestId !== requestSequence.current) return;
    setData({
      latest: latestResult.value,
      panels: Object.fromEntries(panelEntries),
      logs: logsResult.value,
      logsError: logsResult.error,
      refreshedAt: now,
    });
    if (latestResult.error) setError(latestResult.error);
    else setError("");
    setLoading(false);
  }, [selectedServerId]);

  useEffect(() => {
    if (!selectedServerId) return;
    const initial = window.setTimeout(() => { void loadServerDashboard(); }, 0);
    const interval = window.setInterval(() => { void loadServerDashboard(); }, 15000);
    return () => {
      window.clearTimeout(initial);
      window.clearInterval(interval);
      requestSequence.current += 1;
    };
  }, [loadServerDashboard, selectedServerId]);

  function openServer(serverId: string) {
    requestSequence.current += 1;
    setLoading(true);
    setData(null);
    setError("");
    setSelectedServerId(serverId);
  }

  function backToServers() {
    requestSequence.current += 1;
    setLoading(false);
    setSelectedServerId("");
    setData(null);
    setError("");
  }

  if (selectedServer) {
    const latestByMetric = new Map(data?.latest.map((sample) => [sample.metric_name, sample] as const) ?? []);
    const lastUpdated = data?.refreshedAt ? new Date(data.refreshedAt).toLocaleTimeString() : "Loading";
    const storageSamples = data?.latest.filter((sample) => sample.metric_name === "host.disk.used.percent") ?? [];
    const logs = data?.logs.slice(0, 20) ?? [];
    return (
      <section className="overview-workspace">
        <header className="overview-page-header">
          <div>
            <button className="overview-back-button" type="button" onClick={backToServers}><ArrowLeft size={14} /> All servers</button>
            <div className="eyebrow">DASHBOARD / HOST DETAIL</div>
            <h1>{selectedServer.name}</h1>
            <p>{selectedServer.hostname || "Hostname not reported"} · Updated {lastUpdated} · refreshes every 15 seconds</p>
          </div>
          <div className={"overview-health-pill " + healthClass(selectedServer.status)}><span />{healthLabel(selectedServer.status)}</div>
        </header>

        {error && <div className="notice notice-error" role="alert">{error}</div>}

        <section className="overview-summary-grid" aria-label="Latest machine readings">
          <article className="overview-summary-card">
            <span>CPU</span><strong>{formatMetric(latestByMetric.get("host.cpu.utilization")?.value, "percent")}</strong><small>Latest host sample</small>
          </article>
          <article className="overview-summary-card">
            <span>Memory used</span><strong>{formatMetric(latestByMetric.get("host.memory.used")?.value, "bytes")}</strong><small>Latest host sample</small>
          </article>
          <article className="overview-summary-card">
            <span>Storage volumes</span><strong>{storageSamples.length.toLocaleString()}</strong><small>Mounted volumes reporting usage</small>
          </article>
          <article className="overview-summary-card">
            <span>Processes</span><strong>{formatMetric(latestByMetric.get("host.process.count")?.value, "count")}</strong><small>Latest host inventory</small>
          </article>
        </section>

        <div className="overview-section-heading"><div><h2>Performance</h2><p>Real host metrics aggregated from committed telemetry. Empty intervals remain empty.</p></div><button className="button button-secondary" type="button" onClick={() => void loadServerDashboard()} disabled={loading}><RefreshCw size={14} /> {loading ? "Refreshing…" : "Refresh data"}</button></div>
        <section className="overview-metric-grid">
          {metricPanels.map((definition) => (
            <MetricCard
              key={definition.key}
              definition={definition}
              result={data?.panels[definition.key]}
              latest={latestByMetric.get(definition.metric)}
              loading={loading}
            />
          ))}
          <article className="overview-metric-card overview-logs-card">
            <div className="overview-metric-heading">
              <span className="overview-metric-icon"><TerminalSquare size={15} /></span>
              <div><h3>Host logs</h3><p>Latest real events · last hour</p></div>
            </div>
            {data?.logsError ? (
              <div className="overview-chart-empty">{data.logsError}</div>
            ) : logs.length === 0 && loading && data === null ? (
              <div className="overview-chart-empty">Loading recent host log events…</div>
            ) : logs.length === 0 ? (
              <div className="overview-chart-empty">No log events were returned for this server in the last hour.</div>
            ) : (
              <div className="overview-log-list">
                {logs.map((event) => (
                  <article className="overview-log-row" key={event.event_id}>
                    <div><time>{formatLogTime(event.event_time_unix_nanos)}</time><span className={severityClass(event.severity)}>{event.severity}</span></div>
                    <strong>{event.source}</strong>
                    <p>{event.body}</p>
                  </article>
                ))}
              </div>
            )}
            <div className="overview-metric-footer"><span>{data?.logs.length ?? 0} events loaded</span><span>Auto-refresh · 15s</span></div>
          </article>
        </section>
      </section>
    );
  }

  return (
    <section className="overview-workspace">
      <header className="overview-page-header">
        <div>
          <div className="eyebrow">MONITORING / FLEET OVERVIEW</div>
          <h1>Dashboard</h1>
          <p>Every enrolled machine in this installation. Select a server to inspect its real metrics and logs.</p>
        </div>
        <div className="overview-fleet-status">
          <span><i className="overview-status-dot is-online" />{connectedCount} connected</span>
          <span>{otherCount} pending or disconnected</span>
        </div>
      </header>

      {servers.length === 0 ? (
        <div className="overview-empty-state">
          <ServerIcon size={24} />
          <h2>No servers connected yet</h2>
          <p>Add a machine from the Servers section. Its card will appear here after enrollment.</p>
        </div>
      ) : (
        <section className="overview-server-grid" aria-label="Server health cards">
          {servers.map((server) => (
            <button
              className="overview-server-card"
              key={server.id}
              type="button"
              onClick={() => openServer(server.id)}
            >
              <div className="overview-server-card-top">
                <span className="overview-server-icon"><ServerIcon size={17} /></span>
                <span className={"overview-health-pill " + healthClass(server.status)}><span />{healthLabel(server.status)}</span>
              </div>
              <strong>{server.name}</strong>
              <span className="overview-server-host">{server.hostname || "Hostname not reported"}</span>
              <div className="overview-server-footer">
                <span>Last heartbeat</span>
                <time>{formatWhen(server.agent_last_seen_at)}</time>
              </div>
              <span className="overview-server-open">Open server dashboard <span aria-hidden="true">→</span></span>
            </button>
          ))}
        </section>
      )}
    </section>
  );
}
