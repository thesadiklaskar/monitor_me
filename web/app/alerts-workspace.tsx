"use client";

import { FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import { apiRequest } from "@/lib/client-api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

type Server = { id: string; name: string };
type Rule = { id: string; name: string; server_id?: string | null; metric_name: string; operator: "gt" | "gte" | "lt" | "lte"; threshold: number; enabled: boolean; created_at: string };
type Event = { id: string; rule_id: string; rule_name: string; server_id: string; server_name: string; metric_name: string; status: "firing" | "resolved"; last_value: number; triggered_at: string; last_observed_at: string; resolved_at?: string | null };
type Notification = { id: string; event_id: string; kind: "firing" | "recovered"; title: string; message: string; read_at?: string | null; created_at: string };

const METRICS = [
  { value: "host.cpu.utilization", label: "CPU utilization", unit: "%" },
  { value: "host.memory.used", label: "Memory used", unit: "bytes" },
  { value: "host.memory.swap.used", label: "Swap used", unit: "bytes" },
  { value: "host.disk.read.bytes_per_second", label: "Disk read", unit: "bytes/s" },
  { value: "host.network.receive.bytes_per_second", label: "Network receive", unit: "bytes/s" },
  { value: "host.process.count", label: "Process count", unit: "processes" },
  { value: "process.memory.rss", label: "Process RSS", unit: "bytes" },
];

function formatNumber(value: number, unit: string) {
  if (unit === "bytes") return (value / 1024 ** 3).toFixed(2) + " GiB";
  if (unit === "bytes/s") return (value / 1024 ** 2).toFixed(2) + " MiB/s";
  if (unit === "%") return value.toFixed(1) + "%";
  return Math.round(value).toLocaleString() + " " + unit;
}
function formatTime(value: string) {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "Unknown time" : date.toLocaleString();
}

export default function AlertsWorkspace({ servers, role, onBack }: {
  servers: Server[]; role: string; onBack: () => void;
}) {
  const [rules, setRules] = useState<Rule[]>([]);
  const [events, setEvents] = useState<Event[]>([]);
  const [notifications, setNotifications] = useState<Notification[]>([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [ruleName, setRuleName] = useState("");
  const [metric, setMetric] = useState(METRICS[0].value);
  const [operator, setOperator] = useState<Rule["operator"]>("gt");
  const [threshold, setThreshold] = useState("85");
  const [serverId, setServerId] = useState("");
  const canAdminister = role === "owner" || role === "admin";
  const unreadCount = useMemo(() => notifications.filter((item) => !item.read_at).length, [notifications]);

  const load = useCallback(async () => {
    try {
      const [ruleResponse, eventResponse, notificationResponse] = await Promise.all([
        apiRequest<{ rules: Rule[] }>("/api/alert-rules"),
        apiRequest<{ events: Event[] }>("/api/alerts"),
        apiRequest<{ notifications: Notification[] }>("/api/notifications"),
      ]);
      setRules(Array.isArray(ruleResponse.rules) ? ruleResponse.rules : []);
      setEvents(Array.isArray(eventResponse.events) ? eventResponse.events : []);
      setNotifications(Array.isArray(notificationResponse.notifications) ? notificationResponse.notifications : []);
      setError("");
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : "Alert data could not be loaded.");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    const timer = window.setTimeout(() => { void load(); }, 0);
    const refresh = window.setInterval(() => { void load(); }, 30000);
    return () => { window.clearTimeout(timer); window.clearInterval(refresh); };
  }, [load]);

  async function createRule(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const parsed = Number(threshold);
    if (!ruleName.trim() || !Number.isFinite(parsed)) { setError("Enter a rule name and finite threshold."); return; }
    setBusy(true); setError("");
    try {
      await apiRequest<{ rule: Rule }>("/api/alert-rules", {
        method: "POST",
        body: JSON.stringify({ name: ruleName.trim(), metric_name: metric, operator, threshold: parsed, server_id: serverId || null }),
      });
      setRuleName("");
      await load();
    } catch (submitError) {
      setError(submitError instanceof Error ? submitError.message : "Alert rule could not be created.");
    } finally { setBusy(false); }
  }

  async function deleteRule(rule: Rule) {
    setBusy(true); setError("");
    try {
      await apiRequest<null>("/api/alert-rules/" + encodeURIComponent(rule.id), { method: "DELETE" });
      await load();
    } catch (deleteError) {
      setError(deleteError instanceof Error ? deleteError.message : "Alert rule could not be deleted.");
    } finally { setBusy(false); }
  }

  async function markRead(item: Notification) {
    try {
      await apiRequest<null>("/api/notifications/" + encodeURIComponent(item.id) + "/read", { method: "POST", body: "{}" });
      setNotifications((current) => current.map((entry) => entry.id === item.id ? { ...entry, read_at: new Date().toISOString() } : entry));
    } catch (readError) {
      setError(readError instanceof Error ? readError.message : "Notification could not be marked as read.");
    }
  }

  const metricInfo = METRICS.find((item) => item.value === metric) ?? METRICS[0];

  return (
    <section className="alerts-workspace">
      <header className="alerts-workspace-header">
        <div>
          <button className="detail-back-button" type="button" onClick={onBack}>← Back to servers</button>
          <div className="eyebrow">Operations and notifications</div>
          <h1>Alerts</h1>
          <p>Rules evaluate committed telemetry. Breaches create in-app notifications; normal readings create recovery events.</p>
        </div>
        <div className="alerts-counts"><span>{rules.length} rules</span><span>{events.filter((item) => item.status === "firing").length} firing</span><span>{unreadCount} unread</span></div>
      </header>
      {error && <div className="notice notice-error" role="alert">{error}</div>}
      {canAdminister && (
        <section className="alert-form-card">
          <h2>Create threshold rule</h2>
          <form className="form-stack" onSubmit={createRule}>
            <div className="alert-form-grid">
              <label className="field"><span>Rule name</span><Input value={ruleName} onChange={(event) => setRuleName(event.target.value)} required minLength={2} maxLength={120} placeholder="High CPU utilization" /></label>
              <label className="field"><span>Metric</span><select value={metric} onChange={(event) => setMetric(event.target.value)}>{METRICS.map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}</select></label>
              <label className="field"><span>Condition</span><select value={operator} onChange={(event) => setOperator(event.target.value as Rule["operator"])}><option value="gt">Greater than</option><option value="gte">At least</option><option value="lt">Less than</option><option value="lte">At most</option></select></label>
              <label className="field"><span>Threshold ({metricInfo.unit})</span><Input type="number" step="any" value={threshold} onChange={(event) => setThreshold(event.target.value)} required /></label>
              <label className="field"><span>Server scope</span><select value={serverId} onChange={(event) => setServerId(event.target.value)}><option value="">All servers</option>{servers.map((server) => <option key={server.id} value={server.id}>{server.name}</option>)}</select></label>
                          </div>
            <div className="alerts-form-footer"><span>Rules evaluate metrics from every matching machine.</span><Button className="button button-primary" variant="default" type="submit" disabled={busy}>{busy ? "Saving…" : "Create rule"}</Button></div>
          </form>
        </section>
      )}
      <div className="alerts-sections-grid">
        <section className="alert-list-card">
          <div className="panel-heading"><div><h2>Alert rules</h2><p>Installation-wide threshold policies</p></div></div>
          {loading ? <div className="project-empty">Loading alert rules…</div> : rules.length === 0 ? <div className="project-empty">No alert rules configured.</div> : rules.map((rule) => {
            const metricDef = METRICS.find((item) => item.value === rule.metric_name);
            const server = servers.find((item) => item.id === rule.server_id);
            return <div className="alert-rule-row" key={rule.id}>
              <div><strong>{rule.name}</strong><small>{metricDef?.label ?? rule.metric_name} {rule.operator} {formatNumber(rule.threshold, metricDef?.unit ?? "")}</small><small>{server?.name ?? "All servers"}</small></div>
              {canAdminister && <Button className="project-inline-delete" variant="destructive" size="sm" type="button" disabled={busy} onClick={() => void deleteRule(rule)}>Delete</Button>}
            </div>;
          })}
        </section>
        <section className="alert-list-card">
          <div className="panel-heading"><div><h2>Recent events</h2><p>Firing and recovered transitions</p></div></div>
          {loading ? <div className="project-empty">Loading alert events…</div> : events.length === 0 ? <div className="project-empty">No alert events recorded.</div> : events.map((item) => (
            <div className="alert-event-row" key={item.id}>
              <div><strong>{item.rule_name}</strong><small>{item.server_name} · {item.metric_name}</small><small>Last value {item.last_value.toLocaleString()} · {formatTime(item.last_observed_at)}</small></div>
              <span className={item.status === "firing" ? "alert-state alert-state-firing" : "alert-state alert-state-resolved"}>{item.status}</span>
            </div>
          ))}
        </section>
      </div>
      <section className="alert-list-card alert-notifications-card">
        <div className="panel-heading"><div><h2>In-app notifications</h2><p>Notifications are stored in this installation</p></div><Button className="button button-secondary" variant="secondary" type="button" onClick={() => { void load(); }}>Refresh</Button></div>
        {loading ? <div className="project-empty">Loading notifications…</div> : notifications.length === 0 ? <div className="project-empty">No notifications yet.</div> : notifications.map((item) => (
          <div className={item.read_at ? "alert-notification-row" : "alert-notification-row alert-notification-unread"} key={item.id}>
            <div><strong>{item.title}</strong><p>{item.message}</p><small>{formatTime(item.created_at)}</small></div>
            {!item.read_at && <Button className="project-inline-delete" variant="ghost" size="sm" type="button" onClick={() => void markRead(item)}>Mark read</Button>}
          </div>
        ))}
      </section>
    </section>
  );
}
