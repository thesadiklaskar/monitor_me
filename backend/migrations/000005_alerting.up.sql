BEGIN;

CREATE TABLE alert_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    name TEXT NOT NULL CHECK (length(trim(name)) BETWEEN 2 AND 120),
    server_id UUID,
    project_id UUID,
    metric_name TEXT NOT NULL CHECK (metric_name ~ '^[A-Za-z][A-Za-z0-9_.-]{0,127}$'),
    operator TEXT NOT NULL CHECK (operator IN ('gt', 'gte', 'lt', 'lte')),
    threshold DOUBLE PRECISION NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_by UUID REFERENCES users (id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (id, organization_id),
    FOREIGN KEY (server_id, organization_id)
        REFERENCES servers (id, organization_id) ON DELETE CASCADE,
    FOREIGN KEY (project_id, organization_id)
        REFERENCES projects (id, organization_id) ON DELETE CASCADE
);

CREATE INDEX alert_rules_match_idx
    ON alert_rules (organization_id, metric_name)
    WHERE enabled;

CREATE TABLE alert_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    rule_id UUID NOT NULL,
    server_id UUID NOT NULL,
    project_id UUID,
    metric_name TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('firing', 'resolved')),
    last_value DOUBLE PRECISION NOT NULL,
    triggered_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_observed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at TIMESTAMPTZ,
    UNIQUE (id, organization_id),
    FOREIGN KEY (rule_id, organization_id)
        REFERENCES alert_rules (id, organization_id) ON DELETE CASCADE,
    FOREIGN KEY (server_id, organization_id)
        REFERENCES servers (id, organization_id) ON DELETE CASCADE,
    FOREIGN KEY (project_id, organization_id)
        REFERENCES projects (id, organization_id) ON DELETE SET NULL (project_id),
    CHECK ((status = 'firing' AND resolved_at IS NULL) OR
           (status = 'resolved' AND resolved_at IS NOT NULL))
);

CREATE UNIQUE INDEX alert_events_active_idx
    ON alert_events (rule_id, server_id, COALESCE(project_id, '00000000-0000-0000-0000-000000000000'::uuid))
    WHERE status = 'firing';

CREATE INDEX alert_events_org_timeline_idx
    ON alert_events (organization_id, triggered_at DESC);

CREATE TABLE alert_notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    event_id UUID NOT NULL,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('firing', 'recovered')),
    title TEXT NOT NULL,
    message TEXT NOT NULL,
    read_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (event_id, organization_id)
        REFERENCES alert_events (id, organization_id) ON DELETE CASCADE,
    UNIQUE (event_id, user_id, kind)
);

CREATE INDEX alert_notifications_user_idx
    ON alert_notifications (organization_id, user_id, created_at DESC);

COMMIT;
