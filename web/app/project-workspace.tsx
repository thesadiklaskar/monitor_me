"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { apiRequest } from "@/lib/client-api";
import { Button } from "@/components/ui/button";

type Server = { id: string; name: string; hostname?: string };
type Project = { id: string; name: string; slug: string; created_at: string };
type Mapping = {
  id: string;
  server_id: string;
  project_id: string;
  project_name: string;
  project_slug: string;
  workload_type: string;
  workload_selector: string;
  attribution_method: string;
};
type MappingResponse = { mappings: Mapping[] };

function slugFromName(value: string): string {
  return value.toLowerCase().trim().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "").slice(0, 64);
}

export default function ProjectWorkspace({
  servers, projects, role, onProjectsChanged, onBack,
}: {
  servers: Server[];
  projects: Project[];
  role: string;
  onProjectsChanged: (projects: Project[]) => void;
  onBack: () => void;
}) {
  const [mappings, setMappings] = useState<Record<string, Mapping[]>>({});
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [projectName, setProjectName] = useState("");
  const [projectSlug, setProjectSlug] = useState("");
  const [mappingServerId, setMappingServerId] = useState("");
  const [mappingProjectId, setMappingProjectId] = useState("");
  const [workloadType, setWorkloadType] = useState("systemd");
  const [workloadSelector, setWorkloadSelector] = useState("");
  const [attributionMethod, setAttributionMethod] = useState("systemd");

  const canAdminister = role === "owner" || role === "admin";
  const canManageMappings = ["owner", "admin", "operator"].includes(role);
  const mappingCount = useMemo(
    () => Object.values(mappings).reduce((total, values) => total + values.length, 0),
    [mappings],
  );

  const loadMappings = useCallback(async () => {
    try {
      const results = await Promise.all(servers.map(async (server) => {
        const response = await apiRequest<MappingResponse>(
          `/api/servers/${encodeURIComponent(server.id)}/project-mappings`,
        );
        return [server.id, Array.isArray(response.mappings) ? response.mappings : []] as const;
      }));
      setMappings(Object.fromEntries(results));
      setError("");
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : "Workload mappings could not be loaded.");
    } finally {
      setLoading(false);
    }
  }, [servers]);

  useEffect(() => {
    const timer = window.setTimeout(() => { void loadMappings(); }, 0);
    return () => window.clearTimeout(timer);
  }, [loadMappings]);

  async function refreshProjects() {
    const response = await apiRequest<{ projects: Project[] }>("/api/projects");
    onProjectsChanged(Array.isArray(response.projects) ? response.projects : []);
  }

  async function createProject(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setBusy(true);
    setError("");
    try {
      await apiRequest<{ project: Project }>("/api/projects", {
        method: "POST",
        body: JSON.stringify({ name: projectName, slug: projectSlug }),
      });
      setProjectName("");
      setProjectSlug("");
      await refreshProjects();
    } catch (submitError) {
      setError(submitError instanceof Error ? submitError.message : "Project could not be created.");
    } finally {
      setBusy(false);
    }
  }

  async function deleteProject(project: Project) {
    setBusy(true);
    setError("");
    try {
      await apiRequest<null>(`/api/projects/${encodeURIComponent(project.id)}`, { method: "DELETE" });
      await refreshProjects();
      await loadMappings();
    } catch (deleteError) {
      setError(deleteError instanceof Error ? deleteError.message : "Project could not be deleted.");
    } finally {
      setBusy(false);
    }
  }

  async function createMapping(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const targetServerId = mappingServerId || servers[0]?.id || "";
    const targetProjectId = mappingProjectId || projects[0]?.id || "";
    if (!targetServerId || !targetProjectId) return;
    setBusy(true);
    setError("");
    try {
      await apiRequest<{ mapping: Mapping }>(
        `/api/servers/${encodeURIComponent(targetServerId)}/project-mappings`,
        {
          method: "POST",
          body: JSON.stringify({
            project_id: targetProjectId,
            workload_type: workloadType,
            workload_selector: workloadSelector,
            attribution_method: attributionMethod,
          }),
        },
      );
      setWorkloadSelector("");
      await loadMappings();
    } catch (submitError) {
      setError(submitError instanceof Error ? submitError.message : "Workload mapping could not be created.");
    } finally {
      setBusy(false);
    }
  }

  async function deleteMapping(mapping: Mapping) {
    setBusy(true);
    setError("");
    try {
      await apiRequest<null>(`/api/project-mappings/${encodeURIComponent(mapping.id)}`, { method: "DELETE" });
      await loadMappings();
    } catch (deleteError) {
      setError(deleteError instanceof Error ? deleteError.message : "Workload mapping could not be removed.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="project-workspace">
      <header className="project-workspace-header">
        <div>
          <button className="detail-back-button" type="button" onClick={onBack}>← Back to servers</button>
          <div className="eyebrow">Organization workspace</div>
          <h1>Projects and workload mappings</h1>
          <p>Assign Linux cgroup, container or Systemd selectors to projects. Host totals remain separate from mapped workload metrics.</p>
        </div>
        <div className="phase-label">{projects.length} projects · {mappingCount} mappings</div>
      </header>

      {error && <div className="notice notice-error" role="alert">{error}</div>}

      {canAdminister && (
        <section className="project-form-card">
          <h2>Create project</h2>
          <form onSubmit={createProject} className="form-stack">
            <div className="project-form-grid">
              <label className="field">
                <span>Project name</span>
                <input value={projectName} onChange={(event) => { setProjectName(event.target.value); setProjectSlug(slugFromName(event.target.value)); }} required minLength={2} maxLength={120} placeholder="Payments API" />
              </label>
              <label className="field">
                <span>Project slug</span>
                <input value={projectSlug} onChange={(event) => setProjectSlug(slugFromName(event.target.value))} required minLength={3} maxLength={64} />
              </label>
            </div>
            <div className="project-form-actions">
              <Button className="button button-primary" variant="default" type="submit" disabled={busy}>{busy ? "Saving…" : "Create project"}</Button>
            </div>
          </form>
        </section>
      )}

      {canManageMappings && servers.length > 0 && projects.length > 0 && (
        <section className="project-form-card">
          <h2>Map a workload to a project</h2>
          <form onSubmit={createMapping} className="form-stack">
            <div className="project-form-grid">
              <label className="field"><span>Server</span>
                <select value={mappingServerId || servers[0].id} onChange={(event) => setMappingServerId(event.target.value)}>
                  {servers.map((server) => <option key={server.id} value={server.id}>{server.name}</option>)}
                </select>
              </label>
              <label className="field"><span>Project</span>
                <select value={mappingProjectId || projects[0].id} onChange={(event) => setMappingProjectId(event.target.value)}>
                  {projects.map((project) => <option key={project.id} value={project.id}>{project.name}</option>)}
                </select>
              </label>
              <label className="field"><span>Workload type</span>
                <select value={workloadType} onChange={(event) => { setWorkloadType(event.target.value); setAttributionMethod(event.target.value); }}>
                  <option value="systemd">Systemd unit</option>
                  <option value="container">Container cgroup</option>
                  <option value="cgroup">Linux cgroup</option>
                </select>
              </label>
              <label className="field"><span>Attribution method</span>
                <select value={attributionMethod} onChange={(event) => setAttributionMethod(event.target.value)}>
                  <option value="systemd">Systemd</option>
                  <option value="container">Container</option>
                  <option value="cgroup">cgroup</option>
                  <option value="explicit">Explicit</option>
                  <option value="process_tree">Process tree</option>
                </select>
              </label>
              <label className="field"><span>Workload selector</span>
                <input value={workloadSelector} onChange={(event) => setWorkloadSelector(event.target.value)} required maxLength={512} placeholder="payments-api.service" />
              </label>
            </div>
            <div className="project-form-actions">
              <Button className="button button-primary" variant="default" type="submit" disabled={busy}>{busy ? "Saving…" : "Create mapping"}</Button>
            </div>
          </form>
        </section>
      )}

      {loading ? (
        <div className="project-empty">Loading projects and workload mappings…</div>
      ) : projects.length === 0 ? (
        <div className="project-empty">No projects have been created for this organization yet.</div>
      ) : (
        <div className="project-workspace-grid">
          {projects.map((project) => {
            const projectMappings = servers.flatMap((server) =>
              (mappings[server.id] ?? []).filter((mapping) => mapping.project_id === project.id)
                .map((mapping) => ({ ...mapping, serverName: server.name })),
            );
            return (
              <article className="project-card" key={project.id}>
                <div className="project-card-heading">
                  <div><h3>{project.name}</h3><code>{project.slug}</code></div>
                  {canAdminister && <Button className="project-inline-delete" variant="destructive" size="sm" type="button" onClick={() => void deleteProject(project)} disabled={busy}>Delete</Button>}
                </div>
                <div className="mapping-list">
                  <strong>{projectMappings.length} workload mappings</strong>
                  {projectMappings.length === 0 ? (
                    <p className="detail-data-note">No workloads are assigned to this project yet.</p>
                  ) : projectMappings.map((mapping) => (
                    <div className="mapping-row" key={mapping.id}>
                      <div>
                        <strong>{mapping.serverName} · {mapping.workload_type}</strong>
                        <small>{mapping.workload_selector} · {mapping.attribution_method}</small>
                      </div>
                      {canManageMappings && <Button className="project-inline-delete" variant="destructive" size="sm" type="button" onClick={() => void deleteMapping(mapping)} disabled={busy}>Remove</Button>}
                    </div>
                  ))}
                </div>
              </article>
            );
          })}
        </div>
      )}
    </section>
  );
}
