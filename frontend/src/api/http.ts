/**
 * Общий HTTP-клиент для настоящего бэкенда на Go.
 * Бэкенд отдаёт ошибки обычным текстом (http.Error), поэтому читаем их через text().
 */

export class ApiError extends Error {
  constructor(
    public status: number,
    public serverText: string,
  ) {
    super(serverText || `HTTP ${status}`);
    this.name = "ApiError";
  }
}

const API_BASE = (import.meta.env.VITE_API_BASE_URL ?? "").replace(/\/+$/, "");

import { readableError } from "./errors";

export async function request<T>(path: string, options: RequestInit & { token?: string } = {}): Promise<T> {
  const { token, ...init } = options;
  const headers = new Headers(init.headers);
  headers.set("Accept", "application/json");
  if (init.body && !(init.body instanceof FormData)) headers.set("Content-Type", "application/json");
  if (token) headers.set("Authorization", `Bearer ${token}`);

  const response = await fetch(`${API_BASE}/api/v1${path}`, { ...init, headers, credentials: "omit" });
  const text = await response.text();
  // Пользователю — русское сообщение; исходный текст бэкенда остаётся в message для отладки
  if (!response.ok) throw Object.assign(new ApiError(response.status, readableError(response.status, text)), { rawText: text.trim() });
  return (text ? JSON.parse(text) : undefined) as T;
}

/** Файл с авторизацией (например, PDF): обычная ссылка не подойдёт — ей не передать токен. */
export async function requestBlob(path: string, token?: string): Promise<Blob> {
  const response = await fetch(`${API_BASE}/api/v1${path}`, { headers: token ? { Authorization: `Bearer ${token}` } : {}, credentials: "omit" });
  if (!response.ok) {
    const text = await response.text();
    throw new ApiError(response.status, readableError(response.status, text));
  }
  return response.blob();
}
