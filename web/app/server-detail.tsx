"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { apiRequest } from "@/lib/client-api";

type Server = {
  id: string;
  name: string;
  hostname?: string;
  status: string;
  agent_last_seen_at?: string | null;
};

type SeriesPoint = {
  bucket: string;
  average: number;
  minimum: number;
  maximum: number;
  sample_count: number;
};

type SeriesResponse = {
  server_id: string;
  metric: string;
  from: string;
  to: string;
  step: string;
  points: SeriesPoint[];
};

type LiveTicketResponse = { ticket: string; expires_at: string };
type LiveSample = {
  metric_name: string;
  value: number;
  unit: string;
  labels?: Record<string, string>;
  observed_at_unix_nanos: number;
};
type LiveEvent = { type: string; server_id: string; samples: LiveSample[] };

type MetricDefinition = {
  key: string;
  label: string;
  description: string;
  unit: "percent" | "bytes" | "bytes/second" | "count" | "seconds";
};

const METRICS: MetricDefinition[] = [
  { key: "host.cpu.utilization", label: "CPU utilization", description: "Host-wide busy CPU time", unit: "percent" },
  { key: "host.memory.used", label: "Memory used", description: "Total memory minus OS-reported available memory", unit: "bytes" },
  { key: "host.memory.swap.used", label: "Swap used", description: "Swap pages currently in use", unit: "bytes" },
  { key: "host.disk.read.bytes_per_second", label: "Disk read", description: "Read throughput across visible block devices", unit: "bytes/second" },
  { key: "host.network.receive.bytes_per_second", label: "Network receive", description: "Receive throughput across visible interfaces", unit: "bytes/second" },
  { key: "host.process.count", label: "Process count", description: "Readable processes observed by the Linux collector", unit: "count" },
  { key: "host.uptime.seconds", label: "Host uptime", description: "Elapsed time since the host booted", unit: "seconds" },
];

const RANGES = [
  { value: "15m", label: "Last 15 minutes", milliseconds: 15 * 60_000 },
  { value: "1h", label: "Last hour", milliseconds: 60 * 60_000 },
  { value: "6h", label: "Last 6 hours", milliseconds: 6 * 60 * 60_000 },
  { value: "24h", label: "Last 24 hours", milliseconds: 24 * 60 * 60_000 },
];

function formatValue(value: number, unit: MetricDefinition["unit"]): string {
  if (!Number.isFinite(value)) return "—";
  switch (unit) {
    case "percent": return `${value.toFixed(1)}%`;
    case "bytes": return `${(value / (1024 ** 3)).toFixed(2)} GiB`;
    case "bytes/second": return `${(value / (1024 ** 2)).toFixed(2)} MiB/s`;
    case "count": return Math.round(value).toLocaleString();
    case "seconds": {
      const total = Math.max(0, Math.floor(value));
      const days = Math.floor(total / 86400);
      const hours = Math.floor((total % 86400) / 3600);
      const minutes = Math.floor((total % 3600) / 60);
      return days > 0 ? `${days}d ${hours}h` : hours > 0 ? `${hours}h ${minutes}m` : `${minutes}m`;
    }
  }
}

function timeLabel(value: string): string {
  const date = new Date(value);
  return Number.isNaN(date.getTime())
    ? "Unknown"
    : new Intl.DateTimeFormat(undefined, { hour: "2-digit", minute: "2-digit", second: "2-digit" }).format(date);
}

function safeWebSocketURL(apiPublicURL: string): URL {
  const base = new URL(apiPublicURL);
  if (!["http:", "https:"].includes(base.protocol) || base.username || base.password) {
    throw new Error("The public API URL is not a valid HTTP(S) origin.");
  }
  const socketURL = new URL("/v1/live/ws", base);
  socketURL.protocol = socketURL.protocol === "https:" ? "wss:" : "ws:";
  return socketURL;
}

