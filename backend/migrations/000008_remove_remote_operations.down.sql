-- Rollback recreates empty legacy tables only. Retired command history cannot be restored.
BEGIN;
CREATE TABLE IF NOT EXISTS remote_operations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    server_id UUID NOT NULL,
    agent_id UUID NOT NULL REFERENCES agents (id) ON DELETE CASCADE,
    created_by UUID REFERENCES users (id) ON DELETE SET NULL,
    operation TEXT NOT NULL CHECK (operation IN ('restart_service', 'stop_service')),
    target TEXT NOT NULL CHECK (target ~ '^[A-Za-z0-9_.:@-]{1,180}\\.service$'),
    status TEXT NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued', 'dispatched', 'succeeded', 'failed', 'expired')),
    result TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    dispatched_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    UNIQUE (id, organization_id),
    FOREIGN KEY (server_id, organization_id)
        REFERENCES servers (id, organization_id) ON DELETE CASCADE,
    CHECK ((status IN ('succeeded', 'failed', 'expired') AND completed_at IS NOT NULL) OR
           (status IN ('queued', 'dispatched') AND completed_at IS NULL))
);
CREATE INDEX IF NOT EXISTS remote_operations_agent_queue_idx
    ON remote_operations (agent_id, created_at) WHERE status = 'queued';
CREATE INDEX IF NOT EXISTS remote_operations_org_history_idx
    ON remote_operations (organization_id, created_at DESC);
CREATE TABLE IF NOT EXISTS remote_operation_audit (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    remote_operation_id UUID NOT NULL,
    actor_user_id UUID REFERENCES users (id) ON DELETE SET NULL,
    actor_agent_id UUID REFERENCES agents (id) ON DELETE SET NULL,
    action TEXT NOT NULL CHECK (action IN ('queued', 'dispatched', 'succeeded', 'failed', 'expired')),
    details JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (remote_operation_id, organization_id)
        REFERENCES remote_operations (id, organization_id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS remote_operation_audit_org_idx
    ON remote_operation_audit (organization_id, created_at DESC);
COMMIT;
