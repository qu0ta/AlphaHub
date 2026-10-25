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

Раздел обновлён 25 октября 2026 года по текущему backend. Остальные исторические разделы этого файла могут описывать более раннюю реализацию.

Маршрут реализован. Требует Bearer JWT с ролью CLIENT и владение заявкой. Тело не требуется. Только DRAFT. До вызова DS проверяются выбранная компания и доступ пользователя, активность продукта, сумма и срок в пределах продукта, заявленный платёж по долгам и шесть подтверждённых месяцев текущей data_version. DS проверяет отчётный период: шесть полных месяцев перед датой подачи.

Успех — 200 OK, полный Application с заполненным submitted_at; data_version не увеличивается.

| Score | Конечный статус | Результат |
|---|---|---|
| 0–39 | REJECTED | Решение REJECT, источник AUTO, предложение не создаётся |
| 40–69 | MANUAL | Очередь сотрудника; финального решения и предложения пока нет |
| 70–100 | OFFERED | Решение APPROVE, источник AUTO, предложение PENDING на условиях заявки и ставке продукта |

Порог автоматического предложения включает ровно 70. Оно не требует ручного claim или решения сотрудника. При этом OFFERED означает предложение клиенту, а не его согласие, подписание договора или выдачу денег. accepted_offer_id остаётся null.

Предложение действует 7 суток с created_at, сумма/срок берутся из заявки, ставка — из активного продукта. Платёж рассчитывает сервер по аннуитетной формуле с округлением вверх до копейки; при нулевой ставке — сумма/срок. Расчёт использует целочисленную арифметику, поддерживает срок до 600 месяцев и проверяет переполнение int64.

Скоринг, автоматическое решение, предложение, статус и история записываются одной транзакцией. При сбое DS или БД всё откатывается, заявка остаётся DRAFT. Параллельная/повторная подача после успешного запроса возвращает 409 Application is not a draft, без повторного вызова DS и создания дубликатов.

История: DRAFT → SUBMITTED → IN_REVIEW → REJECTED/MANUAL/OFFERED. Начальная подача записана от пользователя, дальнейшие автоматические переходы — с changed_by_user_id=null. Автор автоматического решения также null; policy_version=score-routing-v1. Скоринг хранит risk_band=UNASSESSED: внешние проверки и отдельная модель банковского риска не подменяются этим решением.

Ошибки: 404 Application not found для чужой/отсутствующей заявки; 409 Application is not a draft; 422 для неполных данных/неподтверждённых финансов/невалидного периода; 400 Product not found or inactive. Сетевые ошибки DS и ошибки его ответа обрабатываются общим mapper DS; внутренняя ошибка БД — 500. Точные тексты ошибок смотреть в application/handler.go и dsclient/http_error.go.

Автоматические решения/предложения доступны в банковской карточке (decisions/offers). Клиентские маршруты просмотра и ответа на предложение реализованы и описаны ниже; GET заявки также показывает конечный статус.

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

## Клиентские предложения — обновление 25.10.2026

Все маршруты требуют CLIENT и принадлежность заявки текущему пользователю; чужая заявка/предложение — 404. Успех — JSON, ошибки — текст. Cache-Control: no-store.

### GET /api/v1/applications/{id}/offers

Ответ 200 — массив всех предложений заявки, новые первыми; если их нет — []. Поля: id, decision_id, application_id, data_version (версия данных решения), version (версия предложения), amount_kopecks, term_months, annual_rate_bps, monthly_payment_kopecks, status, created_at, valid_until, responded_at (null до ответа), responded_by_user_id (null до ответа), comment (комментарий клиенту), decision_type.

Если сохранённый PENDING уже истёк, GET возвращает status=EXPIRED. Сам GET не изменяет БД: физический статус заявки может оставаться OFFERED. Финальное принятие всегда проверяет срок ещё раз под блокировкой. Срок принятого предложения не отменяет уже совершённое принятие.

