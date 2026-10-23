/**
 * Демонстрационные данные. Хранятся в localStorage, поэтому клиент и сотрудник
 * банка в одном браузере видят одни и те же заявки — решение банка сразу
 * отражается в кабинете клиента.
 *
 * ИНН ниже проходят проверку контрольной суммы (в макетах Figma ИНН её не проходят,
 * поэтому последние цифры изменены).
 */
import { formatDayTime } from "../../lib/format";
import type { Application, Company, CompanyDocument, FinanceMonth, RegistryCompany, User } from "../../types";

export interface DemoUser extends User {
  createdAt: string;
}

export interface DemoDb {
  version: number;
  users: DemoUser[];
  companies: Record<string, Company>; // userId → компания
  companyDocuments: Record<string, CompanyDocument[]>; // userId → документы
  applications: Application[];
  sequence: number; // для номеров заявок AH-xxxx
}

export const DB_VERSION = 4;

export const CLIENT_ID = "user-client-demo";
export const BANK_ID = "user-bank-demo";

/** Мок реестра ЕГРЮЛ/ЕГРИП: поиск компании по ИНН. */
export const REGISTRY: RegistryCompany[] = [
  {
    inn: "6671123450",
    name: "ООО «Север»",
    businessType: "LLC",
    registrationDate: "2018-03-14",
    okvedCode: "46.39",
    okvedName: "Оптовая торговля",
    source: "EGRUL",
  },
  {
    inn: "7805123400",
    name: "ООО «Маяк»",
    businessType: "LLC",
    registrationDate: "2016-09-02",
    okvedCode: "47.19",
    okvedName: "Розничная торговля в неспециализированных магазинах",
    source: "EGRUL",
  },
  {
    inn: "667998811274",
    name: "ИП Орлов Сергей Николаевич",
    businessType: "IP",
    registrationDate: "2020-05-19",
    okvedCode: "43.32",
    okvedName: "Столярные и плотничные работы",
    source: "EGRIP",
  },
  {
    inn: "7722446686",
    name: "ООО «Вектор»",
    businessType: "LLC",
    registrationDate: "2014-11-27",
    okvedCode: "25.62",
    okvedName: "Обработка металлических изделий",
    source: "EGRUL",
  },
  {
    // Для демонстрации ошибки «бизнес существует меньше 6 месяцев»
    inn: "7707083893",
    name: "ООО «Новый старт»",
    businessType: "LLC",
    registrationDate: "2026-07-01",
    okvedCode: "62.01",
    okvedName: "Разработка компьютерного программного обеспечения",
    source: "EGRUL",
  },
];

/**
 * ИНН, на котором мок имитирует недоступность реестра —
 * чтобы показать состояние «источник недоступен».
 */
export const REGISTRY_UNAVAILABLE_INN = "7736050003";

/** 6 последних полных календарных месяцев до текущей даты: YYYY-MM. */
export function lastFullMonths(count = 6, now = new Date()): string[] {
  const result: string[] = [];
  for (let i = count; i >= 1; i -= 1) {
    const date = new Date(now.getFullYear(), now.getMonth() - i, 1);
    result.push(`${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, "0")}`);
  }
  return result;
}

/** Финансы «как будто извлечены из PDF». */
export function demoFinancials(scale = 1): FinanceMonth[] {
  const rows: Array<[number, number, number, number]> = [
    [2_450_000, 2_180_000, 1_320_000, 310_000],
    [2_610_000, 2_340_000, 1_410_000, 325_000],
    [2_380_000, 2_120_000, 1_290_000, 298_000],
    [2_780_000, 2_510_000, 1_470_000, 342_000],
    [2_920_000, 2_640_000, 1_550_000, 356_000],
    [3_040_000, 2_760_000, 1_610_000, 371_000],
  ];
  return lastFullMonths().map((period, index) => {
    const [revenue, inflow, outflow, tax] = rows[index];
    return {
      period,
      revenue: Math.round(revenue * scale),
      operatingInflow: Math.round(inflow * scale),
      operatingOutflow: Math.round(outflow * scale),
      taxOutflow: Math.round(tax * scale),
    };
  });
}

const minutesAgo = (minutes: number) => new Date(Date.now() - minutes * 60_000).toISOString();
const daysFromNow = (days: number) => new Date(Date.now() + days * 86_400_000).toISOString();

function companyFrom(inn: string, representativeName: string): Company {
  const registry = REGISTRY.find((item) => item.inn === inn)!;
  return { ...registry, representativeName, representativePosition: "director", representativePositionOther: "" };
}

