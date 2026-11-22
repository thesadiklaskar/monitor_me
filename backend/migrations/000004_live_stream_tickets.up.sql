BEGIN;

CREATE TABLE live_stream_tickets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_session_id UUID NOT NULL REFERENCES user_sessions (id) ON DELETE CASCADE,
    organization_id UUID NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    token_hash BYTEA NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX live_stream_tickets_expiry_idx
    ON live_stream_tickets (expires_at)
    WHERE consumed_at IS NULL;

COMMIT;
