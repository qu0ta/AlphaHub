/**
 * Единая точка доступа к данным для страниц.
 *
 * Страницы вызывают только функции отсюда и не знают, откуда пришли данные:
 * из настоящего бэкенда (VITE_API_MODE=api) или из мока в браузере.
 * Когда бэкенд реализует очередной метод, меняется только этот файл.
 *
 * С backend связаны: авторизация, продукты, компания, заявки (черновик, PDF и
 * извлечённые финансы, подтверждение, отправка и скоринг), предложения, запросы
 * дополнительных данных и кабинет банка. Перевод форматов бэкенда — в backend.ts.
 * В моке остаются только поля интерфейса, которых нет на бэкенде (должность
 * представителя, документы компании), и симуляция подписания и выдачи.
 */
import type { Application, BankDecision, Company, CompanyDocument, DeclineReason, FinanceMonth, Product, RegistryCompany, Role, User } from "../types";
import { PURPOSE_LABELS } from "../lib/labels";
import { fromApiCompany, fromApiFinancials, toApiFinancials, type ApiCompany, type ApiFinancials } from "./contracts";
import { API_MODE, ApiError, request } from "./http";
import {
  bps,
  clientDecision,
  clientHistory,
  fromBankDetail,
  fromDecisionRow,
  fromQueueRow,
  kopecks,
  mapStatus,
  numberFromId,
  toInfoRequest,
  toOffers,
  type BackendApplication,
  type BackendInfoRequest,
  type BackendOffer,
  type BankDecisionRow,
  type BankDetail,
  type BankQueueRow,
  type Page as BackendPage,
} from "./backend";
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
  annual_rate_bps?: number;
  requires_collateral?: boolean;
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
      requiresCollateral: row.requires_collateral ?? false,
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
type ApiApplication = BackendApplication;

/** Последняя известная data_version каждой заявки — без неё бэкенд не примет PATCH. */
const versions = new Map<string, number>();

const remember = (row: ApiApplication) => {
  versions.set(row.id, row.data_version);
  return row;
};

/**
 * Склейка: данные из бэкенда главнее для полей, которые он хранит
 * (сумма, срок, статус, даты). В моке остаются только поля интерфейса,
 * которых у бэкенда нет: должность представителя, исправленные ячейки и т. п.
 * Статус всегда берётся с бэкенда: проверка, скоринг и решение идут там.
 */
function mergeApplication(row: ApiApplication, local: Application): Application {
  return {
    ...local,
    id: row.id,
    number: numberFromId(row.id),
    productCode: row.product_code,
    amount: row.requested_amount_kopecks === null ? local.amount : row.requested_amount_kopecks / 100,
    term: row.requested_term_months ?? local.term,
    status: mapStatus(row.status, row.updated_at),
    createdAt: row.created_at,
    updatedAt: row.updated_at,
    submittedAt: row.submitted_at ?? local.submittedAt,
  };
}

