BEGIN;

CREATE TABLE financial_records (
    id UUID PRIMARY KEY,
    application_id UUID NOT NULL REFERENCES applications (id),
    source_document_id UUID,
    confirmed_by_user_id UUID REFERENCES users (id),
    data_version INTEGER NOT NULL,
    period_month DATE NOT NULL,
    revenue_kopecks BIGINT NOT NULL,
    operating_inflow_kopecks BIGINT NOT NULL,
    operating_outflow_kopecks BIGINT NOT NULL,
    tax_outflow_kopecks BIGINT NOT NULL,
    operating_expenses_kopecks BIGINT NOT NULL,
    existing_debt_payment_kopecks BIGINT NOT NULL,
    confirmed_at TIMESTAMPTZ,

    CONSTRAINT financial_records_document_fk FOREIGN KEY (source_document_id, application_id)
        REFERENCES documents (id, application_id),
    CONSTRAINT financial_records_version_check CHECK (data_version > 0),
    CONSTRAINT financial_records_month_check CHECK (isfinite(period_month) AND EXTRACT(DAY FROM period_month) = 1),
    CONSTRAINT financial_records_revenue_check CHECK (revenue_kopecks >= 0),
    CONSTRAINT financial_records_operating_inflow_check CHECK (operating_inflow_kopecks >= 0),
    CONSTRAINT financial_records_operating_outflow_check CHECK (operating_outflow_kopecks >= 0),
    CONSTRAINT financial_records_tax_outflow_check CHECK (tax_outflow_kopecks >= 0),
    CONSTRAINT financial_records_expenses_check CHECK (operating_expenses_kopecks >= 0),
    CONSTRAINT financial_records_debt_check CHECK (existing_debt_payment_kopecks >= 0),
    CONSTRAINT financial_records_period_unique UNIQUE (application_id, data_version, period_month)
);
CREATE INDEX financial_records_document_idx ON financial_records (source_document_id, application_id);
CREATE INDEX financial_records_confirmer_idx ON financial_records (confirmed_by_user_id);

COMMIT;
