-- Add user authentication, scoped server enrollment, and revocable agent sessions.

BEGIN;

ALTER TABLE users
    ADD COLUMN password_hash TEXT NOT NULL DEFAULT '!disabled';

ALTER TABLE users
    ALTER COLUMN password_hash DROP DEFAULT;

CREATE TABLE user_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    organization_id UUID NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    token_hash BYTEA NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    last_seen_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (organization_id, user_id)
        REFERENCES organization_memberships (organization_id, user_id) ON DELETE CASCADE
);

CREATE INDEX user_sessions_expiry_idx
    ON user_sessions (expires_at)
    WHERE revoked_at IS NULL;

ALTER TABLE agent_enrollment_tokens
    ADD COLUMN server_id UUID REFERENCES servers (id) ON DELETE CASCADE;

CREATE INDEX agent_enrollment_tokens_server_idx
    ON agent_enrollment_tokens (server_id, expires_at);

ALTER TABLE agents
    ADD COLUMN capabilities JSONB NOT NULL DEFAULT '[]'::jsonb
        CHECK (jsonb_typeof(capabilities) = 'array');

CREATE TABLE agent_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents (id) ON DELETE CASCADE,
    token_hash BYTEA NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    last_seen_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX agent_sessions_agent_idx
    ON agent_sessions (agent_id, expires_at)
    WHERE revoked_at IS NULL;

COMMIT;
