# OpenAPI

`go.json` — OpenAPI 3.0.3 для всех 36 реальных маршрутов Go из `backend/cmd/main.go`.
Схемы сверены с `internal/*/model.go`; при изменении маршрута/DTO обновлять этот файл и `../requests/alphahub.http` в том же коммите.
Импортируйте файл в Postman (Import → Files) или откройте локальным Swagger Editor.
Спецификация описывает существующее поведение; генерация серверного кода не применяется.

DS: актуальная спецификация работающего сервиса — `http://localhost:8001/openapi.json`,
Swagger UI — `http://localhost:8001/docs`. Зафиксированный контракт также находится в `../ds/`.
`DetailedScoringResponse` не используется HTTP-ручкой скоринга: она возвращает `ScoringResponse`.

Ошибки Go намеренно описаны в обоих фактических форматах: текст `http.Error` и JSON-конверт DS.
Документация не меняет формат ответа приложения.

Проверка покрытия актуальных маршрутов и DTO: из корня `python scripts/check_api_docs.py`. Скрипт ничего не изменяет. Для формальной проверки схемы OpenAPI подходит openapi-spec-validator. У Go нет встроенной Swagger UI-страницы; файл go.json импортируется в Postman или Swagger Editor.
