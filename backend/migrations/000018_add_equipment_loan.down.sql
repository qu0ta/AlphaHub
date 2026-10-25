BEGIN;
-- Preserve the catalog entry: applications may reference it.
ALTER TABLE applications DROP COLUMN annual_rate_bps;
ALTER TABLE products DROP COLUMN requires_collateral;
COMMIT;
