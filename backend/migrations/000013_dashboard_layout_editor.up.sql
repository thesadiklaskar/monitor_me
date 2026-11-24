BEGIN;

ALTER TABLE dashboard_panels
    ADD COLUMN grid_width SMALLINT NOT NULL DEFAULT 6
    CHECK (grid_width IN (4, 6, 8, 12));

ALTER TABLE dashboard_panels
    DROP CONSTRAINT IF EXISTS dashboard_panels_visualization_check;

ALTER TABLE dashboard_panels
    ADD CONSTRAINT dashboard_panels_visualization_check
    CHECK (visualization IN ('timeseries', 'stat', 'table', 'bar', 'area', 'gauge', 'heatmap', 'logs'));

COMMIT;
