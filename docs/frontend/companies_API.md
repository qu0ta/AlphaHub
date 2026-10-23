# AlphaHub — Companies API

> Контракт составлен по реализации ветки `backend` репозитория `qu0ta/AlphaHub` (проверено 23 октября 2026). Описывает фактические маршруты и обработку ошибок; не является OpenAPI-спецификацией.

## Общие правила

- **Base URL:** `http://localhost:8080/api/v1` (локальная разработка).
- **Авторизация:** все маршруты требуют `Authorization: Bearer <access_token>` и роль `CLIENT`.
- **Успешные ответы:** JSON-объекты с `Content-Type: application/json`.
- **Ошибки:** **обычный текст**, `text/plain; charset=utf-8` от Go `http.Error()`, **не JSON**. Читайте `response.text()` при `!response.ok`.
- **POST с телом:** `Content-Type: application/json`; декодер отклоняет неизвестные поля и несколько JSON-объектов; лимит тела — 16 KiB.
- **URL без завершающего `/`:** используйте именно пути ниже.
- Привязка компании к пользователю хранится в `users.company_id`; ID компании в заявке — `applications.company_id`.

## Методы

| Метод | Маршрут | Успех | Назначение |
|---|---|---:|---|
| `POST` | `/companies` | `201` | Создать компанию и автоматически привязать текущего пользователя |
| `GET` | `/companies/me` | `200` | Получить компанию текущего пользователя |
| `GET` | `/companies/by-inn/{inn}` | `200` | Найти компанию **в локальной базе AlphaHub** по ИНН |
| `POST` | `/companies/invitations` | `201` | Создать одноразовый код приглашения в свою компанию |
| `POST` | `/companies/join` | `200` | Присоединиться к существующей компании по приглашению |

## Схемы

