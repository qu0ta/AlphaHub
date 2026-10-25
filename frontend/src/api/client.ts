/**
 * Единая точка доступа к данным для страниц.
 *
 * Страницы вызывают только функции отсюда и не знают, откуда пришли данные:
 * из настоящего бэкенда (VITE_API_MODE=api) или из мока в браузере.
 * Когда бэкенд реализует очередной метод, меняется только этот файл.
 *
 * С backend связаны: авторизация, продукты, компания, ядро заявок,
 * загрузка финансового PDF и ручные финансовые данные.
 * Распознавание PDF, предложения и кабинет банка остаются demo-моками.
 */
import type { Application, Company, CompanyDocument, DeclineReason, FinanceMonth, Product, RegistryCompany, Role, User } from "../types";
import { PURPOSE_LABELS } from "../lib/labels";
import { fromApiCompany, fromApiFinancials, toApiFinancials, type ApiCompany, type ApiFinancials } from "./contracts";
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

/* ---------- Компания ---------- */

type FileMeta = { name: string; size: number; type: string };
const fileMeta = (file: File): FileMeta => ({ name: file.name, size: file.size, type: file.type });

export const companyApi = {
  async lookup(inn: string, token?: string | null): Promise<RegistryCompany> {
    if (API_MODE === "mock") return mockCompany.lookup(inn);
    try {
      return fromApiCompany(await request<ApiCompany>(`/company/by-inn/${encodeURIComponent(inn)}`, { token: token ?? undefined }));
    } catch (reason) {
      // В режиме API отсутствие компании нельзя подменять demo-записью.
      if (reason instanceof ApiError && reason.status === 404) throw new ApiError(404, "Компания не найдена в базе сервиса");
      throw reason;
    }
  },
  async get(token: string | null): Promise<Company | null> {
    const userId = currentUserId(token);
    if (API_MODE === "mock") return mockCompany.get(userId);
    try {
      const company = fromApiCompany(await request<ApiCompany>("/company/me", { token: token ?? undefined }));
      const local = await mockCompany.get(userId);
      const merged = local ? { ...company, representativeName: local.representativeName, representativePosition: local.representativePosition, representativePositionOther: local.representativePositionOther } : company;
      if (!merged.representativeName) {
        merged.representativeName = (await request<ApiUser>("/auth/me", { token: token ?? undefined })).fullName;
      }
      // Мок хранит только расширенные UI-поля и последующие demo-этапы заявки.
      // Синхронизация нужна, чтобы заявка backend открылась и в новом браузере.
      return mockCompany.save(userId, merged);
    } catch (reason) {
      if (reason instanceof ApiError && reason.status === 404) return null;
      throw reason;
    }
  },
  async save(token: string | null, company: Company): Promise<Company> {
    const userId = currentUserId(token);
    if (API_MODE === "mock") return mockCompany.save(userId, company);
    let saved = company;
    // Пользователь уже привязан к компании? Тогда повторно привязывать не нужно.
    let linked: ApiCompany | null = null;
    try {
      linked = await request<ApiCompany>("/company/me", { token: token ?? undefined });
    } catch (reason) {
      if (!(reason instanceof ApiError && reason.status === 404)) throw reason;
    }
    if (linked && linked.inn !== company.inn) throw new ApiError(409, "Вы уже привязаны к другой компании");
    if (linked) {
      saved = { ...fromApiCompany(linked), representativeName: company.representativeName, representativePosition: company.representativePosition, representativePositionOther: company.representativePositionOther };
    } else {
      // POST /company: бэкенд создаёт компанию или, если ИНН уже есть в базе, привязывает к существующей.
      const row = await request<ApiCompany>("/company", {
        method: "POST",
        token: token ?? undefined,
        body: JSON.stringify({
          inn: company.inn,
          name: company.name,
          business_type: company.businessType === "IP" ? "ИП" : "ООО",
          registration_date: company.registrationDate,
          main_okved: company.okvedCode,
        }),
      });
      saved = { ...fromApiCompany(row), representativeName: company.representativeName, representativePosition: company.representativePosition, representativePositionOther: company.representativePositionOther };
    }
    // Backend пока не хранит должность и не отдаёт её через API.
    return mockCompany.save(userId, saved);
  },
  listDocuments: (token: string | null): Promise<CompanyDocument[]> => mockCompany.listDocuments(currentUserId(token)),
  uploadDocument: (token: string | null, file: File): Promise<CompanyDocument> =>
    mockCompany.uploadDocument(currentUserId(token), fileMeta(file)),
  deleteDocument: (token: string | null, id: string): Promise<void> => mockCompany.deleteDocument(currentUserId(token), id),
};

/* ---------- Заявки клиента ---------- */

/**
 * Заявка в формате Go-бэкенда (docs/frontend/applications_API.md).
 * Бэкенд пока хранит только «ядро» заявки: продукт, сумму, срок, цель, статус и даты.
 * Финансы, документ, скоринг, предложения и история живут в моке под тем же id —
 * поэтому в режиме api заявка собирается из двух частей (mergeApplication).
 */
