ALTER TABLE applications
    DROP CONSTRAINT IF EXISTS applications_required_data_check;

ALTER TABLE applications
    ALTER COLUMN company_id SET NOT NULL,
    ALTER COLUMN requested_amount_kopecks SET NOT NULL,
    ALTER COLUMN requested_term_months SET NOT NULL;
