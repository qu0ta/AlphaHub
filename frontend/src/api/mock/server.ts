/**
 * Мок бэкенда: те же операции, что потом будут HTTP-запросами к Go.
 *
 * Как устроено время: заявка «сама» проходит этапы (проверка → скоринг → решение),
 * но без таймеров. При каждом чтении функция advance() смотрит, сколько времени
 * прошло с последней смены статуса, и переводит заявку дальше. Поэтому сценарий
 * переживает перезагрузку страницы и работает в двух вкладках (клиент и банк).
 */
import { annuityPayment, formatDayTime, formatLongDate, formatMoney } from "../../lib/format";
import { DECLINE_REASON_LABELS } from "../../lib/labels";
import { isValidInn } from "../../lib/validation";
import type {
  Application,
  BankDecision,
  Company,
  CompanyDocument,
  DeclineReason,
  HistoryEvent,
  Offer,
  Product,
  RegistryCompany,
  User,
} from "../../types";
import { ApiError } from "../http";
import { clone, delay, readDb, uid, updateDb } from "./db";
import { demoFinancials, REGISTRY, REGISTRY_UNAVAILABLE_INN, type DemoDb } from "./seed";

/* ---------- Справочники ---------- */

export const MOCK_PRODUCTS: Product[] = [
 {code:"equipment_loan",name:"Кредит на оборудование",description:"Финансирование покупки оборудования",minAmount:500000,maxAmount:5000000,minTerm:12,maxTerm:36,rate:18,active:true},
  {
    code: "WORKING_CAPITAL",
    name: "Оборотный кредит",
    description: "Для ежедневных задач: закупки товаров, расчёты с поставщиками и пополнение оборотных средств.",
    minAmount: 100_000,
    maxAmount: 10_000_000,
    minTerm: 3,
    maxTerm: 36,
    rate: 19.9,
    active: true,
  },
  {
    code: "INVESTMENT",
    name: "Инвестиционный кредит",
    description: "Для крупных планов: новое оборудование, расширение производства и развитие бизнеса.",
    minAmount: 1_000_000,
    maxAmount: 30_000_000,
    minTerm: 12,
    maxTerm: 84,
    rate: 18.5,
    active: false,
  },
];

const RATE_FLOOR = 16.9; // ниже банк не опускает ставку — это и есть «запас»
const RATE_STEP = 1.5;

/* Задержки этапов, мс — подобраны под демонстрацию */
const STAGE = {
  documentExtraction: 2_500,
  companyDocument: 10_000,
  submittedToReview: 3_000,
  reviewToScore: 4_000,
  signedToDisbursed: 4_000,
  retentionReview: 4_000,
};

const MAX_PDF_BYTES = 10 * 1024 * 1024;

/* ---------- Вспомогательное ---------- */

const now = () => new Date().toISOString();
const daysFromNow = (days: number) => new Date(Date.now() + days * 86_400_000).toISOString();
const elapsed = (iso: string | null) => (iso ? Date.now() - new Date(iso).getTime() : 0);

function findUser(db: DemoDb, userId: string) {
  const user = db.users.find((item) => item.id === userId);
  if (!user) throw new ApiError(401, "Invalid or expired token");
  return user;
}

function product(code: string) {
  return MOCK_PRODUCTS.find((item) => item.code === code) ?? MOCK_PRODUCTS[0];
}

/** Добавить событие в историю: прошлое «текущее» становится «выполненным». */
function pushEvent(app: Application, title: string, text = "", tone: HistoryEvent["tone"] = "current") {
  app.history.forEach((event) => {
    if (event.tone === "current") event.tone = "done";
  });
  app.history.push({ id: uid("h"), at: now(), title, text, tone });
}

function setStatus(app: Application, status: Application["status"]) {
  app.status = status;
  app.updatedAt = now();
}

function activeOffer(app: Application) {
  return app.offers.find((offer) => offer.status === "ACTIVE") ?? null;
}

function makeOffer(app: Application, amount: number, term: number, rate: number): Offer {
  return {
    id: uid("offer"),
    number: app.offers.length + 1,
    amount,
    term,
    rate,
    monthlyPayment: annuityPayment(amount, term, rate),
    validUntil: daysFromNow(3),
    status: "ACTIVE",
  };
}

