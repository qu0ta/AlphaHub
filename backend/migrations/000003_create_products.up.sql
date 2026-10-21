BEGIN;

CREATE TABLE products (
    code VARCHAR(50) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT NOT NULL,
    min_amount_kopecks BIGINT NOT NULL,
    max_amount_kopecks BIGINT NOT NULL,
    min_term_months INTEGER NOT NULL,
    max_term_months INTEGER NOT NULL,
    base_annual_rate_bps INTEGER NOT NULL,
    CONSTRAINT products_code_check CHECK (btrim(code) <> ''),
    CONSTRAINT products_name_check CHECK (btrim(name) <> ''),
    CONSTRAINT products_amount_check CHECK (
        min_amount_kopecks > 0 AND max_amount_kopecks >= min_amount_kopecks
    ),
    CONSTRAINT products_term_check CHECK (
        min_term_months > 0 AND max_term_months >= min_term_months
    ),
    CONSTRAINT products_rate_check CHECK (base_annual_rate_bps >= 0)
);

COMMIT;
