# Сценарии приёмки

Все проверки ниже относятся к реальным Go/DS API. Запуск — README, запросы — `requests/alphahub.http` (открыть в VS Code с REST Client; блоки выполнять по одному). Обычный curl.exe также подходит; SQL вручную для сценариев не нужен. При первом запуске октября 2026 можно использовать готовые PDF. Для другого месяца сначала сгенерировать свежие PDF по DEMO_DATA.md.

## Общая подготовка каждой заявки

1. Войти соответствующим демо-клиентом: POST `/auth/login`, получить access_token. Authorization: Bearer TOKEN для клиентских запросов.
2. GET `/auth/me` и `/company/me`: нужная роль/ИНН. GET `/products`: два активных продукта.
3. POST `/applications` с `{"product_code":"WORKING_CAPITAL"}` → 201, DRAFT. Сохранить id.
4. GET `/applications/{id}`, затем PATCH с текущим `data_version`, суммой в копейках, сроком и purpose → 200. В PATCH обязательно передать company_id из GET /company/me: связь пользователя с компанией сама не заполняет компанию черновика.
5. POST `/applications/{id}/documents` multipart: `file` — PDF той же компании, `document_type=FINANCIAL_STATEMENT` → 201. GET financials: шесть месяцев и долг; повторно GET application для актуальной версии.
6. Если правите показатели — PUT financials со всеми шестью строками, долгом и актуальной версией. После любого изменения перечитать версию.
7. POST `/financials/confirm` с JSON `{"data_version":N}` → 200; `is_confirmation_current=true`. POST `/submit` → 200 и итоговый статус; GET `/scoring` сверить балл/модель/снимок входа.

## Автоматическое одобрение и принятие

Клиент client@demo.ru, stable.pdf, 200000000 коп. / 12 мес. / 1990 bps. Ожидается score=83, model_version=financial_rules_v2, OFFERED. GET offers → PENDING на запрошенных условиях, срок действия 7 дней. POST `/offers/{offerID}/respond` с action=ACCEPT и актуальной data_version → ACCEPTED и выбранный accepted_offer_id. Повтор того же ACCEPT безопасен; противоположное действие → 409. Проверить сохранение после перезагрузки API. Подпись/выдача не проверяются: серверные ручки отсутствуют.

## Автоматический отказ

Клиент risky@demo.ru, risky.pdf, **500000000** коп. / 12 мес. (не исходные 600000000 из DS fixture). Ожидается score=16, REJECTED; AUTO-решение REJECT и отсутствие PENDING-оффера. Балл обусловлен финансовыми данными, не технической ошибкой DS.

## Ручная проверка и одобрение сотрудником

Клиент medium@demo.ru, medium.pdf, 100000000 коп. (1 млн ₽) / 18 мес. Ожидается score=57, model_version=financial_rules_v2, MANUAL. Прежние 50000000 коп. дают 73 балла и OFFERED, поэтому для ручных сценариев используйте 1 млн ₽.
Войти bank@demo.ru. GET `/bank/applications/page?q=7700000023` → заявка в items, корректный total. GET карточки → компания, финансовые данные, скоринг, документы и история. POST claim с текущей версией → assigned_employee_id сотрудника. Перечитать версию, POST decisions action=APPROVE, reason_code, comment и future valid_until без offer → 201, OFFERED. Оффер повторяет запрошенные сумму/срок и сохранённую ставку.

## Встречное предложение

Создать отдельную medium-заявку и взять в работу. POST decisions: action=COUNTER_OFFER, reason_code=ADJUSTED_TERMS, непустой comment, valid_until в будущем, offer={amount_kopecks:40000000,term_months:18,annual_rate_bps:1990}, актуальная версия → 201, OFFERED; новый платёж рассчитан сервером. Клиент GET offers → видит именно новые условия; ACCEPT → ACCEPTED. Отдельным сценарием DECLINE → OFFER_DECLINED, не REJECTED. Нового автоматического улучшенного оффера после DECLINE нет.

## Отказ сотрудника

