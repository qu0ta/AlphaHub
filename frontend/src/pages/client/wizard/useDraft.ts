import { useCallback, useEffect, useRef, useState } from "react";
import { applicationsApi, type ApplicationPatch } from "../../../api/client";
import type { Application } from "../../../types";

export type SaveState = "idle" | "saving" | "saved" | "error";

/**
 * Черновик заявки с автосохранением.
 *
 * Как работает: каждое изменение сразу попадает в локальное состояние
 * (поле на экране обновляется мгновенно), а на «сервер» уходит пачкой
 * через 600 мс тишины — так мы не отправляем запрос на каждую нажатую клавишу.
 * Перед переходом между шагами вызываем flush(), чтобы ничего не потерять.
 */
export function useDraft(token: string | null, initial: Application | null) {
  const [app, setApp] = useState<Application | null>(initial);
  const [saveState, setSaveState] = useState<SaveState>("idle");
  const pending = useRef<ApplicationPatch>({});
  const timer = useRef<number | undefined>(undefined);
  const appId = initial?.id;

  useEffect(() => {
    setApp(initial);
  }, [initial]);

  const flush = useCallback(async () => {
    window.clearTimeout(timer.current);
    const patch = pending.current;
    if (!appId || Object.keys(patch).length === 0) return;
    pending.current = {};
    setSaveState("saving");
    try {
      await applicationsApi.update(token, appId, patch);
      setSaveState("saved");
    } catch {
      // Возвращаем несохранённое в очередь — попробуем при следующем изменении
      pending.current = { ...patch, ...pending.current };
      setSaveState("error");
    }
  }, [appId, token]);

  const change = useCallback(
    (patch: ApplicationPatch) => {
      // Любое изменение снимает подтверждение данных (словарь данных: data_confirmed)
      const withReset = "dataConfirmed" in patch ? patch : { ...patch, dataConfirmed: false };
      setApp((current) => (current ? { ...current, ...withReset } : current));
      pending.current = { ...pending.current, ...withReset };
      window.clearTimeout(timer.current);
      timer.current = window.setTimeout(() => void flush(), 600);
    },
    [flush],
  );

  // Уходим со страницы — сохраняем то, что не успело уйти
  useEffect(() => () => void flush(), [flush]);

  return { app, setApp, change, flush, saveState };
}
