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
  organization_id: string;
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
  { name: "host.memory.cache", label: "Memory cache", unit: "bytes" },
  { name: "host.memory.swap.used", label: "Swap used", unit: "bytes" },
  { name: "host.disk.read.bytes_per_second", label: "Disk read throughput", unit: "bytes/second" },
  { name: "host.disk.write.bytes_per_second", label: "Disk write throughput", unit: "bytes/second" },
  { name: "host.network.receive.bytes_per_second", label: "Network receive throughput", unit: "bytes/second" },
  { name: "host.network.transmit.bytes_per_second", label: "Network transmit throughput", unit: "bytes/second" },
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
    let current: string[] = [];
    let previousTime: number | null = null;
    for (const point of values) {
      if (previousTime !== null && point.time - previousTime > config.stepMs * 2.2) {
        if (current.length > 0) segments.push(current.join(" "));
        current = [];
      }
      const x = Math.min(890, Math.max(10, ((point.time - start) / config.durationMs) * 890 + 10));
      const y = 224 - ((point.average - min) / verticalRange) * 190;
      current.push(`${x.toFixed(1)},${y.toFixed(1)}`);
      previousTime = point.time;
    }
    if (current.length > 0) segments.push(current.join(" "));

    const latest = values[values.length - 1];
    const average = values.reduce((sum, point) => sum + point.average, 0) / values.length;
    return {
      segments,
      min: rawMin,
      max: rawMax,
      average,
      latest,
      count: values.reduce((sum, point) => sum + point.sample_count, 0),
      values,
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
        <div><span>Latest average</span><strong>{formatValue(chart.latest.average, unit)}</strong></div>
        <div><span>Range average</span><strong>{formatValue(chart.average, unit)}</strong></div>
        <div><span>Minimum</span><strong>{formatValue(chart.min, unit)}</strong></div>
        <div><span>Maximum</span><strong>{formatValue(chart.max, unit)}</strong></div>
      </div>
      <svg className="metric-chart" viewBox="0 0 920 260" role="img" aria-label={`Historical ${unit} metric over the selected time range`}>
        <title>Historical metric values. Empty periods are left as gaps.</title>
        {[34, 81, 128, 175, 224].map((y) => (
          <line key={y} x1="10" x2="900" y1={y} y2={y} className="chart-grid-line" />
        ))}
        {chart.segments.map((segment, index) => (
          <polyline key={index} points={segment} className="chart-series-line" />
        ))}
      </svg>
      <div className="chart-axis-row">
        <span>{formatTimestamp(startTime)}</span>
        <span>{chart.count.toLocaleString()} stored samples · gaps remain visible</span>
        <span>Now</span>
      </div>
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
  const [livePoints, setLivePoints] = useState<MetricPoint[]>([]);
  const [loading, setLoading] = useState(false);
  const [historyError, setHistoryError] = useState("");
  const [liveState, setLiveState] = useState<"disabled" | "connecting" | "live" | "reconnecting">("connecting");
  const [liveMessage, setLiveMessage] = useState("");

  const selectedMetric = metricDefinitions.find((item) => item.name === metric) ?? metricDefinitions[0];
  const rangeConfig = ranges[range];

  const loadHistory = useCallback(async () => {
    setLoading(true);
    setHistoryError("");
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
    } catch (error) {
      setHistoryError(error instanceof Error ? error.message : "Historical metric data could not be loaded.");
    } finally {
      setLoading(false);
    }
  }, [metric, rangeConfig.durationMs, rangeConfig.step, server.id]);

  useEffect(() => {
    void loadHistory();
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
            const observed = event.samples
              .filter((sample) => sample.metric_name === metric && Number.isFinite(sample.value))
              .map((sample) => {
                const time = Number(sample.observed_at_unix_nanos) / 1_000_000;
                return {
                  bucket: new Date(time).toISOString(),
                  average: sample.value,
                  minimum: sample.value,
                  maximum: sample.value,
                  sample_count: 1,
                } satisfies MetricPoint;
              });
            if (observed.length === 0) return;
            setLivePoints((previous) => {
              const cutoff = Date.now() - rangeConfig.durationMs;
              return [...previous, ...observed]
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

  const points = useMemo(() => {
    const byBucket = new Map<string, MetricPoint>();
    for (const point of series?.points ?? []) byBucket.set(point.bucket, point);
    for (const point of livePoints) byBucket.set(point.bucket, point);
    return [...byBucket.values()].sort(
      (left, right) => new Date(left.bucket).getTime() - new Date(right.bucket).getTime(),
    );
  }, [livePoints, series]);

  const isStale = ["stale", "offline"].includes(server.status.toLowerCase());
  const effectiveLiveState = apiPublicURL ? liveState : "disabled";
  const effectiveLiveMessage = apiPublicURL
    ? liveMessage
    : "Live updates are not configured. Historical data remains available.";

  return (
    <section className="server-detail">
      <header className="detail-topbar">
        <div>
          <button className="detail-back-button" type="button" onClick={onBack}>← Back to servers</button>
          <div className="eyebrow">Server detail</div>
          <h2>{server.name}</h2>
          <p>{server.hostname || "Hostname not provided"} · Added {new Date(server.created_at).toLocaleString()}</p>
        </div>
        <div className="detail-status">
          <span className={`status-pill ${server.status.toLowerCase() === "online" && !isStale ? "status-pill-online" : "status-pill-pending"}`}>
            {isStale ? "Stale / no recent heartbeat" : server.status === "online" ? "Connected" : server.status}
          </span>
          <span>Last heartbeat: {server.agent_last_seen_at ? new Date(server.agent_last_seen_at).toLocaleString() : "Not received"}</span>
        </div>
      </header>

      <div className="detail-toolbar">
        <label className="metric-select-field">
          <span>Metric</span>
          <select value={metric} onChange={(event) => { setLivePoints([]); setMetric(event.target.value); }}>
            {metricDefinitions.map((definition) => (
              <option key={definition.name} value={definition.name}>{definition.label}</option>
            ))}
          </select>
        </label>
        <div className="range-switch" role="group" aria-label="Historical time range">
          {(["1h", "6h", "24h"] as const).map((value) => (
            <button
              key={value}
              type="button"
              className={range === value ? "range-button range-button-active" : "range-button"}
              onClick={() => { setLivePoints([]); setRange(value); }}
            >
              {value}
            </button>
          ))}
        </div>
        <button className="button button-secondary" type="button" onClick={() => void loadHistory()} disabled={loading}>
          {loading ? "Refreshing…" : "Refresh history"}
        </button>
      </div>

      <div className="live-status-strip">
        <span className={`status-dot ${effectiveLiveState === "live" ? "status-dot-online" : effectiveLiveState === "disabled" ? "status-dot-disabled" : "status-dot-pending"}`} />
        <strong>{effectiveLiveState === "live" ? "Live" : effectiveLiveState === "disabled" ? "History only" : effectiveLiveState === "connecting" ? "Connecting" : "Reconnecting"}</strong>
        <span>{effectiveLiveMessage}</span>
      </div>

      {historyError && <div className="notice notice-error" role="alert">{historyError}</div>}

      <article className="metric-panel">
        <div className="metric-panel-heading">
          <div>
            <h3>{selectedMetric.label}</h3>
            <p>{selectedMetric.name} · {selectedMetric.unit} · {ranges[range].label}</p>
          </div>
          <span className="metric-sample-count">{points.reduce((sum, point) => sum + point.sample_count, 0).toLocaleString()} points</span>
        </div>
        <ServerMetricChart points={points} range={range} unit={selectedMetric.unit} loading={loading} startTime={series?.from ?? null} />
      </article>

      <div className="detail-footnote">
        Historical values use bounded PostgreSQL aggregation. Live values are published only after the backend confirms the durable batch commit. Gaps represent missing samples and are not zero-filled.
      </div>
    </section>
  );
}

export default ServerDetail;
