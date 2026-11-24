BEGIN;

ALTER TABLE dashboard_panels
    DROP CONSTRAINT IF EXISTS dashboard_panels_visualization_check;

ALTER TABLE dashboard_panels
    ADD CONSTRAINT dashboard_panels_visualization_check
    CHECK (visualization IN ('timeseries', 'stat', 'table', 'bar', 'area', 'gauge', 'heatmap'));

ALTER TABLE dashboard_panels
    DROP COLUMN IF EXISTS grid_width;

COMMIT;
