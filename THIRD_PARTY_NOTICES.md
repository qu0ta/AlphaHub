# Third-party notices

Инвентарь для локальной демонстрационной поставки AlphaHub. Версии JavaScript фиксируются `frontend/package-lock.json`, Python runtime — `ds/requirements.txt`, Go — `backend/go.mod`/`go.sum`. Транзитивные зависимости также включены в [frontend-инвентарь](docs/licenses/frontend.md) и [Python-инвентарь](docs/licenses/python.md); лицензии взяты из metadata установленных пакетов/lockfile, исходные LICENSE и NOTICE пакетов сохраняются в их дистрибутивах. Этот документ не меняет лицензии сторонних компонентов и не назначает лицензию собственному коду проекта.

| Компоненты | Лицензия | Источник |
|---|---|---|
| React, React DOM, React Router, Vite, TypeScript, plugin-react, Vitest, @types | MIT (см. транзитивные исключения в инвентаре) | npm package-lock и LICENSE соответствующих пакетов |
| Lucide | ISC; часть иконок происходит из Feather под MIT | lucide-react LICENSE |
| github.com/golang-jwt/jwt/v5 | MIT | go.mod v5.3.1, LICENSE модуля |
| github.com/google/uuid | BSD-3-Clause | go.mod v1.6.0, LICENSE модуля |
| github.com/jackc/pgx/v5 и pgpassfile/pgservicefile/puddle | MIT | go.mod, LICENSE каждого модуля |
| github.com/joho/godotenv | MIT | go.mod v1.5.1, LICENSE модуля |
| golang.org/x/crypto, x/sync, x/text | BSD-3-Clause | go.mod, LICENSE каждого модуля |
| FastAPI, Pydantic, pdfplumber, pdfminer.six | MIT | wheel metadata и LICENSE, версии в requirements.txt |
| uvicorn, Starlette | BSD-3-Clause | wheel metadata и LICENSE |
| Остальные Python runtime-пакеты, включая cryptography, Pillow, pypdfium2 | См. точные SPDX/metadata в Python-инвентаре | LICENSE/NOTICE в wheel; pypdfium2 также содержит уведомления PDFium и его зависимостей |
| httpx (dev) | BSD-3-Clause | requirements-dev.txt, LICENSE |
| pytest (dev) | MIT | requirements-dev.txt, LICENSE |
| ReportLab (генерация PDF, dev) | BSD-3-Clause | requirements-dev.txt, LICENSE |
| PostgreSQL | PostgreSQL License | Docker postgres:16-alpine и COPYRIGHT PostgreSQL |
| golang-migrate | MIT | migrate/migrate:v4.20.1 |
| nginx | BSD-2-Clause | nginx:1.28.0-alpine, LICENSE nginx |
| Go toolchain | BSD-3-Clause | golang:1.26.0-bookworm, Go LICENSE |
| Python | PSF-2.0 и включённые notices | python:3.13.7-slim-bookworm |
| Node.js | MIT и включённые notices | node:24-slim, LICENSE Node.js |
| Debian / Alpine base layers | Разные лицензии системных пакетов | copyright/license-файлы пакетного менеджера внутри образа |

Для точной версии и текста уведомления используйте пакет из соответствующего lockfile, а не одноимённый пакет последней версии. При распространении готовых образов должны сохраняться лицензии системных библиотек и PDFium из исходных образов/wheels. Отдельная выгрузка SBOM не заменяет эти тексты.

## Данные, модели и шрифты

`ds/generator.py`, fixtures и demo_pdfs создают синтетические финансовые данные с фиксированным seed; банковская тайна, реальные выписки и внешние обучающие датасеты для демонстрации не используются. `financial_rules_v2` — собственная детерминированная формула; сторонних весов обученной модели в базовом Docker-образе нет.

Docling упоминается в коде как необязательный fallback, но в минимальную сборку не установлен. Его лицензии и лицензии загружаемых моделей необходимо включить отдельно перед распространением расширенной сборки. Нельзя считать наличие импорта подтверждением включённой ML-модели.

Генератор PDF ищет локально установленный шрифт с кириллицей (Arial или DejaVu Sans). Системный Arial не копируется в Docker/repo. При добавлении файла DejaVu Sans приложите его DejaVu/Bitstream Vera copyright и license. Уже сохранённые демо-PDF могут содержать подмножество шрифта; не извлекайте его для повторного распространения как отдельного TTF.

## Моделируемые интеграции

ЕГРЮЛ, ФНС/«Прозрачный бизнес», БКИ не вызываются как реальные внешние сервисы. Используются локальные записи компаний и демонстрационные блоки интерфейса/данных. Демо-ответы не являются отчётами этих организаций. Подпись и выдача кредита не интегрированы с платёжной/банковской системой.
