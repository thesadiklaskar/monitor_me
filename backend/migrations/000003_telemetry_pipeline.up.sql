BEGIN;

CREATE TABLE telemetry_batches (
    agent_id UUID NOT NULL REFERENCES agents (id) ON DELETE CASCADE,
    batch_id TEXT NOT NULL CHECK (batch_id ~ '^[0-9a-f]{32}$'),
    schema_version INTEGER NOT NULL CHECK (schema_version = 1),
    payload_hash BYTEA NOT NULL CHECK (octet_length(payload_hash) = 32),
    collected_at TIMESTAMPTZ NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    sample_count INTEGER NOT NULL CHECK (sample_count BETWEEN 1 AND 4096),
    PRIMARY KEY (agent_id, batch_id)
);

CREATE TABLE telemetry_samples (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    organization_id UUID NOT NULL,
    server_id UUID NOT NULL,
    agent_id UUID NOT NULL,
    batch_id TEXT NOT NULL,
    metric_name TEXT NOT NULL CHECK (length(metric_name) BETWEEN 1 AND 128),
    value DOUBLE PRECISION NOT NULL,
    unit TEXT NOT NULL CHECK (length(unit) BETWEEN 1 AND 64),
    labels JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(labels) = 'object'),
    observed_at TIMESTAMPTZ NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    project_id UUID,
    attribution_method TEXT NOT NULL
        CHECK (attribution_method IN ('host_total', 'cgroup', 'container', 'systemd', 'explicit', 'process_tree', 'unclassified', 'shared')),
    FOREIGN KEY (agent_id, batch_id)
        REFERENCES telemetry_batches (agent_id, batch_id) ON DELETE CASCADE,
    FOREIGN KEY (server_id, organization_id)
        REFERENCES servers (id, organization_id) ON DELETE CASCADE,
    FOREIGN KEY (project_id, organization_id)
        REFERENCES projects (id, organization_id) ON DELETE SET NULL
);

CREATE INDEX telemetry_samples_server_metric_time_idx
    ON telemetry_samples (organization_id, server_id, metric_name, observed_at DESC);

CREATE INDEX telemetry_samples_project_metric_time_idx
    ON telemetry_samples (organization_id, project_id, metric_name, observed_at DESC)
    WHERE project_id IS NOT NULL;

CREATE INDEX telemetry_samples_retention_idx
    ON telemetry_samples (observed_at);

CREATE INDEX telemetry_batches_received_idx
    ON telemetry_batches (received_at);

COMMIT;
