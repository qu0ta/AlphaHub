BEGIN;

CREATE TABLE users (
    id UUID PRIMARY KEY,
    fullName VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL,
    password_hash TEXT NOT NULL,
    role VARCHAR(30) NOT NULL DEFAULT 'CLIENT',
    company_id UUID,
    company_position VARCHAR(255),
    CONSTRAINT users_role_check CHECK (role IN ('CLIENT', 'BANK_EMPLOYEE'))
);
CREATE UNIQUE INDEX users_email_unique ON users (LOWER(email));

COMMIT;
