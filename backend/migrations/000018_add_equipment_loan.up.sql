BEGIN;
ALTER TABLE products ADD COLUMN requires_collateral BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE applications ADD COLUMN annual_rate_bps INTEGER CHECK (annual_rate_bps >= 0);
-- Recover historical rates from the actual request where available.
UPDATE applications a SET annual_rate_bps = s.rate
FROM (SELECT DISTINCT ON (application_id) application_id,
 (features_snapshot->>'annual_rate_bps')::integer AS rate
 FROM scoring_results
 WHERE jsonb_typeof(features_snapshot->'annual_rate_bps')='number'
 AND features_snapshot->>'annual_rate_bps' ~ '^[0-9]{1,9}$'
 ORDER BY application_id,created_at DESC,id DESC) s
WHERE a.id=s.application_id;
-- Legacy applications without a scoring snapshot use today's catalog rate.
UPDATE applications a SET annual_rate_bps=p.base_annual_rate_bps
FROM products p WHERE p.code=a.product_code AND a.annual_rate_bps IS NULL;
INSERT INTO products(code,name,description,min_amount_kopecks,max_amount_kopecks,
 min_term_months,max_term_months,base_annual_rate_bps,is_active,requires_collateral)
VALUES ('equipment_loan','Кредит на оборудование','Финансирование покупки оборудования',
 50000000,500000000,12,36,1800,true,false)
ON CONFLICT (code) DO NOTHING;
COMMIT;
