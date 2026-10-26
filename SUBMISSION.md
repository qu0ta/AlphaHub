# Комплект для технической и продуктовой проверки

Репозиторий: https://github.com/qu0ta/AlphaHub. Версия материалов: **submission-2026-10-26** (Git tag; итоговый SHA: `git rev-parse "submission-2026-10-26^{commit}"`). Коммиты после дедлайна не входят в оцениваемую версию: в форме сдачи укажите этот тег и его SHA. Дату и время дедлайна определяют организаторы.

Для приватного репозитория приглашение аккаунту жюри `alfa-hack-2026` отправлено владельцем. Принятие приглашения требуется проверить в GitHub Manage access; отправленное приглашение ещё не означает подтверждённый доступ.

| Требование | Материал |
|---|---|
| Код и история | GitHub, тег версии выше |
| Единый Docker-запуск | compose.yaml и Dockerfile в backend/, frontend/, ds/ |
| Фиксация языка и библиотек | Dockerfile, backend/go.mod и go.sum, frontend/package-lock.json, ds/requirements.txt; отдельные исследовательские зависимости ds/experiments/requirements.txt |
| Назначение, окружение, конфигурация, запуск, компоненты и сценарий | README.md и .env.example |
| Синтетические данные и аккаунты | Миграции 17–19, ds/demo_pdfs/, ds/fixtures/, docs/DEMO_DATA.md |
| Сторонние компоненты, модели, датасеты, моделируемые интеграции | THIRD_PARTY_NOTICES.md и docs/licenses/ |
| Архитектура | docs/ARCHITECTURE.md |
| Описание API | docs/API_CONTRACTS.md, docs/frontend/, docs/ds/CONTRACT.md |
| OpenAPI/Swagger | docs/openapi/go.json (36 операций), DS /docs и /openapi.json, docs/ds/openapi.json |
| Коллекция запросов | docs/requests/alphahub.http (все 36 операций Go и две DS) |
| Состояния заявки | docs/APPLICATION_STATES.md |
| Модель данных | docs/DATA_MODEL.md, миграции 1–20 |
| Тестовые сценарии | docs/TEST_SCENARIOS.md, scripts/smoke_demo.py; результаты docs/DELIVERY_CHECK.md |
| Аналитика и сравнение формулы/ML | docs/ds/SCORING_BENCHMARK.md, SCORING_BENCHMARK_RESULTS.md и ds/experiments/README.md |

## Самостоятельная проверка

1. Установить Docker Desktop/Engine с Linux containers и Compose v2, клонировать репозиторий и выбрать тег версии.
2. Из корня выполнить `docker compose up --build -d`, дождаться healthy; migrate должен завершиться с кодом 0.
3. Открыть http://localhost:5173. Демо-аккаунты: client@demo.ru, medium@demo.ru, risky@demo.ru, bank@demo.ru. Пароль всех аккаунтов: AlphaHubTest2026!.
4. Пройти README и docs/TEST_SCENARIOS.md либо выполнить `python scripts/smoke_demo.py --base-url http://localhost:5173` в тестовом экземпляре: скрипт создаёт демо-заявки.
5. Для просмотра DS API открыть http://localhost:8001/docs. Go OpenAPI импортировать из docs/openapi/go.json. Запросы .http выполнять по одному; альтернативные решения взаимоисключающие.

Демо-PDF относятся к апрелю–сентябрю 2026 и подходят для первой подачи в октябре 2026; для другого периода см. docs/DEMO_DATA.md. OCR/Docling не включён: используйте стандартные stable.pdf, medium.pdf и risky.pdf с соответствующими компаниями.

## Границы поставки

Базовый Docker запускает формульный financial_rules_v2. ML-эксперименты и искусственные метки вынесены в отдельное исследовательское окружение, не выдают реальную банковскую вероятность дефолта. ЕГРЮЛ/ФНС/БКИ моделируются локально. Серверные подпись/выдача и улучшенный оффер после отклонения не реализованы; в диаграммах эти этапы обозначены отдельно. Публикация в облаке — желательная, но не обязательная часть предоставленного списка; публичный стенд здесь не заявлен.
