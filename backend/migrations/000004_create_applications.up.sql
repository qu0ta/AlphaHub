BEGIN;

CREATE TABLE applications (
    id UUID PRIMARY KEY,
    company_id UUID NOT NULL REFERENCES companies (id),
    created_by_user_id UUID NOT NULL REFERENCES users (id),
    product_code VARCHAR(50) NOT NULL REFERENCES products (code),
    assigned_employee_id UUID REFERENCES users (id),
    accepted_offer_id UUID,
    signed_by_user_id UUID REFERENCES users (id),
    requested_amount_kopecks BIGINT NOT NULL,
    requested_term_months INTEGER NOT NULL,
    purpose TEXT NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'DRAFT',
    data_version INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    submitted_at TIMESTAMPTZ,
    signed_at TIMESTAMPTZ,
    disbursed_at TIMESTAMPTZ,
    CONSTRAINT applications_amount_check CHECK (requested_amount_kopecks > 0),
    CONSTRAINT applications_term_check CHECK (requested_term_months > 0),
    CONSTRAINT applications_status_check CHECK (btrim(status) <> ''),
    CONSTRAINT applications_version_check CHECK (data_version > 0),
    CONSTRAINT applications_updated_at_check CHECK (updated_at >= created_at),
    CONSTRAINT applications_submitted_at_check CHECK (submitted_at >= created_at),
    CONSTRAINT applications_signed_at_check CHECK (signed_at >= created_at),
    CONSTRAINT applications_disbursed_at_check CHECK (disbursed_at >= created_at)
);
CREATE INDEX applications_company_idx ON applications (company_id);
CREATE INDEX applications_creator_idx ON applications (created_by_user_id);
CREATE INDEX applications_product_idx ON applications (product_code);
CREATE INDEX applications_employee_idx ON applications (assigned_employee_id);
CREATE INDEX applications_signer_idx ON applications (signed_by_user_id);
CREATE INDEX applications_status_created_idx ON applications (status, created_at DESC);

COMMIT;
