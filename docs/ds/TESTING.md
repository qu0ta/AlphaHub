# Проверка подключения Go → DS

В backend/.env: DS_BASE_URL=http://localhost:8001 (без /api/v1).
Перед запуском примените актуальные миграции по корневому README.
Перезапустите Go после изменения кода/env. Все запросы к Go ниже требуют Bearer JWT клиента.

1. Создать черновик: POST /api/v1/applications с {"product_code":"<код продукта>"}.
2. PATCH /api/v1/applications/{id}:

```json
{
  "company_id": "<UUID компании пользователя>",
  "requested_amount_kopecks": 50000000,
  "requested_term_months": 12,
  "purpose": "Оборотные средства",
  "data_version": 1
}
```

Сумма и срок должны попадать в ограничения продукта. ИНН компании обязан совпадать с PDF.
Ставка фиксируется из каталога в applications.annual_rate_bps при создании заявки; при расчёте используются эта ставка и первоначальная дата подачи в UTC.
Отдельно отправлять эти четыре поля в extract не нужно.

3. POST /api/v1/applications/{id}/documents: Body → form-data, file типа File, выбрать PDF.
   Content-Type вручную не задавать. Ожидается 201, processing_status=PROCESSED.
4. GET /api/v1/applications/{id}/financials: проверить извлечение и актуальную data_version.
5. Если нужны исправления: PUT на тот же URL с data_version из GET, financials и
   declared_monthly_debt_payment_kopecks. После исправления версия увеличивается.
6. POST /api/v1/applications/{id}/financials/confirm с JSON {"data_version":N}, где N — актуальная версия заявки.
7. POST /api/v1/applications/{id}/submit без тела. Итоговый статус — REJECTED при score < 40, MANUAL при 40–69 либо OFFERED при score ≥ 70. Проверенные демо-условия v2 — в [DEMO_DATA.md](../DEMO_DATA.md).
8. GET /api/v1/applications/{id}/scoring: score, explanation, model_version и features_snapshot.

Для подачи в октябре 2026 нужны месяцы апрель–сентябрь 2026. При другой дате нужен
соответствующий набор шести завершённых месяцев. Исправление заявки после загрузки
увеличивает data_version: финансы нужно заново сохранить через PUT и подтвердить.

При несовпадении ИНН — 422, при изменении заявки во время извлечения — 409.
Без подтверждённых финансов текущей версии submit возвращает 422.
Если DS недоступен — 503; его серверная ошибка — 502. При ошибке скоринга заявка
остаётся DRAFT; после восстановления DS повторите submit. Уже поданную заявку
повторный submit не пересчитывает (409). Высокий score не означает одобрения.

## Автоматическая проверка

```powershell
cd C:\Users\Станислав\GolandProjects\AlphaHub\backend
go test ./...
$env:TEST_DATABASE_URL = "<URL локального тестового PostgreSQL>"
go test ./internal/integration -run TestDSWorkflow -count=1 -v
```

Интеграционный тест создаёт отдельную временную схему, применяет в ней миграции и
удаляет схему после проверки. Рабочие записи не используются. DS заменён HTTP-сервером
httptest: проверяются multipart, сборка JSON, подтверждение, сохранение snapshot,
откат при ошибке DS, повторная подача и запрет чтения чужого результата.

Для живого скоринга:

```powershell
$env:DS_BASE_URL = "http://localhost:8001"
go test ./internal/dsclient -run '^TestLiveScoring$' -count=1 -v
```

Этот тест отправляет фиксированные месяцы апрель–сентябрь 2026 и ожидает score=90. Для проверки текущих PDF и опубликованных примеров из ds/ выполните `python -m unittest test_demo_scenarios`.
