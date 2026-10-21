/**
 * Проверки из словаря данных (Miro). Бэкенд повторяет все эти проверки,
 * фронтенд проверяет заранее, чтобы пользователь сразу видел ошибку.
 */

const INN10_WEIGHTS = [2, 4, 10, 3, 5, 9, 4, 6, 8];
const INN11_WEIGHTS = [7, 2, 4, 10, 3, 5, 9, 4, 6, 8];
const INN12_WEIGHTS = [3, 7, 2, 4, 10, 3, 5, 9, 4, 6, 8];

function checkDigit(digits: number[], weights: number[]): number {
  const sum = weights.reduce((acc, weight, index) => acc + weight * digits[index], 0);
  return (sum % 11) % 10;
}

/** Контрольная сумма ИНН: 10 цифр — организация (ООО), 12 цифр — ИП. */
export function isValidInn(inn: string): boolean {
  if (!/^\d{10}$|^\d{12}$/.test(inn)) return false;
  const digits = inn.split("").map(Number);
  if (digits.length === 10) return checkDigit(digits, INN10_WEIGHTS) === digits[9];
  return checkDigit(digits, INN11_WEIGHTS) === digits[10] && checkDigit(digits, INN12_WEIGHTS) === digits[11];
}

/** Текст ошибки для поля ИНН или null, если всё в порядке. */
export function innError(inn: string): string | null {
  if (!inn) return "Укажите ИНН";
  if (inn.length !== 10 && inn.length !== 12) return "Введите 10 цифр для ООО или 12 для ИП";
  if (!isValidInn(inn)) return "ИНН не прошёл проверку контрольной суммы";
  return null;
}

export function isValidEmail(email: string): boolean {
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.trim());
}

/** Длина пароля в байтах UTF-8: бэкенд ограничивает 8–72 байтами (AUTH.md). */
export function utf8Length(value: string): number {
  return new TextEncoder().encode(value).length;
}

export type PasswordStrength = { score: 0 | 1 | 2 | 3 | 4; label: string };

/**
 * Индикатор надёжности пароля. Пользователю показываем шкалу,
 * а не счётчик байтов — так решили в обсуждении с Codex.
 */
export function passwordStrength(password: string): PasswordStrength {
  if (!password) return { score: 0, label: "" };
  if (password.length < 8) return { score: 1, label: "Слишком короткий пароль" };
  let points = 0;
  if (/[a-zа-яё]/.test(password) && /[A-ZА-ЯЁ]/.test(password)) points += 1;
  if (/\d/.test(password)) points += 1;
  if (/[^\p{L}\p{N}]/u.test(password)) points += 1;
  if (password.length >= 12) points += 1;
  if (points <= 1) return { score: 2, label: "Средний пароль" };
  if (points === 2) return { score: 3, label: "Хороший пароль" };
  return { score: 4, label: "Надёжный пароль" };
}

export function passwordError(password: string): string | null {
  if (!password) return "Придумайте пароль";
  if (password.length < 8) return "Минимум 8 символов";
  if (utf8Length(password) > 72) return "Пароль слишком длинный";
  return null;
}
