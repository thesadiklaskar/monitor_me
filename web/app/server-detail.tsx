"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { apiRequest } from "@/lib/client-api";

type Server = {
  id: string;
  name: string;
  hostname?: string;
  status: string;
  agent_id?: string | null;
  agent_last_seen_at?: string | null;
  created_at: string;
};

type MetricPoint = {
  bucket: string;
  average: number;
  minimum: number;
  maximum: number;
  sample_count: number;
};

type LatestSample = {
  metric_name: string;
  value: number;
  unit: string;
  labels?: Record<string, string>;
  observed_at: string;
};

type LatestResponse = {
  server_id: string;
  window_seconds: number;
  samples: LatestSample[];
};

type MetricSeries = {
  server_id: string;
  metric: string;
  from: string;
  to: string;
  step: string;
  points: MetricPoint[];
};

type LiveSample = {
  metric_name: string;
  value: number;
  unit: string;
  labels?: Record<string, string>;
  observed_at_unix_nanos: number;
};

type LiveEvent = {
  type: string;
  server_id: string;
  batch_id: string;
  samples: LiveSample[];
};

type LiveTicket = {
  ticket: string;
  expires_at: string;
  expires_in_seconds: number;
  single_use: boolean;
};

type TimeRange = "1h" | "6h" | "24h";

type MetricDefinition = {
  name: string;
  label: string;
  unit: string;
};

const metricDefinitions: MetricDefinition[] = [
  { name: "host.cpu.utilization", label: "CPU utilization", unit: "percent" },
  { name: "host.memory.used", label: "Memory used", unit: "bytes" },
  { name: "host.memory.available", label: "Memory available", unit: "bytes" },
  { name: "host.memory.total", label: "Memory total", unit: "bytes" },
  { name: "host.memory.cache", label: "Memory cache", unit: "bytes" },
  { name: "host.memory.swap.used", label: "Swap used", unit: "bytes" },
  { name: "host.disk.used.bytes", label: "Disk space used", unit: "bytes" },
  { name: "host.disk.available.bytes", label: "Disk space available", unit: "bytes" },
  { name: "host.disk.total.bytes", label: "Disk capacity", unit: "bytes" },
  { name: "host.disk.used.percent", label: "Disk usage", unit: "percent" },
  { name: "host.disk.read.bytes_per_second", label: "Disk read throughput", unit: "bytes/second" },
  { name: "host.disk.write.bytes_per_second", label: "Disk write throughput", unit: "bytes/second" },
  { name: "host.network.receive.bytes_per_second", label: "Network receive throughput", unit: "bytes/second" },
  { name: "host.network.transmit.bytes_per_second", label: "Network transmit throughput", unit: "bytes/second" },
  { name: "host.network.receive.packets_per_second", label: "Network receive packets", unit: "packets/second" },
  { name: "host.network.transmit.packets_per_second", label: "Network transmit packets", unit: "packets/second" },
  { name: "agent.cpu.utilization", label: "Agent CPU utilization", unit: "percent" },
];

const ranges: Record<TimeRange, { label: string; durationMs: number; step: string; stepMs: number }> = {
  "1h": { label: "Last hour", durationMs: 60 * 60 * 1000, step: "15s", stepMs: 15_000 },
  "6h": { label: "Last 6 hours", durationMs: 6 * 60 * 60 * 1000, step: "30s", stepMs: 30_000 },
  "24h": { label: "Last 24 hours", durationMs: 24 * 60 * 60 * 1000, step: "2m", stepMs: 120_000 },
};

function formatTimestamp(value?: string | null): string {
  if (!value) return "Not available";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "Not available";
  return new Intl.DateTimeFormat(undefined, { dateStyle: "medium", timeStyle: "medium" }).format(date);
}

function formatBytes(value: number): string {
  if (!Number.isFinite(value)) return "Unavailable";
  const units = ["B", "KiB", "MiB", "GiB", "TiB"];
  let scaled = Math.abs(value);
  let index = 0;
  while (scaled >= 1024 && index < units.length - 1) {
    scaled /= 1024;
    index += 1;
  }
  const signed = value < 0 ? -scaled : scaled;
  return `${signed.toFixed(index === 0 ? 0 : 2)} ${units[index]}`;
}

function formatValue(value: number, unit: string): string {
  if (!Number.isFinite(value)) return "Unavailable";
  if (unit === "percent") return `${value.toFixed(1)}%`;
  if (unit === "bytes") return formatBytes(value);
  if (unit === "bytes/second") return `${formatBytes(value)}/s`;
  return `${value.toFixed(2)} ${unit}`;
}

