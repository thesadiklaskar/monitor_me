BEGIN;

ALTER TABLE dashboard_panels
    ADD COLUMN label_key TEXT NOT NULL DEFAULT ''
        CHECK (label_key = '' OR label_key ~ '^[A-Za-z][A-Za-z0-9_.-]{0,63}$'),
    ADD COLUMN label_value TEXT NOT NULL DEFAULT ''
        CHECK (length(label_value) <= 256 AND label_value !~ E'[\\r\\n]');

COMMIT;
