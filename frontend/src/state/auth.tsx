import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { ApiError, authApi } from "../api/client";
import type { User } from "../types";

/**
 * Сессия пользователя (Context).
 *
 * Context нужен, потому что пользователь и токен используются почти на всех
 * экранах: в меню (ФИО), в защищённых маршрутах (роль) и в запросах (токен).
 * Передавать их пропсами через все уровни было бы неудобно.
 *
 * Сценарии восстановления сессии взяты из docs/frontend/AUTH.md.
 */
interface AuthContextValue {
  user: User | null;
  token: string | null;
  initializing: boolean;
  sessionError: boolean;
  login(email: string, password: string): Promise<User>;
  register(name: string, email: string, password: string): Promise<User>;
  retrySession(): Promise<void>;
  logout(): void;
}

const AuthContext = createContext<AuthContextValue | null>(null);
const TOKEN_KEY = "alphahub_token";
const USER_KEY = "alphahub_user";

function readUser(): User | null {
  try {
    const raw = sessionStorage.getItem(USER_KEY);
    return raw ? (JSON.parse(raw) as User) : null;
  } catch {
    return null;
  }
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(readUser);
  const [token, setToken] = useState<string | null>(() => sessionStorage.getItem(TOKEN_KEY));
  const [initializing, setInitializing] = useState(() => Boolean(sessionStorage.getItem(TOKEN_KEY)));
  const [sessionError, setSessionError] = useState(false);
  // Номер «версии» сессии: поздний ответ me после выхода не должен вернуть сессию
  const sessionVersion = useRef(0);

  const clear = useCallback(() => {
    setUser(null);
    setToken(null);
    sessionStorage.removeItem(TOKEN_KEY);
    sessionStorage.removeItem(USER_KEY);
  }, []);

  const restoreSession = useCallback(async () => {
    const storedToken = sessionStorage.getItem(TOKEN_KEY);
    if (!storedToken) {
      setInitializing(false);
      return;
    }
    const version = ++sessionVersion.current;
    setInitializing(true);
    setSessionError(false);
    try {
      const nextUser = await authApi.me(storedToken);
      if (version !== sessionVersion.current) return;
      setUser(nextUser);
      sessionStorage.setItem(USER_KEY, JSON.stringify(nextUser));
    } catch (reason) {
      if (version !== sessionVersion.current) return;
      if (reason instanceof ApiError && reason.status === 401) clear();
      else setSessionError(true);
    } finally {
      if (version === sessionVersion.current) setInitializing(false);
    }
  }, [clear]);

  useEffect(() => {
    void restoreSession();
    return () => {
      sessionVersion.current += 1;
    };
  }, [restoreSession]);

  const value = useMemo<AuthContextValue>(
    () => ({
      user,
      token,
      initializing,
      sessionError,
      async login(email, password) {
        const session = await authApi.login(email, password);
        sessionVersion.current += 1;
        setToken(session.token);
        setUser(session.user);
        sessionStorage.setItem(TOKEN_KEY, session.token);
        sessionStorage.setItem(USER_KEY, JSON.stringify(session.user));
        return session.user;
      },
      register: (name, email, password) => authApi.register(name, email, password),
      retrySession: restoreSession,
      logout() {
        sessionVersion.current += 1;
        setSessionError(false);
        clear();
      },
    }),
    [clear, initializing, restoreSession, sessionError, token, user],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const value = useContext(AuthContext);
  if (!value) throw new Error("useAuth must be used within AuthProvider");
  return value;
}
