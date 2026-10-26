# AlphaHub — полный текущий контракт HTTP API

Дата сверки: 23 октября 2026. Основание: код main на коммите d14a6e708eaf2371a6268e4f497b10ca2ed2d185.
Документ самостоятельный: включает все 21 прикладной маршрут Go и 2 прикладных маршрута DS.
Описывает текущее поведение, включая ограничения и несовпадения валидации. Новые функции этим документом не добавляются.
Примеры UUID, пользователей, продуктов и ответов иллюстративные, не реальные записи и не результаты отправки каждого запроса.

## 1. Адреса, форматы, единицы

| Сервис | Пример базового адреса | Назначение |
|---|---|---|
| Go backend | http://localhost:8080 | Авторизация, компании, заявки, документы, финансы |
| DS FastAPI | http://192.168.1.104:8000 | Извлечение PDF и расчёт скоринга |

HTTP_PORT задаёт порт Go, по умолчанию 8080. DS_BASE_URL задаётся Go без /api/v1: пути клиент добавляет сам.
Все нижеуказанные /api/v1/... пути добавляются к базовому адресу соответствующего сервиса.
Для GET и POST без тела Content-Type не нужен. Для JSON: Content-Type: application/json.
Для файла используйте multipart/form-data с автоматически сформированным boundary, не задавайте заголовок вручную.

* Деньги — целые копейки, 50 000 000 = 500 000 рублей. Не передавайте float и строку вместо числа.
* Ставка — целые базисные пункты, 1990 = 19,90% годовых.
* Срок — месяцы. UUID — JSON-строки. ИНН — строка, не число.
* period_month: YYYY-MM. registration_date/application_date: YYYY-MM-DD.
* Временные метки — RFC3339, могут содержать доли секунды и смещение часового пояса.
* application_date при подаче формируется Go из UTC. Дата регистрации проверяется по локальной дате сервера.
* Примеры null обозначают явный JSON null. omitempty-поля могут отсутствовать целиком.
* Go списки продуктов, заявок и документов возвращаются массивами без обёртки и пагинации.
* Query-параметры фильтрации/пагинации в текущих обработчиках не реализованы.

## 2. Авторизация и владение

```http
Authorization: Bearer <access_token>
```

Регистрация, вход и чтение продуктов публичны. /auth/me требует валидный JWT без ограничения CLIENT.
Остальные Go маршруты ниже требуют CLIENT; BANK_EMPLOYEE получит 403 Access denied.
JWT: HS256, issuer alphahub-api, срок 2 часа. Refresh/logout/revoke маршрутов нет.
Роль для middleware берётся из токена. /auth/me дополнительно читает пользователя из БД.
Принадлежность к одной компании не даёт доступа к чужим заявкам: проверяется created_by_user_id.
Поиск компании по ИНН доступен CLIENT и не ограничен собственной компанией.

Общие ответы middleware (text/plain; charset=utf-8):

| HTTP | Текст |
|---|---|
| 401 | Bearer token is required |
| 401 | Invalid or expired token |
| 401 | Invalid user ID |
| 401 | Invalid user role |
| 403 | Access denied |

## 3. JSON, ограничения и ошибки

Успешные прикладные ответы — application/json. Обычные ошибки Go — текст, а не единая JSON-обёртка.
http.Error добавляет завершающий перевод строки. Не вызывайте response.json() без проверки Content-Type.
Ошибка маршрутизации 404 page not found отличается от бизнес-ошибки Application not found.
При несовпадении метода Go ServeMux может ответить 405. GET-маршруты ServeMux также обслуживают HEAD.

Auth JSON ограничен 8192 байтами; company/application/financials — 16 КиБ.
Go отвергает неизвестные JSON-поля и несколько JSON-объектов в одном теле. Ошибка декодирования,
включая превышение JSON-лимита, обычно отображается как 400. Это не общая схема required:
обычные int/string DTO получают нулевые значения для отсутствующих полей; конкретные проверки ниже.
Непредвиденная ошибка БД/сервера обычно даёт 500 Internal server error.

### 3.1. Ошибки DS и преобразование в Go

DS использует JSON:

```json
{"error":{"code":"PDF_EXTRACTION_ERROR","message":"Описание ошибки извлечения"}}
```

В текущем ds/api.py в эту обёртку приведены RequestValidationError, HTTPException и необработанные исключения.
DS-клиент также поддерживает старый detail-объект с code/message и непустой detail-массив для 422.
Для последнего код VALIDATION_ERROR и сообщение DS rejected scoring input.
Некорректная обёртка HTTP-ошибки даёт локальный код INVALID_ERROR_RESPONSE.

| Событие | Ответ публичного Go API |
|---|---|
| DS ответил 4xx | Тот же HTTP-статус и JSON error с code/message |
| DS ответил 5xx или иной неуспешный статус вне 4xx | 502 с JSON error |
| Сетевая ошибка или таймаут Do | 503, текст DS service unavailable; retry the request |
| Успешный HTTP-ответ DS содержит некорректный JSON/DTO, слишком большой body, ошибка чтения | Сейчас обычная ошибка обработчика, 500 |

Таймаут HTTP-клиента DS 30 секунд. Максимальный ответ 2 МиБ. Автоматических повторных POST нет.
Go проверяет обязательные поля ответа, не допускает null вместо нужных полей; score целый 0..100,
model_version непустая строка, оба массива факторов — массивы строк без null. Дополнительные поля игнорируются.

## 4. Реестр маршрутов

| № | Сервис | Метод и путь | Доступ | Успех |
|---|---|---|---|---|
| 1 | Go | `POST /api/v1/auth/register` | Публичный | 201 |
| 2 | Go | `POST /api/v1/auth/login` | Публичный | 200 |
| 3 | Go | `GET /api/v1/auth/me` | Bearer, без ограничения CLIENT | 200 |
| 4 | Go | `GET /api/v1/products` | Публичный | 200 |
| 5 | Go | `GET /api/v1/products/{code}` | Публичный | 200 |
| 6 | Go | `POST /api/v1/company` | CLIENT + Bearer | 201 |
| 7 | Go | `GET /api/v1/company/me` | CLIENT + Bearer | 200 |
| 8 | Go | `GET /api/v1/company/by-inn/{inn}` | CLIENT + Bearer | 200 |
| 11 | Go | `POST /api/v1/applications` | CLIENT + Bearer | 201 |
| 12 | Go | `GET /api/v1/applications` | CLIENT + Bearer | 200 |
| 13 | Go | `GET /api/v1/applications/{id}` | CLIENT + Bearer | 200 |
| 14 | Go | `PATCH /api/v1/applications/{id}` | CLIENT + Bearer | 200 |
| 15 | Go | `POST /api/v1/applications/{id}/documents` | CLIENT + Bearer | 201 |
| 16 | Go | `GET /api/v1/applications/{id}/documents` | CLIENT + Bearer | 200 |
| 17 | Go | `GET /api/v1/applications/{id}/financials` | CLIENT + Bearer | 200 |
| 18 | Go | `PUT /api/v1/applications/{id}/financials` | CLIENT + Bearer | 200 |
| 19 | Go | `POST /api/v1/applications/{id}/financials/confirm` | CLIENT + Bearer | 200 |
| 20 | Go | `POST /api/v1/applications/{id}/submit` | CLIENT + Bearer | 200 |
| 21 | Go | `GET /api/v1/applications/{id}/scoring` | CLIENT + Bearer | 200 |
| 22 | DS | `POST /api/v1/financials/extract` | DS: авторизация не реализована | 200 |
| 23 | DS | `POST /api/v1/scoring/evaluate` | DS: авторизация не реализована | 200 |

