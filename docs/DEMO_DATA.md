# Демо-данные

Миграции 17/18/19 — единственный необходимый SQL-seed для чистого Docker-запуска. Пароль всех четырёх аккаунтов `AlphaHubTest2026!`; пароль хранится как bcrypt cost 10. Сиды идемпотентны по конфликтам и не меняют существующие записи. Клиенты привязываются по INN, поэтому UUID существующей компании может отличаться от исходного демо-UUID.

Синтетические fixtures `ds/fixtures/{stable,medium,risky}.json` и PDF `ds/demo_pdfs/` используют фиксированные seed=42 и дату 2026-10-22. Это искусственные показатели, не сведения о реальных компаниях. Названия/ИНН служат демонстрационными идентификаторами. Генератор — `ds/generator.py`, PDF — `ds/pdf_generator.py`.

Для `financial_rules_v2`, продукта `WORKING_CAPITAL` и ставки 1990 bps используйте:

| PDF | Сумма, ₽ | Срок, мес. | Score | Статус |
|---|---:|---:|---:|---|
| stable.pdf | 2 000 000 | 12 | 83 | OFFERED |
| medium.pdf | 1 000 000 | 18 | 57 | MANUAL |
| risky.pdf | 5 000 000 | 12 | 16 | REJECTED |

Финансовые PDF и fixtures не менялись. Для ручного сценария изменена только запрошенная сумма: прежние 500 000 ₽ / 18 мес. для medium.pdf теперь дают 73 балла и OFFERED. Ожидания smoke-скрипта проверяет `ds/test_demo_scenarios.py` через API извлечения и скоринга.

Чтобы сделать свежие PDF, установите `ds/requirements-dev.txt`. Из каталога `ds/` запустите следующий Python-код (сохраните, например, как локальный `refresh_demo.py`):

```python
from datetime import date
from pathlib import Path
from generator import generate_company_financials
from pdf_generator import create_financial_pdf

names = {"stable": "ООО «Северный Маяк»", "medium": "ООО «Деловой Вектор»", "risky": "ООО «Городские Решения»"}
for profile, name in names.items():
    data = generate_company_financials(profile=profile, seed=42, reference_date=date.today())
    create_financial_pdf(data, name, Path("generated_demo") / f"{profile}.pdf")
```

Генератору нужен кириллический TTF: на Windows используется системный Arial; на Linux установите DejaVu Sans или положите `ds/fonts/DejaVuSans.ttf` с его лицензией. Генерация выполняется отдельно от минимального runtime-контейнера: reportlab не нужен для чтения уже готовых PDF. Не перезаписывайте фиксированные fixtures, на них опираются регрессионные тесты.

В заявке risky ограничьте сумму 5 млн ₽, хотя исходный fixture запрашивает 6 млн: лимит продукта проверяется Go, и PDF содержит финансовые показатели, а не обязательные условия запрошенного кредита.

Docling — необязательный fallback для иных раскладок таблицы. Он не включён в базовый requirements/runtime и может требовать отдельные веса и загрузки. Неподдерживаемый PDF возвращает явную ошибку. Три стандартных demo PDF обрабатываются pdfplumber без Docling и внешних моделей.
