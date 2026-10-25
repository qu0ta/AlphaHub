BEGIN;

-- Причина, по которой клиент отклонил предложение: видна сотруднику банка.
ALTER TABLE offers
    ADD COLUMN decline_reason VARCHAR(20),
    ADD COLUMN decline_comment TEXT,
    ADD CONSTRAINT offers_decline_reason_check
        CHECK (decline_reason IS NULL OR decline_reason IN ('RATE', 'AMOUNT', 'TERM', 'OTHER')),
    ADD CONSTRAINT offers_decline_comment_check
        CHECK (decline_comment IS NULL OR char_length(decline_comment) <= 2000);

COMMIT;