## 5. Подробные контракты маршрутов

У всех защищённых Go маршрутов дополнительно возможны общие 401/403, у всех Go маршрутов — 500 при внутренней ошибке. Ошибки ниже перечисляют основные явные ветви реализации.

### 5.1. Регистрация

**`POST /api/v1/auth/register`**

Доступ: Публичный. Успех: **201**.

Все три поля обязательны по смыслу. fullName обрезается по краям и не может быть пустым. Email приводится к нижнему регистру и проверяется mail.ParseAddress без display name. Пароль: 8–72 байта, не символа. Роль всегда CLIENT; передать role или company_id нельзя. Регистрация не возвращает токен.

**Запрос**

```json
{
  "fullName": "Иван Иванов",
  "email": "ivan@example.com",
  "password": "ExamplePassword123!"
}
```

**Пример ответа**

```json
{
  "id": "11111111-1111-4111-8111-111111111111",
  "fullName": "Иван Иванов",
  "email": "ivan@example.com",
  "role": "CLIENT"
}
```

**Ошибки:** 400: Invalid register request / name is required / invalid email / password must contain 8-72 bytes. 409: Email already registered.

### 5.2. Вход

**`POST /api/v1/auth/login`**

Доступ: Публичный. Успех: **200**.

Email нормализуется. JWT HS256, issuer alphahub-api, срок 2 часа; claims sub, iss, iat, exp, role. Refresh token не выдаётся.

**Запрос**

```json
{
  "email": "ivan@example.com",
  "password": "ExamplePassword123!"
}
```

**Пример ответа**

```json
{
  "access_token": "<JWT>",
  "token_type": "Bearer"
}
```

**Ошибки:** 400: Invalid login request / Email and password are required. 401: Invalid email or password.

### 5.3. Текущий пользователь

**`GET /api/v1/auth/me`**

Доступ: Bearer, без ограничения CLIENT. Успех: **200**.

Пользователь перечитывается из БД. company_id отсутствует в JSON, если пользователь не связан с компанией.

**Запрос**

Тело не требуется. Параметры пути указаны в URL; query-параметры не требуются.

**Пример ответа**

```json
{
  "id": "11111111-1111-4111-8111-111111111111",
  "fullName": "Иван Иванов",
  "email": "ivan@example.com",
  "role": "CLIENT",
  "company_id": "22222222-2222-4222-8222-222222222222"
}
```

**Ошибки:** 401: Bearer token is required / Invalid or expired token / User not found.

### 5.4. Список продуктов

**`GET /api/v1/products`**

Доступ: Публичный. Успех: **200**.

Список сортируется по name. В текущем SQL нет фильтра is_active: неактивные продукты тоже возвращаются. Пустой результат — []. Пагинации нет.

**Запрос**

Тело не требуется. Параметры пути указаны в URL; query-параметры не требуются.

**Пример ответа**

```json
[
  {
    "code": "WORKING_CAPITAL",
    "name": "Оборотный кредит",
    "description": "Пополнение оборотных средств",
    "min_amount_kopecks": 10000000,
    "max_amount_kopecks": 500000000,
    "min_term_months": 3,
    "max_term_months": 36,
    "base_annual_rate_bps": 1990,
    "is_active": true
  }
]
```

### 5.5. Продукт по коду

**`GET /api/v1/products/{code}`**

Доступ: Публичный. Успех: **200**.

code — строка, например WORKING_CAPITAL. Пробелы по краям обрезаются. Неактивный продукт может возвращаться.

**Запрос**

Тело не требуется. Параметры пути указаны в URL; query-параметры не требуются.

**Пример ответа**

```json
{
  "code": "WORKING_CAPITAL",
  "name": "Оборотный кредит",
  "description": "Пополнение оборотных средств",
  "min_amount_kopecks": 10000000,
  "max_amount_kopecks": 500000000,
  "min_term_months": 3,
  "max_term_months": 36,
  "base_annual_rate_bps": 1990,
  "is_active": true
}
```

**Ошибки:** 400: Invalid product code. 404: Product not found.

### 5.6. Создание компании

**`POST /api/v1/company`**

Доступ: CLIENT + Bearer. Успех: **201**.

Создаёт компанию и привязывает текущего пользователя одной транзакцией. ИНН: 10 или 12 ASCII-цифр; контрольная сумма не проверяется. name: 1–255 символов; business_type: 1–50; main_okved: 1–20. Тексты trim, валидный UTF-8. business_type не enum. registration_date строго YYYY-MM-DD, год >=1, не позже сегодняшней даты сервера. Все поля обязательны по смыслу.

**Запрос**

```json
{
  "inn": "7700000023",
  "name": "ООО Пример",
  "business_type": "OOO",
  "registration_date": "2020-01-01",
  "main_okved": "62.01"
}
```

**Пример ответа**

```json
{
  "id": "22222222-2222-4222-8222-222222222222",
  "inn": "7700000023",
  "name": "ООО Пример",
  "business_type": "OOO",
  "registration_date": "2020-01-01",
  "main_okved": "62.01"
}
```

**Ошибки:** 400: Invalid request body / Invalid company input. 404: User not found. 409: User already has a company.

Если компания с таким ИНН уже есть в базе (например, демо-компании из миграции 17), новая не создаётся: пользователь привязывается к существующей и получает её данные (201). Приглашения и `/company/join` удалены 25.10.2026.

### 5.7. Своя компания

**`GET /api/v1/company/me`**

Доступ: CLIENT + Bearer. Успех: **200**.

Возвращает компанию, связанную с текущим пользователем.

**Запрос**

Тело не требуется. Параметры пути указаны в URL; query-параметры не требуются.

**Пример ответа**

```json
{
  "id": "22222222-2222-4222-8222-222222222222",
  "inn": "7700000023",
  "name": "ООО Пример",
  "business_type": "OOO",
  "registration_date": "2020-01-01",
  "main_okved": "62.01"
}
```

**Ошибки:** 404: Company not found.

### 5.8. Поиск компании

**`GET /api/v1/company/by-inn/{inn}`**

Доступ: CLIENT + Bearer. Успех: **200**.

inn — 10 или 12 цифр. Поиск только в локальной БД. Доступен любому CLIENT; не привязывает пользователя к найденной компании.

**Запрос**

Тело не требуется. Параметры пути указаны в URL; query-параметры не требуются.

**Пример ответа**

```json
{
  "id": "22222222-2222-4222-8222-222222222222",
  "inn": "7700000023",
  "name": "ООО Пример",
  "business_type": "OOO",
  "registration_date": "2020-01-01",
  "main_okved": "62.01"
}
```

**Ошибки:** 400: Invalid company input. 404: Company not found.

### 5.11. Создать черновик

**`POST /api/v1/applications`**

Доступ: CLIENT + Bearer. Успех: **201**.

product_code trim, непустой, до 50 байт; продукт должен существовать и быть активным. Создаёт DRAFT, data_version=1. Компания, сумма, срок пока null; purpose="". Заголовок Location: /api/v1/applications/{новый-id}. Повторный POST создаёт новую заявку.

**Запрос**

```json
{
  "product_code": "WORKING_CAPITAL"
}
```

**Пример ответа**

