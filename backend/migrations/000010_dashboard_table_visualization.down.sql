BEGIN;
UPDATE dashboard_panels SET visualization = 'timeseries' WHERE visualization = 'table';
ALTER TABLE dashboard_panels
    DROP CONSTRAINT IF EXISTS dashboard_panels_visualization_check;
ALTER TABLE dashboard_panels
    ADD CONSTRAINT dashboard_panels_visualization_check
    CHECK (visualization IN ('timeseries', 'stat'));
COMMIT;
