BEGIN;

ALTER TABLE dashboard_panels
    DROP COLUMN IF EXISTS label_key,
    DROP COLUMN IF EXISTS label_value;

COMMIT;