```json
{
  "id": "33333333-3333-4333-8333-333333333333",
  "company_id": null,
  "created_by_user_id": "11111111-1111-4111-8111-111111111111",
  "product_code": "WORKING_CAPITAL",
  "assigned_employee_id": null,
  "accepted_offer_id": null,
  "signed_by_user_id": null,
  "requested_amount_kopecks": null,
  "requested_term_months": null,
  "purpose": "",
  "status": "DRAFT",
  "data_version": 1,
  "created_at": "2026-10-23T12:00:00Z",
  "updated_at": "2026-10-23T12:00:00Z",
  "submitted_at": null,
  "signed_at": null,
  "disbursed_at": null
}
```

**Ошибки:** 400: Invalid request body / Invalid application input / Product not found or inactive.

### 5.12. Список своих заявок

**`GET /api/v1/applications`**

Доступ: CLIENT + Bearer. Успех: **200**.

Только created_by_user_id текущего пользователя. Порядок created_at DESC, id DESC. Пустой список []; пагинация и фильтры не реализованы.

**Запрос**

Тело не требуется. Параметры пути указаны в URL; query-параметры не требуются.

**Пример ответа**

```json
[
  {
    "id": "33333333-3333-4333-8333-333333333333",
    "company_id": "22222222-2222-4222-8222-222222222222",
    "created_by_user_id": "11111111-1111-4111-8111-111111111111",
    "product_code": "WORKING_CAPITAL",
    "assigned_employee_id": null,
    "accepted_offer_id": null,
    "signed_by_user_id": null,
    "requested_amount_kopecks": 50000000,
    "requested_term_months": 12,
    "purpose": "Оборотные средства",
    "status": "DRAFT",
    "data_version": 2,
    "created_at": "2026-10-23T12:00:00Z",
    "updated_at": "2026-10-23T12:00:00Z",
    "submitted_at": null,
    "signed_at": null,
    "disbursed_at": null
  }
]
```

### 5.13. Заявка по ID

**`GET /api/v1/applications/{id}`**

Доступ: CLIENT + Bearer. Успех: **200**.

id — UUID. Заявки других пользователей скрыты, даже если пользователи одной компании.

**Запрос**

Тело не требуется. Параметры пути указаны в URL; query-параметры не требуются.

**Пример ответа**

```json
{
  "id": "33333333-3333-4333-8333-333333333333",
  "company_id": "22222222-2222-4222-8222-222222222222",
  "created_by_user_id": "11111111-1111-4111-8111-111111111111",
  "product_code": "WORKING_CAPITAL",
  "assigned_employee_id": null,
  "accepted_offer_id": null,
  "signed_by_user_id": null,
  "requested_amount_kopecks": 50000000,
  "requested_term_months": 12,
  "purpose": "Оборотные средства",
  "status": "DRAFT",
  "data_version": 2,
  "created_at": "2026-10-23T12:00:00Z",
  "updated_at": "2026-10-23T12:00:00Z",
  "submitted_at": null,
  "signed_at": null,
  "disbursed_at": null
}
```

**Ошибки:** 400: Invalid application input. 404: Application not found.

### 5.14. Изменить черновик

**`PATCH /api/v1/applications/{id}`**

Доступ: CLIENT + Bearer. Успех: **200**.

Обязательна актуальная data_version >=1 и хотя бы одно изменяемое поле. Только DRAFT. company_id должен совпадать с компанией текущего пользователя. purpose trim, максимум 2000 Unicode-символов; пустое значение допустимо для черновика. null/пропущенные pointer-поля не очищают значения. Каждое успешное изменение повышает data_version на 1. Пределы продукта проверяются при submit; положительность суммы/срока обеспечивается БД, нарушение сейчас может дать 500.

**Запрос**

```json
{
  "company_id": "22222222-2222-4222-8222-222222222222",
  "requested_amount_kopecks": 50000000,
  "requested_term_months": 12,
  "purpose": "Оборотные средства",
  "data_version": 1
}
```

**Пример ответа**

```json
{
  "id": "33333333-3333-4333-8333-333333333333",
  "company_id": "22222222-2222-4222-8222-222222222222",
  "created_by_user_id": "11111111-1111-4111-8111-111111111111",
  "product_code": "WORKING_CAPITAL",
  "assigned_employee_id": null,
  "accepted_offer_id": null,
  "signed_by_user_id": null,
  "requested_amount_kopecks": 50000000,
  "requested_term_months": 12,
  "purpose": "Оборотные средства",
  "status": "DRAFT",
  "data_version": 2,
  "created_at": "2026-10-23T12:00:00Z",
  "updated_at": "2026-10-23T12:00:00Z",
  "submitted_at": null,
  "signed_at": null,
  "disbursed_at": null
}
```

**Ошибки:** 400: Invalid request boty (опечатка в текущем коде) / Invalid application input. 403: Company access denied. 404: Application not found. 409: Application is not a draft / Application version conflict.

### 5.15. Загрузка и разбор PDF

**`POST /api/v1/applications/{id}/documents`**

Доступ: CLIENT + Bearer. Успех: **201**.

Ровно один файл в поле file. Размер 1..10 485 760 байт; общее тело максимум 11 МиБ. Имя после удаления пути: расширение .pdf без учёта регистра, до 255 байт. Начало содержимого %PDF-. Нужны владелец, DRAFT и выбранная компания. Перед записью вызывается DS extract. ИНН результата должен совпасть с компанией. Сохранение документа, шести месяцев и общего долга атомарно; data_version +1; финансы неподтверждены. Если заявка изменилась во время вызова DS — 409. При сбое записи удаляется созданный локальный файл; ошибка извлечения не создаёт документ.

**Запрос**

Body → form-data: ключ `file`, тип File, значение — PDF.

```text
file: <binary PDF>
```

**Пример ответа**

```json
{
  "id": "44444444-4444-4444-8444-444444444444",
  "application_id": "33333333-3333-4333-8333-333333333333",
  "document_type": "FINANCIAL_STATEMENT",
  "original_filename": "risky.pdf",
  "processing_status": "PROCESSED",
  "uploaded_at": "2026-10-23T12:00:00Z"
}
```

**Ошибки:** 400: invalid document input / invalid PDF. 413: PDF exceeds 10 MiB. 404: Application not found. 409: Application is not a draft / application changed during PDF extraction; retry upload. 422: select application company before uploading PDF / PDF INN does not match application company. Также ошибки DS по разделу преобразования ошибок.

### 5.16. Документы заявки

**`GET /api/v1/applications/{id}/documents`**

Доступ: CLIENT + Bearer. Успех: **200**.

Только владелец заявки. Список метаданных, без PDF и storage_key. Сортировка uploaded_at DESC, id DESC. Пустой список []. Скачать или удалить PDF отдельным HTTP-маршрутом сейчас нельзя.

**Запрос**

Тело не требуется. Параметры пути указаны в URL; query-параметры не требуются.

**Пример ответа**

```json
[
  {
    "id": "44444444-4444-4444-8444-444444444444",
    "application_id": "33333333-3333-4333-8333-333333333333",
    "document_type": "FINANCIAL_STATEMENT",
    "original_filename": "risky.pdf",
    "processing_status": "PROCESSED",
    "uploaded_at": "2026-10-23T12:00:00Z"
  }
]
```

**Ошибки:** 400: invalid document input. 404: Application not found.

### 5.17. Финансы заявки

**`GET /api/v1/applications/{id}/financials`**

