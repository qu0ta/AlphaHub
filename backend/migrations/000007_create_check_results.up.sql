BEGIN;

CREATE TABLE check_results (
    id UUID PRIMARY KEY,
    application_id UUID NOT NULL REFERENCES applications (id),
    data_version INTEGER NOT NULL,
    check_type VARCHAR(50) NOT NULL,
    execution_status VARCHAR(50) NOT NULL,
    outcome VARCHAR(50),
    reason_code VARCHAR(100),
    details JSONB NOT NULL DEFAULT '{}'::jsonb,
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ,
    CONSTRAINT check_results_version_check CHECK (data_version > 0),
    CONSTRAINT check_results_type_check CHECK (btrim(check_type) <> ''),
    CONSTRAINT check_results_status_check CHECK (btrim(execution_status) <> ''),
    CONSTRAINT check_results_finished_at_check CHECK (finished_at >= started_at)
);
CREATE INDEX check_results_application_version_idx ON check_results (application_id, data_version);

COMMIT;
