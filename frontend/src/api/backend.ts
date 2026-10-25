/**
 * Форматы Go-бэкенда и их перевод в модели фронтенда.
 *
 * Бэкенд хранит деньги в копейках, ставку в базисных пунктах (1990 = 19,90%)
 * и называет статусы по-своему (OFFERED, NEEDS_INFO…). Страницы работают
 * с моделью Application из types.ts, поэтому всё преобразование собрано здесь:
 * если бэкенд что-то переименует, править нужно только этот файл.
 */
import { PURPOSE_LABELS } from "../lib/labels";
import type {
  Application,
  ApplicationStatus,
  BankDecision,
  CheckResult,
  FinanceMonth,
  FinancialDocument,
  HistoryEvent,
  InfoRequest,
  Offer,
  PurposeCode,
  ScoreFactor,
} from "../types";
import { fromApiCompany, type ApiCompany, type ApiFinanceMonth } from "./contracts";

/* ---------- Форматы бэкенда ---------- */

export interface BackendApplication {
  id: string;
  product_code: string;
  company_id: string | null;
  assigned_employee_id?: string | null;
  requested_amount_kopecks: number | null;
  requested_term_months: number | null;
  purpose: string;
  status: string;
  data_version: number;
  created_at: string;
  updated_at: string;
  submitted_at: string | null;
}

export interface BackendOffer {
  id: string;
  version: number;
  data_version?: number;
  amount_kopecks: number;
  term_months: number;
  annual_rate_bps: number;
  monthly_payment_kopecks: number;
  status: string;
  created_at: string;
  valid_until: string;
  responded_at?: string | null;
  comment?: string;
  decision_type?: string;
}

export interface BackendInfoRequest {
  id: string;
  description: string;
  required_items: string[];
  status: "OPEN" | "RESPONDED" | "RESOLVED" | string;
  response_comment: string | null;
  created_at: string;
  responded_at: string | null;
}

export interface BackendDecision {
  id: string;
  application_id: string;
  decided_by_user_id: string | null;
  decision_type: string;
  decision_source: string;
  reason_code: string;
  comment: string;
  created_at: string;
}

interface BackendExplanation {
  positive_factors?: string[];
  negative_factors?: string[];
}

export interface BackendScoring {
  score: number;
  explanation: BackendExplanation | null;
  created_at: string;
}

export interface BankDetail {
  application: BackendApplication;
  company: ApiCompany | null;
  product: { code: string; name: string; base_annual_rate_bps: number };
  declared_monthly_debt_payment_kopecks: number | null;
  financials: Array<ApiFinanceMonth & { data_version: number; confirmed_at: string | null }>;
  scoring: BackendScoring | null;
  documents: Array<{ id: string; document_type: string; original_filename: string; processing_status: string; uploaded_at: string }>;
  checks: Array<{ id: string; check_type: string; execution_status: string; outcome: string | null }>;
  history: Array<{ id: string; from_status: string | null; to_status: string; reason: string; created_at: string }>;
  info_requests: BackendInfoRequest[];
  decisions: BackendDecision[];
  offers: BackendOffer[];
}

/** Строка очереди и архива (/bank/applications/page, /bank/archive). */
export interface BankQueueRow {
  id: string;
  company_name: string;
  inn: string;
  product_code: string;
  requested_amount_kopecks: number | null;
  requested_term_months: number | null;
  status: string;
  score: number | null;
  submitted_at: string | null;
  assigned_employee_id: string | null;
}

/** Строка «Моих решений» (/bank/decisions). */
export interface BankDecisionRow extends BackendDecision {
  company_name: string;
  inn: string;
  application_status: string;
}

export interface Page<T> {
  items: T[];
  total: number;
}

/* ---------- Простые преобразования ---------- */

const rubles = (kopecks: number | null | undefined) => (kopecks === null || kopecks === undefined ? null : kopecks / 100);
export const percent = (bps: number) => bps / 100;
export const bps = (rate: number) => Math.round(rate * 100);
export const kopecks = (value: number) => Math.round(value * 100);

/** Через сколько после принятия предложения показываем «Кредит выдан» (подписания и выдачи на бэкенде нет). */
const DISBURSE_DELAY_MS = 5000;