Доступ: CLIENT + Bearer. Успех: **200**.

Возвращает месяцы последней имеющейся финансовой версии, отсортированные по period_month. data_version в ответе — текущая версия ЗАЯВКИ, она может отличаться от версии строк после PATCH заявки. Общий долг берётся из applications. confirmed_at и confirmed_by_user_id null до подтверждения.

**Запрос**

Тело не требуется. Параметры пути указаны в URL; query-параметры не требуются.

**Пример ответа**

```json
{
  "application_id": "33333333-3333-4333-8333-333333333333",
  "data_version": 3,
  "declared_monthly_debt_payment_kopecks": 10151981,
  "financials": [
    {
      "period_month": "2026-04",
      "revenue_kopecks": 100000000,
      "operating_inflow_kopecks": 110000000,
      "operating_outflow_kopecks": 60000000,
      "tax_outflow_kopecks": 10000000
    },
    {
      "period_month": "2026-05",
      "revenue_kopecks": 100000000,
      "operating_inflow_kopecks": 110000000,
      "operating_outflow_kopecks": 60000000,
      "tax_outflow_kopecks": 10000000
    },
    {
      "period_month": "2026-06",
      "revenue_kopecks": 100000000,
      "operating_inflow_kopecks": 110000000,
      "operating_outflow_kopecks": 60000000,
      "tax_outflow_kopecks": 10000000
    },
    {
      "period_month": "2026-07",
      "revenue_kopecks": 100000000,
      "operating_inflow_kopecks": 110000000,
      "operating_outflow_kopecks": 60000000,
      "tax_outflow_kopecks": 10000000
    },
    {
      "period_month": "2026-08",
      "revenue_kopecks": 100000000,
      "operating_inflow_kopecks": 110000000,
      "operating_outflow_kopecks": 60000000,
      "tax_outflow_kopecks": 10000000
    },
    {
      "period_month": "2026-09",
      "revenue_kopecks": 100000000,
      "operating_inflow_kopecks": 110000000,
      "operating_outflow_kopecks": 60000000,
      "tax_outflow_kopecks": 10000000
    }
  ],
  "confirmed_at": null,
  "confirmed_by_user_id": null
}
```

**Ошибки:** 400: Invalid financial input. 404: Financial data or application not found, включая отсутствие финансов.

### 5.18. Сохранить исправленные финансы

**`PUT /api/v1/applications/{id}/financials`**

Доступ: CLIENT + Bearer. Успех: **200**.

Полная замена набора через новую версию, не частичное исправление месяца. Актуальная data_version>=1, обязательный ненулевой JSON-параметр declared_monthly_debt_payment_kopecks>=0, ровно 6 разных YYYY-MM. Денежные значения неотрицательны. Версия +1, подтверждение новой версии сбрасывается. Период относительно даты подачи здесь не проверяется: проверяется при submit. Пропущенное или null числовое поле Month в текущем Go DTO может стать 0 — отправляйте все поля явно. period_month фактически должен быть без пробелов: проверка trim не нормализует сохраняемое значение.

**Запрос**

```json
{
  "data_version": 3,
  "declared_monthly_debt_payment_kopecks": 10151981,
  "financials": [
    {
      "period_month": "2026-04",
      "revenue_kopecks": 100000000,
      "operating_inflow_kopecks": 110000000,
      "operating_outflow_kopecks": 60000000,
      "tax_outflow_kopecks": 10000000
    },
    {
      "period_month": "2026-05",
      "revenue_kopecks": 100000000,
      "operating_inflow_kopecks": 110000000,
      "operating_outflow_kopecks": 60000000,
      "tax_outflow_kopecks": 10000000
    },
    {
      "period_month": "2026-06",
      "revenue_kopecks": 100000000,
      "operating_inflow_kopecks": 110000000,
      "operating_outflow_kopecks": 60000000,
      "tax_outflow_kopecks": 10000000
    },
    {
      "period_month": "2026-07",
      "revenue_kopecks": 100000000,
      "operating_inflow_kopecks": 110000000,
      "operating_outflow_kopecks": 60000000,
      "tax_outflow_kopecks": 10000000
    },
    {
      "period_month": "2026-08",
      "revenue_kopecks": 100000000,
      "operating_inflow_kopecks": 110000000,
      "operating_outflow_kopecks": 60000000,
      "tax_outflow_kopecks": 10000000
    },
    {
      "period_month": "2026-09",
      "revenue_kopecks": 100000000,
      "operating_inflow_kopecks": 110000000,
      "operating_outflow_kopecks": 60000000,
      "tax_outflow_kopecks": 10000000
    }
  ]
}
```

**Пример ответа**

```json
{
  "application_id": "33333333-3333-4333-8333-333333333333",
  "data_version": 4,
  "declared_monthly_debt_payment_kopecks": 10151981,
  "financials": [
    {
      "period_month": "2026-04",
      "revenue_kopecks": 100000000,
      "operating_inflow_kopecks": 110000000,
      "operating_outflow_kopecks": 60000000,
      "tax_outflow_kopecks": 10000000
    },
    {
      "period_month": "2026-05",
      "revenue_kopecks": 100000000,
      "operating_inflow_kopecks": 110000000,
      "operating_outflow_kopecks": 60000000,
      "tax_outflow_kopecks": 10000000
    },
    {
      "period_month": "2026-06",
      "revenue_kopecks": 100000000,
      "operating_inflow_kopecks": 110000000,
      "operating_outflow_kopecks": 60000000,
      "tax_outflow_kopecks": 10000000
    },
    {
      "period_month": "2026-07",
      "revenue_kopecks": 100000000,
      "operating_inflow_kopecks": 110000000,
      "operating_outflow_kopecks": 60000000,
      "tax_outflow_kopecks": 10000000
    },
    {
      "period_month": "2026-08",
      "revenue_kopecks": 100000000,
      "operating_inflow_kopecks": 110000000,
      "operating_outflow_kopecks": 60000000,
      "tax_outflow_kopecks": 10000000
    },
    {
      "period_month": "2026-09",
      "revenue_kopecks": 100000000,
      "operating_inflow_kopecks": 110000000,
      "operating_outflow_kopecks": 60000000,
      "tax_outflow_kopecks": 10000000
    }
  ],
  "confirmed_at": null,
  "confirmed_by_user_id": null
}
```

**Ошибки:** 400: Invalid request body / Invalid financial input. 404: Financial data or application not found. 409: Application is not a draft / Application version conflict.

### 5.19. Подтвердить финансы

**`POST /api/v1/applications/{id}/financials/confirm`**

Доступ: CLIENT + Bearer. Успех: **200**.

Только DRAFT. Подтверждает ровно 6 строк с data_version равной текущей версии заявки. Устанавливает confirmed_at и confirmed_by_user_id. data_version не повышает. Не принимает ожидаемую версию в теле. Повторный confirm обновит время подтверждения. После PATCH заявки прежних строк текущей версии может не быть: сначала PUT финансов.

**Запрос**

Тело не требуется. Параметры пути указаны в URL; query-параметры не требуются.

**Пример ответа**

