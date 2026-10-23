/**
 * Куда вести пользователя сразу после входа.
 * После регистрации — на заполнение компании (экран 03). Флаг ставит RegisterPage,
 * снимает экран компании. Без флага маршрут /register перенаправил бы
 * на «Мои заявки» раньше, чем страница успеет вызвать navigate.
 *
 * Константа вынесена из App.tsx: файлы с компонентами должны экспортировать
 * только компоненты, иначе ломается горячая перезагрузка (Fast Refresh) в Vite.
 */
export const AFTER_LOGIN_KEY = "alphahub_after_login";

export function setAfterLogin(path: string) {
  sessionStorage.setItem(AFTER_LOGIN_KEY, path);
}

export function clearAfterLogin() {
  sessionStorage.removeItem(AFTER_LOGIN_KEY);
}

export function getAfterLogin(): string | null {
  return sessionStorage.getItem(AFTER_LOGIN_KEY);
}