### `CompanyResponse`

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440001",
  "inn": "7707083893",
  "name": "ООО Альфа Тест",
  "business_type": "OOO",
  "registration_date": "2020-05-15",
  "main_okved": "46.90"
}
```

| Поле | Тип | Примечание |
|---|---|---|
| `id` | `string` | UUID, формируется сервером |
| `inn` | `string` | 10 или 12 цифр |
| `name` | `string` | Название организации, до 255 символов Unicode |
| `business_type` | `string` | Непустая строка, до 50 символов; **строгого enum нет** |
| `registration_date` | `string` | `YYYY-MM-DD` |
| `main_okved` | `string` | Непустая строка, до 20 символов |

Поля представителя (`representativeName`, `representativePosition`) и `okvedName` **не поддерживаются** текущим `CompanyResponse`. `business_type` не следует автоматически трактовать как фронтовые `LLC | IP` без согласованного отображения.

## 1. Создание компании

`POST /api/v1/companies`

```http
Authorization: Bearer <access_token>
Content-Type: application/json
```

```json
{
  "inn": "7707083893",
  "name": "ООО Альфа Тест",
  "business_type": "OOO",
  "registration_date": "2020-05-15",
  "main_okved": "46.90"
}
```

**Успех:** `201 Created`, тело `CompanyResponse`. Компания создаётся в `companies`, после чего её ID записывается в `users.company_id` **в одной транзакции**. Пользователь должен быть ещё не привязан к компании; ИНН должен быть уникальным.

**Проверки:** ИНН — 10/12 цифр (без проверки контрольной суммы); текстовые поля непустые и ограничены длиной; дата строго `YYYY-MM-DD`, не в будущем.

| HTTP | Текст ошибки | Причина |
|---|---|---|
| `400` | `Invalid request body` | Некорректный JSON, неизвестное поле, несколько объектов, превышение лимита |
| `400` | `Invalid company input` | Не прошла валидация полей |
| `409` | `Company with this INN already exists` | ИНН уже присутствует в базе |
| `409` | `User already has a company` | Текущий пользователь уже привязан |
| `404` | `User not found` | Пользователь по JWT отсутствует в БД |
| `500` | `Internal server error` | Другая ошибка БД / сервиса |

## 2. Моя компания

`GET /api/v1/companies/me`

**Тело:** отсутствует. **Успех:** `200 OK`, `CompanyResponse`.

Если у пользователя нет компании: `404 Company not found` (не `200 null`).

## 3. Поиск по ИНН

`GET /api/v1/companies/by-inn/7707083893`

**Тело:** отсутствует. **Успех:** `200 OK`, `CompanyResponse`.

| HTTP | Текст ошибки | Причина |
|---|---|---|
| `400` | `Invalid company input` | ИНН не является строкой из 10/12 цифр |
| `404` | `Company not found` | Нет компании с таким ИНН в локальной таблице |
| `500` | `Internal server error` | Ошибка БД |

**Важно:** это НЕ запрос к ФНС/ЕГРЮЛ/ЕГРИП. `404` здесь означает «не найдена у нас», а не «такая организация не существует». Поиск возвращает реквизиты, но **не выдаёт права** на компанию.

## 4. Создание приглашения

`POST /api/v1/companies/invitations`

**Тело:** отсутствует (`Body → none`). Приглашение может создать любой пользователь с ролью `CLIENT`, у которого заполнен `users.company_id`.

**Успех:** `201 Created`, `Cache-Control: no-store`:

```json
{
  "code": "<одноразовый-случайный-код>",
  "expires_at": "2026-10-23T12:45:00Z"
}
```

`code` — строка Base64URL без padding, созданная из 32 криптографически случайных байтов. Код показывается только в этом ответе; в БД хранится SHA-256-хеш. Срок жизни — **15 минут**. Каждый код используется **один раз**.

| HTTP | Текст ошибки | Причина |
|---|---|---|
| `404` | `Company not found` | Пользователь не связан с компанией (реально репозиторий возвращает `ErrNotFound`) |
| `500` | `Internal server error` | Не удалось сохранить приглашение |

Не записывайте код в аналитику, URL или клиентские логи. Показывайте его только тому, кто должен передать приглашение.

## 5. Присоединение по приглашению

`POST /api/v1/companies/join`

```http
Authorization: Bearer <access_token>
Content-Type: application/json
```

```json
{
  "code": "<код-из-POST-companies-invitations>"
}
```

**Успех:** `200 OK`, `CompanyResponse`. Сервер проверяет срок действия и неиспользованность кода, связывает текущего пользователя с компанией (`users.company_id`), отмечает код использованным — **в одной транзакции**. Пользователь до этого не должен иметь компании.

| HTTP | Текст ошибки | Причина |
|---|---|---|
| `400` | `Invalid request body` | Ошибка структуры JSON |
| `400` | `Invalid or expired invitation` | Неверный формат, неизвестный, просроченный или использованный код |
| `409` | `User already has a company` | Пользователь уже связан с компанией |
| `404` | `User not found` | Пользователя нет в БД |
| `500` | `Internal server error` | Ошибка сохранения в БД |

## Общие ошибки авторизации

Применимы ко **всем пяти** маршрутам:

| HTTP | Текст | Причина |
|---|---|---|
| `401` | `Bearer token is required` | Нет заголовка или некорректная схема |
| `401` | `Invalid or expired token` | Неверная подпись или истёк JWT |
| `401` | `Invalid user ID` | Отсутствует корректный `sub` в claims |
| `401` | `Invalid user role` | Нет допустимой роли в claims |
| `403` | `Access denied` | Роль не `CLIENT` |

## Связь с Applications API

После `POST /companies` или `POST /companies/join` возьмите `id` из `CompanyResponse` (или вызовите `GET /companies/me`). Затем создайте черновик:

```http
POST /api/v1/applications
Authorization: Bearer <access_token>
Content-Type: application/json
```

```json
{"product_code":"WORKING_CAPITAL"}
```

Сохраните `id` заявки и `data_version` из ответа. Пример заполнения:

```http
PATCH /api/v1/applications/{application_id}
Authorization: Bearer <access_token>
Content-Type: application/json
```

```json
{
  "company_id": "<id из CompanyResponse>",
  "requested_amount_kopecks": 150000000,
  "requested_term_months": 18,
  "purpose": "Пополнение оборотных средств",
  "data_version": 1
}
```

Сервер проверяет, что `company_id` **равен `users.company_id` отправителя**. Успех обновления — `200 OK`, версия увеличивается. После заполнения и проверки лимитов продукта отправка:

```http
POST /api/v1/applications/{application_id}/submit
Authorization: Bearer <access_token>
```

При успешной проверке — `200 OK` и статус `SUBMITTED`; `Submit` также проверяет связь пользователя с компанией.

**Важно:** существующий `GET /applications` фильтруется по `created_by_user_id`, поэтому приглашённый пользователь **не получает автоматически список заявок коллег**. Он может создавать собственные заявки для той же компании.

## Пример TypeScript-клиента

```ts
type CompanyResponse = {
  id: string;
  inn: string;
  name: string;
  business_type: string;
  registration_date: string;
  main_okved: string;
};