```json
{
  "application_id": "33333333-3333-4333-8333-333333333333",
  "data_version": 3,
  "declared_monthly_debt_payment_kopecks": 10151981,
  "financials": [
    {
      "period_month": "2026-04",
      "revenue_kopecks": 100000000,
      "operating_inflow_kopecks": 110000000,
      "operating_outflow_kopecks": 60000000,
      "tax_outflow_kopecks": 10000000
    },
    {
      "period_month": "2026-05",
      "revenue_kopecks": 100000000,
      "operating_inflow_kopecks": 110000000,
      "operating_outflow_kopecks": 60000000,
      "tax_outflow_kopecks": 10000000
    },
    {
      "period_month": "2026-06",
      "revenue_kopecks": 100000000,
      "operating_inflow_kopecks": 110000000,
      "operating_outflow_kopecks": 60000000,
      "tax_outflow_kopecks": 10000000
    },
    {
      "period_month": "2026-07",
      "revenue_kopecks": 100000000,
      "operating_inflow_kopecks": 110000000,
      "operating_outflow_kopecks": 60000000,
      "tax_outflow_kopecks": 10000000
    },
    {
      "period_month": "2026-08",
      "revenue_kopecks": 100000000,
      "operating_inflow_kopecks": 110000000,
      "operating_outflow_kopecks": 60000000,
      "tax_outflow_kopecks": 10000000
    },
    {
      "period_month": "2026-09",
      "revenue_kopecks": 100000000,
      "operating_inflow_kopecks": 110000000,
      "operating_outflow_kopecks": 60000000,
      "tax_outflow_kopecks": 10000000
    }
  ],
  "confirmed_at": "2026-10-23T12:00:00Z",
  "confirmed_by_user_id": "11111111-1111-4111-8111-111111111111"
}
```

**Ошибки:** 400: Invalid financial input (в том числе нет шести строк текущей версии). 404: Financial data or application not found. 409: Application is not a draft.

### 5.20. Подать заявку и рассчитать скоринг

**`POST /api/v1/applications/{id}/submit`**

Доступ: CLIENT + Bearer. Успех: **200**.

Без JSON-тела. Только владелец и DRAFT. Обязательны company_id, сумма, срок, непустая цель; пользователь должен быть связан с компанией. Активный продукт и его лимиты проверяются. Нужны долг и 6 подтверждённых финансовых строк текущей версии. Go собирает ScoringInput, вызывает DS, сохраняет результат и snapshot, меняет статус на SUBMITTED, пишет status_history в одной транзакции. data_version не меняется. На время HTTP-вызова удерживается блокировка заявки. Ошибка DS откатывает подачу; повторите после исправления. Повторная подача SUBMITTED вернёт 409 и не вызывает DS. Автоматического банковского решения нет.

**Запрос**

Тело не требуется. Параметры пути указаны в URL; query-параметры не требуются.

**Пример ответа**

```json
{
  "id": "33333333-3333-4333-8333-333333333333",
  "company_id": "22222222-2222-4222-8222-222222222222",
  "created_by_user_id": "11111111-1111-4111-8111-111111111111",
  "product_code": "WORKING_CAPITAL",
  "assigned_employee_id": null,
  "accepted_offer_id": null,
  "signed_by_user_id": null,
  "requested_amount_kopecks": 50000000,
  "requested_term_months": 12,
  "purpose": "Оборотные средства",
  "status": "SUBMITTED",
  "data_version": 3,
  "created_at": "2026-10-23T12:00:00Z",
  "updated_at": "2026-10-23T12:00:00Z",
  "submitted_at": "2026-10-23T12:00:00Z",
  "signed_at": null,
  "disbursed_at": null
}
```

**Ошибки:** 400: Invalid application input / Product not found or inactive. 404: Application not found. 409: Application is not a draft. 422: Application has incomplete or invalid data / confirm current financial data before submitting / invalid financial reporting period or scoring parameters: <детали>. Также ошибки DS.

### 5.21. Последний сохранённый скоринг

**`GET /api/v1/applications/{id}/scoring`**

Доступ: CLIENT + Bearer. Успех: **200**.

Возвращает последний сохранённый результат владельцу заявки: сортировка created_at DESC, id DESC. features_snapshot — реально отправленный JSON. risk_band сохраняется UNASSESSED, но в этот ответ не входит. Новый расчёт не запускается. Значение score в примере иллюстративное, это не обещание расчёта для приведённых чисел.

**Запрос**

Тело не требуется. Параметры пути указаны в URL; query-параметры не требуются.

**Пример ответа**

```json
{
  "score": 65,
  "explanation": {
    "positive_factors": [
      "Положительный денежный поток"
    ],
    "negative_factors": [
      "Высокая долговая нагрузка"
    ]
  },
  "model_version": "financial_rules_v1",
  "data_version": 3,
  "features_snapshot": {
    "inn": "7700000023",
    "financials": [
      {
        "period_month": "2026-04",
        "revenue_kopecks": 100000000,
        "operating_inflow_kopecks": 110000000,
        "operating_outflow_kopecks": 60000000,
        "tax_outflow_kopecks": 10000000
      },
      {
        "period_month": "2026-05",
        "revenue_kopecks": 100000000,
        "operating_inflow_kopecks": 110000000,
        "operating_outflow_kopecks": 60000000,
        "tax_outflow_kopecks": 10000000
      },
      {
        "period_month": "2026-06",
        "revenue_kopecks": 100000000,
        "operating_inflow_kopecks": 110000000,
        "operating_outflow_kopecks": 60000000,
        "tax_outflow_kopecks": 10000000
      },
      {
        "period_month": "2026-07",
        "revenue_kopecks": 100000000,
        "operating_inflow_kopecks": 110000000,
        "operating_outflow_kopecks": 60000000,
        "tax_outflow_kopecks": 10000000
      },
      {
        "period_month": "2026-08",
        "revenue_kopecks": 100000000,
        "operating_inflow_kopecks": 110000000,
        "operating_outflow_kopecks": 60000000,
        "tax_outflow_kopecks": 10000000
      },
      {
        "period_month": "2026-09",
        "revenue_kopecks": 100000000,
        "operating_inflow_kopecks": 110000000,
        "operating_outflow_kopecks": 60000000,
        "tax_outflow_kopecks": 10000000
      }
    ],
    "declared_monthly_debt_payment_kopecks": 10151981,
    "amount_kopecks": 50000000,
    "term_months": 12,
    "annual_rate_bps": 1990,
    "application_date": "2026-10-23"
  },
  "created_at": "2026-10-23T12:00:00Z"
}
```

**Ошибки:** 400: Invalid application input. 404: Application not found (также когда заявка есть, но результата ещё нет).

### 5.22. DS: извлечь данные PDF

**`POST /api/v1/financials/extract`**

Доступ: DS: авторизация не реализована. Успех: **200**.

Адрес DS, не Go. Обязательное поле file. Размер до 10 МиБ. Допускаются MIME application/pdf, application/octet-stream или отсутствие MIME; проверяется %PDF-. Сначала стандартный парсер, при неподдерживаемом расположении таблицы — fallback Docling. Неполные данные не дополняются догадками. Ответ содержит только сведения из PDF: параметров кредита здесь нет.

**Запрос**

Body → form-data: ключ `file`, тип File, значение — PDF.

```text
file: <binary PDF>
```

**Пример ответа**