### POST /api/v1/applications/{id}/offers/{offerID}/respond

```json
{"data_version": 3, "action": "ACCEPT"}
```

Или action=DECLINE. data_version обязателен и должен совпасть с текущей версией заявки и версией данных решения. offerID должен принадлежать этой заявке. Принимается только PENDING до valid_until и только при статусе заявки OFFERED.

Ответ 200: {application_id, data_version, status, offer}. ACCEPT переводит предложение в ACCEPTED, заявку в ACCEPTED и сохраняет accepted_offer_id. DECLINE переводит предложение в DECLINED, заявку в OFFER_DECLINED. Заполняются responded_at/responded_by_user_id; другие PENDING этой заявки отзываются (WITHDRAWN). Запись статуса, ответа и истории атомарна.

Повтор такого же ответа на уже завершённую операцию безопасен и возвращает 200 без новой истории. Противоположный ответ, ответ на старую версию или другое состояние — 409. Просроченный PENDING — 409 offer has expired. При двух одновременных противоположных ответах успешен только один. Принятие предложения не означает подписание или выдачу кредита.

Ошибки: 400 Invalid request body / Invalid application input; 404 Application not found; 409 Application version conflict / operation is not allowed in the current state / offer has expired; общие 401/403/500.

## Запросы дополнительных данных — обновление 25.10.2026

### GET /api/v1/applications/{id}/info-requests

Ответ 200 — массив запросов, новые первыми; пустой массив — []. Поля: id, application_id, created_by_user_id (сотрудник/null), description, required_items (массив строк), status, response_comment (string/null), created_at, responded_at (null до ответа), resolved_at (null до завершения).

Статусы запроса: OPEN → RESPONDED → RESOLVED. Отдельный запрос может быть открыт только сотрудником, назначенным на MANUAL. После создания статус заявки — NEEDS_INFO.

### Действия клиента в NEEDS_INFO

1. Прочитать description и required_items. Это список требований специалиста, а не автоматическая проверка наличия файлов.
2. Загрузить запрошенные PDF. document_type=SUPPORTING_DOCUMENT сохраняет вложение без изменения финансов. FINANCIAL_STATEMENT вызывает DS и создаёт новую неподтверждённую версию финансов.
3. При необходимости исправить финансы через существующий PUT financials. Он также создаёт новую версию.
4. Подтвердить текущий набор через POST financials/confirm с просмотренным financial_data_version.
5. Отправить ответ ниже с актуальным data_version заявки.

### POST /api/v1/applications/{id}/info-requests/{requestID}/respond

```json
{"data_version": 4, "response_comment": "Дополнительный PDF загружен, финансовые данные проверены"}
```

Комментарий обязателен, после trim непустой, до 2000 символов. Заявка должна быть NEEDS_INFO, запрос — OPEN и принадлежать этой заявке. Перед ответом нужны шесть подтверждённых месяцев текущей версии и заявленный платёж по долгам.

Go повторно вызывает DS, сохраняя первоначальную application_date из submitted_at. Заявка возвращается в MANUAL к текущему назначенному сотруднику, даже если новый score стал 70+: начатое ручное рассмотрение завершает сотрудник. Если назначение было освобождено, заявка возвращается в общую очередь без исполнителя. Сам ответ не увеличивает data_version.

Ответ 200 — полный Application. Запрос становится RESPONDED; окончательное решение банка переводит его в RESOLVED. При ошибке DS ни ответ, ни новый скоринг, ни статус не фиксируются. Повтор ответа после успеха — 409 без повторного скоринга.

Ошибки: 400 для тела/UUID/пустого комментария; 404 для чужой заявки/запроса; 409 для версии или состояния; 422 для неподтверждённых финансов/невалидного периода; ошибки DS — через общий mapper. В NEEDS_INFO используется этот respond, а не submit (submit работает только для DRAFT).
