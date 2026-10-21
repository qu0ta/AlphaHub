BEGIN;

CREATE TABLE info_requests (
    id UUID PRIMARY KEY,
    application_id UUID NOT NULL REFERENCES applications (id),
    created_by_user_id UUID REFERENCES users (id),
    description TEXT NOT NULL,
    required_items JSONB NOT NULL,
    status VARCHAR(50) NOT NULL,
    response_comment TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    responded_at TIMESTAMPTZ,
    resolved_at TIMESTAMPTZ,
    CONSTRAINT info_requests_description_check CHECK (btrim(description) <> ''),
    CONSTRAINT info_requests_status_check CHECK (btrim(status) <> ''),
    CONSTRAINT info_requests_responded_at_check CHECK (responded_at >= created_at),
    CONSTRAINT info_requests_resolved_at_check CHECK (resolved_at >= created_at)
);
CREATE INDEX info_requests_application_status_idx ON info_requests (application_id, status);
CREATE INDEX info_requests_creator_idx ON info_requests (created_by_user_id);

COMMIT;
