# AlphaHub — материалы для проверки

Мы сделали кредитный кабинет для малого бизнеса. Клиент загружает финансовую сводку в PDF,
проверяет извлечённые показатели и подаёт заявку. Банк получает документы, объяснение скоринга
и историю действий в одной карточке.

Код проекта: [qu0ta/AlphaHub](https://github.com/qu0ta/AlphaHub).
Описание решения и команды — в [README](README.md).

## Запуск

Нужны Git, Docker с Linux containers и Docker Compose v2. На Windows используется WSL2.

```bash
git clone https://github.com/qu0ta/AlphaHub.git
cd AlphaHub
docker compose up --build -d
docker compose ps -a
```

После запуска `migrate` должен завершиться с кодом `0`, остальные сервисы — перейти в `healthy`.
Кабинет доступен по адресу [localhost:5173](http://localhost:5173),
Swagger DS — [localhost:8001/docs](http://localhost:8001/docs).
Настройки портов находятся в [.env.example](.env.example). Для стандартного запуска `.env` не нужен.

## Что показать

Пароль всех демо-аккаунтов: `AlphaHubTest2026!`.

| Аккаунт | PDF | Условия | Ожидаемый результат |
| --- | --- | --- | --- |
| `client@demo.ru` | [stable.pdf](ds/demo_pdfs/stable.pdf) | Оборотный кредит, 2 млн ₽ на 12 месяцев, 19,9% | 83 балла, предложение |
| `medium@demo.ru` | [medium.pdf](ds/demo_pdfs/medium.pdf) | Оборотный кредит, 1 млн ₽ на 18 месяцев, 19,9% | 57 баллов, ручное рассмотрение |
| `risky@demo.ru` | [risky.pdf](ds/demo_pdfs/risky.pdf) | Оборотный кредит, 5 млн ₽ на 12 месяцев, 19,9% | 16 баллов, отказ |
| `client@demo.ru` | [equipment.pdf](ds/demo_pdfs/equipment.pdf) | Кредит на оборудование, 2 млн ₽ на 24 месяца, 18% | 91 балл, предложение |

1. Начните со stable.pdf: создайте заявку, подтвердите финансовые данные, отправьте её и примите предложение.
2. Создайте заявку с medium.pdf. В отдельном сеансе войдите как `bank@demo.ru` и возьмите её в работу.
3. В карточке банка откройте PDF, посмотрите факторы скоринга и историю. Запросите пояснения или предложите другие условия.
4. На отдельных заявках проверьте автоматический отказ с risky.pdf и второй продукт с equipment.pdf.

Данные синтетические. PDF содержат апрель–сентябрь 2026 года и рассчитаны на первую подачу в октябре 2026 года.
Для другой даты нужен соответствующий отчётный период. Баллы выше приведены для указанных условий кредита.

## Документы

| Материал | Где находится |
| --- | --- |
| Описание проекта, технологии и запуск | [README.md](README.md) |
| Архитектура | [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) |
| Состояния заявки | [docs/APPLICATION_STATES.md](docs/APPLICATION_STATES.md) |
| Модель данных | [docs/DATA_MODEL.md](docs/DATA_MODEL.md) |
| API-контракты | [docs/API_CONTRACTS.md](docs/API_CONTRACTS.md) |
| Go OpenAPI | [docs/openapi/go.json](docs/openapi/go.json) |
| Контракт и OpenAPI DS | [CONTRACT.md](docs/ds/CONTRACT.md), [openapi.json](docs/ds/openapi.json) |
| Готовые HTTP-запросы | [docs/requests/alphahub.http](docs/requests/alphahub.http) |
| Демо-данные | [docs/DEMO_DATA.md](docs/DEMO_DATA.md) |
| Сценарии проверки | [docs/TEST_SCENARIOS.md](docs/TEST_SCENARIOS.md) |
| Методика сравнения Rules и ML | [docs/ds/SCORING_BENCHMARK.md](docs/ds/SCORING_BENCHMARK.md) |
| Сторонние компоненты и лицензии | [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) |

Команды тестов приведены в README. Сквозной сценарий запускается командой
`python scripts/smoke_demo.py --base-url http://localhost:5173` и создаёт новые заявки — для него нужна отдельная демо-база.

## Что входит в MVP

В приложении работает формульный скоринг `financial_rules_v2`. Мы отдельно сравнили правила и Logistic Regression
на синтетических данных; обученная модель не участвует в решениях кабинета. Балл скоринга не означает вероятность дефолта.

Стандартный запуск рассчитан на текстовые PDF. OCR и Docling в него не входят.
ЕГРЮЛ, ФНС и БКИ не подключены: реестр компаний локальный, внешние проверки демонстрационные.
Серверный процесс заканчивается принятием или отклонением предложения; дальнейшие подписание и выдача в интерфейсе симулируются.

Для сдачи нужно указать SHA выбранного коммита (`git rev-parse HEAD`) и проверить доступ аккаунта жюри
`alfa-hack-2026` к приватному репозиторию. Новую версию этих материалов пока не публиковали.
