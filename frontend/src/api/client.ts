/**
 * Единая точка доступа к данным для страниц.
 *
 * Все данные — с Go-бэкенда (/api/v1). Страницы вызывают только функции
 * отсюда и не знают о форматах API: перевод копеек в рубли, статусов,
 * предложений и т. п. собран в backend.ts.
 *
 * В браузере (lib/localStore.ts) хранятся лишь поля интерфейса, которых нет
 * в API: должность представителя и отметки «исправлено вручную» в таблице финансов.
 * Подписание и выдача кредита на бэкенде не реализованы — после принятия
 * предложения их шаги показываются по времени (см. mapStatus в backend.ts).
 */
import { PURPOSE_LABELS } from "../lib/labels";
import { localUi } from "../lib/localStore";
import type {
  Application,
  BankDecision,
  Company,
  DeclineReason,
  FinanceMonth,
  FinancialDocument,
  Product,
  RegistryCompany,
  Role,
  User,
} from "../types";
import {
  blankApplication,
  bps,
  clientDecision,
  clientHistory,
  fromBankDetail,
  fromDecisionRow,
  fromQueueRow,
  kopecks,
  mapStatus,
  parsePurpose,
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
import { fromApiCompany, fromApiFinancials, toApiFinancials, type ApiCompany, type ApiFinancials } from "./contracts";
import { ApiError, request, requestBlob } from "./http";

export { ApiError };

/** Изменения черновика из мастера заявки. */
export type ApplicationPatch = Partial<
  Pick<Application, "amount" | "term" | "purposeCode" | "purposeComment" | "debtPayment" | "financials" | "editedCells" | "dataConfirmed">
>;

export type QueueView = "queue" | "mine" | "archive";

/**
 * ID пользователя из JWT (claim `sub`). Подпись здесь не проверяется —
 * это только ключ для локальных полей интерфейса, доступом управляет бэкенд.
 */
function currentUserId(token: string | null): string {
  if (!token) throw new ApiError(401, "Bearer token is required");
  try {
    const payload = JSON.parse(atob(token.split(".")[1].replace(/-/g, "+").replace(/_/g, "/"))) as { sub?: string };
    if (payload.sub) return payload.sub;
  } catch {
    // токен не JWT — упадём ниже
  }
  throw new ApiError(401, "Invalid user ID");
}

const auth = (token: string | null) => ({ token: token ?? undefined });
const isNotFound = (reason: unknown) => reason instanceof ApiError && reason.status === 404;

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
    const session = await request<LoginResponse>("/auth/login", { method: "POST", body: JSON.stringify({ email, password }) });
    const user = await request<ApiUser>("/auth/me", { token: session.access_token });
    return { token: session.access_token, user: toUser(user) };
  },

  async register(name: string, email: string, password: string): Promise<User> {
    const user = await request<ApiUser>("/auth/register", { method: "POST", body: JSON.stringify({ fullName: name, email, password }) });
    return toUser(user);
  },

  async me(token: string): Promise<User> {
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

export const companyApi = {
  /** Поиск по ИНН в базе сервиса (реестр компаний смоделирован на бэкенде). */
  async lookup(inn: string, token?: string | null): Promise<RegistryCompany> {
    try {
      return fromApiCompany(await request<ApiCompany>(`/company/by-inn/${encodeURIComponent(inn)}`, auth(token ?? null)));
    } catch (reason) {
      if (isNotFound(reason)) throw new ApiError(404, "Компания не найдена в базе сервиса");
      throw reason;
    }
  },

  /** Компания текущего пользователя или null, если он ещё не привязан. */
  async get(token: string | null): Promise<Company | null> {
    const userId = currentUserId(token);
    let row: ApiCompany;
    try {
      row = await request<ApiCompany>("/company/me", auth(token));
    } catch (reason) {
      if (isNotFound(reason)) return null;
      throw reason;
    }
    const company = fromApiCompany(row);
    const representative = localUi.getRepresentative(userId);
    return {
      ...company,
      representativeName: representative?.name || (await request<ApiUser>("/auth/me", auth(token))).fullName,
      representativePosition: representative?.position ?? "director",
      representativePositionOther: representative?.positionOther ?? "",
    };
  },

  /**
   * POST /company создаёт компанию или, если ИНН уже есть в базе, привязывает к существующей.
   * Должность представителя API не хранит — запоминаем её в браузере.
   */
  async save(token: string | null, company: Company): Promise<Company> {
    const userId = currentUserId(token);
    let linked: ApiCompany | null = null;
    try {
      linked = await request<ApiCompany>("/company/me", auth(token));
    } catch (reason) {
      if (!isNotFound(reason)) throw reason;
    }
    if (linked && linked.inn !== company.inn) throw new ApiError(409, "Вы уже привязаны к другой компании");
    const row =
      linked ??
      (await request<ApiCompany>("/company", {
        method: "POST",
        ...auth(token),
        body: JSON.stringify({
          inn: company.inn,
          name: company.name,
          business_type: company.businessType === "IP" ? "ИП" : "ООО",
          registration_date: company.registrationDate,
          main_okved: company.okvedCode,
        }),
      }));
    localUi.setRepresentative(userId, {
      name: company.representativeName,
      position: company.representativePosition,
      positionOther: company.representativePositionOther,
    });
    return {
      ...fromApiCompany(row),
      representativeName: company.representativeName,
      representativePosition: company.representativePosition,
      representativePositionOther: company.representativePositionOther,
    };
  },
};

/* ---------- Заявки клиента ---------- */

interface ApiDocument {
  id: string;
  document_type: string;
  original_filename: string;
  processing_status: string;
  uploaded_at: string;
}

const appPath = (id: string) => `/applications/${encodeURIComponent(id)}`;

/** Финансовый PDF заявки (последний загруженный). */
function toDocument(rows: ApiDocument[]): FinancialDocument | null {
  const doc = rows.find((row) => row.document_type === "FINANCIAL_STATEMENT");
  if (!doc) return null;
  const status: FinancialDocument["status"] = /FAIL|ERROR|REJECT/i.test(doc.processing_status) ? "ERROR" : /PROCESS(ED)?$|UPLOADED/i.test(doc.processing_status) ? "EXTRACTED" : "PROCESSING";
  return { id: doc.id, name: doc.original_filename, sizeBytes: 0, uploadedAt: doc.uploaded_at, status };
}

/** Заявка в строке списка: без финансов и документов (они нужны только на экране заявки). */
function fromRow(row: BackendApplication, company: Company | null): Application {
  return {
    ...blankApplication(row.id),
    productCode: row.product_code,
    status: mapStatus(row.status, row.updated_at),
    company: row.company_id && company?.id === row.company_id ? company : null,
    amount: row.requested_amount_kopecks === null ? null : row.requested_amount_kopecks / 100,
    term: row.requested_term_months,
    ...parsePurpose(row.purpose),
    dataConfirmed: false,
    createdAt: row.created_at,
    updatedAt: row.updated_at,
    submittedAt: row.submitted_at,
  };
}

/** Полная заявка: ядро, компания, финансы, PDF и — после отправки — предложения, запросы, ход заявки. */
async function loadApplication(token: string | null, id: string): Promise<Application> {
  const [row, company] = await Promise.all([request<BackendApplication>(appPath(id), auth(token)), companyApi.get(token)]);
  const [financials, documents] = await Promise.all([
    request<ApiFinancials>(`${appPath(id)}/financials`, auth(token)).catch((reason) => (isNotFound(reason) ? null : Promise.reject(reason))),
    request<ApiDocument[]>(`${appPath(id)}/documents`, auth(token)).catch((reason) => (isNotFound(reason) ? [] : Promise.reject(reason))),
  ]);
  const finance = financials ? fromApiFinancials(financials) : null;
  const app: Application = {
    ...fromRow(row, company),
    debtPayment: finance?.debtPayment ?? null,
    financials: finance && finance.financials.length ? finance.financials : null,
    dataConfirmed: finance?.confirmed ?? false,
    editedCells: localUi.getEditedCells(id),
    document: toDocument(documents),
  };
  if (row.status === "DRAFT") return app;

  const [offers, requests] = await Promise.all([
    request<BackendOffer[]>(`${appPath(id)}/offers`, auth(token)),
    request<BackendInfoRequest[]>(`${appPath(id)}/info-requests`, auth(token)),
  ]);
  return {
    ...app,
    offers: toOffers(offers),
    // Пока банк ждёт ответа — показываем открытый запрос
    infoRequest: app.status === "NEED_DATA" ? toInfoRequest(requests.filter((item) => item.status === "OPEN")) : toInfoRequest(requests),
    decision: clientDecision(offers),
    history: clientHistory(row, offers, requests),
  };
}

/** Цель для бэкенда — одна строка: название из справочника или текст для «Другое». */
function purposeText(code: Application["purposeCode"], comment: string): string {
  if (!code) return "";
  return code === "other" ? comment.trim() : PURPOSE_LABELS[code];
}

/** Сохраняет сумму, срок и цель черновика (PATCH с текущей data_version). */
async function patchCore(token: string | null, row: BackendApplication, patch: ApplicationPatch, companyId?: string) {
  const current = parsePurpose(row.purpose);
  const code = "purposeCode" in patch ? (patch.purposeCode ?? null) : current.purposeCode;
  const comment = "purposeComment" in patch ? (patch.purposeComment ?? "") : current.purposeComment;
  // Бэкенд не принимает null («оставить как есть»), поэтому отправляем только заполненное
  const body: Record<string, unknown> = { data_version: row.data_version, purpose: purposeText(code, comment) };
  if (patch.amount) body.requested_amount_kopecks = kopecks(patch.amount);
  if (patch.term) body.requested_term_months = patch.term;
  if (companyId && !row.company_id) body.company_id = companyId;
  return request<BackendApplication>(appPath(row.id), { method: "PATCH", ...auth(token), body: JSON.stringify(body) });
}

/** Сохраняет финансы и платёж по кредитам (новая версия, подтверждение снимается). */
async function putFinancials(token: string | null, id: string, debtPayment: number, financials: FinanceMonth[]) {
  const row = await request<BackendApplication>(appPath(id), auth(token));
  return request<ApiFinancials>(`${appPath(id)}/financials`, {
    method: "PUT",
    ...auth(token),
    body: JSON.stringify(toApiFinancials(row.data_version, debtPayment, financials)),
  });
}

export const applicationsApi = {
  async list(token: string | null): Promise<Application[]> {
    const [rows, company] = await Promise.all([request<BackendApplication[]>("/applications", auth(token)), companyApi.get(token)]);
    return rows.map((row) => fromRow(row, company));
  },

  get: loadApplication,

  async create(token: string | null, productCode: string): Promise<Application> {
    const [row, company] = await Promise.all([
      request<BackendApplication>("/applications", { method: "POST", ...auth(token), body: JSON.stringify({ product_code: productCode }) }),
      companyApi.get(token),
    ]);
    // Сразу привязываем заявку к компании пользователя
    if (company?.id) await patchCore(token, row, {}, company.id);
    return loadApplication(token, row.id);
  },

  /**
   * Автосохранение мастера. Сумма/срок/цель → PATCH заявки;
   * финансы и платёж по кредитам → PUT financials (если таблица уже есть);
   * отметки «исправлено вручную» → в браузер.
   */
  async update(token: string | null, id: string, patch: ApplicationPatch): Promise<void> {
    if (patch.editedCells) localUi.setEditedCells(id, patch.editedCells);
    if (["amount", "term", "purposeCode", "purposeComment"].some((key) => key in patch)) {
      const [row, company] = await Promise.all([request<BackendApplication>(appPath(id), auth(token)), companyApi.get(token)]);
      await patchCore(token, row, patch, company?.id);
    }
    if ("financials" in patch || "debtPayment" in patch) {
      const current = await request<ApiFinancials>(`${appPath(id)}/financials`, auth(token)).catch((reason) => (isNotFound(reason) ? null : Promise.reject(reason)));
      if (!current) return; // таблицы ещё нет — платёж сохранится вместе с финансами после загрузки PDF
      const mapped = fromApiFinancials(current);
      const financials = patch.financials ?? mapped.financials;
      const debt = patch.debtPayment !== undefined ? patch.debtPayment : mapped.debtPayment;
      await putFinancials(token, id, debt ?? 0, financials);
    }
  },

  /** Загрузка финансового PDF: показатели извлекает DS на бэкенде. Повторная загрузка заменяет данные. */
  async uploadFinancialDocument(token: string | null, id: string, file: File): Promise<Application> {
    const body = new FormData();
    body.append("file", file);
    body.append("document_type", "FINANCIAL_STATEMENT");
    await request(`${appPath(id)}/documents`, { method: "POST", ...auth(token), body });
    localUi.setEditedCells(id, []);
    return loadApplication(token, id);
  },

  async saveFinancials(token: string | null, id: string, debtPayment: number, financials: FinanceMonth[]): Promise<Application> {
    await putFinancials(token, id, debtPayment, financials);
    return loadApplication(token, id);
  },

  /** Подтверждение финансов: тело { data_version } текущей версии (берём свежим GET). */
  async confirmFinancials(token: string | null, id: string): Promise<Application> {
    const current = await request<ApiFinancials>(`${appPath(id)}/financials`, auth(token));
    await request(`${appPath(id)}/financials/confirm`, { method: "POST", ...auth(token), body: JSON.stringify({ data_version: current.data_version }) });
    return loadApplication(token, id);
  },

  /** Отправка: скоринг и решение (автоотказ / ручная проверка / автопредложение) выполняет бэкенд. */
  async submit(token: string | null, id: string): Promise<Application> {
    await request(`${appPath(id)}/submit`, { method: "POST", ...auth(token) });
    return loadApplication(token, id);
  },

  /** Ответ на запрос банка: необязательный PDF (SUPPORTING_DOCUMENT) + пояснение. */
  async respondToInfoRequest(token: string | null, id: string, response: { file: File | null; explanation: string }): Promise<Application> {
    if (response.file) {
      const body = new FormData();
      body.append("file", response.file);
      body.append("document_type", "SUPPORTING_DOCUMENT");
      await request(`${appPath(id)}/documents`, { method: "POST", ...auth(token), body });
    }
    const [row, requests] = await Promise.all([
      request<BackendApplication>(appPath(id), auth(token)),
      request<BackendInfoRequest[]>(`${appPath(id)}/info-requests`, auth(token)),
    ]);
    const open = requests.find((item) => item.status === "OPEN");
    if (!open) throw new ApiError(409, "Открытого запроса банка нет");
    await request(`${appPath(id)}/info-requests/${encodeURIComponent(open.id)}/respond`, {
      method: "POST",
      ...auth(token),
      body: JSON.stringify({ data_version: row.data_version, response_comment: response.explanation }),
    });
    return loadApplication(token, id);
  },

  acceptOffer: (token: string | null, id: string) => respondToOffer(token, id, "ACCEPT"),

  /**
   * Отказ от предложения. Причину бэкенд пока не принимает, а повторного
   * рассмотрения (экран 18) на бэкенде нет — отказ окончательный.
   */
  declineOffer: (token: string | null, id: string, _reason: DeclineReason, _comment: string) => respondToOffer(token, id, "DECLINE"),
};

async function respondToOffer(token: string | null, id: string, action: "ACCEPT" | "DECLINE"): Promise<Application> {
  const [row, offers] = await Promise.all([
    request<BackendApplication>(appPath(id), auth(token)),
    request<BackendOffer[]>(`${appPath(id)}/offers`, auth(token)),
  ]);
  const pending = offers.find((offer) => offer.status === "PENDING");
  if (!pending) throw new ApiError(409, "Предложение больше не действует");
  await request(`${appPath(id)}/offers/${encodeURIComponent(pending.id)}/respond`, {
    method: "POST",
    ...auth(token),
    body: JSON.stringify({ data_version: row.data_version, action }),
  });
  return loadApplication(token, id);
}

/* ---------- Кабинет банка ---------- */

type BankDecisionInput = Omit<BankDecision, "at">;
type OfferConditions = { amount: number; term: number; rate: number };

/** Последняя загруженная карточка: из неё берём data_version и запрошенные условия для решения. */
const bankDetails = new Map<string, BankDetail>();

/** Сотрудник из токена: id нужен, чтобы отличать свои решения от чужих. */
const bankMe = (token: string | null, name: string) => ({ id: currentUserId(token), name });

/** Пункты запроса данных: 1..20 разных непустых строк (так требует бэкенд). */
function requiredItems(comment: string) {
  const items = [...new Set(comment.split(/\n|;/).map((line) => line.trim().slice(0, 500)).filter(Boolean))].slice(0, 20);
  return items.length ? items : [comment.trim().slice(0, 500)];
}

export const bankApi = {
  async list(token: string | null, params: { view: QueueView; inn: string; employeeName: string }): Promise<Application[]> {
    const query = new URLSearchParams({ limit: "100" });
    if (params.inn) query.set("q", params.inn);
    if (params.view === "mine") {
      const page = await request<BackendPage<BankDecisionRow>>(`/bank/decisions?${query}`, auth(token));
      return page.items.map((row) => fromDecisionRow(row, bankMe(token, params.employeeName)));
    }
    const path = params.view === "archive" ? "/bank/archive" : "/bank/applications/page";
    const page = await request<BackendPage<BankQueueRow>>(`${path}?${query}`, auth(token));
    return page.items.map(fromQueueRow);
  },

  /**
   * Открыть PDF заявки в новой вкладке. Вкладку открываем сразу по клику
   * (иначе браузер заблокирует всплывающее окно), а адрес подставляем, когда файл скачан.
   */
  async openDocument(token: string | null, applicationId: string, documentId: string): Promise<void> {
    const tab = window.open("", "_blank");
    try {
      const blob = await requestBlob(`/bank/applications/${encodeURIComponent(applicationId)}/documents/${encodeURIComponent(documentId)}/download`, token ?? undefined);
      const url = URL.createObjectURL(blob);
      if (tab) tab.location.href = url;
      else window.open(url, "_blank");
      // Ссылка на файл в памяти браузера: освобождаем через минуту, когда вкладка уже открыла PDF
      window.setTimeout(() => URL.revokeObjectURL(url), 60_000);
    } catch (reason) {
      tab?.close();
      throw reason;
    }
  },

  async get(token: string | null, id: string, employeeName = ""): Promise<Application> {
    const detail = await request<BankDetail>(`/bank/applications/${encodeURIComponent(id)}`, auth(token));
    bankDetails.set(id, detail);
    return fromBankDetail(detail, bankMe(token, employeeName));
  },

  /**
   * Решение сотрудника. Порядок на бэкенде: сначала взять заявку в работу (claim),
   * потом решение. «Одобрить» без изменений → APPROVE (условия заявки и ставка продукта),
   * с изменёнными суммой/сроком/ставкой → COUNTER_OFFER. Запрос данных — отдельный маршрут.
   */
  async decide(token: string | null, id: string, decision: BankDecisionInput, conditions?: OfferConditions): Promise<Application> {
    const base = `/bank/applications/${encodeURIComponent(id)}`;
    const detail = bankDetails.get(id) ?? (await request<BankDetail>(base, auth(token)));
    const dataVersion = detail.application.data_version;
    const comment = decision.comment.trim();

    await request(`${base}/claim`, { method: "POST", ...auth(token), body: JSON.stringify({ data_version: dataVersion }) });

    if (decision.type === "REQUEST_DATA") {
      await request(`${base}/info-requests`, {
        method: "POST",
        ...auth(token),
        body: JSON.stringify({ data_version: dataVersion, description: comment, required_items: requiredItems(comment) }),
      });
    } else if (decision.type === "REJECT") {
      await request(`${base}/decisions`, {
        method: "POST",
        ...auth(token),
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
      await request(`${base}/decisions`, { method: "POST", ...auth(token), body: JSON.stringify(body) });
    }
    return bankApi.get(token, id, decision.employeeName);
  },
};
