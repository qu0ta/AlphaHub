# AlphaHub — Documents API

По текущему backend на 25 октября 2026 года. Все успехи JSON без обёртки, кроме скачивания PDF; ошибки — text/plain с переводом строки.

## Авторизация

Загрузка и список — CLIENT, Authorization: Bearer <access_token>. Доступ имеет только created_by_user_id заявки. Чужая и отсутствующая заявка возвращают одинаковый 404.

## Модель Document

id, application_id, document_type, original_filename, processing_status, uploaded_at. Все значения — строки; идентификаторы UUID, время RFC 3339. storage_key и путь файла не возвращаются.

## POST /api/v1/applications/{id}/documents

Только собственная заявка в DRAFT или NEEDS_INFO, с выбранной компанией. Тело multipart/form-data, ровно один файл в поле file. Не задавайте Content-Type вручную при использовании браузерного FormData: браузер добавляет boundary.

Необязательное текстовое поле document_type:

- FINANCIAL_STATEMENT — значение по умолчанию. Go вызывает DS extraction, проверяет ИНН, сохраняет PDF и шесть неподтверждённых месяцев. data_version увеличивается на 1. Ответный processing_status=PROCESSED.
- SUPPORTING_DOCUMENT — дополнительный PDF для специалиста. DS не вызывается, финансы, data_version и подтверждение не меняются. processing_status=UPLOADED.

Неизвестный тип и несколько полей document_type — 400. Клиент не может задать processing_status. Пустой document_type эквивалентен отсутствию поля.

Ограничения: непустой PDF до 10 MiB (10 485 760 байт), расширение .pdf без учёта регистра, первые байты %PDF-. Имя без компонентов пути — до 255 байт UTF-8. Общий multipart — до 11 MiB. Полная структура PDF локально не проверяется.

```bash
curl -X POST "http://localhost:8080/api/v1/applications/$APPLICATION_ID/documents" \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -F "file=@certificate.pdf;type=application/pdf" \
  -F "document_type=SUPPORTING_DOCUMENT"
```

Ответ 201 Created — Document. Каждая загрузка создаёт новый документ. Исходное имя сохраняется только в метаданных, файл хранится под сгенерированным ключом в uploads. При ошибке записи БД сервер пытается удалить файл. Для FINANCIAL_STATEMENT сбой DS/ИНН не создаёт документ или новую финансовую версию. Повторная проверка статуса/версии под блокировкой защищает от параллельного ответа на запрос данных или редактирования заявки.

В NEEDS_INFO финансовую выписку после извлечения нужно проверить и подтвердить через financials/confirm; затем ответить на info-request. Дополнительный PDF сам по себе не является подтверждением исправленных финансов.

## GET /api/v1/applications/{id}/documents

Ответ 200 — Document[], при отсутствии документов []. Любой статус собственной заявки. Порядок uploaded_at DESC, id DESC. Фильтров и пагинации списка документов нет.

## Скачивание PDF

- GET /api/v1/applications/{id}/documents/{documentID}/download — CLIENT-владелец, любой статус заявки.
- GET /api/v1/bank/applications/{id}/documents/{documentID}/download — BANK_EMPLOYEE, только поданная заявка, кроме DRAFT.

Проверяются роль в JWT и БД, идентификатор заявки и принадлежность документа именно этой заявке. ID сотрудника/владельца берётся из токена. Отсутствие, несовпадение заявки и недоступный документ — 404 Document not found.

Успех — 200 application/pdf, Content-Disposition: inline с безопасным именем, Cache-Control: no-store, X-Content-Type-Options: nosniff. Поддерживаются HEAD и Range (206; неверный диапазон — стандартный ответ ServeContent). Файловые пути не передаются в URL. Чтение ограничено корнем хранилища, без выхода через ../, абсолютные пути или симлинки наружу.

## Ошибки

| HTTP | Текст/значение | Причина |
|---|---|---|
| 400 | invalid document input | UUID, multipart, тип документа или количество файлов |
| 400 | invalid PDF | Пустой файл, имя/расширение или отсутствие %PDF- |
| 401/403 | Ошибки авторизации | Нет токена или неверная роль |
| 404 | Application not found | Чужая/отсутствующая заявка для списка/загрузки |
| 404 | Document not found | Недоступный документ или файл при скачивании |
| 409 | Application is not a draft | Для загрузки статус отличается от DRAFT и NEEDS_INFO |
| 409 | application changed during PDF extraction; retry upload | Версия изменилась во время обработки |
| 413 | PDF exceeds 10 MiB | Превышение размера файла или тела |
| 422 | select application company before uploading PDF | Компания не выбрана |
| 422 | PDF INN does not match application company | ИНН из извлечения отличается |
| 500 | Internal server error | Ошибка БД/хранилища |

Ошибки DS для FINANCIAL_STATEMENT обрабатываются общим dsclient/http_error.go; SUPPORTING_DOCUMENT от доступности DS не зависит. Порядок локальной валидации и проверки заявки может влиять на первую возвращённую ошибку.

Удаление и замена файла отдельными маршрутами пока не реализованы.
