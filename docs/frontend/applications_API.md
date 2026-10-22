# AlphaHub — API-контракт Applications

Проверено 22 октября 2026 года по ветке `backend`, commit `ad4487e77b309f5a882fbc2fa2a7f7fbb38fc379` репозитория `qu0ta/AlphaHub`. Документ описывает реализацию исходного кода; запросы к работающему серверу не выполнялись.

## Общие правила

Базовый префикс: `/api/v1`. Локальный порт по умолчанию — `8080`, переопределяется `HTTP_PORT`.

Все четыре маршрута требуют JWT в заголовке `Authorization: Bearer <access_token>` и роль `CLIENT`. Идентификатор владельца берётся из JWT claim `sub`, роль — из `role`. Пользователь видит и изменяет только свои заявки. Чужая заявка возвращает тот же 404, что и отсутствующая.

Успех: `Content-Type: application/json`, объект или массив напрямую, без обёртки `data`. Ошибки через `http.Error`: `Content-Type: text/plain; charset=utf-8`, текст с завершающим переводом строки. Фронтенд должен читать ошибку через `response.text()`, а не `response.json()`.

Для POST/PATCH отправляйте `Content-Type: application/json`. Обработчик сам этот заголовок не проверяет. Лимит тела — 16384 байта; превышение возвращает 400. Неизвестные поля, некорректные типы, пустое тело и несколько JSON-значений отклоняются с 400.

## Модель Application

Все перечисленные ключи присутствуют в JSON, включая nullable-поля.

| Поле | JSON-тип | Значение |
|---|---|---|
| `id` | string | UUID заявки |
| `company_id` | string или null | UUID компании |
| `created_by_user_id` | string | UUID создателя |
| `product_code` | string | Код продукта |
| `assigned_employee_id` | string или null | UUID сотрудника |
| `accepted_offer_id` | string или null | UUID принятого предложения |
| `signed_by_user_id` | string или null | UUID подписанта |
| `requested_amount_kopecks` | integer или null | Сумма в копейках, Go int64 |
| `requested_term_months` | integer или null | Срок в месяцах |
| `purpose` | string | Цель финансирования |
| `status` | string | При создании `DRAFT` |
| `data_version` | integer | При создании 1, увеличивается при PATCH |
| `created_at` | string | Дата/время RFC 3339, возможны доли секунды |
| `updated_at` | string | Дата/время RFC 3339 |
| `submitted_at` | string или null | Дата/время отправки |
| `signed_at` | string или null | Дата/время подписания |
| `disbursed_at` | string или null | Дата/время выдачи |

Для int64 учитывайте предел точных целых JavaScript `Number.MAX_SAFE_INTEGER`: сервер сериализует сумму числом, не строкой.

## POST /api/v1/applications

Создаёт черновик. Единственное поле запроса:

```json
{"product_code":"EXISTING_ACTIVE_PRODUCT_CODE"}
```

`product_code` обязателен: после TrimSpace непустой, не более 50 байт UTF-8. Продукт должен существовать и иметь `is_active = true`. Подставьте реальный код активного продукта.

Ответ **201 Created** — полный Application. Начальные значения: `status: "DRAFT"`, `data_version: 1`, `purpose: ""`; сумма и срок — null. Владельца назначает сервер. Остальные nullable-связи и даты жизненного цикла нового черновика — null.

Заголовок `Location` в текущем коде содержит **`/api/v1/application/{id}`** (единственное число). Это несоответствие реальному GET-маршруту: для загрузки используйте `/api/v1/applications/{id}` и `id` из ответа.

Ошибки: 400 `Invalid request body`, 400 `Invalid application input`, 400 `Product not found or inactive`, общие ошибки авторизации и 500.

## GET /api/v1/applications

Ответ **200 OK** — `Application[]`; если заявок нет — `[]`. Только заявки текущего пользователя. Порядок: `created_at DESC, id DESC`. Пагинация и фильтры не реализованы; query-параметры не используются.

Ошибки: общие ошибки авторизации, 400 `Invalid application input` при некорректном UUID пользователя из JWT, 500.

## GET /api/v1/applications/{id}

`id` должен разбираться как UUID. Ответ **200 OK** — полный Application.

Ошибки: 400 `Invalid application input` для некорректного UUID; 404 `Application not found` для отсутствующей или чужой заявки; общие ошибки авторизации и 500.

## PATCH /api/v1/applications/{id}

Изменяет только собственный черновик `DRAFT`.

```json
{
  "requested_amount_kopecks": 100000000,
  "requested_term_months": 12,
  "purpose": "Пополнение оборотных средств",
  "data_version": 1
}
```

