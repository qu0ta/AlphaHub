/**
 * Хранилище демо-данных в localStorage.
 * Это «маленький бэкенд в браузере»: все экраны работают с ним через
 * функции из api/mock/server.ts, а не напрямую.
 */
import { createSeed, DB_VERSION, type DemoDb } from "./seed";

const STORAGE_KEY = "alphahub_demo_db";

let cache: DemoDb | null = null;

export function readDb(): DemoDb {
  if (cache) return cache;
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw) {
      const parsed = JSON.parse(raw) as DemoDb;
      // Если структура данных поменялась — начинаем с чистого сида
      if (parsed.version === DB_VERSION) {
        cache = parsed;
        return parsed;
      }
    }
  } catch {
    // localStorage недоступен или испорчен — работаем с сидом в памяти
  }
  cache = createSeed();
  writeDb(cache);
  return cache;
}

export function writeDb(db: DemoDb) {
  cache = db;
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(db));
  } catch {
    // Нет места или режим инкогнито — данные останутся только в памяти
  }
}

/** Изменить данные и сразу сохранить. */
export function updateDb<T>(change: (db: DemoDb) => T): T {
  const db = readDb();
  const result = change(db);
  writeDb(db);
  return result;
}

export function resetDb() {
  cache = null;
  try {
    localStorage.removeItem(STORAGE_KEY);
  } catch {
    // ignore
  }
}

/*
  Другая вкладка (например, кабинет банка) поменяла данные —
  сбрасываем кэш, чтобы прочитать свежие.
*/
if (typeof window !== "undefined") {
  window.addEventListener("storage", (event) => {
    if (event.key === STORAGE_KEY) cache = null;
  });
}

export const delay = (ms = 300) => new Promise((resolve) => setTimeout(resolve, ms));

export const uid = (prefix: string) => `${prefix}-${Math.random().toString(36).slice(2, 10)}`;

/** Глубокая копия, чтобы страницы не могли случайно изменить «базу». */
export const clone = <T,>(value: T): T => structuredClone(value);
