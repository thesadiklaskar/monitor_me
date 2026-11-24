-- Monitor Me is a monitoring-only agent. Host control commands are not part of the product.
-- Existing queued operations and their audit trail are intentionally retired.
BEGIN;
DROP TABLE IF EXISTS remote_operation_audit;
DROP TABLE IF EXISTS remote_operations;
COMMIT;
