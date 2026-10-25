BEGIN;

ALTER TABLE offers
    DROP CONSTRAINT IF EXISTS offers_decline_comment_check,
    DROP CONSTRAINT IF EXISTS offers_decline_reason_check,
    DROP COLUMN IF EXISTS decline_comment,
    DROP COLUMN IF EXISTS decline_reason;

COMMIT;
