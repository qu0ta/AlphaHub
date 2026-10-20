# Локальный запуск

Нужны Docker Compose и Go 1.26.0 либо совместимый новый toolchain согласно go.mod.
Автозагрузка toolchain старым Go зависит от GOTOOLCHAIN и сети.
Начальные команды PowerShell выполняются из корня AlphaHub.

## База данных

~~~powershell
docker compose up -d postgres
docker compose ps
~~~

Compose поднимает только PostgreSQL 16, не backend.
Контейнер alphahub-postgres, БД/пользователь alphahub.
С хоста подключение localhost:5433, внутри контейнера порт 5432.
Данные сохраняются в volume postgres_data.

## Миграция

Автоматического применения нет. Дождитесь готовности БД, на новой базе выполните один раз:

~~~powershell
Get-Content -Raw -Encoding UTF8 backend/migrations/001_create_users.sql |
  docker compose exec -T postgres psql -U alphahub -d alphahub -v ON_ERROR_STOP=1
~~~

Миграция не идемпотентна: повторное выполнение сообщает о существующей таблице.
Не удаляйте volume для устранения этой ошибки. Проверить схему:

~~~powershell
docker compose exec -T postgres psql -U alphahub -d alphahub -c '\d users'
~~~

## Окружение

.env читается относительно текущего каталога; запускайте из backend.
Существующий .env не перезаписывайте. Пример backend/.env:

~~~dotenv
DATABASE_URL=postgres://alphahub:alphahub_dev_local@localhost:5433/alphahub?sslmode=disable
JWT_SECRET=<собственный-случайный-секрет-минимум-32-байта>
HTTP_PORT=8080
~~~

Пароль БД взят из локального compose.yaml.
JWT_SECRET — заполнитель: сгенерируйте собственный секрет минимум 32 байта.
Не используйте буквально текст в скобках. Секрет нужен только backend.
Можно задать переменные окружением: godotenv.Load не перезаписывает уже заданные.
.env исключён из Git.

## Запуск

~~~powershell
Set-Location backend
go run ./cmd
~~~

Ожидаются PostgreSQL connected и Server started on http://localhost:8080.
Порт зависит от HTTP_PORT. Доступность БД проверяется, наличие users при старте — нет.

## Smoke test в другом PowerShell

~~~powershell
$body = @{ name = 'Frontend Demo'; email = 'frontend-demo@example.com'; password = 'ExamplePass123!' } | ConvertTo-Json
$user = Invoke-RestMethod -Method Post -Uri 'http://localhost:8080/api/v1/auth/register' -ContentType 'application/json' -Body $body
$loginBody = @{ email = 'frontend-demo@example.com'; password = 'ExamplePass123!' } | ConvertTo-Json
$session = Invoke-RestMethod -Method Post -Uri 'http://localhost:8080/api/v1/auth/login' -ContentType 'application/json' -Body $loginBody
Invoke-RestMethod -Method Get -Uri 'http://localhost:8080/api/v1/auth/me' -Headers @{ Authorization = "Bearer $($session.access_token)" }
~~~

Создаёт локального пользователя. При повторе регистрации ожидается 409 — продолжайте с login.
Не публикуйте токен и не выводите в общий лог.

## Диагностика

| Симптом | Проверить |
| --- | --- |
| DATABASE_URL is required | Окружение и каталог запуска |
| Auth initialization failed | JWT_SECRET минимум 32 байта |
| PostgreSQL connection failed | Docker, DSN, порт 5433, готовность БД |
| register = 500 | Миграция, БД, серверные логи, длина полей |
| login = 401 при проблемах БД | Маскирование ошибки поиска в сервисе |
| curl работает, браузер нет | CORS, адрес запроса, dev proxy |
| /health = 404 | Маршрут не подключён |
| me = 401 | Bearer, срок JWT, JWT_SECRET, наличие пользователя |

Остановка без удаления данных: docker compose stop postgres.
