BEGIN;

DROP TABLE IF EXISTS agent_sessions;
ALTER TABLE agents DROP COLUMN IF EXISTS capabilities;
DROP INDEX IF EXISTS agent_enrollment_tokens_server_idx;
ALTER TABLE agent_enrollment_tokens DROP COLUMN IF EXISTS server_id;
DROP TABLE IF EXISTS user_sessions;
ALTER TABLE users DROP COLUMN IF EXISTS password_hash;

COMMIT;