function MetricChart({ points, unit }: { points: SeriesPoint[]; unit: MetricDefinition["unit"] }) {
  const width = 760;
  const height = 250;
  const left = 58;
  const right = 18;
  const top = 14;
  const bottom = 35;
  const innerWidth = width - left - right;
  const innerHeight = height - top - bottom;
  const values = points.map((point) => point.average).filter(Number.isFinite);
  const maximum = Math.max(1, ...values);
  const minimum = Math.min(0, ...values);
  const range = Math.max(1, maximum - minimum);
  const coordinates = points.map((point, index) => {
    const x = left + (points.length < 2 ? innerWidth / 2 : (index / (points.length - 1)) * innerWidth);
    const y = top + ((maximum - point.average) / range) * innerHeight;
    return { x, y, point };
  });
  const line = coordinates.map((point) => `${point.x.toFixed(1)},${point.y.toFixed(1)}`).join(" ");
  const area = coordinates.length
    ? `${left},${top + innerHeight} ${line} ${left + innerWidth},${top + innerHeight}`
    : "";

  return (
    <div className="metric-chart-wrap">
      <svg className="metric-chart" viewBox={`0 0 ${width} ${height}`} role="img" aria-label="Historical metric line chart">
        {[0, 1, 2, 3].map((step) => {
          const y = top + (innerHeight / 3) * step;
          const value = maximum - (range / 3) * step;
          return (
            <g key={step}>
              <line x1={left} y1={y} x2={width - right} y2={y} className="chart-grid-line" />
              <text x={left - 9} y={y + 4} textAnchor="end" className="chart-axis-label">
                {unit === "percent" ? `${value.toFixed(0)}%` : formatValue(value, unit)}
              </text>
            </g>
          );
        })}
        {coordinates.length > 1 && <polygon points={area} className="chart-area" />}
        {coordinates.length > 1 && <polyline points={line} className="chart-line" />}
        {coordinates.length === 1 && <circle cx={coordinates[0].x} cy={coordinates[0].y} r="5" className="chart-point" />}
        {coordinates.length > 0 && (
          <>
            <text x={left} y={height - 8} className="chart-axis-label">{timeLabel(points[0].bucket)}</text>
            <text x={width - right} y={height - 8} textAnchor="end" className="chart-axis-label">{timeLabel(points[points.length - 1].bucket)}</text>
          </>
        )}
      </svg>
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
  const [metricKey, setMetricKey] = useState(METRICS[0].key);
  const [rangeKey, setRangeKey] = useState(RANGES[1].value);
  const [series, setSeries] = useState<SeriesResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [liveStatus, setLiveStatus] = useState("Connecting");
  const [liveValue, setLiveValue] = useState<{ value: number; observedAt: string } | null>(null);

  const metric = useMemo(() => METRICS.find((candidate) => candidate.key === metricKey) ?? METRICS[0], [metricKey]);
  const selectedRange = useMemo(() => RANGES.find((candidate) => candidate.value === rangeKey) ?? RANGES[1], [rangeKey]);

  const loadHistory = useCallback(async () => {
    setLoading(true);
    setError("");
    const to = new Date();
    const from = new Date(to.getTime() - selectedRange.milliseconds);
    const stepSeconds = Math.max(1, Math.ceil(selectedRange.milliseconds / 500 / 1000));
    const query = new URLSearchParams({
      metric: metric.key,
      from: from.toISOString(),
      to: to.toISOString(),
      step: `${stepSeconds}s`,
      limit: "500",
    });
    try {
      const result = await apiRequest<SeriesResponse>(
        `/api/servers/${encodeURIComponent(server.id)}/metrics?${query.toString()}`,
      );
      setSeries({ ...result, points: Array.isArray(result.points) ? result.points : [] });
      setLiveValue(null);
    } catch (loadError) {
      setSeries(null);
      setError(loadError instanceof Error ? loadError.message : "Historical metric data could not be loaded.");
    } finally {
      setLoading(false);
    }
  }, [metric.key, selectedRange.milliseconds, server.id]);

  useEffect(() => {
    void loadHistory();
  }, [loadHistory]);

  useEffect(() => {
    if (!apiPublicURL) {
      setLiveStatus("Live endpoint not configured");
      return;
    }
    let stopped = false;
    let socket: WebSocket | null = null;
    let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
    let attempts = 0;

    const scheduleReconnect = () => {
      if (stopped || reconnectTimer) return;
      attempts += 1;
      const delay = Math.min(30_000, 1_000 * (2 ** Math.min(attempts - 1, 5)));
      setLiveStatus(`Reconnecting in ${Math.ceil(delay / 1000)}s`);
      reconnectTimer = setTimeout(() => {
        reconnectTimer = null;
        void connect();
      }, delay);
    };

    const connect = async () => {
      try {
        setLiveStatus("Refreshing live ticket");
        const ticket = await apiRequest<LiveTicketResponse>("/api/live/ticket", {
          method: "POST",
          body: "{}",
        });
        if (stopped) return;
        const url = safeWebSocketURL(apiPublicURL);
        url.searchParams.set("ticket", ticket.ticket);
        socket = new WebSocket(url);
        socket.onopen = () => {
          attempts = 0;
          setLiveStatus("Live");
          void loadHistory();
        };
        socket.onmessage = (message) => {
          try {
            const event = JSON.parse(String(message.data)) as LiveEvent;
            if (event.type !== "telemetry.batch" || event.server_id !== server.id || !Array.isArray(event.samples)) return;
            const samples = event.samples.filter((sample) => sample.metric_name === metric.key && Number.isFinite(sample.value));
            if (samples.length === 0) return;
            const value = samples.reduce((total, sample) => total + sample.value, 0) / samples.length;
            const observedAtNanos = Math.max(...samples.map((sample) => sample.observed_at_unix_nanos));
            const observedAt = new Date(observedAtNanos / 1_000_000);
            if (Number.isNaN(observedAt.getTime())) return;
            setLiveValue({ value, observedAt: observedAt.toISOString() });
            setSeries((current) => {
              if (!current) return current;
              const stepMs = Math.max(1_000, Number.parseFloat(current.step) * 1000 || 1_000);
              const bucketTime = Math.floor(observedAt.getTime() / stepMs) * stepMs;
              const bucket = new Date(bucketTime).toISOString();
              const point: SeriesPoint = {
                bucket,
                average: value,
                minimum: value,
                maximum: value,
                sample_count: samples.length,
              };
              const points = current.points.filter((candidate) => candidate.bucket !== bucket);
              points.push(point);
              points.sort((left, right) => new Date(left.bucket).getTime() - new Date(right.bucket).getTime());
              return { ...current, points: points.slice(-500) };
            });
          } catch {
            // Ignore malformed Pub/Sub events; durable history remains authoritative.
          }
        };
        socket.onerror = () => socket?.close();
        socket.onclose = () => {
          if (!stopped) scheduleReconnect();
        };
      } catch {
        if (!stopped) scheduleReconnect();
      }
    };

    void connect();
    return () => {
      stopped = true;
      if (reconnectTimer) clearTimeout(reconnectTimer);
      socket?.close(1000, "Server detail closed");
    };
  }, [apiPublicURL, loadHistory, metric.key, server.id]);

  const currentPoint = series?.points[series.points.length - 1];
  const currentValue = liveValue?.value ?? currentPoint?.average ?? null;
  const currentTimestamp = liveValue?.observedAt ?? currentPoint?.bucket ?? null;

  return (
    <section className="server-detail">
      <header className="server-detail-header">
        <div>
          <button className="detail-back-button" type="button" onClick={onBack}>← Back to servers</button>
          <div className="eyebrow">Server monitoring</div>
          <h1>{server.name}</h1>
          <p className="detail-subtitle">{server.hostname || "Hostname not provided"} · Last heartbeat {server.agent_last_seen_at ? new Date(server.agent_last_seen_at).toLocaleString() : "not yet received"}</p>
        </div>
        <div className="detail-live-state"><span className={`status-dot ${liveStatus === "Live" ? "status-dot-online" : "status-dot-pending"}`} />{liveStatus}</div>
      </header>

      <div className="detail-summary-grid">
        <article className="summary-card"><span>Server status</span><strong className="detail-status-value">{server.status.replaceAll("_", " ")}</strong><small>Backend-reported state</small></article>
        <article className="summary-card"><span>{metric.label}</span><strong>{currentValue === null ? "—" : formatValue(currentValue, metric.unit)}</strong><small>{currentTimestamp ? `Observed ${new Date(currentTimestamp).toLocaleTimeString()}` : "No samples in selected window"}</small></article>
        <article className="summary-card"><span>Samples in range</span><strong>{series?.points.length.toLocaleString() ?? "—"}</strong><small>Bounded PostgreSQL aggregation</small></article>
      </div>

      <section className="inventory-panel detail-chart-panel">
        <div className="panel-heading detail-controls">
          <div><h2>{metric.label}</h2><p>{metric.description}. Historical values are bucket averages; live values are latest observations.</p></div>
          <div className="detail-selectors">
            <label className="detail-select-label">Metric
              <select value={metricKey} onChange={(event) => setMetricKey(event.target.value)}>
                {METRICS.map((item) => <option key={item.key} value={item.key}>{item.label}</option>)}
              </select>
            </label>
            <label className="detail-select-label">Time range
              <select value={rangeKey} onChange={(event) => setRangeKey(event.target.value)}>
                {RANGES.map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}
              </select>
            </label>
            <button className="button button-secondary" type="button" onClick={() => void loadHistory()} disabled={loading}>{loading ? "Loading…" : "Refresh history"}</button>
          </div>
        </div>
        {error && <div className="notice notice-error" role="alert">{error}</div>}
        {loading && !series ? (
          <div className="detail-chart-message">Loading metric history…</div>
        ) : series && series.points.length > 0 ? (
          <>
            <MetricChart points={series.points} unit={metric.unit} />
            <div className="chart-legend"><span>Average per bucket</span><span>{series.points.length.toLocaleString()} points · {series.step} step</span></div>
          </>
        ) : (
          <div className="detail-chart-message"><strong>No historical samples in this time range.</strong><span>Wait for the agent to send telemetry, or select a wider range. Missing data is not shown as zero.</span></div>
        )}
      </section>
      <p className="detail-data-note">Historical queries use the server's organization scope, a 100ms minimum aggregation step, a 30-day maximum range and a 5,000-point backend limit. The live stream is ephemeral; reconnects obtain a fresh single-use ticket and refresh this durable history.</p>
    </section>
  );
}
