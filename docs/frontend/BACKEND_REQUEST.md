# Что фронтенду нужно от backend

Сейчас готовы только авторизация и чтение активных продуктов. Остальные экраны работают на моках.

## Приоритет 1: компания

### Поиск по ИНН

`GET /api/v1/companies/lookup?inn=6671123456`

```json
{
  "inn": "6671123456",
  "name": "ООО «Север»",
  "legal_form": "ООО",
  "registered_at": "2018-03-14",
  "okved": "62.01"
}
```

Нужны отдельные ошибки: неверный ИНН, компания не найдена, реестр недоступен.

### Профиль компании

- `POST /api/v1/client/company` — сохранить ИНН, ФИО представителя и должность.
- `GET /api/v1/client/company` — получить компанию текущего клиента.

## Приоритет 2: заявка

- `POST /api/v1/client/applications` — создать черновик.
- `PATCH /api/v1/client/applications/{id}` — сохранить шаг.
- `POST /api/v1/client/applications/{id}/submit` — отправить.
- `GET /api/v1/client/applications` — список клиента.
- `GET /api/v1/client/applications/{id}` — карточка, статус и история.

Минимальные поля: `product_code`, `amount_kopecks`, `term_months`, `purpose`, `comment`, `monthly_debt_payment_kopecks`, `status`.

## Приоритет 3: документы

- `POST /api/v1/client/applications/{id}/documents` — multipart PDF до 10 МБ.
- `GET /api/v1/client/applications/{id}/financials` — статус обработки и 6 месяцев показателей.
- `PUT /api/v1/client/applications/{id}/financials` — исправленные показатели.

Нужны состояния: `UPLOADED`, `PROCESSING`, `EXTRACTED`, `FAILED`.

## Приоритет 4: статус и оффер

- `POST /api/v1/client/applications/{id}/additional-data` — ответ на запрос банка.
- `GET /api/v1/client/applications/{id}/offer` — текущее предложение.
- `POST /api/v1/client/applications/{id}/offer/accept`.
- `POST /api/v1/client/applications/{id}/offer/decline` с обязательной причиной.

Оффер: сумма, срок, ставка bps, платёж в копейках, срок действия, тип и раунд.

## Приоритет 5: сотрудник банка

- `GET /api/v1/bank/applications?inn=&status=` — очередь.
- `GET /api/v1/bank/applications/{id}` — компания, финансы, проверки, скоринг.
- `POST /api/v1/bank/applications/{id}/request-data`.
- `POST /api/v1/bank/applications/{id}/approve`.
- `POST /api/v1/bank/applications/{id}/reject`.

## Общие требования

- Bearer JWT и проверка роли на backend.
- Деньги только integer в копейках.
- Ставка integer в базисных пунктах.
- Даты ISO 8601.
- Единый JSON ошибок: `{ "code": "...", "message": "...", "fields": {} }`.
- Для каждого перехода статуса записывать историю: кто, когда, откуда и куда.
- Владение заявкой проверять на сервере.
- Предоставить seed-клиента и seed-сотрудника для демонстрации.