/* ---------- Скоринг (упрощённая модель для демо) ---------- */

function average(values: Array<number | null>) {
  const numbers = values.filter((value): value is number => value !== null);
  return numbers.length ? numbers.reduce((a, b) => a + b, 0) / numbers.length : 0;
}

/**
 * Оценка 0–100. Для демо важна предсказуемость:
 * при платеже по текущим кредитам 120 000 ₽ (данные по умолчанию) балл ≈ 60–67 →
 * заявка уходит сотруднику банка. При платеже 0 ₽ балл выше 75 → банк сразу
 * присылает предложение.
 */
export function scoreApplication(app: Application) {
  const financials = app.financials ?? [];
  const inflow = average(financials.map((row) => row.operatingInflow));
  const outflow = average(financials.map((row) => row.operatingOutflow));
  const taxes = average(financials.map((row) => row.taxOutflow));
  const revenue = average(financials.map((row) => row.revenue));
  const debt = app.debtPayment ?? 0;
  const newPayment = annuityPayment(app.amount ?? 0, app.term ?? 12, product(app.productCode).rate);
  const freeCash = inflow - outflow - taxes - debt;
  const coverage = newPayment ? freeCash / newPayment : 0;

  const revenues = financials.map((row) => row.revenue ?? 0);
  const volatility = revenue ? (Math.max(...revenues) - Math.min(...revenues)) / revenue : 0;

  const registration = app.company ? new Date(app.company.registrationDate) : new Date();
  const ageYears = (Date.now() - registration.getTime()) / (365 * 86_400_000);

  let score = 50 + Math.min(20, coverage * 3);
  const factors: Application["scoreFactors"] = [];

  if (coverage >= 2) factors.push({ text: "стабильная выручка", positive: true });
  if (ageYears >= 3) {
    score += 10;
    factors.push({ text: "бизнес работает больше 3 лет", positive: true });
  }
  if (revenue && debt / revenue > 0.04) {
    score -= 15;
    factors.push({ text: "высокая долговая нагрузка", positive: false });
  }
  if (volatility > 0.2) {
    score -= 5;
    factors.push({ text: "колебания поступлений", positive: false });
  }
  if (coverage < 1) {
    score -= 25;
    factors.push({ text: "платёж не покрывается свободным денежным потоком", positive: false });
  }

  return { score: Math.max(0, Math.min(100, Math.round(score))), factors };
}

/* ---------- Автоматическое продвижение заявки ---------- */

