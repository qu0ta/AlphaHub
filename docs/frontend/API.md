# Контракт HTTP API

## Общие правила

Запросы register/login: Content-Type: application/json. Рекомендуется Accept: application/json.
Успешные ответы: application/json, объект без обёртки.
Ошибки обработчиков: text/plain; charset=utf-8, текст с переводом строки в конце.
Единого error.code, JSON-ошибок полей и request ID нет.

Лимит тела — 8192 байта. Отправляйте ровно один JSON-объект с известными полями.
Неизвестные поля запрещены декодером, но login игнорирует ошибку декодирования (см. ограничения).
Не полагайтесь на обработку null, повторяющихся ключей и вариантов регистра имён стандартным Go-декодером.
Content-Type входящего запроса явно не проверяется.
Path-параметров нет, query-параметры не используются.
Пути со слешем в конце отдельно не зарегистрированы.
Неизвестный путь: стандартный 404 Go ServeMux; неверный метод известного пути: 405 с Allow.
GET /me также допускает HEAD по правилам Go, без тела ответа.

## UserResponse

| Поле | Тип | Наличие | Значение |
| --- | --- | --- | --- |
| id | string | Всегда | UUID пользователя |
| name | string | Всегда | Имя |
| email | string | Всегда | Email |
| role | string | Всегда | CLIENT или BANK_EMPLOYEE согласно схеме БД |
| company_id | string | Необязательно | UUID компании; пустая строка не сериализуется |

~~~json
{
  "id": "86e85897-d21d-4a64-a4a0-00e04a479aef",
  "name": "Анна",
  "email": "anna@example.com",
  "role": "CLIENT"
}
~~~

Пароль и password_hash не возвращаются. UUID храните строками.
Существующий сотрудник может иметь role = BANK_EMPLOYEE и company_id со строковым UUID.
Публичного API назначения компании или создания сотрудника нет.

## POST /api/v1/auth/register

Авторизация не нужна.

~~~json
{
  "name": "Анна",
  "email": "anna@example.com",
  "password": "ExamplePass123!"
}
~~~

| Поле | Тип | Проверка |
| --- | --- | --- |
| name | string | Обязательно, TrimSpace, затем непустое |
| email | string | Обязательно, TrimSpace и lowercase, Go mail.ParseAddress и равенство parsed.Address нормализованному вводу |
| password | string | Обязательно, 8–72 байта UTF-8 включительно |

Email передавайте без отображаемого имени вида Name <email>.
Уникальность email проверяется без учёта регистра.
Пароль не обрезается и не нормализуется; цифры/спецсимволы/разные регистры не обязательны.
JS string.length не подходит для байтов: используйте new TextEncoder().encode(password).length.
Проверки сервиса идут по порядку: имя → email → пароль; возвращается первая ошибка.

name/email ограничены БД VARCHAR(255), но сервис длину не проверяет.
Превышение может привести к 500. Рекомендуется ограничить ввод фронтом; серверная проверка также нужна.
Не передавайте id, role, company_id, password_hash или password_confirmation.

### Успех

201 Created, UserResponse. name обрезано по краям; email обрезан и lowercase.
role всегда CLIENT; company_id отсутствует. Токен и cookie не выдаются.

### Ошибки

| HTTP | Текст | Причина |
| --- | --- | --- |
| 400 | Invalid register request | JSON, неизвестное поле, неверный тип, лимит тела |
| 400 | name is required | Имя пустое |
| 400 | invalid email | Неверный email |
| 400 | password must contain 8-72 bytes | Неверная длина пароля |
| 409 | Email already registered | Повтор email |
| 500 | Internal server error | БД, хеширование, другая внутренняя ошибка |

При нескольких JSON-значениях текст может содержать две ошибки; см. ограничения.

### curl (Bash / Git Bash / WSL)

~~~bash
curl -i http://localhost:8080/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"name":"Анна","email":"anna@example.com","password":"ExamplePass123!"}'
~~~

## POST /api/v1/auth/login

Авторизация не нужна.

~~~json
{
  "email": "anna@example.com",
  "password": "ExamplePass123!"
}
~~~

Оба поля — обязательные строки. email обрезается и приводится к lowercase.
Пароль не преобразуется. Login не повторяет проверку формата email и диапазона длины пароля регистрации.

### Успех

200 OK:

~~~json
{
  "access_token": "<JWT>",
  "token_type": "Bearer"
}
~~~

Оба поля обязательны. Нет expires_in, refresh_token и пользователя.
Токен выдаётся на 2 часа. Получите профиль отдельным запросом me.

### Ошибки

| HTTP | Текст | Причина |
| --- | --- | --- |
| 400 | Email and password are required | Не заполнены credentials после разбора |
| 401 | Invalid email or password | Нет пользователя, неверный пароль, также ошибка поиска в БД |
| 500 | Internal server error | Другая ошибка сервиса, например выдачи токена |

Ошибка decodeJSON игнорируется: malformed JSON не гарантирует чистый 400.

~~~bash
curl -i http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"anna@example.com","password":"ExamplePass123!"}'
~~~

## GET /api/v1/auth/me

Тело не требуется.

~~~http
Authorization: Bearer <access_token>
Accept: application/json
~~~

В заголовке должно быть два поля, разделённых пробелами; схема Bearer без учёта регистра.
Cookie и query token не используются.

Успех: 200 OK, UserResponse. Профиль, роль и компания читаются из БД на момент запроса.

| HTTP | Текст | Причина |
| --- | --- | --- |
| 401 | Bearer token is required | Нет заголовка, неверная схема или число частей |
| 401 | Invalid or expired token | Ошибка проверки JWT |
| 401 | User not found | Токен принят, пользователь отсутствует |
| 500 | Internal server error | БД или другая внутренняя ошибка |

~~~bash
curl -i http://localhost:8080/api/v1/auth/me \
  -H "Authorization: Bearer $ACCESS_TOKEN"
~~~

ACCESS_TOKEN — значение из login. Не подставляйте JWT_SECRET: это серверный секрет.
