BEGIN;

CREATE TABLE dashboards (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    name TEXT NOT NULL CHECK (length(trim(name)) BETWEEN 2 AND 120),
    description TEXT NOT NULL DEFAULT '' CHECK (length(description) <= 500),
    project_id UUID,
    is_default BOOLEAN NOT NULL DEFAULT false,
    created_by UUID REFERENCES users (id) ON DELETE SET NULL,
    updated_by UUID REFERENCES users (id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (id, organization_id),
    FOREIGN KEY (project_id, organization_id)
        REFERENCES projects (id, organization_id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX dashboards_one_default_per_org_idx
    ON dashboards (organization_id) WHERE is_default;
CREATE INDEX dashboards_org_order_idx
    ON dashboards (organization_id, is_default DESC, lower(name));

CREATE TABLE dashboard_panels (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    dashboard_id UUID NOT NULL,
    title TEXT NOT NULL CHECK (length(trim(title)) BETWEEN 2 AND 120),
    visualization TEXT NOT NULL CHECK (visualization IN ('timeseries', 'stat')),
    server_id UUID NOT NULL,
    project_id UUID,
    metric_name TEXT NOT NULL CHECK (metric_name ~ '^[A-Za-z][A-Za-z0-9_.-]{0,127}$'),
    time_range TEXT NOT NULL CHECK (time_range IN ('1h', '6h', '24h', '7d', '30d')),
    position INTEGER NOT NULL DEFAULT 0 CHECK (position BETWEEN 0 AND 1000),
    created_by UUID REFERENCES users (id) ON DELETE SET NULL,
    updated_by UUID REFERENCES users (id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (id, organization_id),
    FOREIGN KEY (dashboard_id, organization_id)
        REFERENCES dashboards (id, organization_id) ON DELETE CASCADE,
    FOREIGN KEY (server_id, organization_id)
        REFERENCES servers (id, organization_id) ON DELETE CASCADE,
    FOREIGN KEY (project_id, organization_id)
        REFERENCES projects (id, organization_id) ON DELETE CASCADE
);

CREATE INDEX dashboard_panels_dashboard_order_idx
    ON dashboard_panels (organization_id, dashboard_id, position, id);
CREATE INDEX dashboard_panels_metric_scope_idx
    ON dashboard_panels (organization_id, server_id, metric_name);

COMMIT;
