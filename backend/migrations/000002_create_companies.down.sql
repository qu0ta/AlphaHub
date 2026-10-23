BEGIN;

DROP INDEX users_company_idx;
ALTER TABLE users DROP CONSTRAINT users_company_fk;
DROP TABLE companies;

COMMIT;