type CreateCompanyRequest = Omit<CompanyResponse, "id">;

type InvitationResponse = { code: string; expires_at: string };

async function apiRequest<T>(path: string, token: string, options: RequestInit = {}): Promise<T> {
  const response = await fetch(`/api/v1${path}`, {
    ...options,
    headers: {
      Accept: "application/json",
      ...(options.body ? { "Content-Type": "application/json" } : {}),
      Authorization: `Bearer ${token}`,
      ...options.headers,
    },
  });
  const text = await response.text();
  if (!response.ok) throw new Error(`HTTP ${response.status}: ${text.trim()}`);
  return JSON.parse(text) as T;
}

export const companiesApi = {
  create: (token: string, data: CreateCompanyRequest) =>
    apiRequest<CompanyResponse>("/companies", token, { method: "POST", body: JSON.stringify(data) }),
  me: (token: string) => apiRequest<CompanyResponse>("/companies/me", token),
  byInn: (token: string, inn: string) =>
    apiRequest<CompanyResponse>(`/companies/by-inn/${encodeURIComponent(inn)}`, token),
  createInvitation: (token: string) =>
    apiRequest<InvitationResponse>("/companies/invitations", token, { method: "POST" }),
  join: (token: string, code: string) =>
    apiRequest<CompanyResponse>("/companies/join", token, { method: "POST", body: JSON.stringify({ code }) }),
};
```

Используйте относительный `/api/v1` за Vite proxy / reverse proxy либо настройте CORS. На данный момент Go-бэкенд не предоставляет CORS middleware.

## Ограничения текущей реализации

1. Нет фактического запроса в государственный реестр, нет проверки контрольной суммы ИНН.
2. Первый пользователь может самостоятельно создать компанию; действительное право представлять организацию не подтверждается внешним источником.
3. Любой уже привязанный `CLIENT` может выдавать приглашения; отдельные роли внутри компании отсутствуют.
4. Нет endpoint для изменения реквизитов, удаления компании, открепления пользователя и отзыва приглашения.
5. Поля представителя компании и названия ОКВЭД не входят в текущий backend-контракт.
6. Для работы приглашений необходима применённая миграция `000015_create_company_invitations.up.sql`.
7. Описание подготовлено по коду GitHub; локальный запуск сервера и интеграционные тесты этим документом не подтверждаются.

## Исходники контракта

- `backend/cmd/main.go`
- `backend/internal/company/model.go`
- `backend/internal/company/handler.go`
- `backend/internal/company/service.go`
- `backend/internal/company/repository.go`
- `backend/internal/application/repository.go`
- `backend/migrations/000015_create_company_invitations.up.sql`
