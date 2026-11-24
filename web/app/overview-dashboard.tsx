"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Activity, ArrowDown, ArrowLeft, ArrowUp, Cpu, Database, FileText, HardDrive, Network, RefreshCw, Server as ServerIcon } from "lucide-react";
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
  unit: "percent" | "bytes" | "bytes/second" | "count" | "rate";
  detail: string;
  icon: typeof Cpu;
};

type PanelResult = { points: SeriesPoint[]; error?: string };
type Loaded<T> = { value: T; error?: string };
type StorageVolume = {
  key: string;
  mountpoint: string;
  device: string;
  filesystem: string;
  totalBytes?: number;
  usedBytes?: number;
  availableBytes?: number;
  usedPercent?: number;
  observedAt?: string;
};

type DashboardData = {
  latest: LatestSample[];
  panels: Record<string, PanelResult>;
  storageHistory: Record<string, PanelResult>;
  logs: LogEvent[];
  logsError?: string;
  refreshedAt: number;
};

const timeWindowMs = 60 * 60 * 1000;
const metricPanels: MetricDefinition[] = [
  { key: "cpu", title: "CPU utilization", metric: "host.cpu.utilization", unit: "percent", detail: "Total processor usage", icon: Cpu },
  { key: "cores", title: "Processor cores", metric: "host.cpu.core.utilization", unit: "percent", detail: "Average across reported cores", icon: Activity },
  { key: "memory-used", title: "Memory used", metric: "host.memory.used", unit: "bytes", detail: "Physical memory in use", icon: Database },
  { key: "memory-free", title: "Memory available", metric: "host.memory.available", unit: "bytes", detail: "Memory available to the host", icon: Database },
  { key: "swap-used", title: "Swap used", metric: "host.memory.swap.used", unit: "bytes", detail: "Swap space currently in use", icon: Database },
  { key: "storage", title: "Storage usage", metric: "host.disk.used.percent", unit: "percent", detail: "Average trend across reported mounts; see volume details below", icon: HardDrive },
  { key: "disk-read", title: "Disk read", metric: "host.disk.read.bytes_per_second", unit: "bytes/second", detail: "Read throughput across devices", icon: ArrowDown },
  { key: "disk-write", title: "Disk write", metric: "host.disk.write.bytes_per_second", unit: "bytes/second", detail: "Write throughput across devices", icon: ArrowUp },
  { key: "disk-read-ops", title: "Disk read IOPS", metric: "host.disk.read.operations_per_second", unit: "rate", detail: "Read operations per second", icon: ArrowDown },
  { key: "disk-write-ops", title: "Disk write IOPS", metric: "host.disk.write.operations_per_second", unit: "rate", detail: "Write operations per second", icon: ArrowUp },
  { key: "network-in", title: "Network receive", metric: "host.network.receive.bytes_per_second", unit: "bytes/second", detail: "Incoming traffic across interfaces", icon: Network },
  { key: "network-out", title: "Network transmit", metric: "host.network.transmit.bytes_per_second", unit: "bytes/second", detail: "Outgoing traffic across interfaces", icon: Network },
  { key: "network-in-packets", title: "Packets received", metric: "host.network.receive.packets_per_second", unit: "rate", detail: "Incoming packets per second", icon: Network },
  { key: "network-out-packets", title: "Packets transmitted", metric: "host.network.transmit.packets_per_second", unit: "rate", detail: "Outgoing packets per second", icon: Network },
  { key: "processes", title: "Processes", metric: "host.process.count", unit: "count", detail: "Processes in the latest host sample", icon: FileText },
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
  if (unit === "rate") return value.toLocaleString(undefined, { maximumFractionDigits: 1 }) + "/s";
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

function MiniChart({
  points,
  label,
  unit,
}: {
  points: SeriesPoint[];
  label: string;
  unit: MetricDefinition["unit"];
}) {
  const values = points
    .map((point) => ({
      time: new Date(point.bucket).getTime(),
      average: point.average,
      minimum: Number.isFinite(point.minimum) ? Math.min(point.minimum, point.average) : point.average,
      maximum: Number.isFinite(point.maximum) ? Math.max(point.maximum, point.average) : point.average,
      count: Math.max(0, point.sample_count),
    }))
    .filter((point) => Number.isFinite(point.time) && Number.isFinite(point.average))
    .sort((left, right) => left.time - right.time);

  if (values.length === 0) {
    return <div className="overview-chart-empty">No real samples in this time range</div>;
  }

  const left = 60;
  const right = 424;
  const top = 12;
  const bottom = 122;
  const minTime = values[0].time;
  const maxTime = values[values.length - 1].time;
  const rawMin = Math.min(...values.map((point) => point.minimum));
  const rawMax = Math.max(...values.map((point) => point.maximum));
  const rawSpread = rawMax - rawMin;
  const padding = rawSpread === 0 ? Math.max(Math.abs(rawMax) * 0.08, 1) : rawSpread * 0.1;
  const domainMin = Math.max(0, rawMin - padding);
  const domainMax = rawMax + padding || 1;
  const spread = Math.max(domainMax - domainMin, 1);
  const xAt = (time: number) => maxTime === minTime
    ? (left + right) / 2
    : left + ((time - minTime) / (maxTime - minTime)) * (right - left);
  const yAt = (value: number) => bottom - ((value - domainMin) / spread) * (bottom - top);
  const averages = values.map((point) => xAt(point.time).toFixed(1) + "," + yAt(point.average).toFixed(1));
  const upper = values.map((point) => xAt(point.time).toFixed(1) + "," + yAt(point.maximum).toFixed(1));
  const lower = [...values].reverse().map((point) => xAt(point.time).toFixed(1) + "," + yAt(point.minimum).toFixed(1));
  const area = upper.concat(lower).join(" ");
  const weightedCount = values.reduce((sum, point) => sum + point.count, 0);
  const weightedAverage = weightedCount > 0
    ? values.reduce((sum, point) => sum + point.average * point.count, 0) / weightedCount
    : values.reduce((sum, point) => sum + point.average, 0) / values.length;
  const minimum = Math.min(...values.map((point) => point.minimum));
  const maximum = Math.max(...values.map((point) => point.maximum));
  const last = values[values.length - 1];
  const tickValues = [0, 1, 2, 3].map((index) => domainMax - (spread * index) / 3);
  const timeLabels = [
    values[0],
    values[Math.floor((values.length - 1) / 2)],
    last,
  ];
  const formatTime = (time: number) => new Date(time).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });

  return (
    <div className="overview-chart">
      <svg viewBox="0 0 440 164" role="img" aria-label={label + " average and range for the last hour"}>
        {tickValues.map((tick, index) => {
          const y = yAt(tick);
          return (
            <g key={index}>
              <line x1={left} y1={y} x2={right} y2={y} className="overview-chart-gridline" />
              <text x={left - 8} y={y + 3} textAnchor="end" className="overview-chart-axis-label">{formatMetric(tick, unit)}</text>
            </g>
          );
        })}
        <polygon points={area} className="overview-chart-range-band" />
        {values.length > 1 && (
          <polygon
            points={averages.join(" ") + " " + right + "," + bottom + " " + left + "," + bottom}
            className="overview-chart-area"
          />
        )}
        {values.length > 1 && <polyline points={averages.join(" ")} className="overview-chart-line" />}
        {values.map((point, index) => (
          <circle
            key={index}
            cx={xAt(point.time)}
            cy={yAt(point.average)}
            r={index === values.length - 1 ? 3.5 : 1.8}
            className={index === values.length - 1 ? "overview-chart-dot overview-chart-dot-current" : "overview-chart-dot"}
          >
            <title>
              {new Date(point.time).toLocaleString()} · average {formatMetric(point.average, unit)} · min {formatMetric(point.minimum, unit)} · max {formatMetric(point.maximum, unit)} · {point.count} samples
            </title>
          </circle>
        ))}
        <line x1={left} y1={bottom} x2={right} y2={bottom} className="overview-chart-axis-line" />
        {timeLabels.map((point, index) => (
          <text
            key={index}
            x={xAt(point.time)}
            y={bottom + 19}
            textAnchor={index === 0 ? "start" : index === 2 ? "end" : "middle"}
            className="overview-chart-axis-label"
          >
            {formatTime(point.time)}
          </text>
        ))}
      </svg>
      <div className="overview-chart-stats">
        <span>Min <strong>{formatMetric(minimum, unit)}</strong></span>
        <span>Avg <strong>{formatMetric(weightedAverage, unit)}</strong></span>
        <span>Max <strong>{formatMetric(maximum, unit)}</strong></span>
      </div>
      <div className="overview-chart-range">
        <span>{weightedCount.toLocaleString()} real samples</span>
        <span>{values.length.toLocaleString()} time buckets · 1 hour</span>
      </div>
    </div>
  );
}

