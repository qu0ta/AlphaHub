/**
 * Модели данных фронтенда. Суммы — в рублях (в API — копейки).
 * Названия полей близки к словарю данных в Miro, чтобы при интеграции
 * с бэкендом преобразование было механическим.
 */

export type Role = "CLIENT" | "BANK_EMPLOYEE";

export interface User {
  id: string;
  name: string;
  email: string;
  role: Role;
  company_id?: string | null;
}

/* ---------- Компания ---------- */

export type BusinessType = "LLC" | "IP";
export type RepresentativePosition = "director" | "accountant" | "other";

/** Сведения из реестра (ЕГРЮЛ/ЕГРИП) — клиент их не редактирует. */
export interface RegistryCompany {
  id?: string;
  inn: string;
  name: string;
  businessType: BusinessType;
  registrationDate: string; // YYYY-MM-DD
  okvedCode: string;
  okvedName: string;
  source: "EGRUL" | "EGRIP";
}

export interface Company extends RegistryCompany {
  id?: string;
  representativeName: string;
  representativePosition: RepresentativePosition;
  representativePositionOther: string;
}

export type DocumentStatus = "PROCESSING" | "READY";

export interface CompanyDocument {
  id: string;
  name: string;
  sizeBytes: number;
  uploadedAt: string;
  status: DocumentStatus;
}

/* ---------- Продукты ---------- */

export interface Product {
  code: string;
  name: string;
  description: string;
  minAmount: number;
  maxAmount: number;
  minTerm: number;
  maxTerm: number;
  rate: number;
  active: boolean;
  requiresCollateral?: boolean;
}

/* ---------- Заявка ---------- */

/**
 * Статусы по диаграмме состояний в Miro.
 * RETENTION_REVIEW — «Повторное рассмотрение» (экран 18 в Figma): клиент отклонил
 * предложение, банк проверяет запас. В диаграмме Miro такого состояния нет —
 * нужно согласовать с бэкендом.
 */
export type ApplicationStatus =
  | "DRAFT"
  | "SUBMITTED"
  | "NEED_DATA"
  | "IN_REVIEW"
  | "MANUAL"
  | "OFFER"
  | "RETENTION_REVIEW"
  | "RETENTION_OFFER"
  | "SIGNED"
  | "DISBURSED"
  | "REJECTED"
  | "DECLINED";

export type PurposeCode = "goods" | "materials" | "operating_expenses" | "other";

/** Один месяц финансовой сводки, рубли. null — значение не распознано. */
export interface FinanceMonth {
  period: string; // YYYY-MM
  revenue: number | null;
  operatingInflow: number | null;
  operatingOutflow: number | null;
  taxOutflow: number | null;
}

export type FinanceField = Exclude<keyof FinanceMonth, "period">;

export interface FinancialDocument {
  id: string;
  name: string;
  sizeBytes: number;
  uploadedAt: string;
  status: "UPLOADED" | "PROCESSING" | "EXTRACTED" | "ERROR";
  error?: string;
}

export type HistoryTone = "done" | "current" | "error";

export interface HistoryEvent {
  id: string;
  at: string;
  title: string;
  text: string;
  tone: HistoryTone;
}

export interface Offer {
  id: string;
  number: number; // 1 — первое предложение, 2+ — улучшенные
  amount: number;
  term: number;
  rate: number;
  monthlyPayment: number;
  validUntil: string;
  status: "ACTIVE" | "ACCEPTED" | "DECLINED" | "EXPIRED";
  declineReason?: DeclineReason;
  declineComment?: string;
}

export type DeclineReason = "rate" | "amount" | "term" | "other";

export interface InfoRequest {
  id: string;
  createdAt: string;
  dueDate: string;
  items: string[];
  comment: string;
  response?: {
    at: string;
    fileName: string | null;
    explanation: string;
  };
}

export interface ScoreFactor {
  text: string;
  positive: boolean;
}

export interface CheckResult {
  name: string;
  result: string;
  ok: boolean;
}

export type DecisionType = "APPROVE" | "REJECT" | "REQUEST_DATA";

export interface BankDecision {
  type: DecisionType;
  comment: string;
  at: string;
  employeeName: string;
}

export interface Application {
  id: string;
  number: string; // AH-1042
  userId: string;
  productCode: string;
  status: ApplicationStatus;
  company: Company | null; // снимок данных компании на момент отправки
  amount: number | null;
  term: number | null;
  purposeCode: PurposeCode | null;
  purposeComment: string;
  debtPayment: number | null;
  financials: FinanceMonth[] | null;
  editedCells: string[]; // «2026-04.revenue» — исправлено клиентом
  document: FinancialDocument | null;
  dataConfirmed: boolean;
  createdAt: string;
  updatedAt: string;
  submittedAt: string | null;
  score: number | null;
  scoreFactors: ScoreFactor[];
  checks: CheckResult[];
  history: HistoryEvent[];
  offers: Offer[];
  infoRequest: InfoRequest | null;
  decision: BankDecision | null;
  finalReason: string | null;
}