const STATUSES: Record<string, ApplicationStatus> = {
  DRAFT: "DRAFT",
  SUBMITTED: "SUBMITTED",
  IN_REVIEW: "IN_REVIEW",
  MANUAL: "MANUAL",
  NEEDS_INFO: "NEED_DATA",
  OFFERED: "OFFER",
  REJECTED: "REJECTED",
  OFFER_DECLINED: "DECLINED",
  OFFER_EXPIRED: "DECLINED",
  CANCELLED: "DECLINED",
  SIGNED: "SIGNED",
  DISBURSED: "DISBURSED",
  CLOSED: "DISBURSED",
};

/**
 * Статус бэкенда → статус экранов.
 * ACCEPTED («клиент принял предложение») — последний шаг бэкенда: подписания и выдачи
 * в нём нет. Чтобы сценарий демо дошёл до «Кредит выдан», первые секунды после
 * принятия показываем «Подписание», затем «Выдан» (это симуляция, как и раньше в моке).
 */
export function mapStatus(status: string, changedAt?: string | null): ApplicationStatus {
  if (status === "ACCEPTED") {
    const since = changedAt ? Date.now() - Date.parse(changedAt) : 0;
    return since > DISBURSE_DELAY_MS ? "DISBURSED" : "SIGNED";
  }
  return STATUSES[status] ?? "IN_REVIEW";
}

/** Публичного номера у бэкенда нет — показываем короткий номер из UUID, одинаковый у клиента и банка. */
export const numberFromId = (id: string) => `AH-${id.replace(/-/g, "").slice(0, 6).toUpperCase()}`;

/** Цель хранится одной строкой: название из справочника или свободный текст («Другое»). */
export function parsePurpose(text: string): { purposeCode: PurposeCode | null; purposeComment: string } {
  const value = text.trim();
  if (!value) return { purposeCode: null, purposeComment: "" };
  const code = (Object.keys(PURPOSE_LABELS) as PurposeCode[]).find((key) => key !== "other" && PURPOSE_LABELS[key] === value);
  return code ? { purposeCode: code, purposeComment: "" } : { purposeCode: "other", purposeComment: value };
}

export function toFinanceMonths(rows: ApiFinanceMonth[]): FinanceMonth[] {
  return rows.map((month) => ({
    period: month.period_month,
    revenue: rubles(month.revenue_kopecks),
    operatingInflow: rubles(month.operating_inflow_kopecks),
    operatingOutflow: rubles(month.operating_outflow_kopecks),
    taxOutflow: rubles(month.tax_outflow_kopecks),
  }));
}

/** Предложения бэкенд отдаёт новыми первыми, экраны ждут старые первыми (последнее — at(-1)). */
export function toOffers(rows: BackendOffer[]): Offer[] {
  return [...rows].reverse().map((row, index) => {
    const expired = row.status === "EXPIRED" || row.status === "WITHDRAWN" || (row.status === "PENDING" && Date.parse(row.valid_until) < Date.now());
    const status: Offer["status"] = row.status === "ACCEPTED" ? "ACCEPTED" : row.status === "DECLINED" ? "DECLINED" : expired ? "EXPIRED" : "ACTIVE";
    return {
      id: row.id,
      number: index + 1,
      amount: row.amount_kopecks / 100,
      term: row.term_months,
      rate: percent(row.annual_rate_bps),
      monthlyPayment: row.monthly_payment_kopecks / 100,
      validUntil: row.valid_until,
      status,
    };
  });
}

/** Актуальный запрос допданных (новые первыми). Срок ответа бэкенд не задаёт — показываем 2 дня. */
export function toInfoRequest(rows: BackendInfoRequest[]): InfoRequest | null {
  const row = rows[0];
  if (!row) return null;
  const due = new Date(row.created_at);
  due.setDate(due.getDate() + 2);
  return {
    id: row.id,
    createdAt: row.created_at,
    dueDate: due.toISOString(),
    items: row.required_items,
    comment: row.description,
    response: row.response_comment ? { at: row.responded_at ?? row.created_at, fileName: null, explanation: row.response_comment } : undefined,
  };
}

export function toScoreFactors(explanation: BackendExplanation | null | undefined): ScoreFactor[] {
  return [
    ...(explanation?.positive_factors ?? []).map((text) => ({ text, positive: true })),
    ...(explanation?.negative_factors ?? []).map((text) => ({ text, positive: false })),
  ];
}

