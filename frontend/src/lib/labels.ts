import type { BusinessType, DeclineReason, PurposeCode, RepresentativePosition } from "../types";

export const BUSINESS_TYPE_SHORT: Record<BusinessType, string> = {
  LLC: "ООО",
  IP: "ИП",
};

export const BUSINESS_TYPE_FULL: Record<BusinessType, string> = {
  LLC: "Общество с ограниченной ответственностью",
  IP: "Индивидуальный предприниматель",
};

export const POSITION_LABELS: Record<RepresentativePosition, string> = {
  director: "Генеральный директор",
  accountant: "Главный бухгалтер",
  other: "Другое",
};

/** Цели кредита — по словарю данных в Miro (purpose_code). */
export const PURPOSE_LABELS: Record<PurposeCode, string> = {
  goods: "Закупка товара",
  materials: "Закупка материалов",
  operating_expenses: "Текущие расходы",
  other: "Другое",
};

export const FINANCE_COLUMNS = [
  { key: "revenue", label: "Выручка" },
  { key: "operatingInflow", label: "Поступления от основной деятельности" },
  { key: "operatingOutflow", label: "Операционные выплаты" },
  { key: "taxOutflow", label: "Налоги" },
] as const;

export function positionText(position: RepresentativePosition, other: string) {
  return position === "other" ? other || "Другое" : POSITION_LABELS[position];
}

export const DECLINE_REASON_LABELS: Record<DeclineReason, string> = {
  rate: "Высокая ставка",
  amount: "Недостаточная сумма",
  term: "Не подходит срок",
  other: "Другое",
};
