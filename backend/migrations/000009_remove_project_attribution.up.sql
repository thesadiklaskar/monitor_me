-- Remove project/workload attribution from the monitoring data model.
-- Metric, log, dashboard, alert and machine records remain intact, and only project
-- metadata and project-scoped associations are retired.
BEGIN;

DROP TABLE IF EXISTS server_project_mappings;

DROP INDEX IF EXISTS telemetry_samples_project_metric_time_idx;
DROP INDEX IF EXISTS alert_events_active_idx;

ALTER TABLE telemetry_samples DROP COLUMN IF EXISTS project_id;
ALTER TABLE alert_rules DROP COLUMN IF EXISTS project_id;
ALTER TABLE alert_events DROP COLUMN IF EXISTS project_id;
ALTER TABLE dashboards DROP COLUMN IF EXISTS project_id;
ALTER TABLE dashboard_panels DROP COLUMN IF EXISTS project_id;

CREATE UNIQUE INDEX alert_events_active_idx
    ON alert_events (rule_id, server_id)
    WHERE status = 'firing';

DROP INDEX IF EXISTS projects_organization_idx;
DROP TABLE IF EXISTS projects;

COMMIT;
