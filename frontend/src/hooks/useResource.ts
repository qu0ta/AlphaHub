import { useCallback, useEffect, useRef, useState, type DependencyList } from "react";

/**
 * Загрузка данных для страницы: loading → data или error.
 *
 * Зачем свой хук: на каждой странице повторяется одно и то же —
 * состояние загрузки, ошибка, кнопка «Повторить». Хук убирает дублирование.
 *
 * Защита от гонок: если зависимости поменялись, пока шёл старый запрос,
 * его результат игнорируется (сравниваем номер запроса).
 */
export function useResource<T>(loader: () => Promise<T>, deps: DependencyList) {
  const [data, setData] = useState<T | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<unknown>(null);
  const requestId = useRef(0);

  // loader пересоздаётся при каждом рендере, поэтому запоминаем его по deps —
  // иначе эффект ниже перезапускался бы бесконечно
  const load = useCallback(loader, deps);

  const reload = useCallback(
    async (options: { silent?: boolean } = {}) => {
      const id = ++requestId.current;
      if (!options.silent) setLoading(true);
      try {
        const result = await load();
        if (id !== requestId.current) return;
        setData(result);
        setError(null);
      } catch (reason) {
        if (id !== requestId.current) return;
        setError(reason);
      } finally {
        if (id === requestId.current) setLoading(false);
      }
    },
    [load],
  );

  useEffect(() => {
    void reload();
    // При уходе со страницы «забываем» незавершённый запрос
    return () => {
      requestId.current += 1;
    };
  }, [reload]);

  return { data, setData, loading, error, reload };
}

/**
 * Периодически вызывает callback, пока active = true.
 * Нужен, чтобы статус заявки обновлялся сам, пока банк её рассматривает.
 */
export function usePolling(callback: () => void, active: boolean, intervalMs = 2000) {
  const saved = useRef(callback);
  useEffect(() => {
    saved.current = callback;
  }, [callback]);

  useEffect(() => {
    if (!active) return;
    const timer = window.setInterval(() => saved.current(), intervalMs);
    return () => window.clearInterval(timer);
  }, [active, intervalMs]);
}
