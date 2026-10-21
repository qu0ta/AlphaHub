BEGIN;

CREATE TABLE status_history (
    id UUID PRIMARY KEY,
    application_id UUID NOT NULL REFERENCES applications (id),
    changed_by_user_id UUID REFERENCES users (id),
    from_status VARCHAR(50),
    to_status VARCHAR(50) NOT NULL,
    reason TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT status_history_from_check CHECK (from_status IS NULL OR btrim(from_status) <> ''),
    CONSTRAINT status_history_to_check CHECK (btrim(to_status) <> ''),
    CONSTRAINT status_history_transition_check CHECK (from_status IS DISTINCT FROM to_status)
);
CREATE INDEX status_history_application_created_idx ON status_history (application_id, created_at, id);
CREATE INDEX status_history_author_idx ON status_history (changed_by_user_id);

COMMIT;
