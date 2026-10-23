# AlphaHub — Documents API

Контракт по реализации ветки `backend`, коммит `e18be1a`.

## Общие правила

Базовый префикс: `/api/v1`.

Все маршруты требуют:
- `Authorization: Bearer <access_token>`;
- роль `CLIENT`.

Доступ к документам имеет только создатель заявки
(`created_by_user_id`), а не любой участник компании.
Чужая и отсутствующая заявки возвращают одинаковый 404.

Успешные ответы: `application/json`, без обёртки `data`.
Ошибки: `text/plain; charset=utf-8` с переводом строки.
На фронтенде читать ошибки через `response.text()`.

## Модель Document

| Поле | Тип | Описание |
|---|---|---|
| id | string | UUID документа |
| application_id | string | UUID заявки |
| document_type | string | При загрузке FINANCIAL_STATEMENT |
| original_filename | string | Имя файла без пути |
| processing_status | string | При загрузке UPLOADED |
| uploaded_at | string | Время RFC 3339, возможны доли секунды |

Путь хранения и содержимое файла не возвращаются.

## POST /api/v1/applications/{id}/documents

Загружает PDF в собственную заявку со статусом `DRAFT`.

Тело: `multipart/form-data`.
Ровно один файл в поле `file`.

Ограничения:
- ID заявки и пользователя должны быть корректными UUID.
- Файл непустой и не больше 10 MiB (10 485 760 байт).
- Расширение `.pdf`, регистр не важен.
- Содержимое начинается с `%PDF-`.
- Полная структура PDF не проверяется.
- Имя после удаления пути — не более 255 байт UTF-8.
- Общий размер multipart-тела — не более 11 MiB.

Тип документа и статус назначает сервер.

### Пример запроса

```bash
curl -X POST \
  "http://localhost:8080/api/v1/applications/$APPLICATION_ID/documents" \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -F "file=@statement.pdf;type=application/pdf"
```

### Ответ: 201 Created

```json
{
  "id": "ed874d62-51e8-4df7-99a8-26cd836d37aa",
  "application_id": "a58b3260-3a7d-4f56-b766-d2bc41d73b38",
  "document_type": "FINANCIAL_STATEMENT",
  "original_filename": "statement.pdf",
  "processing_status": "UPLOADED",
  "uploaded_at": "2026-10-23T12:00:00Z"
}
```

Каждая загрузка создаёт новый документ, без замены предыдущего.
Файл сохраняется локально в `uploads` под сгенерированным именем.
При ошибке записи в БД сервер пытается удалить сохранённый файл.

## GET /api/v1/applications/{id}/documents

Возвращает документы собственной заявки.
Ограничений по статусу заявки нет.

```bash
curl \
  "http://localhost:8080/api/v1/applications/$APPLICATION_ID/documents" \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```

### Ответ: 200 OK

Массив объектов Document. При отсутствии документов — `[]`.

Сортировка: `uploaded_at DESC, id DESC`.
Пагинации и фильтров нет.

## Ошибки

| HTTP | Текст | Причина |
|---|---|---|
| 400 | invalid document input | Некорректный UUID, неверный формат multipart или количество файлов в поле file |
| 400 | invalid PDF | Пустой файл, неподходящее имя/расширение или отсутствие префикса %PDF- |
| 401 | Bearer token is required | Нет корректного Bearer-заголовка |
| 401 | Invalid or expired token | Недействительный JWT |
| 401 | Invalid user ID | В JWT отсутствует непустой строковый sub |
| 401 | Invalid user role | Недопустимая роль в JWT |
| 401 | Unauthorized | Нет контекста авторизации |
| 403 | Access denied | Роль отличается от CLIENT |
| 404 | Application not found | Заявка чужая или отсутствует |
| 409 | Application is not a draft | Загрузка в заявку не в статусе DRAFT |
| 413 | PDF exceeds 10 MiB | Превышен лимит файла или общего тела запроса |
| 500 | Internal server error | Ошибка БД, хранилища или чтения файла |

Проверка формата загрузки может вернуть ошибку раньше проверки
существования и статуса заявки.

## Пример для фронтенда

```javascript
async function uploadDocument(baseUrl, applicationId, token, file) {
  const body = new FormData();
  body.append("file", file);

  const response = await fetch(
    `${baseUrl}/api/v1/applications/${applicationId}/documents`,
    {
      method: "POST",
      headers: { Authorization: `Bearer ${token}` },
      body,
    },
  );

  if (!response.ok) {
    throw new Error(await response.text());
  }

  return response.json();
}
```

Не задавайте `Content-Type` вручную:
браузер добавляет multipart boundary автоматически.

## Ограничения реализации

Маршрутов скачивания, удаления, замены и запуска обработки PDF нет.
Статус `UPLOADED` означает загрузку, а не завершённый разбор PDF.

Контракт составлен по исходному коду.
Проверка запросами к работающему серверу не выполнялась.