const STATUS_TITLES: Record<string, string> = {
  DRAFT: "Черновик",
  SUBMITTED: "Заявка отправлена",
  IN_REVIEW: "Автоматическая проверка",
  MANUAL: "Ручная проверка",
  NEEDS_INFO: "Запрошены дополнительные данные",
  OFFERED: "Предложение готово",
  REJECTED: "Отказ",
  ACCEPTED: "Предложение принято",
  OFFER_DECLINED: "Предложение отклонено",
  SIGNED: "Договор подписан",
  DISBURSED: "Кредит выдан",
  CLOSED: "Кредит закрыт",
  CANCELLED: "Заявка отменена",
};

/* ---------- Пустая заявка (поля, которых нет в источнике) ---------- */

export function blankApplication(id: string): Application {
  return {
    id,
    number: numberFromId(id),
    userId: "",
    productCode: "",
    status: "SUBMITTED",
    company: null,
    amount: null,
    term: null,
    purposeCode: null,
    purposeComment: "",
    debtPayment: null,
    financials: null,
    editedCells: [],
    document: null,
    dataConfirmed: true,
    createdAt: "",
    updatedAt: "",
    submittedAt: null,
    score: null,
    scoreFactors: [],
    checks: [],
    history: [],
    offers: [],
    infoRequest: null,
    decision: null,
    finalReason: null,
  };
}

/* ---------- Кабинет банка ---------- */

const CHECK_LABELS: Record<string, string> = {
  EGRUL: "ЕГРЮЛ",
  BKI: "БКИ",
  FNS: "ФНС",
  INN_MATCH: "ИНН в документе",
  PERIOD: "Отчётный период",
};

function decisionFrom(row: BackendDecision, me: { id: string; name: string }): BankDecision {
  return {
    type: row.decision_type === "REJECT" ? "REJECT" : "APPROVE",
    comment: row.comment,
    at: row.created_at,
    employeeName: row.decision_source === "AUTO" ? "Автоматическое решение" : row.decided_by_user_id === me.id ? me.name : "Сотрудник банка",
  };
}

/** Карточка заявки для сотрудника банка (GET /bank/applications/{id}). */
export function fromBankDetail(detail: BankDetail, me: { id: string; name: string }): Application {
  const row = detail.application;
  const status = mapStatus(row.status, row.updated_at);
  const history: HistoryEvent[] = detail.history.map((item, index, all) => ({
    id: item.id,
    at: item.created_at,
    title: STATUS_TITLES[item.to_status] ?? item.to_status,
    text: item.reason,
    tone: item.to_status === "REJECTED" ? "error" : index === all.length - 1 ? "current" : "done",
  }));

  // Последнее действие банка: решение или запрос данных — что новее
  const lastDecision = detail.decisions[0];
  const lastRequest = detail.info_requests[0];
  let decision: BankDecision | null = lastDecision ? decisionFrom(lastDecision, me) : null;
  if (lastRequest && (!lastDecision || Date.parse(lastRequest.created_at) > Date.parse(lastDecision.created_at))) {
    decision = { type: "REQUEST_DATA", comment: lastRequest.description, at: lastRequest.created_at, employeeName: "Сотрудник банка" };
  }

  const pdf = detail.documents.find((doc) => doc.document_type === "FINANCIAL_STATEMENT") ?? detail.documents[0];
  const document: FinancialDocument | null = pdf
    ? { id: pdf.id, name: pdf.original_filename, sizeBytes: 0, uploadedAt: pdf.uploaded_at, status: "EXTRACTED" }
    : null;

  const checks: CheckResult[] = detail.checks.map((check) => ({
    name: CHECK_LABELS[check.check_type] ?? check.check_type,
    result: check.outcome ?? check.execution_status,
    ok: /^(PASS|PASSED|OK|SUCCESS)$/i.test(check.outcome ?? ""),
  }));

  return {
    ...blankApplication(row.id),
    productCode: row.product_code,
    status,
    company: detail.company ? fromApiCompany(detail.company) : null,
    amount: rubles(row.requested_amount_kopecks),
    term: row.requested_term_months,
    ...parsePurpose(row.purpose),
    debtPayment: rubles(detail.declared_monthly_debt_payment_kopecks),
    financials: toFinanceMonths(detail.financials),
    document,
    createdAt: row.created_at,
    updatedAt: row.updated_at,
    submittedAt: row.submitted_at,
    score: detail.scoring?.score ?? null,
    scoreFactors: toScoreFactors(detail.scoring?.explanation),
    checks,
    history,
    offers: toOffers(detail.offers),
    infoRequest: toInfoRequest(detail.info_requests),
    decision,
    finalReason: status === "REJECTED" ? (lastDecision?.comment ?? null) : null,
  };
}

