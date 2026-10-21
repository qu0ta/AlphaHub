BEGIN;

DROP TRIGGER decisions_offer_application_check ON decisions;
DROP TRIGGER offers_application_check ON offers;
DROP TRIGGER applications_offer_application_check ON applications;
DROP FUNCTION check_accepted_offer_application();
DROP INDEX applications_accepted_offer_unique;
ALTER TABLE applications DROP CONSTRAINT applications_accepted_offer_fk;

COMMIT;
