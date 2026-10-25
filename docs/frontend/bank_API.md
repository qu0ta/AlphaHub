# AlphaHub — Bank API

Сверено с текущим backend 25 октября 2026 года. API предназначен для сотрудников банка.

## Авторизация

Authorization: Bearer <access_token>, роль BANK_EMPLOYEE. Без токена — 401, для CLIENT — 403. Успех: application/json без обёртки data. Ответы банковских обработчиков содержат Cache-Control: no-store. Ошибки — text/plain; charset=utf-8 с завершающим переводом строки.

## GET /api/v1/bank/applications

Очередь ручного рассмотрения: только status=MANUAL. Ответ 200 — массив; пустая очередь — []. Сортировка: время последнего перехода в MANUAL по убыванию (без времени — в конце), затем UUID по убыванию.

Поля строки: id, company_name, inn, product_code, requested_amount_kopecks (integer/null), requested_term_months (integer/null), status, score (integer/null), submitted_at (RFC 3339/null), manual_review_started_at (RFC 3339/null), assigned_employee_id (UUID/null).

Скоринг выбирается для текущей data_version. Для поиска и пагинации используйте /api/v1/bank/applications/page; этот старый маршрут сохраняет ответ-массив без пагинации.

## GET /api/v1/bank/applications/{id}

Карточка поданной заявки. Поддерживаются все статусы, кроме DRAFT, при заполненном submitted_at. Черновик, неподанная или отсутствующая заявка возвращают одинаковый 404. Доступ есть у любого сотрудника с ролью BANK_EMPLOYEE; назначение сотрудника выполняется отдельной операцией claim. Чтение не назначает заявку сотруднику и не меняет статус.

Параметр id должен быть UUID.

| Поле ответа | Тип | Содержание |
|---|---|---|
| application | object | Application: идентификаторы, product_code, сумма/срок/цель, статус, data_version, даты; тот же DTO, что в API заявок |
| company | object/null | id, inn, name, business_type, registration_date (YYYY-MM-DD), main_okved |
| product | object | code, name, base_annual_rate_bps |
| declared_monthly_debt_payment_kopecks | integer/null | Заявленный ежемесячный платёж по долгам |
| financials | array | Только записи текущей data_version, по возрастанию месяца |
| scoring | object/null | Последний скоринг текущей data_version |
| documents | array | Метаданные всех документов заявки, новые первыми |
| checks | array | Сохранённые проверки текущей data_version, новые первыми |
| history | array | История статусов, по created_at и id по возрастанию |
| info_requests | array | Запросы допданных, новые первыми |
| decisions | array | Все сохранённые решения по заявке, новые первыми |
| offers | array | Предложения из решений заявки, новые первыми |

Все разделы читаются одним SQL-запросом из согласованного снимка БД. Пустые коллекции возвращаются как [], отсутствующий скоринг — null. Старые финансы и скоринг не подставляются вместо данных текущей версии. Компания и продукт отражают текущие справочники, а не исторические снимки на момент подачи.

### Финансы

Каждая запись: period_month (YYYY-MM), data_version, revenue_kopecks, operating_inflow_kopecks, operating_outflow_kopecks, tax_outflow_kopecks, confirmed_at (RFC 3339/null), confirmed_by_user_id (UUID/null).

Деньги — целые копейки int64. Набор возвращается как хранится в БД; карточка не дополняет пропуски и не рассчитывает показатели за 12 месяцев. Остаток кредитов и просрочки этим API не вычисляются.

### Скоринг

id, data_version, score (целое, включая 0), model_version, explanation, created_at. explanation содержит positive_factors и negative_factors — массивы строк. Score не является одобрением банка. features_snapshot не включается в ответ карточки.

### Документы

id, application_id, document_type, original_filename, processing_status, uploaded_at. storage_key и файловый путь не выдаются. Карточка возвращает метаданные; PDF открывается через GET /api/v1/bank/applications/{id}/documents/{documentID}/download.

### Проверки

id, data_version, check_type, execution_status, outcome (string/null), reason_code (string/null), details (JSON), started_at, finished_at (RFC 3339/null). Возвращаются только реально сохранённые результаты. Пустой массив не означает, что проверки выполнены или пройдены. API не вызывает ЕГРЮЛ/БКИ и не создаёт положительные проверки из макета.

