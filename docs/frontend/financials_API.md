# AlphaHub — Financials API

Сверено с текущим кодом рабочей ветки backend 25 октября 2026 года, включая незакоммиченные изменения. Контракт описывает текущую реализацию; запросы к работающему серверу не выполнялись. Требуется миграция 000016.

## Общие правила

Префикс: /api/v1. Все три маршрута требуют Authorization: Bearer <access_token> и роль CLIENT. Доступ имеет только создатель заявки. Чужая и отсутствующая заявки возвращают одинаковый 404.

Успех: application/json без обёртки data. Ошибки: text/plain; charset=utf-8 с завершающим переводом строки. Читать через response.text().

## Модель ответа

| Поле                                  | Тип              | Значение                                                                             |
|---------------------------------------|------------------|--------------------------------------------------------------------------------------|
| application_id                        | string           | UUID заявки                                                                          |
| data_version                          | integer          | Текущая версия заявки; передавать в PUT                                              |
| financial_data_version                | integer          | Версия возвращённого набора финансов; передавать в поле data_version запроса confirm |
| is_confirmation_current               | boolean          | Подтверждён ли набор финансов для текущей версии заявки                              |
| declared_monthly_debt_payment_kopecks | integer или null | Заявленный ежемесячный платёж по долгам                                              |
| financials                            | Month[]          | Финансовые данные                                                                    |
| confirmed_at                          | string или null  | Время подтверждения RFC 3339                                                         |
| confirmed_by_user_id                  | string или null  | UUID подтвердившего пользователя                                                     |

Month содержит period_month (строка YYYY-MM), revenue_kopecks, operating_inflow_kopecks, operating_outflow_kopecks, tax_outflow_kopecks (целые неотрицательные int64, копейки). Для JavaScript учитывайте предел Number.MAX_SAFE_INTEGER.

## GET /api/v1/applications/{id}/financials

Ответ 200 — модель выше. Чтение разрешено при любом статусе заявки. Месяцы отсортированы по возрастанию.

Если финансовых записей ещё нет, возвращается 404, а не пустой массив.

GET читает заявку и последнюю сохранённую версию финансов в одной read-only транзакции REPEATABLE READ. data_version относится к заявке, financial_data_version — к финансовым записям. После изменения заявки версии могут различаться.

is_confirmation_current вычисляется как: ровно шесть месяцев, заполнены confirmed_at и confirmed_by_user_id, версия финансов совпадает с версией заявки. Одного confirmed_at недостаточно: это может быть подтверждение старого набора.

## PUT /api/v1/applications/{id}/financials

Сохраняет полный набор из шести разных месяцев. Только DRAFT или NEEDS_INFO. Content-Type: application/json.

```json
{
  "data_version": 1,
  "declared_monthly_debt_payment_kopecks": 1000000,
  "financials": [
    {"period_month":"2026-04","revenue_kopecks":100000000,"operating_inflow_kopecks":100000000,"operating_outflow_kopecks":60000000,"tax_outflow_kopecks":6000000},
    {"period_month":"2026-05","revenue_kopecks":100000000,"operating_inflow_kopecks":100000000,"operating_outflow_kopecks":60000000,"tax_outflow_kopecks":6000000},
    {"period_month":"2026-06","revenue_kopecks":100000000,"operating_inflow_kopecks":100000000,"operating_outflow_kopecks":60000000,"tax_outflow_kopecks":6000000},
    {"period_month":"2026-07","revenue_kopecks":100000000,"operating_inflow_kopecks":100000000,"operating_outflow_kopecks":60000000,"tax_outflow_kopecks":6000000},
    {"period_month":"2026-08","revenue_kopecks":100000000,"operating_inflow_kopecks":100000000,"operating_outflow_kopecks":60000000,"tax_outflow_kopecks":6000000},
    {"period_month":"2026-09","revenue_kopecks":100000000,"operating_inflow_kopecks":100000000,"operating_outflow_kopecks":60000000,"tax_outflow_kopecks":6000000}
  ]
}
```

Передавайте актуальный data_version заявки: несовпадение возвращает 409. Версия должна быть не меньше 1. Платёж обязателен, null запрещён, ноль допустим. Нужны ровно шесть разных месяцев и неотрицательные суммы. Непрерывность и привязка к дате подачи этим обработчиком не проверяются.