function MetricCard({
  definition,
  result,
  latest,
  latestValue,
  loading,
}: {
  definition: MetricDefinition;
  result?: PanelResult;
  latest?: LatestSample;
  latestValue?: number;
  loading: boolean;
}) {
  const points = result?.points ?? [];
  const mostRecentPoint = points[points.length - 1];
  const value = latestValue ?? latest?.value ?? mostRecentPoint?.average;
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
      ) : points.length === 0 ? (
        <div className="overview-chart-empty">No real samples in this time range</div>
      ) : (
        <>
          <strong className="overview-metric-value">{formatMetric(value, definition.unit)}</strong>
          <MiniChart points={points} label={definition.title} unit={definition.unit} />
        </>
      )}
    </article>
  );
}

function buildStorageVolumes(samples: LatestSample[]): StorageVolume[] {
  const volumes = new Map<string, StorageVolume>();
  for (const sample of samples) {
    if (!sample.metric_name.startsWith("host.disk.")) continue;
    const labels = sample.labels ?? {};
    const mountpoint = labels.mountpoint;
    if (!mountpoint) continue;
    const device = labels.device ?? "Device not reported";
    const filesystem = labels.fstype ?? "Filesystem not reported";
    const key = [mountpoint, device, filesystem].join("::");
    const volume = volumes.get(key) ?? { key, mountpoint, device, filesystem };
    if (sample.metric_name === "host.disk.total.bytes") volume.totalBytes = sample.value;
    if (sample.metric_name === "host.disk.used.bytes") volume.usedBytes = sample.value;
    if (sample.metric_name === "host.disk.available.bytes") volume.availableBytes = sample.value;
    if (sample.metric_name === "host.disk.used.percent") volume.usedPercent = sample.value;
    if (!volume.observedAt || new Date(sample.observed_at).getTime() > new Date(volume.observedAt).getTime()) {
      volume.observedAt = sample.observed_at;
    }
    volumes.set(key, volume);
  }
  return [...volumes.values()]
    .map((volume) => ({
      ...volume,
      usedPercent: volume.usedPercent ??
        (volume.totalBytes && volume.usedBytes !== undefined ? (volume.usedBytes / volume.totalBytes) * 100 : undefined),
    }))
    .sort((left, right) => (right.usedPercent ?? -1) - (left.usedPercent ?? -1));
}