function baseApplication(partial: Partial<Application> & Pick<Application, "id" | "number" | "userId">): Application {
  const now = new Date().toISOString();
  return {
    productCode: "WORKING_CAPITAL",
    status: "DRAFT",
    company: null,
    amount: null,
    term: null,
    purposeCode: null,
    purposeComment: "",
    debtPayment: null,
    financials: null,
    editedCells: [],
    document: null,
    dataConfirmed: false,
    createdAt: now,
    updatedAt: now,
    submittedAt: null,
    score: null,
    scoreFactors: [],
    checks: [],
    history: [],
    offers: [],
    infoRequest: null,
    decision: null,
    finalReason: null,
    ...partial,
  };
}

/** Заявка других клиентов для банковской очереди. */
function queueApplication(
  id: string,
  number: string,
  inn: string,
  representative: string,
  amount: number,
  term: number,
  score: number,
  status: Application["status"],
  scale: number,
): Application {
  const submittedAt = minutesAgo(60 * 26);
  return baseApplication({
    id,
    number,
    userId: `user-${id}`,
    status,
    company: companyFrom(inn, representative),
    amount,
    term,
    purposeCode: "goods",
    debtPayment: Math.round(90_000 * scale),
    financials: demoFinancials(scale),
    document: {
      id: `doc-${id}`,
      name: "financial_summary.pdf",
      sizeBytes: 1_887_436,
      uploadedAt: submittedAt,
      status: "EXTRACTED",
    },
    dataConfirmed: true,
    createdAt: submittedAt,
    updatedAt: submittedAt,
    submittedAt,
    score,
    scoreFactors: [
      { text: "стабильная выручка", positive: true },
      { text: "высокая долговая нагрузка", positive: false },
    ],
    checks: [
      { name: "ЕГРЮЛ", result: "пройдено", ok: true },
      { name: "БКИ", result: "требуется оценка", ok: false },
    ],
    history: [
      { id: `${id}-h1`, at: submittedAt, title: "Заявка отправлена", text: formatDayTime(submittedAt), tone: "done" },
      { id: `${id}-h2`, at: submittedAt, title: "Данные проверены", text: "Компания и документ приняты", tone: "done" },
      { id: `${id}-h3`, at: submittedAt, title: "Ручная проверка", text: "Заявку рассматривает специалист", tone: "current" },
    ],
  });
}

export function createSeed(): DemoDb {
  const createdAt = minutesAgo(60 * 24 * 3);
  const client: DemoUser = {
    id: CLIENT_ID,
    name: "Петров Иван Сергеевич",
    email: "client@demo.ru",
    role: "CLIENT",
    company_id: "company-sever",
    createdAt,
  };
  const bank: DemoUser = {
    id: BANK_ID,
    name: "Мария Соколова",
    email: "bank@demo.ru",
    role: "BANK_EMPLOYEE",
    company_id: null,
    createdAt,
  };

  const clientCompany = companyFrom("6671123450", client.name);

  const draft = baseApplication({
    id: "app-1048",
    number: "AH-1048",
    userId: CLIENT_ID,
    company: clientCompany,
    createdAt: minutesAgo(60 * 20),
    updatedAt: minutesAgo(60 * 20),
  });

  return {
    version: DB_VERSION,
    users: [client, bank],
    companies: { [CLIENT_ID]: clientCompany },
    companyDocuments: {
      [CLIENT_ID]: [
        {
          id: "cdoc-1",
          name: "Бухгалтерская_отчётность_Север_2025.pdf",
          sizeBytes: 2_516_582,
          uploadedAt: minutesAgo(60 * 22),
          status: "READY",
        },
      ],
    },
    applications: [
      draft,
      {
        ...queueApplication("app-1042", "AH-1042", "6671123450", client.name, 1_300_000, 18, 67, "MANUAL", 1),
        userId: CLIENT_ID,
        company: clientCompany,
        purposeCode: "materials",
        debtPayment: 120_000,
        scoreFactors: [
          { text: "стабильная выручка", positive: true },
          { text: "высокая долговая нагрузка", positive: false },
          { text: "колебания поступлений", positive: false },
        ],
      },
      queueApplication("app-1038", "AH-1038", "7805123400", "Смирнова Анна Олеговна", 2_100_000, 12, 58, "MANUAL", 1.3),
      queueApplication("app-1032", "AH-1032", "667998811274", "Орлов Сергей Николаевич", 850_000, 24, 71, "MANUAL", 0.6),
      {
        ...queueApplication("app-1024", "AH-1024", "7722446686", "Кузнецов Павел Ильич", 4_000_000, 36, 82, "OFFER", 2.2),
        offers: [
          {
            id: "offer-1024-1",
            number: 1,
            amount: 4_000_000,
            term: 36,
            rate: 18.9,
            monthlyPayment: 146_400,
            validUntil: daysFromNow(3),
            status: "ACTIVE",
          },
        ],
        decision: { type: "APPROVE", comment: "Стабильная выручка, нагрузка в норме", at: minutesAgo(60 * 4), employeeName: "Мария Соколова" },
      },
    ],
    sequence: 1048,
  };
}