function advance(app: Application): boolean {
  let changed = false;

  if (app.document?.status === "PROCESSING" && elapsed(app.document.uploadedAt) > STAGE.documentExtraction) {
    if (/ошибк|error/i.test(app.document.name)) {
      app.document.status = "ERROR";
      app.document.error = "Не удалось распознать документ. Заполните показатели вручную.";
      app.financials = app.financials ?? demoFinancials(0).map((row) => ({
        ...row,
        revenue: null,
        operatingInflow: null,
        operatingOutflow: null,
        taxOutflow: null,
      }));
    } else {
      app.document.status = "EXTRACTED";
      app.financials = demoFinancials();
      app.editedCells = [];
    }
    changed = true;
  }

  if (app.status === "SUBMITTED" && elapsed(app.updatedAt) > STAGE.submittedToReview) {
    pushEvent(app, "Данные проверены", "Компания и документ приняты", "done");
    pushEvent(app, "Финансовая оценка", "Специалист анализирует документы и показатели");
    app.checks = [
      { name: "ЕГРЮЛ", result: "пройдено", ok: true },
      { name: "БКИ", result: "требуется оценка", ok: false },
    ];
    setStatus(app, "IN_REVIEW");
    changed = true;
  }

  if (app.status === "IN_REVIEW" && elapsed(app.updatedAt) > STAGE.reviewToScore) {
    const { score, factors } = scoreApplication(app);
    app.score = score;
    app.scoreFactors = factors;
    if (score < 45) {
      pushEvent(app, "Решение принято", "Банк не может одобрить заявку", "error");
      app.finalReason = "Результат автоматической проверки ниже порога";
      setStatus(app, "REJECTED");
    } else if (score >= 75) {
      const p = product(app.productCode);
      const offer = makeOffer(app, app.amount ?? p.minAmount, app.term ?? p.minTerm, p.rate);
      app.offers.push(offer);
      pushEvent(app, "Решение принято", "Банк подготовил предложение", "done");
      pushEvent(app, "Предложение готово", `Ответьте до ${formatLongDate(offer.validUntil)}`);
      setStatus(app, "OFFER");
    } else {
      pushEvent(app, "Подготовка решения", "Сообщим, когда предложение будет готово");
      setStatus(app, "MANUAL");
    }
    changed = true;
  }

  if (app.status === "SIGNED" && elapsed(app.updatedAt) > STAGE.signedToDisbursed) {
    const offer = app.offers.find((item) => item.status === "ACCEPTED");
    pushEvent(app, "Деньги выданы", `${formatMoney(offer?.amount ?? app.amount)} зачислены на расчётный счёт`, "done");
    setStatus(app, "DISBURSED");
    changed = true;
  }

  if (app.status === "RETENTION_REVIEW" && elapsed(app.updatedAt) > STAGE.retentionReview) {
    const declined = [...app.offers].reverse().find((offer) => offer.status === "DECLINED");
    const improved = declined ? improveOffer(app, declined) : null;
    if (improved) {
      app.offers.push(improved);
      pushEvent(app, "Новое предложение", `Банк улучшил условия. Ответьте до ${formatLongDate(improved.validUntil)}`);
      setStatus(app, "RETENTION_OFFER");
    } else {
      pushEvent(app, "Улучшение невозможно", "Заявка завершена по вашему решению", "error");
      app.finalReason = "Клиент отказался от предложения";
      setStatus(app, "DECLINED");
    }
    changed = true;
  }

  return changed;
}

/**
 * Запас условий: банк может снизить ставку до RATE_FLOOR,
 * вернуть запрошенную сумму или запрошенный срок.
 * Количество улучшений не фиксировано — пока есть запас.
 */
function improveOffer(app: Application, declined: Offer): Offer | null {
  const p = product(app.productCode);
  const requestedAmount = Math.min(app.amount ?? declined.amount, p.maxAmount);
  const requestedTerm = Math.min(app.term ?? declined.term, p.maxTerm);
  const lowerRate = Math.round((declined.rate - RATE_STEP) * 10) / 10;

  switch (declined.declineReason) {
    case "amount":
      if (declined.amount < requestedAmount) return makeOffer(app, requestedAmount, declined.term, declined.rate);
      return null;
    case "term":
      if (declined.term !== requestedTerm) return makeOffer(app, declined.amount, requestedTerm, declined.rate);
      if (declined.term + 6 <= p.maxTerm) return makeOffer(app, declined.amount, declined.term + 6, declined.rate);
      return null;
    case "rate":
    case "other":
    default:
      if (lowerRate >= RATE_FLOOR) return makeOffer(app, declined.amount, declined.term, lowerRate);
      return null;
  }
}

/** Прогнать advance по всем заявкам и сохранить, если что-то изменилось. */
function syncAll() {
  const db = readDb();
  let changed = false;
  db.applications.forEach((app) => {
    if (advance(app)) changed = true;
  });
  if (changed) updateDb(() => undefined);
  return db;
}

function getOwnApplication(db: DemoDb, userId: string, id: string) {
  const app = db.applications.find((item) => item.id === id && item.userId === userId);
  if (!app) throw new ApiError(404, "Application not found");
  return app;
}

/* =====================================================================
   Авторизация
   ===================================================================== */

export const mockAuth = {
  async register(name: string, email: string): Promise<User> {
    await delay();
    return updateDb((db) => {
      const normalized = email.trim().toLowerCase();
      if (db.users.some((user) => user.email === normalized)) throw new ApiError(409, "User already exists");
      const user = { id: uid("user"), name: name.trim(), email: normalized, role: "CLIENT" as const, company_id: null, createdAt: now() };
      db.users.push(user);
      return clone(user);
    });
  },

  /** В демо любой непустой пароль подходит — пароли не хранятся. */
  async login(email: string, password: string): Promise<{ token: string; user: User }> {
    await delay();
    const user = readDb().users.find((item) => item.email === email.trim().toLowerCase());
    if (!user || !password) throw new ApiError(401, "Invalid credentials");
    return { token: `mock.${user.id}`, user: clone(user) };
  },

  async me(token: string): Promise<User> {
    await delay(120);
    const id = token.replace(/^mock\./, "");
    return clone(findUser(readDb(), id));
  },
};

