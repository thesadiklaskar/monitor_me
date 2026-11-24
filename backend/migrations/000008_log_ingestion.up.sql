BEGIN;

CREATE TABLE agent_log_batches (
    organization_id UUID NOT NULL,
    server_id UUID NOT NULL,
    agent_id UUID NOT NULL REFERENCES agents (id) ON DELETE CASCADE,
    batch_id TEXT NOT NULL CHECK (batch_id ~ '^[0-9a-f]{32}$'),
    payload_hash BYTEA NOT NULL CHECK (octet_length(payload_hash) = 32),
    event_count INTEGER NOT NULL CHECK (event_count BETWEEN 1 AND 32),
    received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (agent_id, batch_id),
    FOREIGN KEY (server_id, organization_id)
        REFERENCES servers (id, organization_id) ON DELETE CASCADE
);

CREATE INDEX agent_log_batches_retention_idx ON agent_log_batches (received_at);

COMMIT;
