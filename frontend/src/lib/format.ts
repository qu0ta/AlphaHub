/**
 * Форматирование чисел, денег, сроков и дат для интерфейса.
 * Интерфейс работает в рублях, API — в копейках (см. словарь данных в Miro),
 * перевод делается только в слое api/.
 */

const moneyFormatter = new Intl.NumberFormat("ru-RU", { maximumFractionDigits: 0 });

/** 1300000 → «1 300 000 ₽» */
export function formatMoney(rubles: number | null | undefined): string {
  if (rubles === null || rubles === undefined || Number.isNaN(rubles)) return "—";
  return `${moneyFormatter.format(rubles)} ₽`;
}

/** 1300000 → «1 300 000» (без знака рубля — для полей ввода) */
export function formatNumber(value: number | null | undefined): string {
  if (value === null || value === undefined || Number.isNaN(value)) return "";
  return moneyFormatter.format(value);
}

/** Правильное окончание: plural(5, ["месяц", "месяца", "месяцев"]) → «месяцев» */
export function plural(n: number, forms: [string, string, string]): string {
  const abs = Math.abs(n) % 100;
  const last = abs % 10;
  if (abs > 10 && abs < 20) return forms[2];
  if (last > 1 && last < 5) return forms[1];
  if (last === 1) return forms[0];
  return forms[2];
}

export const MONTH_FORMS: [string, string, string] = ["месяц", "месяца", "месяцев"];

/** 18 → «18 месяцев» */
export function formatTerm(months: number | null | undefined): string {
  if (!months) return "—";
  return `${months} ${plural(months, MONTH_FORMS)}`;
}

/** Процентная ставка: 19.9 → «19,9%» */
export function formatRate(rate: number): string {
  return `${rate.toLocaleString("ru-RU", { maximumFractionDigits: 2 })}%`;
}

/** ISO → «21.10.2026» */
export function formatDate(iso: string | null | undefined): string {
  if (!iso) return "—";
  return new Date(iso).toLocaleDateString("ru-RU");
}

/** ISO → «21 октября 2026» */
export function formatLongDate(iso: string): string {
  // toLocaleDateString добавляет «г.» в конце — в макетах его нет
  return new Date(iso).toLocaleDateString("ru-RU", { day: "numeric", month: "long", year: "numeric" }).replace(/\s?г\.$/, "");
}

/** ISO → «21 октября, 12:40» */
export function formatDayTime(iso: string): string {
  const date = new Date(iso);
  const day = date.toLocaleDateString("ru-RU", { day: "numeric", month: "long" });
  const time = date.toLocaleTimeString("ru-RU", { hour: "2-digit", minute: "2-digit" });
  return `${day}, ${time}`;
}

/** ISO → «21.10.2026 в 10:42» */
export function formatDateAt(iso: string): string {
  const date = new Date(iso);
  const time = date.toLocaleTimeString("ru-RU", { hour: "2-digit", minute: "2-digit" });
  return `${date.toLocaleDateString("ru-RU")} в ${time}`;
}

/** «2026-04» → «Апрель 2026» */
export function formatPeriod(period: string): string {
  const [year, month] = period.split("-").map(Number);
  const name = new Date(year, month - 1, 1).toLocaleDateString("ru-RU", { month: "long" });
  return `${name[0].toUpperCase()}${name.slice(1)} ${year}`;
}

/** 2516582 → «2,4 МБ» */
export function formatFileSize(bytes: number): string {
  if (bytes < 1024 * 1024) return `${Math.max(1, Math.round(bytes / 1024))} КБ`;
  return `${(bytes / 1024 / 1024).toLocaleString("ru-RU", { maximumFractionDigits: 1 })} МБ`;
}

/**
 * Аннуитетный ежемесячный платёж.
 * Формула: P = S · r · (1 + r)^n / ((1 + r)^n − 1), где r — месячная ставка.
 */
export function annuityPayment(amount: number, termMonths: number, annualRate: number): number {
  if (!amount || !termMonths) return 0;
  const r = annualRate / 100 / 12;
  if (r === 0) return Math.round(amount / termMonths);
  const k = (1 + r) ** termMonths;
  return Math.round((amount * r * k) / (k - 1));
}

/** Оставляет только цифры: «1 300 000 ₽» → «1300000» */
export function digitsOnly(value: string): string {
  return value.replace(/\D/g, "");
}
