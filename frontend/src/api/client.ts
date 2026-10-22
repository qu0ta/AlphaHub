/**
 * Единая точка доступа к данным для страниц.
 *
 * Страницы вызывают только функции отсюда и не знают, откуда пришли данные:
 * из настоящего бэкенда (VITE_API_MODE=api) или из мока в браузере.
 * Когда бэкенд реализует очередной метод, меняется только этот файл.
 *
 * Сейчас с бэкендом связаны: авторизация (register, login, me) и продукты.
 * Компания, заявки, документы, предложения и кабинет банка — пока мок
 * (у бэкенда есть только часть методов заявок, см. docs/frontend/applications_API.md).
 */
import type { Application, Company, CompanyDocument, DeclineReason, Product, RegistryCompany, Role, User } from "../types";
import { API_MODE, ApiError, request } from "./http";
import {
  MOCK_PRODUCTS,
  mockApplications,
  mockAuth,
  mockBank,
  mockCompany,
  userIdFromToken,
  type ApplicationPatch,
  type QueueView,
} from "./mock/server";

export { ApiError, API_MODE };
export type { ApplicationPatch, QueueView };

/**
 * ID пользователя из токена — нужен моку, чтобы хранить данные по пользователю.
 * В режиме api достаём claim `sub` из JWT. Подпись здесь не проверяется:
 * это только ключ для мок-данных, доступом управляет бэкенд.
 */
function currentUserId(token: string | null): string {
  if (!token) throw new ApiError(401, "Bearer token is required");
  if (token.startsWith("mock.")) return userIdFromToken(token);
  try {
    const payload = JSON.parse(atob(token.split(".")[1].replace(/-/g, "+").replace(/_/g, "/"))) as { sub?: string };
    if (payload.sub) return payload.sub;
  } catch {
    // токен не JWT — упадём ниже
  }
  throw new ApiError(401, "Invalid user ID");
}

/* ---------- Авторизация ---------- */

interface LoginResponse {
  access_token: string;
  token_type: "Bearer";
}

/** Пользователь в формате Go-бэкенда: ФИО приходит в поле fullName. */
interface ApiUser {
  id: string;
  fullName: string;
  email: string;
  role: Role;
  company_id?: string | null;
}

const toUser = (user: ApiUser): User => ({ id: user.id, name: user.fullName, email: user.email, role: user.role, company_id: user.company_id ?? null });

export const authApi = {
  async login(email: string, password: string): Promise<{ token: string; user: User }> {
    if (API_MODE === "mock") return mockAuth.login(email, password);
    const session = await request<LoginResponse>("/auth/login", {
      method: "POST",
      body: JSON.stringify({ email, password }),
    });
    const user = await request<ApiUser>("/auth/me", { token: session.access_token });
    return { token: session.access_token, user: toUser(user) };
  },

  async register(name: string, email: string, password: string): Promise<User> {
    if (API_MODE === "mock") return mockAuth.register(name, email);
    const user = await request<ApiUser>("/auth/register", {
      method: "POST",
      body: JSON.stringify({ fullName: name, email, password }),
    });
    return toUser(user);
  },

  async me(token: string): Promise<User> {
    if (API_MODE === "mock") return mockAuth.me(token);
    return toUser(await request<ApiUser>("/auth/me", { token }));
  },
};

/* ---------- Продукты ---------- */

interface ApiProduct {
  code: string;
  name: string;
  description: string;
  min_amount_kopecks: number;
  max_amount_kopecks: number;
  min_term_months: number;
  max_term_months: number;
  base_annual_rate_bps: number;
  is_active: boolean;
}

export const productsApi = {
  async list(): Promise<Product[]> {
    if (API_MODE === "mock") return structuredClone(MOCK_PRODUCTS);
    const rows = await request<ApiProduct[]>("/products");
    return rows.map((row) => ({
      code: row.code,
      name: row.name,
      description: row.description,
      minAmount: row.min_amount_kopecks / 100,
      maxAmount: row.max_amount_kopecks / 100,
      minTerm: row.min_term_months,
      maxTerm: row.max_term_months,
      rate: row.base_annual_rate_bps / 100,
      active: row.is_active,
    }));
  },
};

/* ---------- Компания (мок) ---------- */

type FileMeta = { name: string; size: number; type: string };
const fileMeta = (file: File): FileMeta => ({ name: file.name, size: file.size, type: file.type });

export const companyApi = {
  lookup: (inn: string): Promise<RegistryCompany> => mockCompany.lookup(inn),
  get: (token: string | null): Promise<Company | null> => mockCompany.get(currentUserId(token)),
  save: (token: string | null, company: Company): Promise<Company> => mockCompany.save(currentUserId(token), company),
  listDocuments: (token: string | null): Promise<CompanyDocument[]> => mockCompany.listDocuments(currentUserId(token)),
  uploadDocument: (token: string | null, file: File): Promise<CompanyDocument> =>
    mockCompany.uploadDocument(currentUserId(token), fileMeta(file)),
  deleteDocument: (token: string | null, id: string): Promise<void> => mockCompany.deleteDocument(currentUserId(token), id),
};

/* ---------- Заявки клиента (мок) ---------- */

export const applicationsApi = {
  list: (token: string | null): Promise<Application[]> => mockApplications.list(currentUserId(token)),
  get: (token: string | null, id: string): Promise<Application> => mockApplications.get(currentUserId(token), id),
  create: (token: string | null, productCode: string): Promise<Application> => mockApplications.create(currentUserId(token), productCode),
  update: (token: string | null, id: string, patch: ApplicationPatch): Promise<Application> =>
    mockApplications.update(currentUserId(token), id, patch),
  uploadFinancialDocument: (token: string | null, id: string, file: File): Promise<Application> =>
    mockApplications.uploadFinancialDocument(currentUserId(token), id, fileMeta(file)),
  removeFinancialDocument: (token: string | null, id: string): Promise<Application> =>
    mockApplications.removeFinancialDocument(currentUserId(token), id),
  submit: (token: string | null, id: string): Promise<Application> => mockApplications.submit(currentUserId(token), id),
  respondToInfoRequest: (token: string | null, id: string, response: { fileName: string | null; explanation: string }) =>
    mockApplications.respondToInfoRequest(currentUserId(token), id, response),
  acceptOffer: (token: string | null, id: string): Promise<Application> => mockApplications.acceptOffer(currentUserId(token), id),
  declineOffer: (token: string | null, id: string, reason: DeclineReason, comment: string): Promise<Application> =>
    mockApplications.declineOffer(currentUserId(token), id, reason, comment),
};

/* ---------- Кабинет банка (мок) ---------- */

export const bankApi = {
  list: (params: { view: QueueView; inn: string; employeeName: string }) => mockBank.list(params),
  get: (id: string) => mockBank.get(id),
  decide: mockBank.decide,
};
