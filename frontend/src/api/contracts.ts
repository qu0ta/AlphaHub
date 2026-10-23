import type { Company, FinanceMonth } from "../types";

export interface ApiCompany {
  id: string;
  inn: string;
  name: string;
  business_type: string;
  registration_date: string;
  main_okved: string;
}

export function fromApiCompany(row: ApiCompany): Company {
  const isIp = /^(ИП|IP)$/i.test(row.business_type.trim());
  return {
    id: row.id,
    inn: row.inn,
    name: row.name,
    businessType: isIp ? "IP" : "LLC",
    registrationDate: row.registration_date,
    okvedCode: row.main_okved,
    okvedName: `Код ОКВЭД ${row.main_okved}`,
    source: isIp ? "EGRIP" : "EGRUL",
    representativeName: "",
    representativePosition: "director",
    representativePositionOther: "",
  };
}

export interface ApiFinanceMonth {
  period_month: string;
  revenue_kopecks: number;
  operating_inflow_kopecks: number;
  operating_outflow_kopecks: number;
  tax_outflow_kopecks: number;
}

export interface ApiFinancials {
  application_id: string;
  data_version: number;
  declared_monthly_debt_payment_kopecks: number | null;
  financials: ApiFinanceMonth[];
  confirmed_at: string | null;
  confirmed_by_user_id: string | null;
}

const rubles = (kopecks: number) => kopecks / 100;
const kopecks = (value: number | null) => Math.round((value ?? 0) * 100);

export function fromApiFinancials(row: ApiFinancials) {
  return {
    version: row.data_version,
    debtPayment: row.declared_monthly_debt_payment_kopecks === null ? null : rubles(row.declared_monthly_debt_payment_kopecks),
    financials: row.financials.map((month): FinanceMonth => ({
      period: month.period_month,
      revenue: rubles(month.revenue_kopecks),
      operatingInflow: rubles(month.operating_inflow_kopecks),
      operatingOutflow: rubles(month.operating_outflow_kopecks),
      taxOutflow: rubles(month.tax_outflow_kopecks),
    })),
    confirmed: row.confirmed_at !== null,
  };
}

export function toApiFinancials(dataVersion: number, debtPayment: number, financials: FinanceMonth[]) {
  return {
    data_version: dataVersion,
    declared_monthly_debt_payment_kopecks: kopecks(debtPayment),
    financials: financials.map((month) => ({
      period_month: month.period,
      revenue_kopecks: kopecks(month.revenue),
      operating_inflow_kopecks: kopecks(month.operatingInflow),
      operating_outflow_kopecks: kopecks(month.operatingOutflow),
      tax_outflow_kopecks: kopecks(month.taxOutflow),
    })),
  };
}
