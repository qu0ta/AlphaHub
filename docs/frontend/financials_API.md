# AlphaHub — Financials API

Сверено с кодом financials из коммита f47f12e, включённого в main. Контракт описывает текущую реализацию; запросы к работающему серверу не выполнялись. Требуется миграция 000016.

## Общие правила

Префикс: /api/v1. Все три маршрута требуют Authorization: Bearer <access_token> и роль CLIENT. Доступ имеет только создатель заявки. Чужая и отсутствующая заявки возвращают одинаковый 404.

Успех: application/json без обёртки data. Ошибки: text/plain; charset=utf-8 с завершающим переводом строки. Читать через response.text().

## Модель ответа

| Поле | Тип | Значение |
|---|---|---|
| application_id | string | UUID заявки |
| data_version | integer | Текущая версия заявки |
| declared_monthly_debt_payment_kopecks | integer или null | Заявленный ежемесячный платёж по долгам |
| financials | Month[] | Финансовые данные |
| confirmed_at | string или null | Время подтверждения RFC 3339 |
| confirmed_by_user_id | string или null | UUID подтвердившего пользователя |

Month содержит period_month (строка YYYY-MM), revenue_kopecks, operating_inflow_kopecks, operating_outflow_kopecks, tax_outflow_kopecks (целые неотрицательные int64, копейки). Для JavaScript учитывайте предел Number.MAX_SAFE_INTEGER.

## GET /api/v1/applications/{id}/financials

Ответ 200 — модель выше. Чтение разрешено при любом статусе заявки. Месяцы отсортированы по возрастанию.

Если финансовых записей ещё нет, возвращается 404, а не пустой массив.

Текущая реализация читает последнюю сохранённую версию финансов, но data_version берёт из самой заявки. После изменения полей заявки эти версии могут различаться; наличие confirmed_at не гарантирует, что текущая версия заявки подтверждена.

## PUT /api/v1/applications/{id}/financials

Сохраняет полный набор из шести разных месяцев. Только DRAFT. Content-Type: application/json.

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

Ответ 200 — сохранённые данные; data_version увеличен на 1, confirmed_at и confirmed_by_user_id равны null. Старые версии записей сохраняются. Порядок месяцев в ответе PUT соответствует запросу; GET сортирует их.

## POST /api/v1/applications/{id}/financials/confirm

Подтверждает финансовые записи текущей версии заявки. Только DRAFT. Тело не требуется и обработчиком не читается.

Нужно ровно шесть записей с data_version, равным текущей версии заявки. Иначе 400. Ответ 200 — финансовые данные с заполненными confirmed_at и confirmed_by_user_id. Версия заявки не увеличивается. Повторный вызов обновляет время подтверждения.

В запросе нет ожидаемой версии: обработчик подтверждает текущую версию под блокировкой, а не обязательно ту, которую пользователь ранее видел. Это ограничение текущей реализации.

## Ошибки

| HTTP | Текст | Причина |
|---|---|---|
| 400 | Invalid request body | Некорректный JSON PUT, неизвестные поля или превышение 16 KiB |
| 400 | Invalid financial input | Некорректный UUID или финансовые данные; при confirm нет шести записей текущей версии |
| 401 | Зависит от auth middleware | Отсутствующий, неверный или истёкший JWT |
| 403 | Access denied | Роль отличается от CLIENT |
| 404 | Financial data or application not found | Нет доступа, заявки или сохранённых финансов |
| 409 | Application is not a draft | PUT/confirm для заявки не в DRAFT |
| 409 | Application version conflict | Устаревший data_version в PUT |
| 500 | Internal server error | Ошибка БД |

## Последовательность для фронтенда

1. Получить заявку и её data_version.
2. Получить финансовые данные; 404 для новой заявки означает отсутствие сохранённого набора.
3. Отправить PUT с полным набором и текущей версией.
4. Сохранить новую версию из ответа.
5. Вызвать confirm после проверки пользователем.

Если затем изменить поля заявки, её версия увеличится. Для повторного подтверждения потребуется снова сохранить финансовые данные с актуальной версией.

## Границы реализации

Ручной ввод финансов не вызывает DS и не читает PDF. Загрузка документов не заполняет financial_records автоматически. Подтверждение не запускает скоринг. Текущий submit заявки не требует подтверждённых финансов — это отдельная незавершённая интеграция.