function formatUptime(value: number): string {
  if (!Number.isFinite(value) || value < 0) return "Unavailable";
  const seconds = Math.floor(value);
  const days = Math.floor(seconds / 86_400);
  const hours = Math.floor((seconds % 86_400) / 3_600);
  const minutes = Math.floor((seconds % 3_600) / 60);
  return days > 0 ? `${days}d ${hours}h` : hours > 0 ? `${hours}h ${minutes}m` : `${minutes}m`;
}

function ServerMetricChart({
  points,
  range,
  unit,
  loading,
  startTime,
}: {
  points: MetricPoint[];
  range: TimeRange;
  unit: string;
  loading: boolean;
  startTime: string | null;
}) {
  const chart = useMemo(() => {
    if (points.length === 0 || !startTime) return null;
    const config = ranges[range];
    const start = new Date(startTime).getTime();
    if (!Number.isFinite(start)) return null;
    const values = points
      .map((point) => ({ ...point, time: new Date(point.bucket).getTime() }))
      .filter((point) => Number.isFinite(point.time) && Number.isFinite(point.average) && point.time >= start)
      .sort((left, right) => left.time - right.time);
    if (values.length === 0) return null;

    const rawMin = Math.min(...values.map((point) => point.minimum));
    const rawMax = Math.max(...values.map((point) => point.maximum));
    const spread = rawMax - rawMin;
    const padding = spread > 0 ? spread * 0.12 : Math.max(Math.abs(rawMax) * 0.12, 1);
    const min = Math.min(0, rawMin - padding);
    const max = rawMax + padding;
    const verticalRange = max - min || 1;
    const segments: string[] = [];
    const areaSegments: string[] = [];
    let current: Array<{ x: number; y: number }> = [];
    let previousTime: number | null = null;

    const finishSegment = () => {
      if (current.length === 0) return;
      const coordinates = current.map((point) => point.x.toFixed(1) + "," + point.y.toFixed(1));
      segments.push(coordinates.join(" "));
      if (current.length > 1) {
        const first = current[0];
        const last = current[current.length - 1];
        areaSegments.push(coordinates.join(" ") + " " + last.x.toFixed(1) + ",214 " + first.x.toFixed(1) + ",214");
      }
      current = [];
    };

    for (const point of values) {
      if (previousTime !== null && point.time - previousTime > config.stepMs * 2.2) finishSegment();
      const x = Math.min(900, Math.max(54, ((point.time - start) / config.durationMs) * 846 + 54));
      const y = 28 + ((max - point.average) / verticalRange) * 186;
      current.push({ x, y });
      previousTime = point.time;
    }
    finishSegment();

    const latest = values[values.length - 1];
    const latestX = Math.min(900, Math.max(54, ((latest.time - start) / config.durationMs) * 846 + 54));
    const latestY = 28 + ((max - latest.average) / verticalRange) * 186;
    const average = values.reduce((sum, point) => sum + point.average, 0) / values.length;
    const ticks = Array.from({ length: 5 }, (_, index) => ({
      y: 28 + index * 46.5,
      value: max - (verticalRange * index) / 4,
    }));
    return {
      segments,
      areaSegments,
      latestX,
      latestY,
      ticks,
      min: rawMin,
      max: rawMax,
      average,
      latest,
      count: values.reduce((sum, point) => sum + point.sample_count, 0),
    };
  }, [points, range, startTime]);

  if (loading && !chart) {
    return <div className="metric-chart-state">Loading historical samples…</div>;
  }
  if (!chart) {
    return (
      <div className="metric-chart-state">
        <strong>No samples in this time range</strong>
        <span>The selected metric has no stored values for this server and period. Missing data is not shown as zero.</span>
      </div>
    );
  }

  return (
    <div className="metric-chart-wrap">
      <div className="metric-summary-row">
        <div className="metric-primary-reading">
          <span>Latest reading</span>
          <strong>{formatValue(chart.latest.average, unit)}</strong>
          <small>{formatTimestamp(chart.latest.bucket)}</small>
        </div>
        <div><span>Period average</span><strong>{formatValue(chart.average, unit)}</strong></div>
        <div><span>Minimum</span><strong>{formatValue(chart.min, unit)}</strong></div>
        <div><span>Maximum</span><strong>{formatValue(chart.max, unit)}</strong></div>
      </div>
      <div className="metric-chart-scroll">
        <svg className="metric-chart" viewBox="0 0 930 250" role="img" aria-label={`Historical ${unit} metric over the selected time range`}>
          <title>Historical metric values. Empty periods remain visible as gaps.</title>
          <defs>
            <linearGradient id="monitorMetricFill" x1="0" x2="0" y1="0" y2="1">
              <stop offset="0%" stopColor="#66e0bb" stopOpacity="0.30" />
              <stop offset="100%" stopColor="#66e0bb" stopOpacity="0.015" />
            </linearGradient>
          </defs>
          {chart.ticks.map((tick, index) => (
            <g key={index}>
              <line x1="54" x2="900" y1={tick.y} y2={tick.y} className="chart-grid-line" />
              <text x="43" y={tick.y + 4} textAnchor="end" className="chart-axis-label">{formatValue(tick.value, unit)}</text>
            </g>
          ))}
          {chart.areaSegments.map((segment, index) => <polygon key={index} points={segment} fill="url(#monitorMetricFill)" />)}
          {chart.segments.map((segment, index) => <polyline key={index} points={segment} className="chart-series-line" />)}
          <line x1={chart.latestX} x2={chart.latestX} y1="28" y2="214" className="chart-latest-guide" />
          <circle cx={chart.latestX} cy={chart.latestY} r="5.5" className="chart-latest-halo" />
          <circle cx={chart.latestX} cy={chart.latestY} r="3" className="chart-latest-dot" />
        </svg>
      </div>
      <div className="chart-axis-row">
        <span>{formatTimestamp(startTime)}</span>
        <span>{chart.count.toLocaleString()} stored samples · missing intervals remain visible</span>
        <span>{formatTimestamp(chart.latest.bucket)}</span>
      </div>
      <div className="chart-footnote">Historical values use committed telemetry. Live readings are added while this view is open.</div>
    </div>
  );
}