function StorageVolumesPanel({
  volumes,
  history,
  loading,
}: {
  volumes: StorageVolume[];
  history: Record<string, PanelResult>;
  loading: boolean;
}) {
  return (
    <section className="overview-storage-section" aria-label="Mounted storage details">
      <header className="overview-section-heading">
        <div>
          <h2>Mounted storage volumes</h2>
          <p>Actual capacity by mountpoint, device and filesystem. Capacity is sampled approximately once per minute.</p>
        </div>
        <span className="overview-storage-count">{volumes.length} reported mounts</span>
      </header>
      {volumes.length === 0 ? (
        <div className="overview-storage-empty">
          {loading ? "Loading real filesystem capacity samples…" : "No readable mounted filesystem samples were returned for this host."}
        </div>
      ) : (
        <div className="overview-storage-grid">
          {volumes.map((volume) => {
            const percent = volume.usedPercent;
            const width = percent === undefined ? 0 : Math.max(0, Math.min(100, percent));
            const volumeHistory = history[volume.mountpoint];
            return (
              <article className="overview-storage-card" key={volume.key}>
                <div className="overview-storage-card-heading">
                  <div>
                    <h3>{volume.mountpoint}</h3>
                    <p>{volume.device} · {volume.filesystem}</p>
                  </div>
                  <strong>{percent === undefined ? "Unavailable" : percent.toFixed(1) + "%"}</strong>
                </div>
                <div className="overview-storage-track" aria-label={percent === undefined ? "Usage unavailable" : "Used " + percent.toFixed(1) + " percent"}>
                  {percent !== undefined && <span style={{ width: width + "%" }} />}
                </div>
                <div className="overview-storage-capacity">
                  <div><span>Used</span><strong>{formatMetric(volume.usedBytes, "bytes")}</strong></div>
                  <div><span>Available</span><strong>{formatMetric(volume.availableBytes, "bytes")}</strong></div>
                  <div><span>Total</span><strong>{formatMetric(volume.totalBytes, "bytes")}</strong></div>
                </div>
                {volumeHistory && (
                  <div className="overview-storage-history">
                    <span>Usage history · last hour</span>
                    {volumeHistory.error
                      ? <small>{volumeHistory.error}</small>
                      : <MiniChart points={volumeHistory.points} label={volume.mountpoint + " storage usage"} unit="percent" />}
                  </div>
                )}
                <footer>Last capacity sample: {formatWhen(volume.observedAt)}</footer>
              </article>
            );
          })}
        </div>
      )}
      <p className="overview-storage-note">Volume history is queried for the eight most-used mounts to keep requests bounded. All reported mounts remain visible with their latest real capacity values.</p>
    </section>
  );
}

