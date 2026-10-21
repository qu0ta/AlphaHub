BEGIN;

CREATE TABLE scoring_results (
    id UUID PRIMARY KEY,
    application_id UUID NOT NULL REFERENCES applications (id),
    data_version INTEGER NOT NULL,
    score INTEGER NOT NULL,
    risk_band VARCHAR(50) NOT NULL,
    model_version VARCHAR(50) NOT NULL,
    features_snapshot JSONB NOT NULL,
    explanation JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT scoring_results_version_check CHECK (data_version > 0),
    CONSTRAINT scoring_results_risk_check CHECK (btrim(risk_band) <> ''),
    CONSTRAINT scoring_results_model_check CHECK (btrim(model_version) <> ''),
    CONSTRAINT scoring_results_identity_unique UNIQUE (id, application_id, data_version)
);
CREATE INDEX scoring_results_application_version_idx ON scoring_results (application_id, data_version);

COMMIT;