### История

id, changed_by_user_id (UUID/null), from_status (string/null), to_status, reason, created_at. История включает все версии заявки.

### Пример PowerShell

```powershell
$base = 'http://localhost:8080/api/v1'
$headers = @{ Authorization = "Bearer $bankToken" }
$queue = Invoke-RestMethod "$base/bank/applications" -Headers $headers
if ($queue.Count -gt 0) {
    $id = $queue[0].id
    Invoke-RestMethod "$base/bank/applications/$id" -Headers $headers |
        ConvertTo-Json -Depth 12
}
```

### Ошибки карточки

| HTTP | Текст | Причина |
|---|---|---|
| 400 | Invalid bank application input | Некорректный UUID |
| 401 | Зависит от auth middleware | Нет токена, токен неверен или истёк |
| 403 | Access denied | Не BANK_EMPLOYEE |
| 404 | Application not found | Нет заявки либо она ещё не подана/имеет статус DRAFT |
| 500 | Internal server error | Ошибка чтения или декодирования данных |

## POST /api/v1/bank/applications/{id}/claim

Берёт заявку MANUAL в работу. Требует BANK_EMPLOYEE; ID сотрудника берётся только из JWT sub. Его существование и актуальная роль дополнительно проверяются в БД. Передавать сотрудника в JSON нельзя.

```json
{"data_version": 2}
```

Передавайте текущую версию из application.data_version карточки. Версия обязательна, целое число не меньше 1. Тело ограничено 16 KiB; неизвестные поля и несколько JSON-значений отклоняются.

Ответ 200:

```json
{
  "application_id": "33333333-3333-4333-8333-333333333333",
  "assigned_employee_id": "88888888-8888-4888-8888-888888888881",
  "data_version": 2,
  "status": "MANUAL",
  "updated_at": "2026-10-25T15:00:00Z"
}
```

Назначение выполняется в транзакции под FOR UPDATE. При одновременном запросе двух сотрудников заявку получает один, второй получает 409. Повторный запрос того же сотрудника с актуальной версией возвращает 200, не меняя updated_at. Нельзя переназначить занятую заявку на себя.

data_version и статус MANUAL остаются прежними: назначение не изменяет финансовые данные. В очередь заявка продолжает попадать с заполненным assigned_employee_id; фронтенд может показать исполнителя и отключить кнопку для занятой заявки. История переходов статусов при назначении не пополняется, поскольку статус не меняется. Освобождение и передача доступны через отдельный POST assignment, описанный ниже.

| HTTP | Текст | Причина |
|---|---|---|
| 400 | Invalid request body | Пустое/некорректное тело, неизвестные поля, превышение лимита |
| 400 | Invalid bank application input | Некорректный UUID или отсутствующий/null/неположительный data_version |
| 401 | Зависит от auth middleware | Нет действительного токена |
| 403 | Access denied | Не BANK_EMPLOYEE либо сотрудник отсутствует/его роль отозвана в БД |
| 404 | Application not found | Заявка отсутствует, не подана или является черновиком |
| 409 | Application is not in manual review | Статус отличается от MANUAL |
| 409 | Application version conflict | Устаревшая версия |
| 409 | Application is assigned to another employee | Заявка уже занята другим сотрудником |
| 500 | Internal server error | Ошибка БД |

Пример PowerShell:

```powershell
$body = @{ data_version = $detail.application.data_version } | ConvertTo-Json
Invoke-RestMethod -Method Post "$base/bank/applications/$id/claim" `
    -Headers $headers -ContentType 'application/json' -Body $body
