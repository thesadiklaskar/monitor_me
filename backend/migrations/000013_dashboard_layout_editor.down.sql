BEGIN;

ALTER TABLE dashboard_panels
    DROP CONSTRAINT IF EXISTS dashboard_panels_visualization_check;

-- Preserve saved panels when older code does not understand newer visual types.
UPDATE dashboard_panels
SET visualization = 'timeseries'
WHERE visualization NOT IN ('timeseries', 'stat', 'table');

ALTER TABLE dashboard_panels
    ADD CONSTRAINT dashboard_panels_visualization_check
    CHECK (visualization IN ('timeseries', 'stat', 'table'));

ALTER TABLE dashboard_panels
    DROP COLUMN IF EXISTS grid_width;

COMMIT;
