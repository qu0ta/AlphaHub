# Модель данных PostgreSQL

Источник — `backend/migrations/000001..000019`. Схема показывает таблицы предметной области; служебная `schema_migrations(version, dirty)` управляется golang-migrate.

```mermaid
erDiagram
 companies o|--o{ users : company_id
 companies o|--o{ applications : company_id
 companies ||--o{ company_invitations : company_id
 users ||--o{ company_invitations : created_by_or_used_by
 users ||--o{ applications : creator_employee_signer
 products ||--o{ applications : product_code
 applications ||--o{ documents : application_id
 users ||--o{ documents : uploaded_by
 applications ||--o{ financial_records : application_id
 documents o|--o{ financial_records : source_document
 users o|--o{ financial_records : confirmed_by
 applications ||--o{ check_results : application_id
 applications ||--o{ scoring_results : application_id
 applications ||--o{ decisions : application_id
 scoring_results o|--o{ decisions : scoring_and_version
 users o|--o{ decisions : decided_by
 decisions ||--o{ offers : decision_id
 users o|--o{ offers : responded_by
 offers o|--o{ applications : accepted_offer_id
 applications ||--o{ info_requests : application_id
 users o|--o{ info_requests : created_by
 applications ||--o{ status_history : application_id
 users o|--o{ status_history : changed_by
 companies {
  uuid id PK
  varchar inn UK
  varchar name
  varchar business_type
  date registration_date
  varchar main_okved
 }
 users {
  uuid id PK
  varchar fullname
  varchar email UK "unique lower(email)"
  text password_hash
  varchar role
  uuid company_id FK "nullable"
  varchar company_position
 }
 products {
  varchar code PK
  varchar name
  text description
  bigint min_amount_kopecks
  bigint max_amount_kopecks
  int min_term_months
  int max_term_months
  int base_annual_rate_bps
  boolean is_active
  boolean requires_collateral
 }
 applications {
  uuid id PK
  uuid company_id FK "nullable draft"
  uuid created_by_user_id FK
  varchar product_code FK
  uuid assigned_employee_id FK
  uuid accepted_offer_id FK
  uuid signed_by_user_id FK
  bigint requested_amount_kopecks
  int requested_term_months
  int annual_rate_bps "frozen product rate"
  bigint declared_monthly_debt_payment_kopecks
  text purpose
  varchar status
  int data_version
  timestamptz created_at
  timestamptz updated_at
  timestamptz submitted_at
  timestamptz signed_at
  timestamptz disbursed_at
 }
 documents {
  uuid id PK
  uuid application_id FK
  uuid uploaded_by_user_id FK
  varchar document_type
  varchar original_filename
  varchar storage_key
  varchar processing_status
  timestamptz uploaded_at
 }
 financial_records {
  uuid id PK
  uuid application_id FK
  uuid source_document_id FK
  uuid confirmed_by_user_id FK
  int data_version
  date period_month
  bigint revenue_kopecks
  bigint operating_inflow_kopecks
  bigint operating_outflow_kopecks
  bigint tax_outflow_kopecks
  bigint operating_expenses_kopecks
  bigint existing_debt_payment_kopecks
  timestamptz confirmed_at
 }
 check_results {
  uuid id PK
  uuid application_id FK
  int data_version
  varchar check_type
  varchar execution_status
  varchar outcome
  varchar reason_code
  jsonb details
  timestamptz started_at
  timestamptz finished_at
 }
 scoring_results {
  uuid id PK
  uuid application_id FK
  int data_version
  int score
  varchar risk_band
  varchar model_version
  jsonb features_snapshot
  jsonb explanation
  timestamptz created_at
 }
 decisions {
  uuid id PK
  uuid application_id FK
  uuid scoring_id FK
  uuid decided_by_user_id FK
  int data_version
  varchar decision_type
  varchar decision_source
  varchar reason_code
  text comment
  varchar policy_version
  timestamptz created_at
 }
 offers {
  uuid id PK
  uuid decision_id FK
  uuid responded_by_user_id FK
  int version
  bigint amount_kopecks
  int term_months
  int annual_rate_bps
  bigint monthly_payment_kopecks
  varchar status
  timestamptz created_at
  timestamptz valid_until
  timestamptz responded_at
 }
 info_requests {
  uuid id PK
  uuid application_id FK
  uuid created_by_user_id FK
  text description
  jsonb required_items
  varchar status
  text response_comment
  timestamptz created_at
  timestamptz responded_at
  timestamptz resolved_at
 }
 status_history {
  uuid id PK
  uuid application_id FK
  uuid changed_by_user_id FK
  varchar from_status
  varchar to_status
  text reason
  timestamptz created_at
 }
 company_invitations {
  text token_hash PK
  uuid company_id FK
  uuid created_by_user_id FK
  uuid used_by_user_id FK
  timestamptz expires_at
  timestamptz used_at
  timestamptz created_at
 }
```

Основные ограничения: INN 10/12 цифр; суммы неотрицательные/положительные по смыслу; срок положительный; балл 0..100; финансовый месяц — первое число. Финансовая строка уникальна по `(application_id,data_version,period_month)`.
Составные FK связывают `(source_document_id,application_id)` с документом именно этой заявки и `(scoring_id,application_id,data_version)` с соответствующим скорингом. Accepted offer ссылается на offers; принадлежность оффера заявке дополнительно проверяет Go в транзакции.

Миграция 14 допускает незаполненный DRAFT, 16 добавляет долг, 17 — демо-компании/продукт, 18 — requires_collateral, сохранённую ставку заявки и equipment_loan, 19 — демо-аккаунты. `company_invitations` существует в БД, но маршруты приглашений не подключены в текущем main.go.