Отдельная medium-заявка, claim. POST decisions action=REJECT, reason_code=MANUAL_REJECTION и непустой comment, без offer/valid_until → 201, REJECTED. Решение появляется в `/bank/decisions` и заявка в `/bank/archive`; в ручной очереди её больше нет.

## Запрос данных и возврат

Отдельная medium-заявка, claim. POST bank info-requests с description, required_items (непустой массив строк), data_version → 201 и NEEDS_INFO. Клиент GET info-requests → OPEN. Если нужно, загрузить SUPPORTING_DOCUMENT либо исправить финансы/подтвердить актуальную версию. POST client `/info-requests/{requestID}/respond` с response_comment и текущей версией → MANUAL; запрос RESPONDED. Повторный скоринг использует первоначальные submitted_at и annual_rate_bps. Высокий пересчитанный балл не создаёт автоматический оффер в обход сотрудника.

## Ошибки и права

| Шаг | Ожидается |
|---|---|
| Финансы confirm без JSON-тела | 400, а не ложный успех |
| PATCH / confirm со старой data_version | 409, данные не перезаписаны |
| Подать неподтверждённые финансы | 422, кредитный отказ не создаётся |
| PDF другой компании | 422, ИНН проверяется |
| PDF больше 10 MiB | 413 |
| Не-PDF / повреждённый PDF | 400 либо ошибка извлечения, без фиктивных финансов |
| Старый финансовый период при подаче | 422, дата берётся с сервера |
| CLIENT вызывает bank route | 403 |
| Без токена или с истёкшим токеном | 401 |
| Чужая заявка клиенту | 404, содержимое не раскрывается |
| Решение без claim / для не-MANUAL | 409 |
| Подтверждение после правки без нового чтения версии | 409 |
| Неизвестный продукт / сумма или срок вне границ | Ошибка 400 или 422 по операции, заявка не подана |
| Недоступный DS при submit | 503; транзакция откатывается, REJECTED не создаётся |
| DS вернул 500 / некорректный ответ | 502; техническая ошибка не превращается в score=0 |

## Автоматические проверки

Готовый прогон семи реальных сценариев через nginx → Go → DS → PostgreSQL:

```powershell
# Из корня. Создаёт новые демо-заявки; используйте отдельную тестовую базу/Compose-проект.
python scripts/smoke_demo.py --base-url http://localhost:5173
```

Использует публичные демо-аккаунты миграции 19. Проверяет PDF upload/download, подтверждение, реальные баллы и статусы, права доступа, устаревшие версии, ручные решения и запрос данных. Скрипт рассчитан на фиксированные PDF октября 2026 и явно откажется запускаться в другом месяце; для другого периода сначала обновите демонстрационный набор и ожидания.

```powershell
# Из backend/, интеграционные тесты в отдельных схемах PostgreSQL
$env:TEST_DATABASE_URL='postgres://alphahub:alphahub_dev_local@localhost:5433/alphahub?sslmode=disable'
go test ./... -count=1
```

DS: `python -m pip install -r requirements-dev.txt`. Из ds/:

```powershell
python -m unittest test_api test_metrics test_schema_contract test_financials_api test_pdf_parser test_pdf_validation test_demo_scenarios
python test_profiles.py
python test_scoring.py
python test_score_result.py
python test_risk_flags.py
python test_pdf_generator.py
```

`test_flexible_pdf_parser.py` зависит от неотслеживаемых `docling_output/stable_p1-1.md` и CSV; его отсутствие на чистом checkout не следует выдавать за проход полной DS-suite. Для Docling нужен отдельный подготовленный набор артефактов. Обычный pytest-сбор всего ds/ не является поддерживаемой командой: часть файлов — исполняемые сценарии, а функция test_pdf(profile) не pytest-fixture.

Фронт: `npm ci`, `npm run typecheck`, `npm run build`. Здесь проверяется сборка, а не все экраны браузера. Отдельные новые правки UI/моков здесь не выполнялись; опубликованный коммит удаления моков включён при слиянии main.
