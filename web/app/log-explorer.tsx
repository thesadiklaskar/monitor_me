"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { FileText, Pause, Play, RefreshCw, Search, X } from "lucide-react";
import { apiRequest } from "@/lib/client-api";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Alert } from "@/components/ui/alert";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@/components/ui/table";

type Server = { id: string; name: string; status: string };
type Project = { id: string; name: string; slug: string };
type LogEvent = {
  event_id: string;
  event_time_unix_nanos: number;
  event_time_unix_micros: number;
  source: string;
  severity: string;
  body: string;
  attributes: Record<string, string>;
  project_id?: string;
};
type LogResponse = { events: LogEvent[]; count: number; has_more: boolean };

type Range = "15m" | "1h" | "6h" | "24h" | "7d";
const ranges: Record<Range, number> = {
  "15m": 15 * 60 * 1000,
  "1h": 60 * 60 * 1000,
  "6h": 6 * 60 * 60 * 1000,
  "24h": 24 * 60 * 60 * 1000,
  "7d": 7 * 24 * 60 * 60 * 1000,
};
const severities = ["", "emergency", "alert", "critical", "error", "warning", "notice", "info", "debug", "unknown"];

function formatTime(unixNanos: number): string {
  const value = new Date(unixNanos / 1_000_000);
  return Number.isNaN(value.getTime()) ? "Invalid event time" : value.toLocaleString();
}

function severityVariant(value: string): "default" | "secondary" | "destructive" | "warning" | "outline" {
  if (["emergency", "alert", "critical", "error"].includes(value)) return "destructive";
  if (value === "warning") return "warning";
  if (value === "info") return "default";
  return "secondary";
}

