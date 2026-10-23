BEGIN;

CREATE TABLE decisions (
    id UUID PRIMARY KEY,
    application_id UUID NOT NULL REFERENCES applications (id),
    scoring_id UUID,
    decided_by_user_id UUID REFERENCES users (id),
    data_version INTEGER NOT NULL,
    decision_type VARCHAR(50) NOT NULL,
    decision_source VARCHAR(50) NOT NULL,
    reason_code VARCHAR(100) NOT NULL,
    comment TEXT NOT NULL,
    policy_version VARCHAR(50) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT decisions_scoring_fk FOREIGN KEY (scoring_id, application_id, data_version)
        REFERENCES scoring_results (id, application_id, data_version),
    CONSTRAINT decisions_version_check CHECK (data_version > 0),
    CONSTRAINT decisions_type_check CHECK (btrim(decision_type) <> ''),
    CONSTRAINT decisions_source_check CHECK (btrim(decision_source) <> ''),
    CONSTRAINT decisions_policy_check CHECK (btrim(policy_version) <> '')
);
CREATE INDEX decisions_application_version_idx ON decisions (application_id, data_version);
CREATE INDEX decisions_scoring_idx ON decisions (scoring_id, application_id, data_version);
CREATE INDEX decisions_author_idx ON decisions (decided_by_user_id);

COMMIT;