/** В списках нет времени смены статуса: принятое предложение показываем уже выданным. */
const SETTLED = "1970-01-01T00:00:00Z";

/** Строка очереди/архива → заявка (только поля таблицы). */
export function fromQueueRow(row: BankQueueRow): Application {
  return {
    ...blankApplication(row.id),
    productCode: row.product_code,
    status: mapStatus(row.status, SETTLED),
    company: { ...fromApiCompany({ id: "", inn: row.inn, name: row.company_name, business_type: "", registration_date: "", main_okved: "" }) },
    amount: rubles(row.requested_amount_kopecks),
    term: row.requested_term_months,
    submittedAt: row.submitted_at,
    score: row.score,
  };
}

/** Строка «Моих решений» → заявка. */
export function fromDecisionRow(row: BankDecisionRow, me: { id: string; name: string }): Application {
  return {
    ...blankApplication(row.application_id),
    status: mapStatus(row.application_status, SETTLED),
    company: { ...fromApiCompany({ id: "", inn: row.inn, name: row.company_name, business_type: "", registration_date: "", main_okved: "" }) },
    submittedAt: row.created_at,
    decision: decisionFrom(row, me),
  };
}

/* ---------- Клиент: заявка после отправки ---------- */

/**
 * Ход заявки для клиента. Отдельной истории для клиента бэкенд не отдаёт,
 * поэтому собираем её из дат заявки, запросов данных и предложений.
 */
export function clientHistory(row: BackendApplication, offers: BackendOffer[], requests: BackendInfoRequest[]): HistoryEvent[] {
  const events: HistoryEvent[] = [];
  if (row.submitted_at) {
    events.push({ id: "submitted", at: row.submitted_at, title: "Заявка отправлена", text: "Данные и финансовая сводка переданы в банк", tone: "done" });
    events.push({ id: "scoring", at: row.submitted_at, title: "Автоматическая проверка", text: "Скоринг по финансовым показателям выполнен", tone: "done" });
  }
  for (const request of requests) {
    events.push({ id: `req-${request.id}`, at: request.created_at, title: "Банк запросил данные", text: request.required_items.join(", "), tone: "done" });
    if (request.responded_at) events.push({ id: `resp-${request.id}`, at: request.responded_at, title: "Вы отправили данные", text: request.response_comment ?? "", tone: "done" });
  }
  for (const offer of offers) {
    const text = `${(offer.amount_kopecks / 100).toLocaleString("ru-RU")} ₽ · ${offer.term_months} мес. · ${percent(offer.annual_rate_bps).toLocaleString("ru-RU")}%`;
    events.push({ id: `offer-${offer.id}`, at: offer.created_at, title: "Предложение банка", text: offer.comment ? `${text}. Комментарий банка: ${offer.comment}` : text, tone: "done" });
    if (offer.responded_at && offer.status === "ACCEPTED") events.push({ id: `acc-${offer.id}`, at: offer.responded_at, title: "Вы приняли предложение", text: "Договор отправлен на подписание", tone: "done" });
    if (offer.responded_at && offer.status === "DECLINED") events.push({ id: `dec-${offer.id}`, at: offer.responded_at, title: "Вы отклонили предложение", text: "Заявка завершена", tone: "done" });
  }
  // Сравниваем как время, а не как строки: бэкенд отдаёт даты в разных часовых поясах (Z и +05:00)
  events.sort((a, b) => Date.parse(a.at) - Date.parse(b.at));

  const status = mapStatus(row.status, row.updated_at);
  if (status === "MANUAL") events.push({ id: "manual", at: row.updated_at, title: "Ручная проверка", text: "Заявку рассматривает специалист банка", tone: "current" });
  if (status === "REJECTED") events.push({ id: "rejected", at: row.updated_at, title: "Отказ банка", text: "Банк не может одобрить заявку", tone: "error" });
  if (status === "DISBURSED") events.push({ id: "disbursed", at: row.updated_at, title: "Кредит выдан", text: "Деньги поступили на расчётный счёт", tone: "done" });
  if (status === "SIGNED") events.push({ id: "signing", at: row.updated_at, title: "Подписание", text: "Оформляем выдачу", tone: "current" });
  return events;
}

/** Комментарий банка к последнему предложению — как решение банка для экрана предложения. */
export function clientDecision(offers: BackendOffer[]): BankDecision | null {
  const latest = offers[0];
  if (!latest?.comment) return null;
  return { type: "APPROVE", comment: latest.comment, at: latest.created_at, employeeName: "Банк" };
}