```json
{
  "inn": "7700000023",
  "financials": [
    {
      "period_month": "2026-04",
      "revenue_kopecks": 100000000,
      "operating_inflow_kopecks": 110000000,
      "operating_outflow_kopecks": 60000000,
      "tax_outflow_kopecks": 10000000
    },
    {
      "period_month": "2026-05",
      "revenue_kopecks": 100000000,
      "operating_inflow_kopecks": 110000000,
      "operating_outflow_kopecks": 60000000,
      "tax_outflow_kopecks": 10000000
    },
    {
      "period_month": "2026-06",
      "revenue_kopecks": 100000000,
      "operating_inflow_kopecks": 110000000,
      "operating_outflow_kopecks": 60000000,
      "tax_outflow_kopecks": 10000000
    },
    {
      "period_month": "2026-07",
      "revenue_kopecks": 100000000,
      "operating_inflow_kopecks": 110000000,
      "operating_outflow_kopecks": 60000000,
      "tax_outflow_kopecks": 10000000
    },
    {
      "period_month": "2026-08",
      "revenue_kopecks": 100000000,
      "operating_inflow_kopecks": 110000000,
      "operating_outflow_kopecks": 60000000,
      "tax_outflow_kopecks": 10000000
    },
    {
      "period_month": "2026-09",
      "revenue_kopecks": 100000000,
      "operating_inflow_kopecks": 110000000,
      "operating_outflow_kopecks": 60000000,
      "tax_outflow_kopecks": 10000000
    }
  ],
  "declared_monthly_debt_payment_kopecks": 10151981
}
```

**Ошибки:** 400 EMPTY_FILE. 413 FILE_TOO_LARGE. 415 UNSUPPORTED_MEDIA_TYPE. 422 VALIDATION_ERROR (например, отсутствует file) / PDF_EXTRACTION_ERROR. 503 EXTRACTION_SERVICE_UNAVAILABLE (нет Docling). 500 INTERNAL_ERROR.

### 5.23. DS: финансовый скоринг

**`POST /api/v1/scoring/evaluate`**

Доступ: DS: авторизация не реализована. Успех: **200**.

Все поля ScoringInput обязательны. Сумма/срок >0; ставка и денежные показатели >=0. annual_rate_bps — строго integer, bool/строки не заменяют число. application_date не раньше 0001-07-01. Ровно шесть полных месяцев перед датой подачи, строго по порядку. Для октября 2026: апрель–сентябрь. Ответ — score 0..100, explanation с двумя массивами строк, model_version. Не возвращает компоненты DetailedScoringResponse, raw_score или score_cap. Результат не является окончательным решением по кредиту.

**Запрос**

```json
{
  "inn": "7700000023",
  "financials": [
    {
      "period_month": "2026-04",
      "revenue_kopecks": 100000000,
      "operating_inflow_kopecks": 110000000,
      "operating_outflow_kopecks": 60000000,
      "tax_outflow_kopecks": 10000000
    },
    {
      "period_month": "2026-05",
      "revenue_kopecks": 100000000,
      "operating_inflow_kopecks": 110000000,
      "operating_outflow_kopecks": 60000000,
      "tax_outflow_kopecks": 10000000
    },
    {
      "period_month": "2026-06",
      "revenue_kopecks": 100000000,
      "operating_inflow_kopecks": 110000000,
      "operating_outflow_kopecks": 60000000,
      "tax_outflow_kopecks": 10000000
    },
    {
      "period_month": "2026-07",
      "revenue_kopecks": 100000000,
      "operating_inflow_kopecks": 110000000,
      "operating_outflow_kopecks": 60000000,
      "tax_outflow_kopecks": 10000000
    },
    {
      "period_month": "2026-08",
      "revenue_kopecks": 100000000,
      "operating_inflow_kopecks": 110000000,
      "operating_outflow_kopecks": 60000000,
      "tax_outflow_kopecks": 10000000
    },
    {
      "period_month": "2026-09",
      "revenue_kopecks": 100000000,
      "operating_inflow_kopecks": 110000000,
      "operating_outflow_kopecks": 60000000,
      "tax_outflow_kopecks": 10000000
    }
  ],
  "declared_monthly_debt_payment_kopecks": 10151981,
  "amount_kopecks": 50000000,
  "term_months": 12,
  "annual_rate_bps": 1990,
  "application_date": "2026-10-23"
}
```

**Пример ответа**

```json
{
  "score": 65,
  "explanation": {
    "positive_factors": [
      "Положительный денежный поток"
    ],
    "negative_factors": [
      "Высокая долговая нагрузка"
    ]
  },
  "model_version": "financial_rules_v1"
}
```

**Ошибки:** 422 VALIDATION_ERROR / SCORING_NOT_CALCULABLE. 500 INTERNAL_ERROR.

## 6. Полный словарь Go DTO

Таблицы извлечены из текущих struct и JSON-тегов, поэтому отражают точное написание полей.
Тип int64 — JSON integer; *T означает pointer (может представлять null), []T — массив.
time.Time сериализуется строкой RFC3339; json.RawMessage — вложенный JSON, не строка с экранированным JSON.
Для запросов обязательность определяется проверками раздела 5, а не только pointer/omitempty.

### auth.User

| JSON-поле | Go-тип | Сериализация |
|---|---|---|
| `id` | `string` | Поле сериализуется |
| `fullName` | `string` | Поле сериализуется |
| `email` | `string` | Поле сериализуется |
| `role` | `string` | Поле сериализуется |
| `company_id` | `string` | Может отсутствовать (omitempty) |

### auth.RegisterRequest

| JSON-поле | Go-тип | Сериализация |
|---|---|---|
| `fullName` | `string` | Поле сериализуется |
| `email` | `string` | Поле сериализуется |
| `password` | `string` | Поле сериализуется |

### auth.LoginRequest

| JSON-поле | Go-тип | Сериализация |
|---|---|---|
| `email` | `string` | Поле сериализуется |
| `password` | `string` | Поле сериализуется |

### auth.UserResponse

| JSON-поле | Go-тип | Сериализация |
|---|---|---|
| `id` | `string` | Поле сериализуется |
| `fullName` | `string` | Поле сериализуется |
| `email` | `string` | Поле сериализуется |
| `role` | `string` | Поле сериализуется |
| `company_id` | `string` | Может отсутствовать (omitempty) |

### auth.LoginResponse

| JSON-поле | Go-тип | Сериализация |
|---|---|---|
| `access_token` | `string` | Поле сериализуется |
| `token_type` | `string` | Поле сериализуется |

### product.Product

| JSON-поле | Go-тип | Сериализация |
|---|---|---|
| `code` | `string` | Поле сериализуется |
| `name` | `string` | Поле сериализуется |
| `description` | `string` | Поле сериализуется |
| `min_amount_kopecks` | `int64` | Поле сериализуется |
| `max_amount_kopecks` | `int64` | Поле сериализуется |
| `min_term_months` | `int` | Поле сериализуется |
| `max_term_months` | `int` | Поле сериализуется |
| `base_annual_rate_bps` | `int` | Поле сериализуется |
| `is_active` | `bool` | Поле сериализуется |

### company.Company

| JSON-поле | Go-тип | Сериализация |
|---|---|---|
| `id` | `string` | Поле сериализуется |
| `inn` | `string` | Поле сериализуется |
| `name` | `string` | Поле сериализуется |
| `business_type` | `string` | Поле сериализуется |
| `main_okved` | `string` | Поле сериализуется |

### company.CreateRequest

| JSON-поле | Go-тип | Сериализация |
|---|---|---|
| `inn` | `string` | Поле сериализуется |
| `name` | `string` | Поле сериализуется |
| `business_type` | `string` | Поле сериализуется |
| `registration_date` | `string` | Поле сериализуется |
| `main_okved` | `string` | Поле сериализуется |

