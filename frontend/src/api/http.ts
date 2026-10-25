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

export async function request<T>(path: string, options: RequestInit & { token?: string } = {}): Promise<T> {
  const { token, ...init } = options;
  const headers = new Headers(init.headers);
  headers.set("Accept", "application/json");
  if (init.body && !(init.body instanceof FormData)) headers.set("Content-Type", "application/json");
  if (token) headers.set("Authorization", `Bearer ${token}`);

  const response = await fetch(`${API_BASE}/api/v1${path}`, { ...init, headers, credentials: "omit" });
  const text = await response.text();
  if (!response.ok) throw new ApiError(response.status, text.trim());
  return (text ? JSON.parse(text) : undefined) as T;
}