/** Заявка после отправки: предложения, запросы данных и ход заявки — с бэкенда. */
async function withLifecycle(token: string, row: ApiApplication, app: Application): Promise<Application> {
  if (row.status === "DRAFT") return app;
  const base = `/applications/${encodeURIComponent(row.id)}`;
  const [offers, requests] = await Promise.all([
    request<BackendOffer[]>(`${base}/offers`, { token }),
    request<BackendInfoRequest[]>(`${base}/info-requests`, { token }),
  ]);
  const status = app.status;
  return {
    ...app,
    offers: toOffers(offers),
    // Открытый запрос показываем только пока банк ждёт ответа
    infoRequest: status === "NEED_DATA" ? toInfoRequest(requests.filter((item) => item.status === "OPEN")) : toInfoRequest(requests),
    decision: clientDecision(offers),
    history: clientHistory(row, offers, requests),
    finalReason: null,
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
    return withLifecycle(token as string, row, merged);
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
    // Скоринг и решение (авто-отказ / ручная проверка / авто-предложение) выполняет бэкенд
    const row = remember(await request<ApiApplication>(`/applications/${encodeURIComponent(id)}/submit`, { method: "POST", token: token ?? undefined }));
    const local = await localPart(userId, row);
    return withLifecycle(token as string, row, mergeApplication(row, local));
  },

  /** Ответ на предложение: ACCEPT или DECLINE для действующего предложения. */
  async respondToOffer(token: string | null, id: string, action: "ACCEPT" | "DECLINE"): Promise<Application> {
    const base = `/applications/${encodeURIComponent(id)}`;
    const [row, offers] = await Promise.all([
      request<ApiApplication>(base, { token: token ?? undefined }),
      request<BackendOffer[]>(`${base}/offers`, { token: token ?? undefined }),
    ]);
    const pending = offers.find((offer) => offer.status === "PENDING");
    if (!pending) throw new ApiError(409, "Предложение больше не действует");
    await request(`${base}/offers/${encodeURIComponent(pending.id)}/respond`, {
      method: "POST",
      token: token ?? undefined,
      body: JSON.stringify({ data_version: row.data_version, action }),
    });
    return this.get(token, id);
  },

  /** Ответ на запрос банка: необязательный PDF (SUPPORTING_DOCUMENT) + пояснение. */
  async respondToInfoRequest(token: string | null, id: string, response: { file?: File | null; explanation: string }): Promise<Application> {
    const base = `/applications/${encodeURIComponent(id)}`;
    if (response.file) {
      const body = new FormData();
      body.append("file", response.file);
      body.append("document_type", "SUPPORTING_DOCUMENT");
      await request(`${base}/documents`, { method: "POST", token: token ?? undefined, body });
    }
    const [row, requests] = await Promise.all([
      request<ApiApplication>(base, { token: token ?? undefined }),
      request<BackendInfoRequest[]>(`${base}/info-requests`, { token: token ?? undefined }),
    ]);
    const open = requests.find((item) => item.status === "OPEN");
    if (!open) throw new ApiError(409, "Открытого запроса банка нет");
    await request(`${base}/info-requests/${encodeURIComponent(open.id)}/respond`, {
      method: "POST",
      token: token ?? undefined,
      body: JSON.stringify({ data_version: row.data_version, response_comment: response.explanation }),
    });
    return this.get(token, id);
  },

  async uploadFinancialDocument(token: string | null, id: string, file: File): Promise<Application> {
    const body = new FormData();
    body.append("file", file);
    await request(`/applications/${encodeURIComponent(id)}/documents`, { method: "POST", token: token ?? undefined, body });
    // Показатели из PDF извлекает DS на бэкенде — берём их оттуда, а не демо-значения мока,
    // иначе при сохранении (PUT financials) настоящие цифры перезаписались бы демо-данными.
    const local = await mockApplications.uploadFinancialDocument(currentUserId(token), id, fileMeta(file));
    const extracted = fromApiFinancials(await request<ApiFinancials>(`/applications/${encodeURIComponent(id)}/financials`, { token: token ?? undefined }));
    versions.set(id, extracted.version);
    const patch = { financials: extracted.financials, debtPayment: extracted.debtPayment ?? local.debtPayment, editedCells: [], dataConfirmed: false };
    try {
      return { ...(await mockApplications.update(currentUserId(token), id, patch)), document: local.document ? { ...local.document, status: "EXTRACTED" } : local.document };
    } catch {
      return { ...local, ...patch };
    }
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
  respondToInfoRequest: (token: string | null, id: string, response: { file: File | null; explanation: string }): Promise<Application> =>
    API_MODE === "api"
      ? apiApplications.respondToInfoRequest(token, id, response)
      : mockApplications.respondToInfoRequest(currentUserId(token), id, { fileName: response.file?.name ?? null, explanation: response.explanation }),
  acceptOffer: (token: string | null, id: string): Promise<Application> =>
    API_MODE === "api" ? apiApplications.respondToOffer(token, id, "ACCEPT") : mockApplications.acceptOffer(currentUserId(token), id),
  /** На бэкенде отказ от предложения окончательный: повторного рассмотрения (экран 18) там нет. */
  declineOffer: (token: string | null, id: string, reason: DeclineReason, comment: string): Promise<Application> =>
    API_MODE === "api" ? apiApplications.respondToOffer(token, id, "DECLINE") : mockApplications.declineOffer(currentUserId(token), id, reason, comment),
};

/* ---------- Кабинет банка ---------- */

type BankDecisionInput = Omit<BankDecision, "at">;
type OfferConditions = { amount: number; term: number; rate: number };

/** Последняя загруженная карточка: из неё берём data_version и запрошенные условия для решения. */
const bankDetails = new Map<string, BankDetail>();

/** Сотрудник из токена: id нужен, чтобы отличать свои решения от чужих. */
function bankMe(token: string | null, name: string) {
  return { id: currentUserId(token), name };
}

/** Строки запроса данных: 1..20 разных непустых пунктов (так требует бэкенд). */
function requiredItems(comment: string) {
  const items = [...new Set(comment.split(/\n|;/).map((line) => line.trim().slice(0, 500)).filter(Boolean))].slice(0, 20);
  return items.length ? items : [comment.trim().slice(0, 500)];
}

const apiBank = {
  async list(token: string | null, params: { view: QueueView; inn: string; employeeName: string }): Promise<Application[]> {
    const query = new URLSearchParams({ limit: "100" });
    if (params.inn) query.set("q", params.inn);
    if (params.view === "mine") {
      const page = await request<BackendPage<BankDecisionRow>>(`/bank/decisions?${query}`, { token: token ?? undefined });
      return page.items.map((row) => fromDecisionRow(row, bankMe(token, params.employeeName)));
    }
    const path = params.view === "archive" ? "/bank/archive" : "/bank/applications/page";
    const page = await request<BackendPage<BankQueueRow>>(`${path}?${query}`, { token: token ?? undefined });
    return page.items.map(fromQueueRow);
  },

  async get(token: string | null, id: string, employeeName = ""): Promise<Application> {
    const detail = await request<BankDetail>(`/bank/applications/${encodeURIComponent(id)}`, { token: token ?? undefined });
    bankDetails.set(id, detail);
    return fromBankDetail(detail, bankMe(token, employeeName));
  },

  /**
   * Решение сотрудника. Порядок на бэкенде: сначала взять заявку в работу (claim),
   * потом решение. «Одобрить» без изменений → APPROVE (условия заявки и ставка продукта),
   * с изменёнными суммой/сроком/ставкой → COUNTER_OFFER. Запрос данных — отдельный маршрут.
   */
  async decide(token: string | null, id: string, decision: BankDecisionInput, conditions?: OfferConditions): Promise<Application> {
    const auth = { token: token ?? undefined };
    const detail = bankDetails.get(id) ?? (await request<BankDetail>(`/bank/applications/${encodeURIComponent(id)}`, auth));
    const base = `/bank/applications/${encodeURIComponent(id)}`;
    const dataVersion = detail.application.data_version;
    const comment = decision.comment.trim();

    await request(`${base}/claim`, { method: "POST", ...auth, body: JSON.stringify({ data_version: dataVersion }) });

    if (decision.type === "REQUEST_DATA") {
      await request(`${base}/info-requests`, {
        method: "POST",
        ...auth,
        body: JSON.stringify({ data_version: dataVersion, description: comment, required_items: requiredItems(comment) }),
      });
    } else if (decision.type === "REJECT") {
      await request(`${base}/decisions`, {
        method: "POST",
        ...auth,
        body: JSON.stringify({ data_version: dataVersion, action: "REJECT", reason_code: "MANUAL_REJECTION", comment }),
      });
    } else {
      const validUntil = new Date(Date.now() + 7 * 24 * 60 * 60 * 1000).toISOString();
      const requested = detail.application;
      const unchanged =
        !conditions ||
        (kopecks(conditions.amount) === requested.requested_amount_kopecks &&
          conditions.term === requested.requested_term_months &&
          bps(conditions.rate) === detail.product.base_annual_rate_bps);
      const body = unchanged
        ? { data_version: dataVersion, action: "APPROVE", reason_code: "MANUAL_APPROVAL", comment, valid_until: validUntil }
        : {
            data_version: dataVersion,
            action: "COUNTER_OFFER",
            reason_code: "ADJUSTED_TERMS",
            comment,
            valid_until: validUntil,
            offer: { amount_kopecks: kopecks(conditions.amount), term_months: conditions.term, annual_rate_bps: bps(conditions.rate) },
          };
      await request(`${base}/decisions`, { method: "POST", ...auth, body: JSON.stringify(body) });
    }
    return this.get(token, id, decision.employeeName);
  },
};

export const bankApi = {
  list: (token: string | null, params: { view: QueueView; inn: string; employeeName: string }): Promise<Application[]> =>
    API_MODE === "api" ? apiBank.list(token, params) : mockBank.list(params),
  get: (token: string | null, id: string, employeeName?: string): Promise<Application> =>
    API_MODE === "api" ? apiBank.get(token, id, employeeName) : mockBank.get(id),
  decide: (token: string | null, id: string, decision: BankDecisionInput, conditions?: OfferConditions): Promise<Application> =>
    API_MODE === "api" ? apiBank.decide(token, id, decision, conditions) : mockBank.decide(id, decision, conditions),
};