### company.Response

| JSON-поле | Go-тип | Сериализация |
|---|---|---|
| `id` | `string` | Поле сериализуется |
| `inn` | `string` | Поле сериализуется |
| `name` | `string` | Поле сериализуется |
| `business_type` | `string` | Поле сериализуется |
| `registration_date` | `string` | Поле сериализуется |
| `main_okved` | `string` | Поле сериализуется |


### company.JoinRequest

| JSON-поле | Go-тип | Сериализация |
|---|---|---|
| `code` | `string` | Поле сериализуется |

### application.Application

| JSON-поле | Go-тип | Сериализация |
|---|---|---|
| `id` | `string` | Поле сериализуется |
| `company_id` | `*string` | Может быть null |
| `created_by_user_id` | `string` | Поле сериализуется |
| `product_code` | `string` | Поле сериализуется |
| `assigned_employee_id` | `*string` | Может быть null |
| `accepted_offer_id` | `*string` | Может быть null |
| `signed_by_user_id` | `*string` | Может быть null |
| `requested_amount_kopecks` | `*int64` | Может быть null |
| `requested_term_months` | `*int` | Может быть null |
| `purpose` | `string` | Поле сериализуется |
| `status` | `string` | Поле сериализуется |
| `data_version` | `int` | Поле сериализуется |
| `created_at` | `time.Time` | Поле сериализуется |
| `updated_at` | `time.Time` | Поле сериализуется |
| `submitted_at` | `*time.Time` | Может быть null |
| `signed_at` | `*time.Time` | Может быть null |
| `disbursed_at` | `*time.Time` | Может быть null |

### application.CreateRequest

| JSON-поле | Go-тип | Сериализация |
|---|---|---|
| `product_code` | `string` | Поле сериализуется |

### application.UpdateRequest

| JSON-поле | Go-тип | Сериализация |
|---|---|---|
| `company_id` | `*string` | Может отсутствовать (omitempty) |
| `requested_amount_kopecks` | `*int64` | Может отсутствовать (omitempty) |
| `requested_term_months` | `*int` | Может отсутствовать (omitempty) |
| `purpose` | `*string` | Может отсутствовать (omitempty) |
| `data_version` | `*int` | Может быть null |

### application.ScoringResult

| JSON-поле | Go-тип | Сериализация |
|---|---|---|
| `data_version` | `int` | Поле сериализуется |
| `score` | `int` | Поле сериализуется |
| `model_version` | `string` | Поле сериализуется |
| `explanation` | `json.RawMessage` | Поле сериализуется |
| `features_snapshot` | `json.RawMessage` | Поле сериализуется |
| `created_at` | `time.Time` | Поле сериализуется |

### documents.Document

| JSON-поле | Go-тип | Сериализация |
|---|---|---|
| `id` | `string` | Поле сериализуется |
| `application_id` | `string` | Поле сериализуется |
| `document_type` | `string` | Поле сериализуется |
| `original_filename` | `string` | Поле сериализуется |
| `processing_status` | `string` | Поле сериализуется |
| `uploaded_at` | `time.Time` | Поле сериализуется |

### financials.Month

| JSON-поле | Go-тип | Сериализация |
|---|---|---|
| `period_month` | `string` | Поле сериализуется |
| `revenue_kopecks` | `int64` | Поле сериализуется |
| `operating_inflow_kopecks` | `int64` | Поле сериализуется |
| `operating_outflow_kopecks` | `int64` | Поле сериализуется |
| `tax_outflow_kopecks` | `int64` | Поле сериализуется |

### financials.Financials

| JSON-поле | Go-тип | Сериализация |
|---|---|---|
| `application_id` | `string` | Поле сериализуется |
| `data_version` | `int` | Поле сериализуется |
| `declared_monthly_debt_payment_kopecks` | `*int64` | Может быть null |
| `financials` | `[]Month` | Поле сериализуется |
| `confirmed_at` | `*time.Time` | Может быть null |
| `confirmed_by_user_id` | `*string` | Может быть null |

### financials.UpdateRequest

| JSON-поле | Go-тип | Сериализация |
|---|---|---|
| `data_version` | `int` | Поле сериализуется |
| `declared_monthly_debt_payment_kopecks` | `*int64` | Может быть null |
| `financials` | `[]Month` | Поле сериализуется |

### dsclient.MonthlyFinancials

| JSON-поле | Go-тип | Сериализация |
|---|---|---|
| `period_month` | `string` | Поле сериализуется |
| `revenue_kopecks` | `int64` | Поле сериализуется |
| `operating_inflow_kopecks` | `int64` | Поле сериализуется |
| `operating_outflow_kopecks` | `int64` | Поле сериализуется |
| `tax_outflow_kopecks` | `int64` | Поле сериализуется |

### dsclient.FinancialExtractionResponse

| JSON-поле | Go-тип | Сериализация |
|---|---|---|
| `inn` | `string` | Поле сериализуется |
| `financials` | `[]MonthlyFinancials` | Поле сериализуется |
| `declared_monthly_debt_payment_kopecks` | `int64` | Поле сериализуется |

### dsclient.ScoringInput

| JSON-поле | Go-тип | Сериализация |
|---|---|---|
| `inn` | `string` | Поле сериализуется |
| `amount_kopecks` | `int64` | Поле сериализуется |
| `term_months` | `int` | Поле сериализуется |
| `financials` | `[]MonthlyFinancials` | Поле сериализуется |
| `declared_monthly_debt_payment_kopecks` | `int64` | Поле сериализуется |
| `annual_rate_bps` | `int` | Поле сериализуется |
| `application_date` | `string` | Поле сериализуется |

### dsclient.ScoringExplanation

| JSON-поле | Go-тип | Сериализация |
|---|---|---|
| `positive_factors` | `[]string` | Поле сериализуется |
| `negative_factors` | `[]string` | Поле сериализуется |

### dsclient.ScoringResponse

| JSON-поле | Go-тип | Сериализация |
|---|---|---|
| `score` | `int` | Поле сериализуется |
| `explanation` | `ScoringExplanation` | Поле сериализуется |
| `model_version` | `string` | Поле сериализуется |


## 7. Жизненный цикл и версии

1. Зарегистрироваться и получить JWT через login.
2. Создать компанию (или привязаться к существующей по ИНН). Проверить company_id через /auth/me.
3. Выбрать активный продукт через /products. Создание продукта по HTTP отсутствует.
4. POST /applications создаёт DRAFT с data_version=1.
5. PATCH заявки задаёт компанию, сумму, срок и цель с актуальной версией. После успеха версия 2.
6. POST /applications/{id}/documents отправляет PDF в DS. ИНН PDF должен совпасть с ИНН компании.
7. Go сохраняет документ PROCESSED и финансы новой версии (в примере 3).
8. GET financials показывает извлечённые данные. При необходимости PUT полного набора с версией из GET.
9. POST financials/confirm подтверждает текущую финансовую версию.
10. POST submit собирает JSON, обращается к DS, атомарно сохраняет score и переводит заявку в SUBMITTED.
11. GET scoring возвращает результат и features_snapshot. Высокий score не означает одобрения.

