# Проверка поставки — 25 октября 2026

Проверен локальный checkout на базе `2a25d4a` с существующими изменениями продукта/ставки и добавленными Docker-файлами, миграцией 19, документацией и smoke-скриптом. Удалённый `origin/main` уже содержит `99fb7d7` с фиксацией frontend-зависимостей и отдельными изменениями интерфейса. Этот коммит целиком не сливался, поскольку изменения экранов/моков исключены из текущей задачи.

| Проверка | Результат |
|---|---|
| Docker build backend/frontend/ds | Все три образа собраны |
| `docker compose up -d --wait` на чистом проекте `alphahub-delivery-check` | PostgreSQL, DS, Go и nginx healthy; migrate exited 0 |
| Миграции 1–19 на новой БД | Успешно; повторный migrate up — no change |
| Demo bank login | bank@demo.ru / AlphaHubTest2026! успешно |
| Семь сценариев `scripts/smoke_demo.py` через nginx | Все прошли с реальными Go, DS и PostgreSQL |
| `go test ./... -count=1` с TEST_DATABASE_URL | Все пакеты прошли; DB-тесты выполнялись в отдельных схемах |
| DS unittest: API, metrics, schema, financials API, PDF parser/validation | 52 теста прошли |
| Дополнительные DS scoring/score_result/risk_flags | 18 тестов прошли |
| DS profiles и PDF-generator сценарии | Прошли для stable/medium/risky |
| `pip install -r requirements-dev.txt`, `pip check` | Установка завершена, конфликтов зависимостей нет |
| Формальная проверка Go OpenAPI | OpenAPI 3.0.3 валиден, все 34 маршрута сверены с main.go |
| `npm run typecheck`, `npm run build` | Прошли |
| Хеши `frontend/src` до/после этой задачи | Без изменений |

Для изоляции использовались порты 15173 (nginx), 18080 (Go), 18001 (DS), 15433 (PostgreSQL), отдельные volumes проекта Compose. Существующая пользовательская БД не очищалась и не использовалась для smoke-заявок. В smoke-проверке создавались новые заявки: автоматические OFFERED/ACCEPTED и REJECTED, отказ клиента OFFER_DECLINED, решения сотрудника APPROVE/COUNTER_OFFER/REJECT и запрос данных NEEDS_INFO → MANUAL.

Проверенные баллы: stable 2 млн ₽ / 12 мес. — 75; medium 500 тыс. ₽ / 18 мес. — 65; risky 5 млн ₽ / 12 мес. — 15. Ставка WORKING_CAPITAL — 1990 bps, входные PDF за апрель–сентябрь 2026.

Не проверялись и не заявляются реализованными: все экраны в браузере, Docling-fallback без его локальных артефактов/моделей, реальная подпись/выдача, реальные внешние ЕГРЮЛ/БКИ/ФНС. Изменения из запрещённого списка интерфейса и моков не выполнялись. Должность представителя и отметки ручных исправлений отражены только в описании границ API.
