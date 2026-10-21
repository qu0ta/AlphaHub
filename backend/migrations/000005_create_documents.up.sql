BEGIN;

CREATE TABLE documents (
    id UUID PRIMARY KEY,
    application_id UUID NOT NULL REFERENCES applications (id),
    uploaded_by_user_id UUID NOT NULL REFERENCES users (id),
    document_type VARCHAR(50) NOT NULL,
    original_filename VARCHAR(255) NOT NULL,
    storage_key VARCHAR(1024) NOT NULL,
    processing_status VARCHAR(50) NOT NULL,
    uploaded_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT documents_type_check CHECK (btrim(document_type) <> ''),
    CONSTRAINT documents_filename_check CHECK (btrim(original_filename) <> ''),
    CONSTRAINT documents_storage_key_check CHECK (btrim(storage_key) <> ''),
    CONSTRAINT documents_status_check CHECK (btrim(processing_status) <> ''),
    CONSTRAINT documents_id_application_unique UNIQUE (id, application_id)
);
CREATE INDEX documents_application_uploaded_idx ON documents (application_id, uploaded_at DESC);
CREATE INDEX documents_uploader_idx ON documents (uploaded_by_user_id);

COMMIT;