export function userIdFromToken(token: string | null): string {
  if (!token) throw new ApiError(401, "Bearer token is required");
  return token.replace(/^mock\./, "");
}

/* =====================================================================
   Компания и документы
   ===================================================================== */

export const mockCompany = {
  async lookup(inn: string): Promise<RegistryCompany> {
    await delay(900);
    if (inn === REGISTRY_UNAVAILABLE_INN) throw new ApiError(503, "Registry unavailable");
    if (!isValidInn(inn)) throw new ApiError(400, "Invalid INN");
    const found = REGISTRY.find((item) => item.inn === inn);
    if (!found) throw new ApiError(404, "Company not found");
    return clone(found);
  },

  async get(userId: string): Promise<Company | null> {
    await delay(150);
    return clone(readDb().companies[userId] ?? null);
  },

  async save(userId: string, company: Company): Promise<Company> {
    await delay();
    return updateDb((db) => {
      db.companies[userId] = clone(company);
      // В режиме api пользователь живёт в настоящем бэкенде, а не в моке
      const user = db.users.find((item) => item.id === userId);
      if (user) user.company_id = `company-${company.inn}`;
      // В черновиках обновляем снимок компании
      db.applications
        .filter((app) => app.userId === userId && app.status === "DRAFT")
        .forEach((app) => {
          app.company = clone(company);
        });
      return clone(company);
    });
  },

  async listDocuments(userId: string): Promise<CompanyDocument[]> {
    await delay(150);
    const db = readDb();
    const docs = db.companyDocuments[userId] ?? [];
    let changed = false;
    docs.forEach((doc) => {
      if (doc.status === "PROCESSING" && elapsed(doc.uploadedAt) > STAGE.companyDocument) {
        doc.status = "READY";
        changed = true;
      }
    });
    if (changed) updateDb(() => undefined);
    return clone(docs);
  },

  async uploadDocument(userId: string, file: { name: string; size: number; type: string }): Promise<CompanyDocument> {
    await delay(600);
    if (file.type !== "application/pdf" && !file.name.toLowerCase().endsWith(".pdf")) {
      throw new ApiError(400, "Нужен файл в формате PDF");
    }
    if (file.size > MAX_PDF_BYTES) throw new ApiError(400, "Файл больше 10 МБ");
    return updateDb((db) => {
      const doc: CompanyDocument = { id: uid("cdoc"), name: file.name, sizeBytes: file.size, uploadedAt: now(), status: "PROCESSING" };
      // В демо храним один актуальный документ: новый заменяет старый
      db.companyDocuments[userId] = [doc];
      return clone(doc);
    });
  },

  async deleteDocument(userId: string, id: string): Promise<void> {
    await delay();
    updateDb((db) => {
      db.companyDocuments[userId] = (db.companyDocuments[userId] ?? []).filter((doc) => doc.id !== id);
    });
  },
};

/* =====================================================================
   Заявки клиента
   ===================================================================== */

export type ApplicationPatch = Partial<
  Pick<Application, "amount" | "term" | "purposeCode" | "purposeComment" | "debtPayment" | "financials" | "editedCells" | "dataConfirmed">
>;

