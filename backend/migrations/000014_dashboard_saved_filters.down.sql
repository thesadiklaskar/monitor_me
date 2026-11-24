BEGIN;

ALTER TABLE dashboards
    DROP CONSTRAINT IF EXISTS dashboards_default_server_scope_fk;

ALTER TABLE dashboards
    DROP COLUMN IF EXISTS default_server_id,
    DROP COLUMN IF EXISTS default_time_range;

COMMIT;
