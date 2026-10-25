BEGIN;
-- Public demo credentials only: AlphaHubTest2026! (bcrypt cost 10).
-- Existing users are never overwritten; look companies up by INN rather than assuming their UUIDs.
INSERT INTO users (id, fullname, email, password_hash, role, company_id, company_position)
SELECT seed.id::uuid, seed.name, seed.email,
       '$2a$10$kf5unm5BDqQD/LF9rvNDmOfWJHLlbo4tOK3xllXP5F5ea4BdVfJHG',
       seed.role, c.id, seed.position
FROM (VALUES
 ('10000000-0000-4000-8000-000000000001', 'Демо сотрудник', 'bank@demo.ru', 'BANK_EMPLOYEE', NULL, 'Кредитный специалист'),
 ('10000000-0000-4000-8000-000000000002', 'Демо клиент', 'client@demo.ru', 'CLIENT', '7700000016', 'Генеральный директор'),
 ('10000000-0000-4000-8000-000000000003', 'Демо клиент Вектор', 'medium@demo.ru', 'CLIENT', '7700000023', 'Генеральный директор'),
 ('10000000-0000-4000-8000-000000000004', 'Демо клиент Решения', 'risky@demo.ru', 'CLIENT', '7700000030', 'Генеральный директор')
) AS seed(id, name, email, role, inn, position)
LEFT JOIN companies c ON c.inn=seed.inn
ON CONFLICT DO NOTHING;
COMMIT;