export function ServerDetail({
  server,
  apiPublicURL,
  onBack,
}: {
  server: Server;
  apiPublicURL: string | null;
  onBack: () => void;
}) {
  const [metric, setMetric] = useState(metricDefinitions[0].name);
  const [range, setRange] = useState<TimeRange>("1h");
  const [series, setSeries] = useState<MetricSeries | null>(null);
  const [latestSamples, setLatestSamples] = useState<LatestSample[]>([]);
  const [inventoryLoading, setInventoryLoading] = useState(true);
  const [inventoryError, setInventoryError] = useState("");
  const [livePoints, setLivePoints] = useState<MetricPoint[]>([]);
  const [loading, setLoading] = useState(false);
  const [historyError, setHistoryError] = useState("");
  const [liveState, setLiveState] = useState<"disabled" | "connecting" | "live" | "reconnecting">("connecting");
  const [liveMessage, setLiveMessage] = useState("");

  const selectedMetric = metricDefinitions.find((item) => item.name === metric) ?? metricDefinitions[0];
  const rangeConfig = ranges[range];

  const loadHistory = useCallback(async () => {
    const to = new Date();
    const from = new Date(to.getTime() - rangeConfig.durationMs);
    const query = new URLSearchParams({
      metric,
      from: from.toISOString(),
      to: to.toISOString(),
      step: rangeConfig.step,
      limit: "1000",
    });
    try {
      const response = await apiRequest<MetricSeries>(
        `/api/servers/${encodeURIComponent(server.id)}/metrics?${query.toString()}`,
      );
      setSeries(response);
      setHistoryError("");
    } catch (error) {
      setHistoryError(error instanceof Error ? error.message : "Historical metric data could not be loaded.");
    } finally {
      setLoading(false);
    }
  }, [metric, rangeConfig.durationMs, rangeConfig.step, server.id]);

  useEffect(() => {
    const timer = window.setTimeout(() => { void loadHistory(); }, 0);
    return () => window.clearTimeout(timer);
  }, [loadHistory]);

  useEffect(() => {
    if (!apiPublicURL) return;

    let stopped = false;
    let socket: WebSocket | null = null;
    let reconnectTimer: ReturnType<typeof setTimeout> | undefined;
    let attempts = 0;

    const scheduleReconnect = () => {
      if (stopped) return;
      attempts += 1;
      const delay = Math.min(30_000, 1_000 * 2 ** Math.min(attempts - 1, 5));
      reconnectTimer = setTimeout(() => { void connect(); }, delay);
    };

    const catchUpAndReconnect = async () => {
      await loadHistory();
      scheduleReconnect();
    };

    const connect = async () => {
      if (stopped) return;
      setLiveState("connecting");
      setLiveMessage("");
      try {
        // A new single-use 30-second ticket is requested for every connection.
        const ticket = await apiRequest<LiveTicket>("/api/live/ticket", {
          method: "POST",
          body: "{}",
        });
        if (stopped) return;

        const target = new URL("/v1/live/ws", apiPublicURL);
        target.protocol = target.protocol === "https:" ? "wss:" : "ws:";
        target.searchParams.set("ticket", ticket.ticket);
        socket = new WebSocket(target.toString());

        socket.onopen = () => {
          attempts = 0;
          setLiveState("live");
          setLiveMessage("Receiving committed telemetry events.");
        };
        socket.onmessage = (message) => {
          try {
            const event = JSON.parse(String(message.data)) as LiveEvent;
            if (event.type !== "telemetry.batch" || event.server_id !== server.id) return;
            // Match the history API aggregation: interface/device-labelled
            // samples in one committed batch are summarized together, not
            // displayed as if the last interface were the whole host.
            const matchingSamples = event.samples.filter(
              (sample) => sample.metric_name === metric && Number.isFinite(sample.value),
            );
            if (matchingSamples.length === 0) return;
            const values = matchingSamples.map((sample) => sample.value);
            const observedAt = Math.max(...matchingSamples.map((sample) => Number(sample.observed_at_unix_nanos)));
            const observedTime = observedAt / 1_000_000;
            if (!Number.isFinite(observedTime)) return;
            const observed: MetricPoint = {
              bucket: new Date(observedTime).toISOString(),
              average: values.reduce((sum, value) => sum + value, 0) / values.length,
              minimum: Math.min(...values),
              maximum: Math.max(...values),
              sample_count: matchingSamples.length,
            };
            setLivePoints((previous) => {
              const cutoff = Date.now() - rangeConfig.durationMs;
              return [...previous, observed]
                .filter((point) => new Date(point.bucket).getTime() >= cutoff)
                .sort((left, right) => new Date(left.bucket).getTime() - new Date(right.bucket).getTime())
                .slice(-500);
            });
          } catch {
            setLiveMessage("A live event could not be decoded. Historical data remains available.");
          }
        };
        socket.onclose = () => {
          if (stopped) return;
          setLiveState("reconnecting");
          setLiveMessage("Live connection closed. Refreshing history before reconnecting.");
          void catchUpAndReconnect();
        };
        socket.onerror = () => {
          socket?.close();
        };
      } catch (error) {
        if (stopped) return;
        setLiveState("reconnecting");
        setLiveMessage(error instanceof Error ? error.message : "Live updates are temporarily unavailable.");
        await catchUpAndReconnect();
      }
    };

    void connect();
    return () => {
      stopped = true;
      if (reconnectTimer) clearTimeout(reconnectTimer);
      socket?.close();
    };
  }, [apiPublicURL, loadHistory, metric, rangeConfig.durationMs, server.id]);


  const loadLatest = useCallback(async () => {
    setInventoryLoading(true);
    try {
      const response = await apiRequest<LatestResponse>(
        `/api/servers/${encodeURIComponent(server.id)}/metrics/latest`,
      );
      setLatestSamples(Array.isArray(response.samples) ? response.samples : []);
      setInventoryError("");
    } catch (error) {
      setInventoryError(error instanceof Error ? error.message : "Process and port inventory could not be loaded.");
    } finally {
      setInventoryLoading(false);
    }
  }, [server.id]);

  useEffect(() => {
    const timer = window.setTimeout(() => { void loadLatest(); }, 0);
    const refresh = window.setInterval(() => { void loadLatest(); }, 30_000);
    return () => { window.clearTimeout(timer); window.clearInterval(refresh); };
  }, [loadLatest]);

  const processInventory = useMemo(
    () => latestSamples
      .filter((sample) => sample.metric_name === "process.memory.rss")
      .sort((left, right) => right.value - left.value)
      .slice(0, 20),
    [latestSamples],
  );
  const tcpStates = useMemo(
    () => latestSamples.filter((sample) => sample.metric_name === "host.network.tcp.connections"),
    [latestSamples],
  );
  const listeningPorts = useMemo(
    () => latestSamples.filter((sample) => sample.metric_name === "host.network.tcp.listening_port"),
    [latestSamples],
  );
  const processCount = latestSamples.find((sample) => sample.metric_name === "host.process.count");
  const hostInfo = latestSamples.find((sample) => sample.metric_name === "host.info");
  const latestMetric = (name: string) => latestSamples.find((sample) => sample.metric_name === name);
  const cpuLatest = latestMetric("host.cpu.utilization");
  const memoryUsed = latestMetric("host.memory.used");
  const memoryTotal = latestMetric("host.memory.total");
  const memoryAvailable = latestMetric("host.memory.available");
  const uptimeLatest = latestMetric("host.uptime.seconds");
  const diskVolumes = latestSamples.filter((sample) => sample.metric_name === "host.disk.used.percent");
  const networkReceive = latestSamples.filter((sample) => sample.metric_name === "host.network.receive.bytes_per_second");
  const networkTransmit = latestSamples.filter((sample) => sample.metric_name === "host.network.transmit.bytes_per_second");

  const points = useMemo(() => {
    const byBucket = new Map<string, MetricPoint>();
    for (const point of series?.points ?? []) byBucket.set(point.bucket, point);
    for (const point of livePoints) byBucket.set(point.bucket, point);
    return [...byBucket.values()].sort(
      (left, right) => new Date(left.bucket).getTime() - new Date(right.bucket).getTime(),
    );
  }, [livePoints, series]);

  const diskPeak = diskVolumes.reduce<LatestSample | null>(
    (peak, sample) => (!peak || sample.value > peak.value ? sample : peak),
    null,
  );
  const receiveRate = networkReceive.reduce((sum, sample) => sum + sample.value, 0);
  const transmitRate = networkTransmit.reduce((sum, sample) => sum + sample.value, 0);
  const memoryUsagePercent = memoryUsed && memoryTotal && memoryTotal.value > 0
    ? Math.max(0, Math.min(100, (memoryUsed.value / memoryTotal.value) * 100))
    : null;
  const cpuPercent = cpuLatest ? Math.max(0, Math.min(100, cpuLatest.value)) : null;
  const diskPercent = diskPeak ? Math.max(0, Math.min(100, diskPeak.value)) : null;
  const platformName = hostInfo?.labels?.platform ?? hostInfo?.labels?.os ?? "Host platform";
  const isWindows = /windows/i.test(platformName);

  const isStale = ["stale", "offline"].includes(server.status.toLowerCase());
  const effectiveLiveState = apiPublicURL ? liveState : "disabled";
  const effectiveLiveMessage = apiPublicURL
    ? liveMessage
    : "Live updates are not configured. Historical data remains available.";

  return (
    <section className="server-detail monitor-detail">
      <header className="detail-topbar monitor-detail-header">
        <div className="monitor-title-block">
          <button className="detail-back-button" type="button" onClick={onBack}>← All servers</button>
          <div className="monitor-title-eyebrow"><span className="monitor-eyebrow-mark" /> HOST MONITORING</div>
          <h2>{server.name}</h2>
          <div className="monitor-host-meta">
            <span>{server.hostname || "Hostname not provided"}</span>
            <span className="monitor-meta-divider">/</span>
            <span>{platformName}</span>
            {hostInfo?.labels?.architecture && <><span className="monitor-meta-divider">/</span><span>{hostInfo.labels.architecture}</span></>}
          </div>
        </div>
        <div className="monitor-status-stack">
          <span className={server.status.toLowerCase() === "online" && !isStale ? "monitor-connection-pill is-connected" : "monitor-connection-pill is-disconnected"}>
            <span className="monitor-connection-dot" />
            {isStale ? "STALE" : server.status.toLowerCase() === "online" ? "CONNECTED" : server.status.toUpperCase()}
          </span>
          <span className="monitor-heartbeat">Last heartbeat · {formatTimestamp(server.agent_last_seen_at)}</span>
        </div>
      </header>

      <div className="live-status-strip monitor-live-strip">
        <span className={effectiveLiveState === "live" ? "monitor-live-indicator is-live" : effectiveLiveState === "disabled" ? "monitor-live-indicator is-muted" : "monitor-live-indicator is-waiting"} />
        <strong>{effectiveLiveState === "live" ? "LIVE TELEMETRY" : effectiveLiveState === "disabled" ? "HISTORY ONLY" : effectiveLiveState === "connecting" ? "CONNECTING TO LIVE STREAM" : "RECONNECTING"}</strong>
        <span>{effectiveLiveMessage}</span>
        <span className="monitor-updated-label">Live stream · committed telemetry</span>
      </div>

      {historyError && <div className="notice notice-error" role="alert">{historyError}</div>}
      {inventoryError && <div className="notice notice-error" role="alert">{inventoryError}</div>}

      <section className="resource-kpi-grid" aria-label="Current resource utilization">
        <article className="resource-kpi-card resource-kpi-cpu">
          <div className="resource-kpi-heading"><span className="resource-kpi-icon">CPU</span><span>Processor</span><span className="resource-kpi-dot" /></div>
          <div className="resource-kpi-reading">{cpuLatest ? formatValue(cpuLatest.value, "percent") : "—"}</div>
          <div className="resource-kpi-track"><span style={{ width: String(cpuPercent ?? 0) + "%" }} /></div>
          <div className="resource-kpi-footer"><span>Current utilization</span><span>{cpuLatest ? "Latest sample" : "Waiting for sample"}</span></div>
        </article>
        <article className="resource-kpi-card resource-kpi-memory">
          <div className="resource-kpi-heading"><span className="resource-kpi-icon">RAM</span><span>Memory</span><span className="resource-kpi-dot" /></div>
          <div className="resource-kpi-reading">{memoryUsed ? formatBytes(memoryUsed.value) : "—"}</div>
          <div className="resource-kpi-track"><span style={{ width: String(memoryUsagePercent ?? 0) + "%" }} /></div>
          <div className="resource-kpi-footer">
            <span>{memoryTotal ? "of " + formatBytes(memoryTotal.value) : "Total unavailable"}</span>
            <span>{memoryUsagePercent === null ? "Waiting for sample" : memoryUsagePercent.toFixed(1) + "% used"}</span>
          </div>
        </article>
        <article className="resource-kpi-card resource-kpi-disk">
          <div className="resource-kpi-heading"><span className="resource-kpi-icon">DSK</span><span>Storage</span><span className="resource-kpi-dot" /></div>
          <div className="resource-kpi-reading">{diskPeak ? formatValue(diskPeak.value, "percent") : "—"}</div>
          <div className="resource-kpi-track"><span style={{ width: String(diskPercent ?? 0) + "%" }} /></div>
          <div className="resource-kpi-footer">
            <span>{diskPeak?.labels?.mountpoint ?? diskPeak?.labels?.device ?? "Mounted volumes"}</span>
            <span>{diskVolumes.length ? diskVolumes.length + " volume(s)" : "Waiting for sample"}</span>
          </div>
        </article>
        <article className="resource-kpi-card resource-kpi-network">
          <div className="resource-kpi-heading"><span className="resource-kpi-icon">NET</span><span>Network throughput</span><span className="resource-kpi-dot" /></div>
          <div className="resource-kpi-reading">{networkReceive.length + networkTransmit.length > 0 ? formatValue(receiveRate + transmitRate, "bytes/second") : "—"}</div>
          <div className="resource-kpi-network-split">
            <span>↓ {formatValue(receiveRate, "bytes/second")}</span>
            <span>↑ {formatValue(transmitRate, "bytes/second")}</span>
          </div>
          <div className="resource-kpi-footer"><span>Combined interface traffic</span><span>{networkReceive.length + networkTransmit.length} series</span></div>
        </article>
      </section>

      <article className="metric-panel metric-panel-primary">
        <div className="metric-panel-heading monitor-chart-heading">
          <div>
            <div className="monitor-section-kicker">PERFORMANCE OVERVIEW</div>
            <h3>{selectedMetric.label}</h3>
            <p>{selectedMetric.name} <span>·</span> {selectedMetric.unit}</p>
          </div>
          <div className="monitor-chart-heading-meta">
            <span className={effectiveLiveState === "live" ? "monitor-chart-live is-live" : "monitor-chart-live"}><span />{effectiveLiveState === "live" ? "LIVE" : "HISTORY"}</span>
            <span>{points.reduce((sum, point) => sum + point.sample_count, 0).toLocaleString()} samples</span>
          </div>
        </div>
        <div className="monitor-chart-controls">
          <div className="metric-preset-row" aria-label="Quick metric selection">
            {[
              { label: "CPU", value: "host.cpu.utilization" },
              { label: "Memory", value: "host.memory.used" },
              { label: "Disk", value: "host.disk.used.percent" },
              { label: "Network", value: "host.network.receive.bytes_per_second" },
            ].map((item) => (
              <button key={item.value} type="button" className={metric === item.value ? "metric-preset metric-preset-active" : "metric-preset"} aria-pressed={metric === item.value} onClick={() => { setLivePoints([]); setLoading(true); setHistoryError(""); setMetric(item.value); }}>
                {item.label}
              </button>
            ))}
          </div>
          <div className="monitor-chart-control-right">
            <label className="metric-select-field monitor-extra-metric">
              <span>More metrics</span>
              <select value={metric} onChange={(event) => { setLivePoints([]); setLoading(true); setHistoryError(""); setMetric(event.target.value); }}>
                {metricDefinitions.map((definition) => <option key={definition.name} value={definition.name}>{definition.label}</option>)}
              </select>
            </label>
            <div className="range-switch" role="group" aria-label="Historical time range">
              {(["1h", "6h", "24h"] as const).map((value) => (
                <button key={value} type="button" className={range === value ? "range-button range-button-active" : "range-button"} onClick={() => { setLivePoints([]); setLoading(true); setHistoryError(""); setRange(value); }}>
                  {value.toUpperCase()}
                </button>
              ))}
            </div>
            <button className="button button-secondary monitor-refresh-button" type="button" onClick={() => { setLoading(true); setHistoryError(""); void loadHistory(); void loadLatest(); }} disabled={loading}>
              {loading ? "Refreshing…" : "↻ Refresh"}
            </button>
          </div>
        </div>
        <ServerMetricChart points={points} range={range} unit={selectedMetric.unit} loading={loading} startTime={series?.from ?? null} />
      </article>

      {latestSamples.length > 0 && (
        <section className="inventory-panel detail-inventory-panel">
          <div className="panel-heading">
            <div>
              <h2>Storage and network interfaces</h2>
              <p>Mounted-volume capacity and per-interface traffic from the latest host snapshot.</p>
            </div>
            <button className="button button-secondary" type="button" onClick={() => { void loadLatest(); }} disabled={inventoryLoading}>
              {inventoryLoading ? "Refreshing…" : "Refresh snapshot"}
            </button>
          </div>
          {diskVolumes.length > 0 && (
            <div>
              <h3>Mounted volumes</h3>
              <div className="table-wrap">
                <table>
                  <thead><tr><th>Volume</th><th>Used</th><th>Capacity</th><th>Available</th></tr></thead>
                  <tbody>{diskVolumes.map((sample) => {
                    const labels = sample.labels ?? {};
                    const key = labels.mountpoint ?? labels.device ?? "volume";
                    const total = latestSamples.find((item) => item.metric_name === "host.disk.total.bytes" && (item.labels?.mountpoint ?? item.labels?.device ?? "volume") === key);
                    const available = latestSamples.find((item) => item.metric_name === "host.disk.available.bytes" && (item.labels?.mountpoint ?? item.labels?.device ?? "volume") === key);
                    return (
                      <tr key={key}>
                        <td>{labels.mountpoint ?? labels.device ?? "Unknown volume"}</td>
                        <td>{formatValue(sample.value, "percent")}</td>
                        <td>{total ? formatBytes(total.value) : "Unavailable"}</td>
                        <td>{available ? formatBytes(available.value) : "Unavailable"}</td>
                      </tr>
                    );
                  })}</tbody>
                </table>
              </div>
            </div>
          )}
          {(networkReceive.length > 0 || networkTransmit.length > 0) && (
            <div>
              <h3>Network interfaces</h3>
              <div className="table-wrap">
                <table>
                  <thead><tr><th>Interface</th><th>Download</th><th>Upload</th></tr></thead>
                  <tbody>{Array.from(new Set([...networkReceive, ...networkTransmit].map((sample) => sample.labels?.interface ?? "unknown"))).map((iface) => {
                    const received = networkReceive.find((sample) => (sample.labels?.interface ?? "unknown") === iface);
                    const transmitted = networkTransmit.find((sample) => (sample.labels?.interface ?? "unknown") === iface);
                    return (
                      <tr key={iface}>
                        <td>{iface}</td>
                        <td>{received ? formatValue(received.value, "bytes/second") : "Unavailable"}</td>
                        <td>{transmitted ? formatValue(transmitted.value, "bytes/second") : "Unavailable"}</td>
                      </tr>
                    );
                  })}</tbody>
                </table>
              </div>
            </div>
          )}
        </section>
      )}

      {hostInfo && (
        <section className="inventory-panel detail-inventory-panel">
          <div className="panel-heading">
            <div>
              <h2>System information</h2>
              <p>Operating system details reported by the host agent.</p>
            </div>
          </div>
          <div className="summary-grid">
            <article className="summary-card">
              <span>Hostname</span>
              <strong>{hostInfo.labels?.hostname ?? "Unavailable"}</strong>
              <small>Machine name</small>
            </article>
            <article className="summary-card">
              <span>Operating system</span>
              <strong>{hostInfo.labels?.platform ?? hostInfo.labels?.os ?? "Unavailable"}</strong>
              <small>{hostInfo.labels?.platform_version ?? "Version unavailable"}</small>
            </article>
            <article className="summary-card">
              <span>Kernel</span>
              <strong>{hostInfo.labels?.kernel_version ?? "Unavailable"}</strong>
              <small>{hostInfo.labels?.architecture ?? "Architecture unavailable"}</small>
            </article>
            <article className="summary-card">
              <span>Environment type</span>
              <strong>
                {hostInfo.labels?.environment_type === "cloud_vm" ? "Cloud VM / VPS" :
                  hostInfo.labels?.environment_type === "virtual_machine" ? "Virtual machine" :
                  hostInfo.labels?.environment_type === "container" ? "Container" :
                  hostInfo.labels?.environment_type === "physical_or_unknown" ? "Physical / unknown" : "Not detected"}
              </strong>
              <small>
                {hostInfo.labels?.cloud_provider_hint ??
                  hostInfo.labels?.virtualization_system ??
                  (hostInfo.labels?.environment_type === "physical_or_unknown" ? "No hypervisor metadata detected" : "Provider or hypervisor not reported")}
              </small>
            </article>
          </div>
        </section>
      )}

      <section className="inventory-panel detail-inventory-panel">
        <div className="panel-heading">
          <div>
            <h2>Process and network inventory</h2>
            <p>{processCount ? `${Math.round(processCount.value).toLocaleString()} processes reported` : "Process count unavailable"} · latest snapshot from the last 15 minutes</p>
          </div>
          <button className="button button-secondary" type="button" onClick={() => { void loadLatest(); }} disabled={inventoryLoading}>
            {inventoryLoading ? "Refreshing…" : "Refresh inventory"}
          </button>
        </div>
        {inventoryError && <div className="notice notice-error" role="alert">{inventoryError}</div>}
        {inventoryLoading && latestSamples.length === 0 ? (
          <div className="monitor-empty">Loading process and TCP inventory…</div>
        ) : (
          <div className="detail-inventory-grid">
            <div>
              <h3>Top processes by resident memory</h3>
              {processInventory.length === 0 ? <p className="detail-footnote">No process RSS samples in the latest snapshot.</p> : (
                <div className="table-wrap"><table>
                  <thead><tr><th>Process</th><th>PID</th><th>RSS</th><th>Workload</th></tr></thead>
                  <tbody>{processInventory.map((sample, index) => (
                    <tr key={`${sample.labels?.pid ?? "pid"}-${sample.labels?.process ?? index}`}>
                      <td>{sample.labels?.process ?? "Unknown"}</td>
                      <td>{sample.labels?.pid ?? "—"}</td>
                      <td>{formatValue(sample.value, sample.unit)}</td>
                      <td>{sample.labels?.workload_selector ?? sample.labels?.cgroup_path ?? sample.labels?.attribution_method ?? "Host / unclassified"}</td>
                    </tr>
                  ))}</tbody>
                </table></div>
              )}
            </div>
            <div>
              <h3>TCP connection states</h3>
              {tcpStates.length === 0 ? <p className="detail-footnote">{isWindows ? "Windows TCP state collection is not included in this agent version yet." : "No TCP state samples in the latest snapshot."}</p> : (
                <div className="table-wrap"><table>
                  <thead><tr><th>Family</th><th>State</th><th>Connections</th></tr></thead>
                  <tbody>{tcpStates.map((sample, index) => (
                    <tr key={`${sample.labels?.family ?? "family"}-${sample.labels?.state ?? index}`}>
                      <td>{sample.labels?.family ?? "—"}</td><td>{sample.labels?.state ?? "Unknown"}</td><td>{Math.round(sample.value).toLocaleString()}</td>
                    </tr>
                  ))}</tbody>
                </table></div>
              )}
              <h3 className="detail-subheading">Listening ports</h3>
              {listeningPorts.length === 0 ? <p className="detail-footnote">{isWindows ? "Windows listening-port collection is not included in this agent version yet." : "No listening-port samples in the latest snapshot."}</p> : (
                <div className="table-wrap"><table>
                  <thead><tr><th>Family</th><th>Port</th><th>Sockets</th></tr></thead>
                  <tbody>{listeningPorts.slice(0, 24).map((sample, index) => (
                    <tr key={`${sample.labels?.family ?? "family"}-${sample.labels?.port ?? index}`}>
                      <td>{sample.labels?.family ?? "—"}</td><td>{sample.labels?.port ?? "—"}</td><td>{Math.round(sample.value).toLocaleString()}</td>
                    </tr>
                  ))}</tbody>
                </table></div>
              )}
            </div>
          </div>
        )}
      </section>
      <div className="detail-footnote">
        Historical values use bounded PostgreSQL aggregation. Live values are published only after the backend confirms the durable batch commit. Gaps represent missing samples and are not zero-filled.
      </div>
    </section>
  );
}

export default ServerDetail;
