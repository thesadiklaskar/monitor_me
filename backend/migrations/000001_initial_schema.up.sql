-- Establish the minimum identity and inventory schema for the control plane.
-- Telemetry partitions and high-volume metric tables are introduced in Phase 3.

CREATE TABLE organizations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL CHECK (length(trim(name)) > 0),
    slug TEXT NOT NULL CHECK (slug ~ '^[a-z0-9][a-z0-9-]{1,62}[a-z0-9]$'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (slug)
);

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT NOT NULL CHECK (length(trim(email)) > 3),
    display_name TEXT NOT NULL CHECK (length(trim(display_name)) > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX users_email_lower_unique ON users (lower(email));

CREATE TABLE organization_memberships (
    organization_id UUID NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (role IN ('owner', 'admin', 'operator', 'viewer')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (organization_id, user_id)
);

CREATE TABLE servers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    name TEXT NOT NULL CHECK (length(trim(name)) > 0),
    hostname TEXT,
    operating_system TEXT,
    architecture TEXT,
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'online', 'stale', 'offline', 'disabled')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (organization_id, name),
    UNIQUE (id, organization_id)
);

CREATE TABLE agents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    server_id UUID NOT NULL UNIQUE REFERENCES servers (id) ON DELETE CASCADE,
    version TEXT,
    public_key_fingerprint TEXT UNIQUE,
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'online', 'stale', 'offline', 'revoked')),
    last_seen_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE agent_enrollment_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    token_hash BYTEA NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    created_by UUID REFERENCES users (id) ON DELETE SET NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (consumed_at IS NULL OR revoked_at IS NULL)
);

CREATE INDEX agent_enrollment_tokens_expiry_idx
    ON agent_enrollment_tokens (expires_at)
    WHERE consumed_at IS NULL AND revoked_at IS NULL;

CREATE TABLE projects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    name TEXT NOT NULL CHECK (length(trim(name)) > 0),
    slug TEXT NOT NULL CHECK (slug ~ '^[a-z0-9][a-z0-9-]{1,62}[a-z0-9]$'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (organization_id, slug),
    UNIQUE (id, organization_id)
);

CREATE TABLE server_project_mappings (
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

CREATE INDEX servers_organization_status_idx ON servers (organization_id, status);
CREATE INDEX agents_status_last_seen_idx ON agents (status, last_seen_at);
CREATE INDEX projects_organization_idx ON projects (organization_id);