| Операция | data_version | Подтверждение | Статус |
|---|---|---|---|
| Создание заявки | 1 | Финансов нет | DRAFT |
| PATCH заявки | +1 | Старые строки не копируются в новую версию | DRAFT |
| Успешная загрузка PDF | +1 | Новые месяцы не подтверждены | DRAFT |
| PUT financials | +1 | Новые месяцы не подтверждены | DRAFT |
| POST confirm | Без изменения | Подтверждает строки текущей версии | DRAFT |
| Успешный submit | Без изменения | Использует подтверждённую версию | SUBMITTED |
| Ошибка DS при submit | Без изменения | Сохраняется прежнее | DRAFT |

После изменения заявки через PATCH не пытайтесь подтвердить старые строки напрямую:
прочитайте financials, сохраните их через PUT с текущей data_version и подтвердите новую версию.
GET financials может показывать прежние месяцы вместе с новой версией заявки; это текущая особенность реализации.
При 409 перечитайте заявку/финансы, сопоставьте изменения, затем отправьте запрос с актуальной версией.
У POST создания заявки/документа нет Idempotency-Key; повтор после потери ответа может создать дополнительную запись.
Повтор submit уже поданной заявки даёт 409; проверьте GET заявки и GET scoring после неизвестного исхода.

## 8. Как формируется ScoringInput

| Поле DS | Источник |
|---|---|
| inn | companies.inn выбранной компании |
| amount_kopecks | applications.requested_amount_kopecks |
| term_months | applications.requested_term_months |
| annual_rate_bps | products.base_annual_rate_bps на момент расчёта |
| application_date | Дата submitted_at в UTC; для новой подачи — момент подачи |
| financials | Шесть подтверждённых строк текущей data_version, ORDER BY period_month |
| declared_monthly_debt_payment_kopecks | applications.declared_monthly_debt_payment_kopecks |

Extract не должен возвращать сумму кредита, срок, ставку и дату заявки. Это параметры заявки и продукта.
Ставка, дата и все входные данные сохраняются в features_snapshot для воспроизводимости результата.
Общий долг не подменяет неизвестные исторические платежи по месяцам.
operating_expenses_kopecks и existing_debt_payment_kopecks при извлечении остаются NULL (миграция 16).
score/explanation/model_version сохраняются в scoring_results. risk_band=UNASSESSED.

## 9. Различия валидации: Go, DS и публичный API

* Go-клиент проверяет разные месяцы и их принадлежность отчётному диапазону, но допускает перестановку;
  DS требует строгого хронологического порядка. Публичный submit читает БД с ORDER BY, поэтому порядок правильный.
* Extract проверяет шесть месяцев без привязки к будущей дате заявки; submit проверяет предыдущие шесть полных месяцев.
* PUT финансов не проверяет требуемый отчётный диапазон; ошибку можно получить только при submit.
* Go DTO Month с обычными int64 не отличает пропущенное число от нуля. DS MonthlyFinancials требует поля.
* Python Pydantic по умолчанию допускает часть преобразований типов, кроме явно strict annual_rate_bps.
  Для стабильного контракта отправляйте точные JSON-типы из примеров, а не полагайтесь на coercion.
* DS inn в ScoringInput объявлен string без проверки формата/контрольной суммы; Go-клиент требует непустую строку,
  а создание компании в публичном API — 10/12 цифр.
* При успешном DS HTTP ответе Go дополнительно проверяет непустую model_version и структуру факторов.
* Неизвестные поля запросов Go отклоняет; текущие Python BaseModel обычно игнорируют дополнительные поля.

## 10. Служебные маршруты DS

FastAPI в текущем api.py создаётся с настройками документации по умолчанию:

| Метод | Путь | Назначение |
|---|---|---|
| GET | /docs | Swagger UI, HTML |
| GET | /redoc | ReDoc, HTML |
| GET | /openapi.json | OpenAPI JSON работающего DS |
| GET | /docs/oauth2-redirect | Служебный HTML Swagger UI |

У Go этих маршрутов нет. Фактическая доступность служебных страниц может зависеть от внешнего reverse proxy.

## 11. Что пока отсутствует в HTTP API

Нет создания/изменения/удаления продукта; удаления/отмены заявки; скачивания/удаления документа;
refresh/logout; редактирования компании; списка всех компаний; повторного скоринга поданной заявки;
ручного банковского решения; предложений, подписания, выдачи кредита; административного списка заявок.
Наличие таблиц decisions/offers/check_results или полей signed_at/disbursed_at не означает наличие таких маршрутов.
Тестовые /api/v1/client/test и /api/v1/bank/test закомментированы и не зарегистрированы.
Стоп-факторы, банковские лимиты и автоматическое одобрение/отказ не включены в подключённый workflow.

## 12. Проверка в Postman

Переменные: base_url=http://localhost:8080, ds_base_url=http://192.168.1.104:8000,
access_token=<JWT>, application_id=<UUID из POST applications>. Не добавляйте /api/v1 к base_url.
Публичный путь загрузки:

```text
{{base_url}}/api/v1/applications/{{application_id}}/documents
```

Authorization → Bearer Token → {{access_token}}. Body → form-data → file типа File.
Ручной Content-Type выключить. Для прямого extract используйте ds_base_url и /api/v1/financials/extract.
Для скоринга DS используйте raw JSON, полный пример находится в разделе соответствующего маршрута.
Не отправляйте PDF на /scoring/evaluate.

401 без токена подтверждает, что защищённый маршрут найден. 404 page not found означает проблему пути/сервиса;
Application not found — бизнес-ответ о владельце/заявке (для GET scoring также об отсутствии результата).
При несовпадении видимого и реального URL смотрите раскрытый адрес в Postman Console.

## 13. Настройки и доказательства проверки

Go загружает .env из текущего каталога процесса. Требуются DATABASE_URL, JWT_SECRET (>=32 байт), DS_BASE_URL;
HTTP_PORT необязателен. PDF хранятся в uploads относительно рабочего каталога. Нужна миграция 16.
ReadHeaderTimeout Go — 5 секунд; это не полный таймаут обработки запроса.

В этом сеансе до создания документа прошли go test ./..., go vet ./... и TestDSWorkflow с изолированной схемой
PostgreSQL и тестовым HTTP DS. Живой TestLiveScoring получил score=90, model=financial_rules_v1.
Это подтверждает проверенные сценарии, но не означает, что каждый маршрут из справочника вызван на живом сервере.
Документ составлен статически по маршрутам, DTO, handlers, services, repositories и Python schemas.

```powershell
cd C:\Users\Станислав\GolandProjects\AlphaHub\backend
go test ./...
go vet ./...
$env:TEST_DATABASE_URL = "<URL локального PostgreSQL для тестовой схемы>"
go test ./internal/integration -run TestDSWorkflow -count=1 -v
$env:DS_BASE_URL = "http://192.168.1.104:8000"
go test ./internal/dsclient -run '^TestLiveScoring$' -count=1 -v
```

TestLiveScoring использует фиксированный вход с апрелем–сентябрём 2026 и application_date=2026-10-23.
TestDSWorkflow строит месяцы относительно даты запуска, создаёт и удаляет отдельную схему.

## 14. Источники сверки

Go: backend/cmd/main.go; backend/internal/{auth,product,company,application,documents,financials}/{model,handler,service,repository}.go
(в product репозиторий находится в файле repository.go); backend/internal/auth/middleware.go; backend/internal/dsclient/{client,validation,http_error}.go.
DS: ds/api.py, ds/financials_api.py, ds/schemas.py. Хранение: backend/migrations/*.up.sql.
Это ссылки на исходники для поддержки документа; все необходимые клиенту формы запросов приведены выше.
