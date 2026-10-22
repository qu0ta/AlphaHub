import { formatMoney, formatTerm } from "../../../lib/format";
import type { Application, FinanceMonth, Product } from "../../../types";

/** Ошибки шага «Параметры кредита» — по словарю данных в Miro. */
export function creditErrors(app: Application, product: Product) {
  return {
    amount: !app.amount
      ? "Укажите сумму"
      : app.amount < product.minAmount || app.amount > product.maxAmount
        ? `Сумма от ${formatMoney(product.minAmount)} до ${formatMoney(product.maxAmount)}`
        : null,
    term: !app.term
      ? "Укажите срок"
      : app.term < product.minTerm || app.term > product.maxTerm
        ? `Срок от ${product.minTerm} до ${formatTerm(product.maxTerm)}`
        : null,
    purpose: !app.purposeCode ? "Выберите цель кредита" : null,
    comment: app.purposeCode === "other" && !app.purposeComment.trim() ? "Опишите цель кредита" : null,
  };
}

const FIELDS: Array<keyof Omit<FinanceMonth, "period">> = ["revenue", "operatingInflow", "operatingOutflow", "taxOutflow"];

/** Все 6 месяцев и все показатели должны быть заполнены (значение ≥ 0). */
export function financeErrors(app: Application) {
  const missingCells = (app.financials ?? []).flatMap((row) => FIELDS.filter((field) => row[field] === null).map((field) => `${row.period}.${field}`));
  return {
    document: !app.document
      ? "Загрузите финансовую сводку"
      : app.document.status === "PROCESSING"
        ? "Дождитесь окончания обработки документа"
        : null,
    financials: !app.financials ? "Нет финансовых данных" : missingCells.length ? "Заполните все показатели за 6 месяцев" : null,
    missingCells,
    debt: app.debtPayment === null ? "Укажите платёж; если кредитов нет — 0 ₽" : null,
  };
}

export function hasErrors(errors: Record<string, unknown>) {
  return Object.entries(errors).some(([key, value]) => (key === "missingCells" ? false : Boolean(value)));
}
