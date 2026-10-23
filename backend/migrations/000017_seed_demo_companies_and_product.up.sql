BEGIN;

INSERT INTO companies (id, inn, name, business_type, registration_date, main_okved)
VALUES
    ('00000000-0000-4000-8000-007700000016', '7700000016',
     'ООО «Северный Маяк»', 'OOO', '2020-01-01', '62.01'),
    ('00000000-0000-4000-8000-007700000030', '7700000030',
     'ООО «Городские Решения»', 'OOO', '2020-01-01', '62.01')
ON CONFLICT (inn) DO NOTHING;

-- 100,000–5,000,000 RUB, 3–36 months, 19.90% annual rate.
INSERT INTO products (
    code, name, description, min_amount_kopecks, max_amount_kopecks,
    min_term_months, max_term_months, base_annual_rate_bps, is_active
)
VALUES (
    'WORKING_CAPITAL', 'Оборотный кредит',
    'Тестовый продукт для пополнения оборотных средств',
    10000000, 500000000, 3, 36, 1990, true
)
ON CONFLICT (code) DO NOTHING;

COMMIT;
