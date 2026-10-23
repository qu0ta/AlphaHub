import { useState, type FormEvent } from "react";
import { Link, useLocation, useNavigate } from "react-router-dom";
import { ApiError } from "../../api/client";
import { Button } from "../../components/ui/Button";
import { Alert } from "../../components/ui/Card";
import { PasswordField, TextField } from "../../components/ui/Field";
import { isValidEmail } from "../../lib/validation";
import { useAuth } from "../../state/auth";
import { AuthLayout } from "./AuthLayout";

/** Экран 01 «Вход». */
export function LoginPage() {
  const { login } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const [email, setEmail] = useState((location.state as { email?: string } | null)?.email ?? "");
  const [password, setPassword] = useState("");
  const [touched, setTouched] = useState(false);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  const emailError = touched && !isValidEmail(email) ? "Введите почту в формате name@company.ru" : null;
  const passwordError = touched && !password ? "Введите пароль" : null;

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setTouched(true);
    setError("");
    if (!isValidEmail(email) || !password) return;
    setLoading(true);
    try {
      const user = await login(email.trim(), password);
      navigate(user.role === "BANK_EMPLOYEE" ? "/bank/applications" : "/applications", { replace: true });
    } catch (reason) {
      // 401 при входе — ошибка формы, а не «сессия истекла» (AUTH.md)
      setError(reason instanceof ApiError && reason.status === 401 ? "Неверная почта или пароль" : "Не удалось войти. Проверьте соединение и повторите попытку.");
    } finally {
      setLoading(false);
    }
  };

  return (
    <AuthLayout mode="login" title="С возвращением" subtitle="Войдите в личный кабинет AlphaHub.">
      <form className="stack stack--28" onSubmit={submit} noValidate>
        <div className="stack">
          <TextField
            label="Электронная почта"
            type="email"
            autoComplete="email"
            placeholder="name@company.ru"
            value={email}
            error={emailError}
            onChange={(event) => setEmail(event.target.value)}
          />
          <PasswordField
            label="Пароль"
            autoComplete="current-password"
            placeholder="Введите пароль"
            value={password}
            error={passwordError}
            onChange={(event) => setPassword(event.target.value)}
          />
        </div>
        {error && <Alert>{error}</Alert>}
        <div className="stack stack--20">
          <Button type="submit" block loading={loading}>
            Войти
          </Button>
          <p className="auth__switch">
            Нет аккаунта? <Link to="/register">Зарегистрироваться</Link>
          </p>
        </div>
      </form>
    </AuthLayout>
  );
}