interface ApiApplication {
  id: string;
  product_code: string;
  company_id: string | null;
  requested_amount_kopecks: number | null;
  requested_term_months: number | null;
  purpose: string;
  status: string;
  data_version: number;
  created_at: string;
  updated_at: string;
  submitted_at: string | null;
}

/** Последняя известная data_version каждой заявки — без неё бэкенд не примет PATCH. */
const versions = new Map<string, number>();

const remember = (row: ApiApplication) => {
  versions.set(row.id, row.data_version);
  return row;
};

/**
 * Склейка: данные из бэкенда главнее для полей, которые он хранит.
 * Статус берём из мока, если заявка в моке уже ушла дальше DRAFT:
 * этапов после SUBMITTED (проверка, скоринг, решение) на бэкенде ещё нет.
 */
function mergeApplication(row: ApiApplication, local: Application): Application {
  const backendStatus = row.status as Application["status"];
  return {
    ...local,
    id: row.id,
    productCode: row.product_code,
    amount: row.requested_amount_kopecks === null ? local.amount : row.requested_amount_kopecks / 100,
    term: row.requested_term_months ?? local.term,
    status: local.status !== "DRAFT" ? local.status : backendStatus,
    createdAt: row.created_at,
    submittedAt: row.submitted_at ?? local.submittedAt,
  };
}

/** Мок-часть заявки; если её нет (заявку создали в другом браузере) — создаём пустую. */
async function localPart(userId: string, row: ApiApplication): Promise<Application> {
  try {
    return await mockApplications.get(userId, row.id);
  } catch {
    return mockApplications.create(userId, row.product_code, row.id);
  }
}

/** Цель для бэкенда — одна строка: название из справочника или текст для «Другое». */
function purposeText(app: Application): string {
  if (!app.purposeCode) return "";
  if (app.purposeCode === "other") return app.purposeComment.trim();
  return PURPOSE_LABELS[app.purposeCode];
}

const CORE_FIELDS: (keyof ApplicationPatch)[] = ["amount", "term", "purposeCode", "purposeComment"];

async function patchBackend(token: string, app: Application): Promise<ApiApplication> {
  // Бэкенд не принимает null («оставить как есть»), поэтому отправляем только заполненное
  const body: Record<string, unknown> = { purpose: purposeText(app) };
  if (app.amount) body.requested_amount_kopecks = Math.round(app.amount * 100);
  if (app.term) body.requested_term_months = app.term;
  if (app.company?.id) body.company_id = app.company.id;

  const send = async () =>
    remember(
      await request<ApiApplication>(`/applications/${encodeURIComponent(app.id)}`, {
        method: "PATCH",
        token,
        body: JSON.stringify({ ...body, data_version: versions.get(app.id) ?? 1 }),
      }),
    );

  try {
    return await send();
  } catch (reason) {
    // 409 из-за версии: перечитываем заявку и повторяем один раз.
    // Перезапись безопасна: черновик редактирует только его владелец, и отправляем мы полное состояние мастера.
    if (!(reason instanceof ApiError) || reason.status !== 409 || /not a draft/i.test(reason.message)) throw reason;
    remember(await request<ApiApplication>(`/applications/${encodeURIComponent(app.id)}`, { token }));
    return send();
  }
}