| Поле | Требование |
|---|---|
| `data_version` | Обязательное целое >= 1, равно текущей версии |
| `requested_amount_kopecks` | Необязательное целое int64 > 0 |
| `requested_term_months` | Необязательное целое > 0 |
| `purpose` | Необязательная строка; TrimSpace, максимум 2000 Unicode-символов после обрезки |

Помимо версии нужно передать хотя бы одно изменяемое поле с ненулевым JSON-значением. Пропущенное поле и `null` означают «оставить прежнее значение»; очистить сумму/срок через null нельзя. `purpose: ""` разрешён и очищает цель. `data_version: null` недопустим. Только версия без изменений — 400.

Ответ **200 OK** — полный обновлённый Application. `data_version` увеличивается на 1, `updated_at` обновляется. Даже передача прежнего значения увеличивает версию. `product_code`, `company_id`, статус и остальные серверные поля менять этим запросом нельзя: неизвестное поле даёт 400.

При 409 `Application version conflict` загрузите актуальную заявку, согласуйте изменения и отправьте новый PATCH с актуальной версией. Не повторяйте автоматически старую запись поверх новых данных. Проверка `DRAFT` выполняется раньше проверки совпадения версии.

Ошибки: 400 **`Invalid request boty`** — именно с опечаткой в текущем коде, для ошибки декодирования; 400 `Invalid application input` для валидации; 404 `Application not found`; 409 `Application is not a draft`; 409 `Application version conflict`; общие ошибки авторизации и 500.

## POST /api/v1/applications/{id}/submit

**Не реализован** в проверенном commit: отсутствуют маршрут, обработчик и метод сервиса. Запрос на этот URL попадает в стандартный 404 маршрутизатора (`404 page not found\n`), а не в защищённый applications-обработчик. Контракт тела и переход статуса не определены. Фронтенду пока не следует вызывать submit или самостоятельно менять `status`.

## Общие ошибки

Тексты в таблице указаны без завершающего `\n`.

| HTTP | Текст | Причина |
|---|---|---|
| 401 | `Bearer token is required` | Нет корректного заголовка Bearer с токеном |
| 401 | `Invalid or expired token` | Проверка JWT не прошла |
| 401 | `Invalid user ID` | Нет непустого строкового `sub` |
| 401 | `Invalid user role` | Роль отсутствует или не CLIENT/BANK_EMPLOYEE |
| 401 | `Unauthorized` | Нет пользователя в контексте обработчика |
| 403 | `Access denied` | Валидный JWT с ролью BANK_EMPLOYEE |
| 400 | `Invalid application input` | Валидация полей или UUID не прошла |
| 500 | `Internal server error` | Внутренняя ошибка |

## Пример обработки ответа на фронтенде

```ts
async function patchApplication(id: string, token: string, dataVersion: number) {
  const response = await fetch(`/api/v1/applications/${encodeURIComponent(id)}`, {
    method: "PATCH",
    headers: {
      Authorization: `Bearer ${token}`,
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ purpose: "Закупка оборудования", data_version: dataVersion }),
  });
  if (!response.ok) {
    const message = (await response.text()).trim();
    throw Object.assign(new Error(message), { status: response.status });
  }
  return response.json(); // Сохранить новую data_version вместе с заявкой.
}
```

## Источники реализации

Все ссылки зафиксированы на проверенный commit:

- [Маршруты](https://github.com/qu0ta/AlphaHub/blob/ad4487e77b309f5a882fbc2fa2a7f7fbb38fc379/backend/cmd/main.go)
- [JSON-модели](https://github.com/qu0ta/AlphaHub/blob/ad4487e77b309f5a882fbc2fa2a7f7fbb38fc379/backend/internal/application/model.go)
- [HTTP-обработчики и точные тексты ошибок](https://github.com/qu0ta/AlphaHub/blob/ad4487e77b309f5a882fbc2fa2a7f7fbb38fc379/backend/internal/application/handler.go)
- [Валидация](https://github.com/qu0ta/AlphaHub/blob/ad4487e77b309f5a882fbc2fa2a7f7fbb38fc379/backend/internal/application/service.go)
- [SQL, владение, версии и DRAFT](https://github.com/qu0ta/AlphaHub/blob/ad4487e77b309f5a882fbc2fa2a7f7fbb38fc379/backend/internal/application/repository.go)
- [JWT и проверка роли](https://github.com/qu0ta/AlphaHub/blob/ad4487e77b309f5a882fbc2fa2a7f7fbb38fc379/backend/internal/auth/middleware.go)
