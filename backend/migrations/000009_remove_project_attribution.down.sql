-- Rollback restores empty project structures only. Project records and
-- mapping history removed by the forward migration cannot be reconstructed.
BEGIN;

CREATE TABLE IF NOT EXISTS projects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    name TEXT NOT NULL CHECK (length(trim(name)) > 0),
    slug TEXT NOT NULL CHECK (slug ~ '^[a-z0-9][a-z0-9-]{1,62}[a-z0-9]$'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (organization_id, slug),
    UNIQUE (id, organization_id)
);

CREATE INDEX IF NOT EXISTS projects_organization_idx ON projects (organization_id);

ALTER TABLE telemetry_samples ADD COLUMN IF NOT EXISTS project_id UUID;
ALTER TABLE alert_rules ADD COLUMN IF NOT EXISTS project_id UUID;
ALTER TABLE alert_events ADD COLUMN IF NOT EXISTS project_id UUID;
ALTER TABLE dashboards ADD COLUMN IF NOT EXISTS project_id UUID;
ALTER TABLE dashboard_panels ADD COLUMN IF NOT EXISTS project_id UUID;

ALTER TABLE telemetry_samples
    ADD CONSTRAINT telemetry_samples_project_id_organization_id_fkey
    FOREIGN KEY (project_id, organization_id)
    REFERENCES projects (id, organization_id) ON DELETE SET NULL (project_id);
ALTER TABLE alert_rules
    ADD CONSTRAINT alert_rules_project_id_organization_id_fkey
    FOREIGN KEY (project_id, organization_id)
    REFERENCES projects (id, organization_id) ON DELETE CASCADE;
ALTER TABLE alert_events
    ADD CONSTRAINT alert_events_project_id_organization_id_fkey
    FOREIGN KEY (project_id, organization_id)
    REFERENCES projects (id, organization_id) ON DELETE CASCADE;
ALTER TABLE dashboards
    ADD CONSTRAINT dashboards_project_id_organization_id_fkey
    FOREIGN KEY (project_id, organization_id)
    REFERENCES projects (id, organization_id) ON DELETE CASCADE;
ALTER TABLE dashboard_panels
    ADD CONSTRAINT dashboard_panels_project_id_organization_id_fkey
    FOREIGN KEY (project_id, organization_id)
    REFERENCES projects (id, organization_id) ON DELETE CASCADE;

CREATE TABLE IF NOT EXISTS server_project_mappings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    server_id UUID NOT NULL,
    project_id UUID NOT NULL,
    workload_type TEXT NOT NULL CHECK (length(trim(workload_type)) > 0),
    workload_selector TEXT NOT NULL CHECK (length(trim(workload_selector)) > 0),
    attribution_method TEXT NOT NULL
        CHECK (attribution_method IN ('cgroup', 'container', 'systemd', 'explicit', 'process_tree')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (server_id, organization_id)
        REFERENCES servers (id, organization_id) ON DELETE CASCADE,
    FOREIGN KEY (project_id, organization_id)
        REFERENCES projects (id, organization_id) ON DELETE CASCADE,
    UNIQUE (server_id, workload_type, workload_selector)
);

CREATE INDEX IF NOT EXISTS telemetry_samples_project_metric_time_idx
    ON telemetry_samples (organization_id, project_id, metric_name, observed_at DESC)
    WHERE project_id IS NOT NULL;

DROP INDEX IF EXISTS alert_events_active_idx;
CREATE UNIQUE INDEX alert_events_active_idx
    ON alert_events (rule_id, server_id, COALESCE(project_id, '00000000-0000-0000-0000-000000000000'::uuid))
    WHERE status = 'firing';

COMMIT;