Лимит JSON — 16 KiB. Неизвестные поля, несколько JSON-значений и некорректные типы отклоняются с 400. Текущий DTO не отличает пропущенные месячные денежные поля или null от нуля: клиент должен передавать все значения явно.

Ответ 200 — сохранённые данные; data_version увеличен на 1, financial_data_version равен новой версии, is_confirmation_current равен false, confirmed_at и confirmed_by_user_id равны null. Старые версии записей сохраняются. Порядок месяцев в ответе PUT соответствует запросу; GET сортирует их.

## POST /api/v1/applications/{id}/financials/confirm

Подтверждает просмотренный пользователем набор финансов. Только DRAFT или NEEDS_INFO. Обязательное JSON-тело, Content-Type: application/json:

```json
{"data_version": 2}
```

В data_version передавайте financial_data_version именно просмотренного набора. Не подставляйте автоматически последнюю версию заявки после конфликта.

Сервер сравнивает ожидаемую версию с текущей версией заявки внутри транзакции под блокировкой FOR UPDATE. При несовпадении возвращает 409 Application version conflict. Отсутствующий data_version, null, 0 или отрицательное значение в текущем коде также дают 409. Пустое тело и некорректный JSON дают 400 Invalid request body. Декодер отклоняет неизвестные поля, несколько JSON-значений и тело больше 16 KiB.

Нужно ровно шесть записей с data_version, равным текущей версии заявки. Иначе 400. Ответ 200 — финансовые данные с заполненными confirmed_at и confirmed_by_user_id. Версия заявки не увеличивается. Повторный вызов обновляет время подтверждения.

После 409 заново получите заявку и финансы, покажите актуальные данные пользователю и запросите повторное подтверждение. Автоматически повторять confirm с новой версией нельзя.

## Ошибки

| HTTP | Текст                                   | Причина                                                                                                     |
|------|-----------------------------------------|-------------------------------------------------------------------------------------------------------------|
| 400  | Invalid request body                    | Некорректный JSON PUT/confirm, неизвестные поля или превышение 16 KiB                                       |
| 400  | Invalid financial input                 | Некорректный UUID или финансовые данные; при confirm нет шести записей текущей версии                       |
| 401  | Зависит от auth middleware              | Отсутствующий, неверный или истёкший JWT                                                                    |
| 403  | Access denied                           | Роль отличается от CLIENT                                                                                   |
| 404  | Financial data or application not found | Нет доступа, заявки или сохранённых финансов                                                                |
| 409  | Application is not a draft              | PUT/confirm для заявки не в DRAFT/NEEDS_INFO                                                                           |
| 409  | Application version conflict            | Несовпадение ожидаемой версии в PUT/confirm; в confirm также отсутствующая, null или неположительная версия |
| 500  | Internal server error                   | Ошибка БД                                                                                                   |

## Последовательность для фронтенда

1. Получить заявку и её data_version.
2. Получить финансовые данные; 404 для новой заявки означает отсутствие сохранённого набора.
3. Отправить PUT с полным набором и текущей версией.
4. Сохранить data_version и financial_data_version из ответа PUT.
5. После проверки пользователем вызвать confirm с JSON {"data_version": <просмотренный financial_data_version>}. При 409 обновить данные и повторить проверку пользователем.

Если затем изменить поля заявки, её версия увеличится. Для повторного подтверждения потребуется снова сохранить финансовые данные с актуальной версией.

## Границы реализации

Ручной PUT финансов не вызывает DS и не читает PDF. Подтверждение само по себе не запускает скоринг. В текущем backend загрузка PDF вызывает DS extraction и сохраняет финансовые данные, а submit проверяет подтверждение финансов текущей версии перед дальнейшей обработкой. Подробные ошибки и поведение этих операций относятся к контрактам документов и заявок.

## Дополнительные данные

В NEEDS_INFO разрешены PUT и confirm с теми же проверками версии. После исправления финансов или загрузки FINANCIAL_STATEMENT нужно подтвердить новую финансовую версию. Затем вызывается клиентский info-requests/{requestID}/respond: он повторно рассчитывает скоринг и возвращает заявку назначенному сотруднику в MANUAL. SUPPORTING_DOCUMENT не изменяет финансы и их подтверждение.
