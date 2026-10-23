
CREATE TABLE company_invitations (
                                     token_hash TEXT PRIMARY KEY,
                                     company_id UUID NOT NULL
                                         REFERENCES companies(id),
                                     created_by_user_id UUID NOT NULL
                                         REFERENCES users(id),
                                     used_by_user_id UUID
                                         REFERENCES users(id),
                                     expires_at TIMESTAMPTZ NOT NULL,
                                     used_at TIMESTAMPTZ,
                                     created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX company_invitations_company_idx
    ON company_invitations(company_id);
