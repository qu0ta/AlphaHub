# AlphaHub: документация для фронтенда

Сверено с исходниками backend 20.10.2026. Описывает текущую реализацию.
Runtime-проверка с PostgreSQL при подготовке этих документов не проводилась.

- [Полный контракт API](API.md): запросы, ответы, ошибки, примеры.
- [Авторизация и UI](AUTH.md): JWT, роли, сессия, выход.
- [TypeScript-интеграция](INTEGRATION.md): типы, fetch, Vite proxy.
- [Локальный запуск](LOCAL_DEVELOPMENT.md): БД, миграция, окружение и smoke test.
- [Ограничения и QA](LIMITATIONS_AND_QA.md): дефекты и проверка интеграции.

| Метод | Путь | Назначение | Токен |
| --- | --- | --- | --- |
| POST | /api/v1/auth/register | Регистрация | Нет |
| POST | /api/v1/auth/login | Получить JWT | Нет |
| GET | /api/v1/auth/me | Текущий пользователь | Bearer |

Локальный origin: http://localhost:8080, порт меняется HTTP_PORT.
Production URL не задан кодом. Префикс API: /api/v1.
Регистрация не выдаёт токен: после неё нужен login, затем me.
Успех — JSON без обёртки data, ошибки — обычный текст.

API заявок, компаний, документов, предложений, списков пользователей и редактирования профиля нет.
Нет refresh/logout, восстановления пароля, подтверждения email, пагинации, фильтров, сортировки,
загрузки файлов, WebSocket и SSE. Функция Health есть, но /health не подключён к маршрутизатору.

Источники истины: backend/cmd/main.go; backend/internal/auth/{handler,model,service,repository}.go;
backend/internal/platform/db/postgres.go; backend/migrations/README.md; compose.yaml.
Рекомендации для UI не означают, что такой frontend уже написан.
