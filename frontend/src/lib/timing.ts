/**
 * Паузы для интерфейса.
 *
 * Бэкенд отвечает почти мгновенно: скоринг, решение и разбор PDF выполняются
 * за один запрос. Без минимального времени индикатор загрузки мигает на долю
 * секунды и пользователь не понимает, что произошло. Эти функции НЕ замедляют
 * сам запрос — они лишь не дают индикатору исчезнуть раньше заданного времени.
 */

export const wait = (ms: number) => new Promise<void>((resolve) => window.setTimeout(resolve, ms));

/**
 * Результат promise, но не раньше чем через ms миллисекунд — и для успеха, и для ошибки.
 * Promise.allSettled ждёт оба: и запрос, и паузу, даже если запрос упал;
 * затем отдаём результат или пробрасываем ошибку.
 */
export async function atLeast<T>(promise: Promise<T>, ms: number): Promise<T> {
  const [result] = await Promise.allSettled([promise, wait(ms)]);
  if (result.status === "rejected") throw result.reason;
  return result.value;
}
