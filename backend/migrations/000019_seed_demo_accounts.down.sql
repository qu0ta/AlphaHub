BEGIN;
-- Demo users may already own applications/decisions. Keep them on rollback to avoid data loss.
-- Restoring this migration with up is safe (ON CONFLICT DO NOTHING).
COMMIT;
