/**
 * Тексты ошибок бэкенда → понятные сообщения на русском.
 *
 * Go-бэкенд отвечает на ошибки коротким английским текстом (http.Error),
 * сервис DS — JSON вида {"error": {"code", "message"}} уже на русском.
 * Пользователю показываем только русский текст: перевод собран здесь,
 * чтобы при новой ошибке бэкенда поправить одно место.
 */

const MESSAGES: Record<string, string> = {
  // Вход и доступ
  "bearer token is required": "Сессия истекла. Войдите снова.",
  "invalid or expired token": "Сессия истекла. Войдите снова.",
  "invalid token": "Сессия истекла. Войдите снова.",
  "invalid user id": "Сессия недействительна. Войдите снова.",
  "invalid user role": "Сессия недействительна. Войдите снова.",
  unauthorized: "Нужно войти в аккаунт.",
  "access denied": "Недостаточно прав для этого действия.",
  "invalid email or password": "Неверная почта или пароль.",
  "invalid credentials": "Неверная почта или пароль.",
  "email and password are required": "Укажите почту и пароль.",
  "email already registered": "Эта почта уже зарегистрирована.",
  "user already exists": "Эта почта уже зарегистрирована.",
  "invalid email": "Введите корректную почту.",
  "name is required": "Укажите ФИО.",
  "password must contain 8-72 bytes": "Пароль должен быть от 8 до 72 символов.",
  "invalid login request": "Проверьте почту и пароль.",
  "invalid register request": "Проверьте данные регистрации.",
  "user not found": "Пользователь не найден.",

  // Компания
  "company not found": "Компания не найдена в базе сервиса.",
  "invalid company input": "Проверьте данные компании.",
  "user already has a company": "Вы уже привязаны к компании.",
  "company access denied": "Нет доступа к этой компании.",

  // Продукты
  "product not found": "Кредитный продукт не найден.",
  "product not found or inactive": "Кредитный продукт недоступен.",
  "invalid product code": "Кредитный продукт не найден.",

  // Заявка
  "application not found": "Заявка не найдена.",
  "invalid application input": "Проверьте данные заявки.",
  "application is not a draft": "Заявка уже отправлена — изменить её нельзя.",
  "application version conflict": "Заявка изменилась в другом окне. Обновите страницу.",
  "application has incomplete or invalid data": "Заполните все шаги заявки: компания, сумма и срок в пределах продукта, цель.",
  "confirm current financial data before submitting": "Подтвердите финансовые данные перед отправкой.",
  "operation is not allowed in the current state": "Сейчас это действие недоступно для заявки.",
  "offer has expired": "Срок действия предложения истёк.",
  "an information request is already open": "Запрос данных уже отправлен клиенту.",

  // Документы и финансы
  "pdf inn does not match application company": "ИНН в PDF не совпадает с ИНН компании в заявке.",
  "select application company before uploading pdf": "Сначала выберите компанию для заявки.",
  "pdf exceeds 10 mib": "Файл больше 10 МБ.",
  "pdf is empty": "Файл пустой.",
  "invalid pdf": "Файл не является корректным PDF.",
  "invalid document input": "Не удалось принять файл. Загрузите один PDF.",
  "document not found": "Документ не найден.",
  "application changed during pdf extraction; retry upload": "Заявка изменилась во время обработки файла. Загрузите PDF ещё раз.",
  "financial data or application not found": "Финансовые данные ещё не загружены.",
  "invalid financial input": "Проверьте значения в таблице финансов.",
  "invalid financial reporting period or scoring parameters": "Нужна сводка за 6 полных месяцев перед датой подачи (для октября 2026 — апрель–сентябрь).",

  // Банк
  "invalid bank application input": "Проверьте условия решения: сумма, срок, ставка от 15 до 30%, комментарий.",
  "bank application not found": "Заявка не найдена.",
  "bank employee access required": "Доступно только сотруднику банка.",
  "application is not in manual review": "Заявка уже не на ручной проверке — обновите страницу.",
  "application is assigned to another employee": "Заявку уже взял в работу другой сотрудник.",
  "claim the application before deciding": "Сначала возьмите заявку в работу.",
  "current scoring and confirmed financials are required": "Для одобрения нужны скоринг и подтверждённые финансы клиента.",
  "offer does not meet active product limits": "Сумма или срок вне лимитов продукта.",

  // Сервис скоринга и общее
  "ds service unavailable": "Сервис проверки временно недоступен. Повторите попытку через минуту.",
  "ds service unavailable; retry the request": "Сервис проверки временно недоступен. Повторите попытку через минуту.",
  "ds returned invalid json": "Сервис проверки ответил с ошибкой. Повторите попытку.",
  "ds returned an unexpected error format": "Сервис проверки ответил с ошибкой. Повторите попытку.",
  "ds response too large": "Сервис проверки ответил с ошибкой. Повторите попытку.",
  "invalid request body": "Некорректный запрос. Обновите страницу и повторите.",
  "invalid request boty": "Некорректный запрос. Обновите страницу и повторите.",
  "only one json object is allowed": "Некорректный запрос. Обновите страницу и повторите.",
  "only one json value is allowed": "Некорректный запрос. Обновите страницу и повторите.",
  "internal server error": "Внутренняя ошибка сервиса. Повторите попытку позже.",
  "404 page not found": "Сервис не нашёл этот адрес.",
};

const BY_STATUS: Record<number, string> = {
  400: "Проверьте введённые данные.",
  401: "Нужно войти в аккаунт.",
  403: "Недостаточно прав для этого действия.",
  404: "Не найдено.",
  409: "Данные изменились. Обновите страницу.",
  422: "Данные не прошли проверку.",
  502: "Сервис проверки ответил с ошибкой. Повторите попытку.",
  503: "Сервис временно недоступен. Повторите попытку через минуту.",
};

/** Текст ответа с ошибкой → сообщение для пользователя. */
export function readableError(status: number, body: string): string {
  let text = body.trim();
  // Ошибки DS приходят JSON-объектом: берём из него message
  if (text.startsWith("{")) {
    try {
      const parsed = JSON.parse(text) as { error?: { message?: string } | string; message?: string; detail?: unknown };
      const nested = typeof parsed.error === "object" ? parsed.error?.message : parsed.error;
      text = String(nested ?? parsed.message ?? text);
    } catch {
      // не JSON — оставляем как есть
    }
  }
  const known = MESSAGES[text.toLowerCase()];
  if (known) return known;
  // Русский текст (например, от DS) показываем как есть
  if (/[а-яё]/i.test(text)) return text;
  return BY_STATUS[status] ?? (status >= 500 ? "Внутренняя ошибка сервиса. Повторите попытку позже." : "Не удалось выполнить действие.");
}
