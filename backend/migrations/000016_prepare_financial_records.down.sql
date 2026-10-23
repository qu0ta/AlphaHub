
BEGIN;

ALTER TABLE applications
    DROP CONSTRAINT IF EXISTS applications_declared_debt_check;

ALTER TABLE applications
    DROP COLUMN IF EXISTS declared_monthly_debt_payment_kopecks;

-- Возвращать NOT NULL можно только после проверки данных.
-- Здесь намеренно нет SET NOT NULL: он способен сломать откат
-- при наличии уже распознанных неполных записей.

COMMIT;
