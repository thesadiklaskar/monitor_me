BEGIN;

ALTER TABLE dashboards
    ADD COLUMN default_server_id UUID,
    ADD COLUMN default_time_range TEXT NOT NULL DEFAULT 'panel'
        CHECK (default_time_range IN ('panel', '1h', '6h', '24h', '7d', '30d'));

ALTER TABLE dashboards
    ADD CONSTRAINT dashboards_default_server_scope_fk
    FOREIGN KEY (default_server_id, organization_id)
    REFERENCES servers (id, organization_id)
    ON DELETE SET NULL (default_server_id);

COMMIT;