export default function LogExplorer({
  servers,
  projects,
  onBack,
}: {
  servers: Server[];
  projects: Project[];
  onBack: () => void;
}) {
  const [serverId, setServerId] = useState(servers[0]?.id ?? "");
  const [projectId, setProjectId] = useState("");
  const [range, setRange] = useState<Range>("1h");
  const [searchText, setSearchText] = useState("");
  const [sourceText, setSourceText] = useState("");
  const [severity, setSeverity] = useState("");
  const [applied, setApplied] = useState({ search: "", source: "", severity: "", projectId: "" });
  const [events, setEvents] = useState<LogEvent[]>([]);
  const [selectedEvent, setSelectedEvent] = useState<LogEvent | null>(null);
  const [loading, setLoading] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [hasMore, setHasMore] = useState(false);
  const [live, setLive] = useState(false);
  const [error, setError] = useState("");
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null);
  const requestSequence = useRef(0);
  const pageRequestSequence = useRef(0);
  const queryWindow = useRef<{ from: string; to: string } | null>(null);

  const selectedServer = useMemo(
    () => servers.find((server) => server.id === serverId) ?? null,
    [servers, serverId],
  );

  const loadLogs = useCallback(async () => {
    pageRequestSequence.current += 1;
    setLoadingMore(false);
    if (!serverId) {
      queryWindow.current = null;
      setHasMore(false);
      setEvents([]);
      setError("Add a machine before searching its logs.");
      setLoading(false);
      return;
    }
    const requestId = ++requestSequence.current;
    setLoading(true);
    setError("");
    const to = new Date();
    const from = new Date(to.getTime() - ranges[range]);
    const window = { from: from.toISOString(), to: to.toISOString() };
    queryWindow.current = window;
    setHasMore(false);
    const query = new URLSearchParams({
      from: window.from,
      to: window.to,
      limit: "201",
    });
    if (applied.projectId) query.set("project_id", applied.projectId);
    if (applied.search) query.set("search", applied.search);
    if (applied.source) query.set("source", applied.source);
    if (applied.severity) query.set("severity", applied.severity);
    try {
      const response = await apiRequest<LogResponse>(
        "/api/servers/" + encodeURIComponent(serverId) + "/logs?" + query.toString(),
      );
      if (requestId !== requestSequence.current) return;
      if (!response || !Array.isArray(response.events)) {
        throw new Error("Log search returned an invalid response.");
      }
      setEvents(response.events);
      setHasMore(response.has_more === true);
      setSelectedEvent((current) => current
        ? response.events.find((item) => item.event_id === current.event_id) ?? null
        : null);
      setLastUpdated(new Date());
    } catch (loadError) {
      if (requestId !== requestSequence.current) return;
      setError(loadError instanceof Error ? loadError.message : "Logs could not be loaded.");
    } finally {
      if (requestId === requestSequence.current) setLoading(false);
    }
  }, [serverId, projectId, range, applied]);

  useEffect(() => {
    const initial = window.setTimeout(() => { void loadLogs(); }, 0);
    return () => {
      window.clearTimeout(initial);
      requestSequence.current += 1;
      pageRequestSequence.current += 1;
    };
  }, [loadLogs]);

  useEffect(() => {
    if (!live) return;
    const timer = window.setInterval(() => { void loadLogs(); }, 10000);
    return () => window.clearInterval(timer);
  }, [live, loadLogs]);

  const loadOlder = useCallback(async () => {
    const window = queryWindow.current;
    const last = events[events.length - 1];
    if (!window || !last || !hasMore || loadingMore) return;
    if (!Number.isSafeInteger(last.event_time_unix_micros) || last.event_time_unix_micros <= 0) {
      setError("The log cursor is invalid. Refresh the search before loading older events.");
      return;
    }

    const pageRequestId = ++pageRequestSequence.current;
    setLoadingMore(true);
    setError("");
    const query = new URLSearchParams({
      from: window.from,
      to: window.to,
      limit: "201",
      before_time: String(last.event_time_unix_micros),
      before_id: last.event_id,
    });
    if (applied.projectId) query.set("project_id", applied.projectId);
    if (applied.search) query.set("search", applied.search);
    if (applied.source) query.set("source", applied.source);
    if (applied.severity) query.set("severity", applied.severity);

    try {
      const response = await apiRequest<LogResponse>(
        "/api/servers/" + encodeURIComponent(serverId) + "/logs?" + query.toString(),
      );
      if (pageRequestId !== pageRequestSequence.current) return;
      if (!response || !Array.isArray(response.events)) {
        throw new Error("Older log page returned an invalid response.");
      }
      setEvents((current) => {
        const known = new Set(current.map((item) => item.event_id));
        return [...current, ...response.events.filter((item) => !known.has(item.event_id))];
      });
      setHasMore(response.has_more === true);
    } catch (pageError) {
      if (pageRequestId === pageRequestSequence.current) {
        setError(pageError instanceof Error ? pageError.message : "Older log events could not be loaded.");
      }
    } finally {
      if (pageRequestId === pageRequestSequence.current) setLoadingMore(false);
    }
  }, [serverId, events, hasMore, loadingMore, applied]);

  function applyFilters() {
    setApplied({ search: searchText.trim(), source: sourceText.trim(), severity, projectId });
  }

  function clearFilters() {
    setSearchText("");
    setSourceText("");
    setSeverity("");
    setProjectId("");
    setApplied({ search: "", source: "", severity: "", projectId: "" });
  }

  return (
    <section className="log-explorer">
      <header className="log-explorer-header">
        <div>
          <button className="detail-back-button" type="button" onClick={onBack}>← Back to servers</button>
          <div className="eyebrow">Searchable host events</div>
          <h1>Log Explorer</h1>
          <p>Search structured Linux journal events stored in your own ClickHouse instance. Results are time-bounded and limited to 200 rows per request.</p>
        </div>
        <div className="log-explorer-header-actions">
          <Button variant="secondary" onClick={() => void loadLogs()} disabled={loading}><RefreshCw size={14} /> Refresh</Button>
          <Button variant={live ? "default" : "secondary"} onClick={() => setLive((value) => !value)}>
            {live ? <Pause size={14} /> : <Play size={14} />} {live ? "Pause live tail" : "Live tail"}
          </Button>
        </div>
      </header>

      <Card className="log-filter-card">
        <CardContent className="log-filter-grid">
          <label className="field"><span>Machine</span>
            <select value={serverId} onChange={(event) => setServerId(event.target.value)}>
              <option value="">Select machine</option>
              {servers.map((server) => <option key={server.id} value={server.id}>{server.name} · {server.status}</option>)}
            </select>
          </label>
          <label className="field"><span>Project</span>
            <select value={projectId} onChange={(event) => setProjectId(event.target.value)}>
              <option value="">All projects</option>
              {projects.map((project) => <option key={project.id} value={project.id}>{project.name}</option>)}
            </select>
          </label>
          <label className="field"><span>Time range</span>
            <select value={range} onChange={(event) => setRange(event.target.value as Range)}>
              <option value="15m">Last 15 minutes</option>
              <option value="1h">Last hour</option>
              <option value="6h">Last 6 hours</option>
              <option value="24h">Last 24 hours</option>
              <option value="7d">Last 7 days</option>
            </select>
          </label>
          <label className="field"><span>Severity</span>
            <select value={severity} onChange={(event) => setSeverity(event.target.value)}>
              {severities.map((item) => <option key={item || "all"} value={item}>{item || "All severities"}</option>)}
            </select>
          </label>
          <label className="field log-search-field"><span>Full-text search</span>
            <Input value={searchText} onChange={(event) => setSearchText(event.target.value)} placeholder="Search message text" maxLength={256} onKeyDown={(event) => { if (event.key === "Enter") applyFilters(); }} />
          </label>
          <label className="field"><span>Source / unit</span>
            <Input value={sourceText} onChange={(event) => setSourceText(event.target.value)} placeholder="sshd.service" maxLength={128} onKeyDown={(event) => { if (event.key === "Enter") applyFilters(); }} />
          </label>
          <div className="log-filter-actions">
            <Button onClick={applyFilters} disabled={!serverId || loading}><Search size={14} /> Search</Button>
            <Button variant="ghost" onClick={clearFilters}><X size={14} /> Clear</Button>
          </div>
        </CardContent>
      </Card>

      {error && <Alert variant="destructive">{error}</Alert>}

      <Card className="log-results-card">
        <CardHeader className="log-results-header">
          <div><CardTitle>Events</CardTitle><p>{selectedServer?.name ?? "No machine selected"} · {events.length} loaded · newest first</p></div>
          <div className="log-results-meta">
            {live && <Badge variant="default">Live tail on</Badge>}
            <span>{lastUpdated ? "Updated " + lastUpdated.toLocaleTimeString() : "Not loaded"}</span>
          </div>
        </CardHeader>
        {loading && events.length === 0 ? (
          <CardContent className="log-loading"><Skeleton className="h-12 w-full" /><Skeleton className="h-12 w-full" /><Skeleton className="h-12 w-full" /></CardContent>
        ) : error && events.length === 0 ? (
          <CardContent className="log-empty-state">The query did not complete. Correct the filters or retry.</CardContent>
        ) : events.length === 0 ? (
          <CardContent className="log-empty-state"><FileText size={22} /><strong>No log events in this time range</strong><span>Check that the Linux machine has journald and that the agent has the logs.journald capability.</span></CardContent>
        ) : (
          <div className="log-table-scroll">
            <Table>
              <TableHeader><TableRow><TableHead>Time</TableHead><TableHead>Severity</TableHead><TableHead>Source</TableHead><TableHead>Message</TableHead></TableRow></TableHeader>
              <TableBody>
                {events.map((event) => (
                  <TableRow key={event.event_id} data-state={selectedEvent?.event_id === event.event_id ? "selected" : undefined} className="log-event-row" onClick={() => setSelectedEvent(event)}>
                    <TableCell className="log-time-cell">{formatTime(event.event_time_unix_nanos)}</TableCell>
                    <TableCell><Badge variant={severityVariant(event.severity)}>{event.severity}</Badge></TableCell>
                    <TableCell className="log-source-cell">{event.source}</TableCell>
                    <TableCell className="log-message-cell">{event.body}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        )}
        {hasMore && (
          <div className="log-load-older">
            <Button variant="secondary" onClick={() => void loadOlder()} disabled={loadingMore || loading}>
              {loadingMore ? "Loading older events…" : "Load older events"}
            </Button>
            <span>Each page is limited to 200 events.</span>
          </div>
        )}
      </Card>

      {selectedEvent && (
        <Card className="log-detail-card">
          <CardHeader className="log-detail-heading">
            <div><CardTitle>Event details</CardTitle><p>{selectedEvent.event_id} · {formatTime(selectedEvent.event_time_unix_nanos)}</p></div>
            <Button variant="ghost" size="icon" aria-label="Close event details" onClick={() => setSelectedEvent(null)}><X size={15} /></Button>
          </CardHeader>
          <CardContent>
            <pre>{JSON.stringify(selectedEvent, null, 2)}</pre>
          </CardContent>
        </Card>
      )}
    </section>
  );
}
