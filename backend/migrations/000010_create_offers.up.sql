BEGIN;

CREATE TABLE offers (
    id UUID PRIMARY KEY,
    decision_id UUID NOT NULL REFERENCES decisions (id),
    responded_by_user_id UUID REFERENCES users (id),
    version INTEGER NOT NULL,
    amount_kopecks BIGINT NOT NULL,
    term_months INTEGER NOT NULL,
    annual_rate_bps INTEGER NOT NULL,
    monthly_payment_kopecks BIGINT NOT NULL,
    status VARCHAR(50) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    valid_until TIMESTAMPTZ NOT NULL,
    responded_at TIMESTAMPTZ,
    CONSTRAINT offers_version_check CHECK (version > 0),
    CONSTRAINT offers_amount_check CHECK (amount_kopecks > 0),
    CONSTRAINT offers_term_check CHECK (term_months > 0),
    CONSTRAINT offers_rate_check CHECK (annual_rate_bps >= 0),
    CONSTRAINT offers_payment_check CHECK (monthly_payment_kopecks >= 0),
    CONSTRAINT offers_status_check CHECK (btrim(status) <> ''),
    CONSTRAINT offers_valid_until_check CHECK (valid_until > created_at),
    CONSTRAINT offers_responded_at_check CHECK (responded_at >= created_at),
    CONSTRAINT offers_decision_version_unique UNIQUE (decision_id, version)
);
CREATE INDEX offers_responder_idx ON offers (responded_by_user_id);

COMMIT;
