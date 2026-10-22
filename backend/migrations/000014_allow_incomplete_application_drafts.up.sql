ALTER TABLE applications
    ALTER COLUMN company_id DROP NOT NULL,
    ALTER COLUMN requested_amount_kopecks DROP NOT NULL,
    ALTER COLUMN requested_term_months DROP NOT NULL;

ALTER TABLE applications
    ADD CONSTRAINT applications_required_data_check
        CHECK (
            status = 'DRAFT'
                OR (
                company_id IS NOT NULL
                    AND requested_amount_kopecks IS NOT NULL
                    AND requested_term_months IS NOT NULL
                    AND length(btrim(purpose)) > 0
                )
            );
