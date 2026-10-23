BEGIN;

ALTER TABLE financial_records
    ALTER COLUMN operating_expenses_kopecks DROP NOT NULL,
    ALTER COLUMN existing_debt_payment_kopecks DROP NOT NULL;

ALTER TABLE applications
    ADD COLUMN declared_monthly_debt_payment_kopecks BIGINT;

ALTER TABLE applications
    ADD CONSTRAINT applications_declared_debt_check
        CHECK (
            declared_monthly_debt_payment_kopecks IS NULL
                OR declared_monthly_debt_payment_kopecks >= 0
            );

COMMIT;
