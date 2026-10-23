BEGIN;

CREATE TABLE companies (
    id UUID PRIMARY KEY,
    inn VARCHAR(12) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    business_type VARCHAR(50) NOT NULL,
    registration_date DATE NOT NULL,
    main_okved VARCHAR(20) NOT NULL,
    CONSTRAINT companies_inn_check CHECK (inn ~ '^([0-9]{10}|[0-9]{12})$'),
    CONSTRAINT companies_name_check CHECK (btrim(name) <> ''),
    CONSTRAINT companies_business_type_check CHECK (btrim(business_type) <> ''),
    CONSTRAINT companies_main_okved_check CHECK (btrim(main_okved) <> '')
);
ALTER TABLE users ADD CONSTRAINT users_company_fk
    FOREIGN KEY (company_id) REFERENCES companies (id);
CREATE INDEX users_company_idx ON users (company_id);

COMMIT;