```

На 409 обновите карточку. Автоматически перезаписывать назначение или подставлять новую версию нельзя.

## Автоматические решения при подаче

Ручные claim/decisions работают только с MANUAL. При подаче заявки score 0–39 создаёт автоматический отказ (REJECTED), 40–69 направляется в MANUAL, 70–100 создаёт автоматическое предложение (OFFERED) без ручного рассмотрения.

Для автоматического предложения проверяются активный продукт и его лимиты, используются сумма/срок заявки и base_annual_rate_bps. Предложение PENDING действует 7 суток, платёж вычисляется тем же расчётом, что для ручных предложений. Клиент ещё должен принять предложение: автоматическая выдача не выполняется.

В decisions: decision_source=AUTO, decided_by_user_id=null, policy_version=score-routing-v1, reason_code=LOW_SCORE или HIGH_SCORE. Решение привязано к сохранённому скорингу текущей версии. В банковской карточке видны как автоматические, так и ручные решения. Автоматические заявки не попадают в очередь MANUAL, assigned_employee_id не назначается.

Сохранение скоринга, решения, предложения, статуса и истории атомарно. Внешние проверки не вызываются и не считаются успешно выполненными. risk_band в скоринге остаётся UNASSESSED.

## POST /api/v1/bank/applications/{id}/decisions

Решение назначенного сотрудника по заявке MANUAL. ID сотрудника берётся из JWT; актуальная роль проверяется также в БД. Сначала выполните claim. Автором решения может быть только assigned_employee_id.

Общие обязательные поля: data_version (текущая версия заявки, integer >= 1), action, reason_code (непустая строка до 100 символов), comment (непустая строка до 2000 символов). Пробелы по краям reason_code/comment удаляются. Тело ограничено 16 KiB, неизвестные поля и несколько JSON-значений запрещены.

### Одобрение запрошенных условий

```json
{
  "data_version": 2,
  "action": "APPROVE",
  "reason_code": "MANUAL_APPROVAL",
  "comment": "Условия проверены специалистом",
  "valid_until": "2026-12-01T23:59:59Z"
}
```

Сумма и срок берутся из заявки, ставка — текущая base_annual_rate_bps продукта. Поле offer для APPROVE запрещено. valid_until обязателен, RFC 3339, должен быть в будущем; пример даты заменяйте на актуальный. Создаются решение и предложение PENDING, заявка переходит MANUAL → OFFERED. Это предложение клиенту, а не принятие предложения, подписание или выдача кредита.

### Другие условия

```json
{
  "data_version": 2,
  "action": "COUNTER_OFFER",
  "reason_code": "ADJUSTED_TERMS",
  "comment": "Предлагаем меньшую сумму на шесть месяцев",
  "valid_until": "2026-12-01T23:59:59Z",
  "offer": {
    "amount_kopecks": 90000000,
    "term_months": 6,
    "annual_rate_bps": 1990
  }
}
```

Все поля offer обязательны, null не допускается. Сумма — целые копейки > 0, срок — 1..600 месяцев (дополнительно в пределах продукта), ставка — целое число 0..2147483647 базисных пунктов; 1990 = 19.90% годовых. Верхние технические границы не являются рекомендацией или тарифной политикой банка.

Создаётся отдельное предложение; исходные requested_amount_kopecks и requested_term_months заявки не перезаписываются. Статус — OFFERED.

Для APPROVE и COUNTER_OFFER обязательны текущий скоринг и ровно шесть финансовых записей текущей версии с заполненными confirmed_at/confirmed_by_user_id. Продукт должен быть активен, сумма/срок предложения — в его min/max. Ставка COUNTER_OFFER задаётся сотрудником; отдельные банковские ограничения ставки в модели продукта пока отсутствуют.

Ежемесячный аннуитетный платёж рассчитывает сервер целочисленной арифметикой с округлением вверх до копейки. При нулевой ставке — сумма/срок с тем же округлением. Если результат не помещается в int64, запрос отклоняется с 400. Клиент не передаёт monthly_payment_kopecks.

### Отказ

```json
{
  "data_version": 2,
  "action": "REJECT",
  "reason_code": "MANUAL_REJECTION",
  "comment": "Причина отказа для клиента"
}
```

Создаётся решение, заявка переходит MANUAL → REJECTED. Предложения нет. Поля offer и valid_until запрещены. Для отказа не требуется полный подтверждённый набор финансов или скоринг; если текущий скоринг есть, его ID сохраняется в решении.

### Ответ и атомарность

Успех — 201 Created:

```json
{
  "application_id": "33333333-3333-4333-8333-333333333333",
  "data_version": 2,
  "status": "REJECTED",
  "decision": {
    "id": "99999999-9999-4999-8999-999999999999",
    "application_id": "33333333-3333-4333-8333-333333333333",
    "scoring_id": null,
    "decided_by_user_id": "88888888-8888-4888-8888-888888888881",
    "data_version": 2,
    "decision_type": "REJECT",
    "decision_source": "MANUAL",
    "reason_code": "MANUAL_REJECTION",
    "comment": "Причина отказа для клиента",
    "policy_version": "manual-v1",
    "created_at": "2026-10-25T15:00:00Z"
  },
  "offer": null
}
```

Для APPROVE/COUNTER_OFFER offer содержит id, decision_id, version=1, amount_kopecks, term_months, annual_rate_bps, monthly_payment_kopecks, status=PENDING, created_at и valid_until. policy_version=manual-v1 обозначает версию ручного процесса, а не автоматическую проверку кредитной политики. Интеграции внешних проверок этим маршрутом не запускаются.

Решение, предложение, статус заявки и status_history сохраняются в одной транзакции под блокировкой заявки. Любая ошибка откатывает все записи. data_version не увеличивается, поскольку входные финансовые данные не меняются. assigned_employee_id сохраняется как исполнитель.

Повторный или параллельный запрос после первого успешного решения получает 409: заявка уже не MANUAL. Для проверки результата после сетевого сбоя заново получите карточку с decisions/offers. Автоматическое повторное создание предложения и идемпотентный ключ пока не реализованы.

### Ошибки решения

| HTTP | Текст | Причина |
|---|---|---|
| 400 | Invalid request body | JSON/типы/неизвестные поля/размер тела |
| 400 | Invalid bank application input | UUID, версия, действие, причина/комментарий, срок действия или поля предложения невалидны |
| 401 | Зависит от auth middleware | Нет действительного токена |
| 403 | Access denied | Не сотрудник банка или роль отозвана |
| 404 | Application not found | Заявка отсутствует/не подана/черновик |
| 409 | Application is not in manual review | Заявка больше не MANUAL, включая повтор решения |
| 409 | Application version conflict | Версия заявки изменилась |
| 409 | Claim the application before deciding | Заявка не назначена |
| 409 | Application is assigned to another employee | Другой исполнитель |
| 409 | Current scoring and confirmed financials are required | Недостаточно данных для предложения |
| 422 | Offer does not meet active product limits | Продукт неактивен либо сумма/срок вне лимитов |
| 500 | Internal server error | Ошибка БД; изменения откатываются |

## Следующие операции

Запрос допданных, скачивание PDF и клиентские ответы на предложения реализованы. Подписание и выдача кредита остаются за пределами этого API. Банковское решение проверяет назначенного исполнителя. ОГРН, юридический адрес и номер вида AX-2026-0142 не формируются карточкой.

## Проверка

Unit-тесты проверяют авторизацию, роль, UUID, 404/500 и успешный ответ. PostgreSQL-тест TestBankApplicationRepository использует TEST_DATABASE_URL и временную схему, применяет миграции и проверяет выбор текущей версии, score=0, скрытие черновиков и отсутствие storage_key/password_hash. Без TEST_DATABASE_URL он пропускается.

Тесты банковского модуля сгруппированы в handler_test.go и repository_test.go. Добавлены HTTP-проверки claim и PostgreSQL-сценарий одновременного назначения двум сотрудникам, повторного запроса, конфликтов версии и запрета назначения черновика/заявки другого статуса.

Тесты решений проверяют HTTP-доступ, обязательность полей, расчёт платежа, а PostgreSQL-сценарии — исполнителя, версии, одобрение/отказ/другие условия, конкурентные запросы и откат при ошибке вставки offers. PostgreSQL-сценарии требуют TEST_DATABASE_URL.

## Запросить дополнительные данные

POST /api/v1/bank/applications/{id}/info-requests

```json
{
  "data_version": 3,
  "description": "Нужна актуальная выписка и пояснение по расходам",
  "required_items": ["Выписка PDF", "Пояснение по расходам"]
}
```

Только текущий исполнитель MANUAL. Описание после trim — 1..2000 символов; required_items — 1..20 разных непустых строк до 500 символов каждая. Тело до 16 KiB, неизвестные поля запрещены. Версия проверяется под блокировкой.

Ответ 201 — InfoRequest (см. клиентский контракт). В одной транзакции создаются OPEN-запрос, статус NEEDS_INFO и запись истории. data_version не меняется. Второй OPEN запрещён. Не назначенная/чужая/устаревшая заявка — 409; роль — 401/403; чужой ID/черновик — 404; тело — 400.

Клиент может загрузить финансовый или дополнительный PDF, исправить и подтвердить финансы. POST клиентского info-requests/{requestID}/respond повторно выполняет скоринг и возвращает MANUAL. Новое значение score не отменяет назначенное ручное рассмотрение. Конечное решение специалиста отмечает RESPONDED-запросы как RESOLVED.

## Освободить или передать назначение

POST /api/v1/bank/applications/{id}/assignment

Освободить:

```json
{"data_version": 3, "employee_id": null}
```

Передать:

```json
{"data_version": 3, "employee_id": "88888888-8888-4888-8888-888888888882"}
```

employee_id обязателен: отсутствие поля не означает освобождение. Доступ только текущему исполнителю, в MANUAL или NEEDS_INFO, при совпадающей версии. Новый исполнитель должен существовать и иметь актуальную роль BANK_EMPLOYEE. Принудительное перехватывание чужой заявки не предусмотрено.

Ответ 200: application_id, assigned_employee_id (UUID/null), data_version, status, updated_at. Версия и статус не меняются. Передача самому себе не меняет updated_at. Изменения сериализуются блокировкой заявки; после передачи прежний исполнитель не может решать или переназначать её. Ошибки — 400/401/403/404/409 по тем же правилам, что claim. История статусов при назначении не пополняется, поскольку перехода статуса нет.

## Поиск, мои решения и архив

- GET /api/v1/bank/applications/page — текущая очередь MANUAL.
- GET /api/v1/bank/decisions — только решения текущего сотрудника (автоматические и чужие не включаются).
- GET /api/v1/bank/archive — завершённые заявки и истёкшие предложения.

Все требуют BANK_EMPLOYEE; роль дополнительно проверяется в БД. Ответ 200:

```json
{"items": [], "total": 0, "limit": 20, "offset": 0}
```

Параметры: q (до 200 символов, поиск подстроки по компании/ИНН/UUID заявки), status, limit (1..100, по умолчанию 20), offset (0..1000000). Некорректные параметры — 400. total учитывает фильтры и остаётся корректным при offset за последней страницей. Запросы параметризованы, введённые символы не являются SQL или шаблоном LIKE.

Для очереди/архива есть assigned: mine, unassigned или UUID сотрудника. У «моих решений» этот параметр запрещён — автор всегда берётся из токена.

items очереди/архива содержит поля старой очереди и data_version. Сортировка — последнее время перехода в MANUAL (при отсутствии — submitted_at), затем UUID, новые первыми. В архив входят REJECTED, ACCEPTED, OFFER_DECLINED, SIGNED, DISBURSED, CLOSED, CANCELLED. OFFERED без действующего PENDING текущей версии отображается в архиве как OFFER_EXPIRED. Это вычисляемое состояние списка: GET не изменяет applications.status. Истёкшие PENDING в карточке предложений отображаются EXPIRED.

items «моих решений» содержит поля DecisionRecord, company_name, inn и application_status. Сортировка — created_at/id по убыванию; status фильтрует фактический статус заявки. Финальные ручные решения сохраняют автора независимо от дальнейшего ответа клиента.

Публичный номер вида AX-2026-0142 пока не создаётся — поиск номера в этих маршрутах использует UUID.

## Открытие документа

GET /api/v1/bank/applications/{id}/documents/{documentID}/download — только BANK_EMPLOYEE для поданной заявки, включая NEEDS_INFO и архив. Черновики скрыты. Документ должен принадлежать указанной заявке; несоответствие и отсутствие файла — 404. Ответ application/pdf с безопасным inline filename, Cache-Control: no-store, nosniff; поддерживаются Range и HEAD. storage_key/файловый путь наружу не передаются.

Клиентский аналог: GET /api/v1/applications/{id}/documents/{documentID}/download — только CLIENT-владелец.
