-- Normalize existing saved panel order so move-up/down operates on dense indexes.
BEGIN;
WITH ranked AS (
    SELECT id,
           row_number() OVER (PARTITION BY organization_id, dashboard_id ORDER BY position, created_at, id) - 1 AS next_position
    FROM dashboard_panels
)
UPDATE dashboard_panels panel
SET position = ranked.next_position, updated_at = now()
FROM ranked
WHERE panel.id = ranked.id
  AND panel.position <> ranked.next_position;
COMMIT;
