# Пример TypeScript-интеграции

Пример для адаптации в frontend-проекте; это не готовый модуль репозитория.

~~~typescript
export type Role = "CLIENT" | "BANK_EMPLOYEE";
export interface UserResponse {
  id: string;
  name: string;
  email: string;
  role: Role;
  company_id?: string;
}
export interface RegisterRequest {
  name: string;
  email: string;
  password: string;
}
export interface LoginRequest { email: string; password: string }
export interface LoginResponse { access_token: string; token_type: "Bearer" }

export class ApiError extends Error {
  constructor(public status: number, public serverText: string) {
    super(serverText || "HTTP " + status);
    this.name = "ApiError";
  }
}

export function createApi(baseUrl = "") {
  const base = baseUrl.replace(/\/+$/, "");
  async function request<T>(
    path: string,
    options: { method?: string; body?: unknown; token?: string; signal?: AbortSignal } = {},
  ): Promise<T> {
    const headers = new Headers({ Accept: "application/json" });
    if (options.body !== undefined) headers.set("Content-Type", "application/json");
    if (options.token) headers.set("Authorization", "Bearer " + options.token);
    const response = await fetch(base + "/api/v1" + path, {
      method: options.method ?? "GET",
      headers,
      body: options.body === undefined ? undefined : JSON.stringify(options.body),
      signal: options.signal,
      credentials: "omit",
    });
    const text = await response.text();
    if (!response.ok) throw new ApiError(response.status, text.trim());
    return JSON.parse(text) as T;
  }
  return {
    register: (body: RegisterRequest) =>
      request<UserResponse>("/auth/register", { method: "POST", body }),
    login: (body: LoginRequest) =>
      request<LoginResponse>("/auth/login", { method: "POST", body }),
    me: (token: string, signal?: AbortSignal) =>
      request<UserResponse>("/auth/me", { token, signal }),
  };
}
~~~

Ошибки — текст, поэтому сначала response.text(), затем проверка статуса.
as T не проверяет схему в runtime; при необходимости добавьте runtime-валидацию.
Сетевые ошибки и AbortError обрабатывайте отдельно от ApiError.
POST не повторяйте автоматически: операция могла выполниться до разрыва связи.

## Использование

~~~typescript
const api = createApi();
let token: string | null = null;
let user: UserResponse | null = null;

async function signIn(email: string, password: string) {
  const result = await api.login({ email, password });
  const profile = await api.me(result.access_token);
  token = result.access_token;
  user = profile;
  return profile;
}
function signOut() {
  token = null;
  user = null;
}
function passwordIsValid(password: string) {
  const bytes = new TextEncoder().encode(password).length;
  return bytes >= 8 && bytes <= 72;
}
~~~

Перенесите состояние в auth store/context, добавьте очистку приватного query-кэша и отмену
запросов при выходе. Пример не решает гонки сессий и не сохраняет токен после перезагрузки.

## Vite proxy и CORS

Backend сейчас не настроен для CORS.
localhost:5173 и localhost:8080 — разные origin. JSON и Authorization могут вызвать preflight,
который сервер не обслуживает как CORS.

Добавьте server.proxy в существующий vite.config.ts, сохранив плагины:

~~~typescript
import { defineConfig } from "vite";
export default defineConfig({
  server: {
    proxy: {
      "/api": {
        target: "http://localhost:8080",
        changeOrigin: true,
      },
    },
  },
});
~~~

Используйте createApi("") и относительный /api/v1; rewrite не требуется.
Перезапустите Vite.
Для production нужен reverse proxy с /api на backend того же origin либо CORS на сервере.
Vite proxy работает только в dev и не входит в статическую сборку.
mode: "no-cors" не позволяет читать ответ и не решает задачу.

Если вводите VITE_API_BASE_URL, это соглашение фронта, сейчас оно не задано репозиторием.
Для данного createApi передавайте только origin без /api/v1; пустое значение — текущий origin.
DATABASE_URL и JWT_SECRET не должны попадать в публичные переменные сборки.
