import { useState, type FormEvent, type ReactNode } from "react";
import { Link, useNavigate } from "react-router-dom";
import { ApiError } from "../../api/client";
import { Button } from "../../components/ui/Button";
import { Alert } from "../../components/ui/Card";
import { Checkbox } from "../../components/ui/Checkbox";
import { PasswordField, TextField } from "../../components/ui/Field";
import { clearAfterLogin, setAfterLogin } from "../../lib/session";
import { isValidEmail, passwordError, passwordStrength } from "../../lib/validation";
import { useAuth } from "../../state/auth";
import { AuthLayout } from "./AuthLayout";

type Form = { name: string; email: string; password: string; repeat: string; consent: boolean };

function validate(form: Form) {
  return {
    name: form.name.trim().split(/\s+/).length < 2 ? "Укажите фамилию и имя" : null,
    email: !isValidEmail(form.email) ? "Введите почту в формате name@company.ru" : null,
    password: passwordError(form.password),
    repeat: form.repeat !== form.password ? "Пароли не совпадают" : null,
    consent: !form.consent ? "Нужно согласие на обработку персональных данных" : null,
  };
}

/** Экран 02 «Регистрация». После регистрации — вход и заполнение компании (экран 03). */
export function RegisterPage() {
  const { register, login } = useAuth();
  const navigate = useNavigate();
  const [form, setForm] = useState<Form>({ name: "", email: "", password: "", repeat: "", consent: false });
  const [touched, setTouched] = useState(false);
  const [error, setError] = useState<ReactNode>("");
  const [loading, setLoading] = useState(false);

  const errors = validate(form);
  const show = (field: keyof Form) => (touched ? errors[field] : null);
  const strength = passwordStrength(form.password);
  const strengthTone = strength.score <= 1 ? "weak" : strength.score === 2 ? "medium" : "strong";

  const set = <K extends keyof Form>(key: K, value: Form[K]) => setForm((current) => ({ ...current, [key]: value }));

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setTouched(true);
    setError("");
    if (Object.values(errors).some(Boolean)) return;

    setLoading(true);
    let created = false;
    try {
      await register(form.name.trim(), form.email.trim(), form.password);
      created = true;
      // После входа ведём на заполнение компании (см. lib/session.ts)
      setAfterLogin("/onboarding/company");
      // Регистрация не выдаёт токен — отдельно входим (AUTH.md)
      await login(form.email.trim(), form.password);
      navigate("/onboarding/company", { replace: true });
    } catch (reason) {
      clearAfterLogin();
      if (created) {
        setError("Аккаунт создан, но войти автоматически не удалось. Войдите вручную.");
      } else if (reason instanceof ApiError && reason.status === 409) {
        setError(
          <>
            Эта почта уже зарегистрирована. <Link to="/login" state={{ email: form.email }}>Войти</Link>
          </>,
        );
      } else {
        setError("Не удалось создать аккаунт. Повторите попытку.");
      }
    } finally {
      setLoading(false);
    }
  };

  return (
    <AuthLayout mode="register" title="Создать аккаунт" subtitle="Начните с ваших контактных данных.">
      <form className="stack stack--28" onSubmit={submit} noValidate>
        <div className="stack">
          <TextField label="ФИО" autoComplete="name" placeholder="Петров Иван Сергеевич" maxLength={255} value={form.name} error={show("name")} onChange={(e) => set("name", e.target.value)} />
          <TextField
            label="Электронная почта"
            type="email"
            autoComplete="email"
            placeholder="name@company.ru"
            maxLength={255}
            value={form.email}
            error={show("email")}
            onChange={(e) => set("email", e.target.value)}
          />
          <div className="stack stack--8">
            <PasswordField label="Пароль" autoComplete="new-password" placeholder="Минимум 8 символов" value={form.password} error={show("password")} onChange={(e) => set("password", e.target.value)} />
            {form.password && (
              <div className={`strength strength--${strengthTone}`} aria-live="polite">
                <span className="strength__scale" aria-hidden="true">
                  {[1, 2, 3, 4].map((part) => (
                    <i key={part} className={part <= strength.score ? "strength__part--on" : ""} />
                  ))}
                </span>
                <span>{strength.label}</span>
              </div>
            )}
          </div>
          <PasswordField label="Повтор пароля" autoComplete="new-password" value={form.repeat} error={show("repeat")} onChange={(e) => set("repeat", e.target.value)} />
        </div>
        <div className="stack stack--8">
          <Checkbox checked={form.consent} onChange={(e) => set("consent", e.target.checked)}>
            Согласие на обработку персональных данных
          </Checkbox>
          {show("consent") && (
            <span className="field__error" role="alert">
              {errors.consent}
            </span>
          )}
        </div>
        {error && <Alert>{error}</Alert>}
        <div className="stack stack--20">
          <Button type="submit" block loading={loading}>
            Создать аккаунт
          </Button>
          <p className="auth__switch">
            Уже есть аккаунт? <Link to="/login">Войти</Link>
          </p>
        </div>
      </form>
    </AuthLayout>
  );
}