export const mockApplications = {
  async list(userId: string): Promise<Application[]> {
    await delay(200);
    const db = syncAll();
    return clone(
      db.applications
        .filter((app) => app.userId === userId)
        .sort((a, b) => b.createdAt.localeCompare(a.createdAt)),
    );
  },

  async get(userId: string, id: string): Promise<Application> {
    await delay(150);
    const db = syncAll();
    return clone(getOwnApplication(db, userId, id));
  },

  /** id передаётся в режиме api: заявку создал бэкенд, мок хранит её расширенные данные под тем же id. */
  async create(userId: string, productCode: string, id?: string): Promise<Application> {
    await delay();
    const p = MOCK_PRODUCTS.find((item) => item.code === productCode);
    if (!p || !p.active) throw new ApiError(400, "Product not found or inactive");
    return updateDb((db) => {
      const company = db.companies[userId];
      if (!company) throw new ApiError(409, "Company is required");
      db.sequence += 1;
      const createdAt = now();
      const app: Application = {
        id: id ?? uid("app"),
        number: `AH-${db.sequence}`,
        userId,
        productCode,
        status: "DRAFT",
        company: clone(company),
        amount: null,
        term: null,
        purposeCode: null,
        purposeComment: "",
        debtPayment: null,
        financials: null,
        editedCells: [],
        document: null,
        dataConfirmed: false,
        createdAt,
        updatedAt: createdAt,
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
      db.applications.push(app);
      return clone(app);
    });
  },

  /** Сохранение черновика — вызывается при каждом изменении в мастере. */
  async update(userId: string, id: string, patch: ApplicationPatch): Promise<Application> {
    await delay(150);
    return updateDb((db) => {
      const app = getOwnApplication(db, userId, id);
      if (app.status !== "DRAFT") throw new ApiError(409, "Application is not a draft");
      Object.assign(app, clone(patch));
      // Любое изменение данных требует повторного подтверждения
      if (!("dataConfirmed" in patch)) app.dataConfirmed = false;
      app.updatedAt = now();
      return clone(app);
    });
  },

  async uploadFinancialDocument(userId: string, id: string, file: { name: string; size: number; type: string }) {
    await delay(500);
    if (file.type !== "application/pdf" && !file.name.toLowerCase().endsWith(".pdf")) {
      throw new ApiError(400, "Нужен текстовый PDF");
    }
    if (file.size > MAX_PDF_BYTES) throw new ApiError(400, "Файл больше 10 МБ");
    return updateDb((db) => {
      const app = getOwnApplication(db, userId, id);
      if (app.status !== "DRAFT") throw new ApiError(409, "Application is not a draft");
      app.document = { id: uid("doc"), name: file.name, sizeBytes: file.size, uploadedAt: now(), status: "PROCESSING" };
      app.financials = null;
      app.editedCells = [];
      app.dataConfirmed = false;
      app.updatedAt = now();
      return clone(app);
    });
  },

  async removeFinancialDocument(userId: string, id: string) {
    await delay(200);
    return updateDb((db) => {
      const app = getOwnApplication(db, userId, id);
      if (app.status !== "DRAFT") throw new ApiError(409, "Application is not a draft");
      app.document = null;
      app.financials = null;
      app.editedCells = [];
      app.dataConfirmed = false;
      return clone(app);
    });
  },

  async submit(userId: string, id: string): Promise<Application> {
    await delay(700);
    return updateDb((db) => {
      const app = getOwnApplication(db, userId, id);
      if (app.status !== "DRAFT") throw new ApiError(409, "Application is not a draft");
      if (!app.dataConfirmed) throw new ApiError(400, "Data is not confirmed");
      if (!app.amount || !app.term || !app.purposeCode || !app.financials || app.debtPayment === null) {
        throw new ApiError(400, "Invalid application input");
      }
      app.company = clone(db.companies[userId] ?? app.company);
      app.submittedAt = now();
      pushEvent(app, "Заявка отправлена", formatDayTime(now()), "current");
      setStatus(app, "SUBMITTED");
      return clone(app);
    });
  },

  async respondToInfoRequest(userId: string, id: string, response: { fileName: string | null; explanation: string }) {
    await delay(600);
    return updateDb((db) => {
      const app = getOwnApplication(db, userId, id);
      if (app.status !== "NEED_DATA" || !app.infoRequest) throw new ApiError(409, "No data requested");
      app.infoRequest.response = { at: now(), ...response };
      pushEvent(app, "Данные дополнены", "Заявка отправлена повторно");
      setStatus(app, "SUBMITTED");
      return clone(app);
    });
  },

  async acceptOffer(userId: string, id: string): Promise<Application> {
    await delay(600);
    return updateDb((db) => {
      const app = getOwnApplication(db, userId, id);
      const offer = activeOffer(app);
      if (!offer || (app.status !== "OFFER" && app.status !== "RETENTION_OFFER")) throw new ApiError(409, "No active offer");
      offer.status = "ACCEPTED";
      pushEvent(app, "Предложение принято", formatDayTime(now()), "done");
      pushEvent(app, "Документы подписаны", "Симуляция подписания завершена");
      setStatus(app, "SIGNED");
      return clone(app);
    });
  },

  async declineOffer(userId: string, id: string, reason: DeclineReason, comment: string): Promise<Application> {
    await delay(600);
    return updateDb((db) => {
      const app = getOwnApplication(db, userId, id);
      const offer = activeOffer(app);
      if (!offer) throw new ApiError(409, "No active offer");
      offer.status = "DECLINED";
      offer.declineReason = reason;
      offer.declineComment = comment;
      pushEvent(app, "Предложение отклонено", `Причина: ${DECLINE_REASON_LABELS[reason].toLowerCase()}`, "done");
      pushEvent(app, "Предложение на повторном рассмотрении", "Банк проверяет, можно ли улучшить условия");
      setStatus(app, "RETENTION_REVIEW");
      return clone(app);
    });
  },
};


/* =====================================================================
   Кабинет сотрудника банка
   ===================================================================== */

export type QueueView = "queue" | "mine" | "archive";

const QUEUE_STATUSES: Application["status"][] = ["SUBMITTED", "IN_REVIEW", "MANUAL", "NEED_DATA"];
const ARCHIVE_STATUSES: Application["status"][] = ["DISBURSED", "REJECTED", "DECLINED"];

export const mockBank = {
  async list(params: { view: QueueView; inn: string; employeeName: string }): Promise<Application[]> {
    await delay(250);
    const db = syncAll();
    return clone(
      db.applications
        .filter((app) => app.status !== "DRAFT")
        .filter((app) => {
          if (params.view === "queue") return QUEUE_STATUSES.includes(app.status) || app.status === "OFFER";
          if (params.view === "archive") return ARCHIVE_STATUSES.includes(app.status);
          return app.decision?.employeeName === params.employeeName;
        })
        .filter((app) => !params.inn || app.company?.inn.includes(params.inn))
        .sort((a, b) => (b.submittedAt ?? "").localeCompare(a.submittedAt ?? "")),
    );
  },

  async get(id: string): Promise<Application> {
    await delay(150);
    const db = syncAll();
    const app = db.applications.find((item) => item.id === id && item.status !== "DRAFT");
    if (!app) throw new ApiError(404, "Application not found");
    return clone(app);
  },

  async decide(
    id: string,
    decision: Omit<BankDecision, "at">,
    conditions?: { amount: number; term: number; rate: number },
  ): Promise<Application> {
    await delay(700);
    return updateDb((db) => {
      const app = db.applications.find((item) => item.id === id);
      if (!app) throw new ApiError(404, "Application not found");
      if (app.status !== "MANUAL") throw new ApiError(409, "Application is not waiting for a decision");
      if (!decision.comment.trim()) throw new ApiError(400, "Comment is required");
      app.decision = { ...decision, at: now() };

      if (decision.type === "APPROVE") {
        const p = product(app.productCode);
        const offer = makeOffer(app, conditions?.amount ?? app.amount ?? p.minAmount, conditions?.term ?? app.term ?? p.minTerm, conditions?.rate ?? p.rate);
        app.offers.push(offer);
        pushEvent(app, "Решение принято", `Банк подготовил предложение. Комментарий: ${decision.comment.trim()}`, "done");
        pushEvent(app, "Предложение готово", `Ответьте до ${formatLongDate(offer.validUntil)}`);
        setStatus(app, "OFFER");
      } else if (decision.type === "REJECT") {
        pushEvent(app, "Решение принято", "Банк не может одобрить заявку", "error");
        app.finalReason = decision.comment;
        setStatus(app, "REJECTED");
      } else {
        const items = decision.comment
          .split(/\n|;/)
          .map((line) => line.trim())
          .filter(Boolean);
        app.infoRequest = { id: uid("req"), createdAt: now(), dueDate: daysFromNow(2), items, comment: decision.comment };
        pushEvent(app, "Проверка данных", "Банк запросил уточнение", "done");
        pushEvent(app, "Нужны дополнительные данные", items.join(", "));
        setStatus(app, "NEED_DATA");
      }
      return clone(app);
    });
  },
};
