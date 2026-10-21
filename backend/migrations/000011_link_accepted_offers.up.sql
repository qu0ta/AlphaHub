BEGIN;

ALTER TABLE applications ADD CONSTRAINT applications_accepted_offer_fk
    FOREIGN KEY (accepted_offer_id) REFERENCES offers (id);
CREATE UNIQUE INDEX applications_accepted_offer_unique
    ON applications (accepted_offer_id) WHERE accepted_offer_id IS NOT NULL;

-- A simple FK does not prevent accepting an offer from another application.
CREATE FUNCTION check_accepted_offer_application() RETURNS TRIGGER
LANGUAGE plpgsql AS $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM applications a
        JOIN offers o ON o.id = a.accepted_offer_id
        JOIN decisions d ON d.id = o.decision_id
        WHERE d.application_id <> a.id AND (
            (TG_TABLE_NAME = 'applications' AND a.id = NEW.id) OR
            (TG_TABLE_NAME = 'offers' AND o.id = NEW.id) OR
            (TG_TABLE_NAME = 'decisions' AND d.id = NEW.id)
        )
    ) THEN
        RAISE EXCEPTION 'Accepted offer must belong to the same application'
            USING ERRCODE = '23514', CONSTRAINT = 'accepted_offer_application_check';
    END IF;
    RETURN NULL;
END;
$$;
CREATE CONSTRAINT TRIGGER applications_offer_application_check
    AFTER INSERT OR UPDATE ON applications DEFERRABLE INITIALLY IMMEDIATE
    FOR EACH ROW EXECUTE FUNCTION check_accepted_offer_application();
CREATE CONSTRAINT TRIGGER offers_application_check
    AFTER UPDATE ON offers DEFERRABLE INITIALLY IMMEDIATE
    FOR EACH ROW EXECUTE FUNCTION check_accepted_offer_application();
CREATE CONSTRAINT TRIGGER decisions_offer_application_check
    AFTER UPDATE ON decisions DEFERRABLE INITIALLY IMMEDIATE
    FOR EACH ROW EXECUTE FUNCTION check_accepted_offer_application();

-- Validate rows that might have been inserted before this migration.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM applications a
        JOIN offers o ON o.id = a.accepted_offer_id
        JOIN decisions d ON d.id = o.decision_id
        WHERE d.application_id <> a.id
    ) THEN
        RAISE EXCEPTION 'Existing accepted offer belongs to another application'
            USING ERRCODE = '23514';
    END IF;
END;
$$;

COMMIT;