async function fetchMetric(
  serverId: string,
  definition: MetricDefinition,
  from: string,
  to: string,
  labelFilter?: { key: string; value: string },
): Promise<PanelResult> {
  const query = new URLSearchParams({
    metric: definition.metric,
    from,
    to,
    step: "1m",
    limit: "150",
  });
  if (labelFilter) {
    query.set("label_key", labelFilter.key);
    query.set("label_value", labelFilter.value);
  }
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
  const orderedServers = useMemo(
    () => [...servers].sort((left, right) => {
      const leftOnline = left.status.toLowerCase() === "online" ? 1 : 0;
      const rightOnline = right.status.toLowerCase() === "online" ? 1 : 0;
      return rightOnline - leftOnline || left.name.localeCompare(right.name);
    }),
    [servers],
  );

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

    const volumeList = buildStorageVolumes(latestResult.value);
    const storageHistoryEntries = await Promise.all(volumeList.slice(0, 8).map(async (volume) => {
      try {
        return [volume.mountpoint, await fetchMetric(
          selectedServerId,
          metricPanels.find((definition) => definition.key === "storage")!,
          from,
          to,
          { key: "mountpoint", value: volume.mountpoint },
        )] as const;
      } catch (loadError) {
        return [volume.mountpoint, { points: [], error: loadError instanceof Error ? loadError.message : "Volume history could not be loaded." }] as const;
      }
    }));

    if (requestId !== requestSequence.current) return;
    setData({
      latest: latestResult.value,
      panels: Object.fromEntries(panelEntries),
      storageHistory: Object.fromEntries(storageHistoryEntries),
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
    const storageVolumes = buildStorageVolumes(data?.latest ?? []);
    const coreSamples = data?.latest.filter((sample) => sample.metric_name === "host.cpu.core.utilization") ?? [];
    const coreAverage = coreSamples.length > 0
      ? coreSamples.reduce((sum, sample) => sum + sample.value, 0) / coreSamples.length
      : undefined;
    const volumesWithPercent = storageVolumes.filter((volume) => volume.usedPercent !== undefined);
    const storageAverage = volumesWithPercent.length > 0
      ? volumesWithPercent.reduce((sum, volume) => sum + (volume.usedPercent ?? 0), 0) / volumesWithPercent.length
      : undefined;
    const logs = data?.logs.slice(0, 20) ?? [];
    return (
      <section className="overview-workspace">
        <header className="overview-page-header">
          <div>
            <button className="overview-back-button" type="button" onClick={backToServers}><ArrowLeft size={14} /> All servers</button>
            <div className="eyebrow">MONITORING / HOST DETAIL</div>
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
            <span>Storage volumes</span><strong>{data ? storageVolumes.length.toLocaleString() : "Loading"}</strong><small>Mounted filesystems with real capacity samples</small>
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
              latestValue={definition.key === "cores" ? coreAverage : definition.key === "storage" ? storageAverage : undefined}
              loading={loading}
            />
          ))}
          <article className="overview-metric-card overview-logs-card">
            <div className="overview-metric-heading">
              <span className="overview-metric-icon"><FileText size={15} /></span>
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
            <div className="overview-metric-footer"><span>{logs.length} events shown</span><span>Auto-refresh · 15s</span></div>
          </article>
        </section>
        <StorageVolumesPanel volumes={storageVolumes} history={data?.storageHistory ?? {}} loading={loading && data === null} />
      </section>
    );
  }

  return (
    <section className="overview-workspace">
      <header className="overview-page-header">
        <div>
          <div className="eyebrow">MONITORING / FLEET OVERVIEW</div>
          <h1>Monitoring</h1>
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
          {orderedServers.map((server) => (
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
              <span className="overview-server-open">Open machine details <span aria-hidden="true">→</span></span>
            </button>
          ))}
        </section>
      )}
    </section>
  );
}
