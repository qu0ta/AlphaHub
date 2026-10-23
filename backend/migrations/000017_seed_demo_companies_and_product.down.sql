-- Intentionally preserve seed data on rollback.
-- These companies/products may already have existed before this migration,
-- or may now be referenced by users, applications and invitations.
-- Deleting by INN/code would remove user data or break foreign keys.
-- Reapplying up is safe: existing records are not changed.
BEGIN;
SELECT 1;
COMMIT;