const apiApplications = {
  async list(token: string | null): Promise<Application[]> {
    const userId = currentUserId(token);
    const company = await companyApi.get(token);
    const rows = await request<ApiApplication[]>("/applications", { token: token ?? undefined });
    return Promise.all(rows.map(async (row) => {
      const local = await localPart(userId, row);
      if (row.company_id && company?.id === row.company_id) local.company = company;
      return mergeApplication(remember(row), local);
    }));
  },

  async get(token: string | null, id: string): Promise<Application> {
    const userId = currentUserId(token);
    const company = await companyApi.get(token);
    const row = remember(await request<ApiApplication>(`/applications/${encodeURIComponent(id)}`, { token: token ?? undefined }));
    const local = await localPart(userId, row);
    if (row.company_id && company?.id === row.company_id) local.company = company;
    let merged = mergeApplication(row, local);
    try {
      const financial = fromApiFinancials(await request<ApiFinancials>(`/applications/${encodeURIComponent(id)}/financials`, { token: token ?? undefined }));
      versions.set(id, financial.version);
      merged = { ...merged, debtPayment: financial.debtPayment, financials: financial.financials, dataConfirmed: financial.confirmed };
    } catch (reason) {
      if (!(reason instanceof ApiError) || reason.status !== 404) throw reason;
    }
    return merged;
  },

  async create(token: string | null, productCode: string): Promise<Application> {
    const userId = currentUserId(token);
    const row = remember(
      await request<ApiApplication>("/applications", {
        method: "POST",
        token: token ?? undefined,
        body: JSON.stringify({ product_code: productCode }),
      }),
    );
    const local = await mockApplications.create(userId, productCode, row.id);
    const linked = local.company?.id ? await patchBackend(token as string, local) : row;
    return mergeApplication(linked, local);
  },

  async update(token: string | null, id: string, patch: ApplicationPatch): Promise<Application> {
    const userId = currentUserId(token);
    const local = await mockApplications.update(userId, id, patch);
    if (!CORE_FIELDS.some((field) => field in patch)) return local;
    const row = await patchBackend(token as string, local);
    return mergeApplication(row, local);
  },

  async submit(token: string | null, id: string): Promise<Application> {
    const userId = currentUserId(token);
    const row = remember(await request<ApiApplication>(`/applications/${encodeURIComponent(id)}/submit`, { method: "POST", token: token ?? undefined }));
    const local = await mockApplications.submit(userId, id);
    return mergeApplication(row, local);
  },

  async uploadFinancialDocument(token: string | null, id: string, file: File): Promise<Application> {
    const body = new FormData();
    body.append("file", file);
    await request(`/applications/${encodeURIComponent(id)}/documents`, { method: "POST", token: token ?? undefined, body });
    // Извлечения на backend пока нет. Мок создаёт редактируемые demo-значения после реальной загрузки PDF.
    return mockApplications.uploadFinancialDocument(currentUserId(token), id, fileMeta(file));
  },

  async saveFinancials(token: string | null, id: string, debtPayment: number, financials: FinanceMonth[]): Promise<Application> {
    const row = await request<ApiFinancials>(`/applications/${encodeURIComponent(id)}/financials`, {
      method: "PUT",
      token: token ?? undefined,
      body: JSON.stringify(toApiFinancials(versions.get(id) ?? 1, debtPayment, financials)),
    });
    const mapped = fromApiFinancials(row);
    versions.set(id, mapped.version);
    const local = await mockApplications.update(currentUserId(token), id, { debtPayment: mapped.debtPayment, financials: mapped.financials, dataConfirmed: false });
    return { ...local, debtPayment: mapped.debtPayment, financials: mapped.financials, dataConfirmed: false };
  },

  async confirmFinancials(token: string | null, id: string): Promise<Application> {
    // Бэкенд подтверждает финансы только для текущей версии заявки: тело { data_version }.
    // Берём версию свежим GET, а не из кэша: после PUT/загрузки PDF она могла вырасти.
    const current = await request<ApiFinancials>(`/applications/${encodeURIComponent(id)}/financials`, { token: token ?? undefined });
    const row = fromApiFinancials(
      await request<ApiFinancials>(`/applications/${encodeURIComponent(id)}/financials/confirm`, {
        method: "POST",
        token: token ?? undefined,
        body: JSON.stringify({ data_version: current.data_version }),
      }),
    );
    versions.set(id, row.version);
    return mockApplications.update(currentUserId(token), id, { debtPayment: row.debtPayment, financials: row.financials, dataConfirmed: row.confirmed });
  },
};

export const applicationsApi = {
  list: (token: string | null): Promise<Application[]> =>
    API_MODE === "api" ? apiApplications.list(token) : mockApplications.list(currentUserId(token)),
  get: (token: string | null, id: string): Promise<Application> =>
    API_MODE === "api" ? apiApplications.get(token, id) : mockApplications.get(currentUserId(token), id),
  create: (token: string | null, productCode: string): Promise<Application> =>
    API_MODE === "api" ? apiApplications.create(token, productCode) : mockApplications.create(currentUserId(token), productCode),
  update: (token: string | null, id: string, patch: ApplicationPatch): Promise<Application> =>
    API_MODE === "api" ? apiApplications.update(token, id, patch) : mockApplications.update(currentUserId(token), id, patch),
  uploadFinancialDocument: (token: string | null, id: string, file: File): Promise<Application> =>
    API_MODE === "api" ? apiApplications.uploadFinancialDocument(token, id, file) : mockApplications.uploadFinancialDocument(currentUserId(token), id, fileMeta(file)),
  removeFinancialDocument: (token: string | null, id: string): Promise<Application> =>
    mockApplications.removeFinancialDocument(currentUserId(token), id),
  submit: (token: string | null, id: string): Promise<Application> =>
    API_MODE === "api" ? apiApplications.submit(token, id) : mockApplications.submit(currentUserId(token), id),
  saveFinancials: (token: string | null, id: string, debtPayment: number, financials: FinanceMonth[]): Promise<Application> =>
    API_MODE === "api" ? apiApplications.saveFinancials(token, id, debtPayment, financials) : mockApplications.update(currentUserId(token), id, { debtPayment, financials }),
  confirmFinancials: (token: string | null, id: string): Promise<Application> =>
    API_MODE === "api" ? apiApplications.confirmFinancials(token, id) : mockApplications.update(currentUserId(token), id, { dataConfirmed: true }),
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
