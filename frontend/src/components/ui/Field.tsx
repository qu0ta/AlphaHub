import { Eye, EyeOff } from "lucide-react";
import { useId, useState, type InputHTMLAttributes, type ReactNode, type TextareaHTMLAttributes } from "react";
import { digitsOnly, formatNumber, MONTH_FORMS, plural } from "../../lib/format";

/**
 * Обёртка поля: подпись сверху, рамка, подсказка или ошибка снизу.
 * Все поля (текст, пароль, сумма, срок) используют её — поэтому выглядят одинаково.
 */
export function FieldShell({
  id,
  label,
  error,
  hint,
  readOnly,
  textarea,
  className = "",
  children,
}: {
  id: string;
  label?: ReactNode;
  error?: string | null;
  hint?: ReactNode;
  readOnly?: boolean;
  textarea?: boolean;
  className?: string;
  children: ReactNode;
}) {
  const controlClass = [
    "field__control",
    error ? "field__control--error" : "",
    readOnly ? "field__control--readonly" : "",
    textarea ? "field__control--textarea" : "",
  ].join(" ");

  return (
    <div className={`field ${className}`}>
      {label && (
        <label className="field__label" htmlFor={id}>
          {label}
        </label>
      )}
      <div className={controlClass}>{children}</div>
      {error ? (
        <span className="field__error" id={`${id}-message`} role="alert">
          {error}
        </span>
      ) : hint ? (
        <span className="field__hint" id={`${id}-message`}>
          {hint}
        </span>
      ) : null}
    </div>
  );
}

type TextFieldProps = Omit<InputHTMLAttributes<HTMLInputElement>, "children"> & {
  label?: ReactNode;
  error?: string | null;
  hint?: ReactNode;
  /** Иконка или текст справа внутри поля */
  end?: ReactNode;
};

export function TextField({ label, error, hint, end, className, readOnly, id, ...props }: TextFieldProps) {
  const autoId = useId();
  const inputId = id ?? autoId;
  return (
    <FieldShell id={inputId} label={label} error={error} hint={hint} readOnly={readOnly} className={className}>
      <input
        id={inputId}
        className="field__input"
        readOnly={readOnly}
        aria-invalid={Boolean(error) || undefined}
        aria-describedby={error || hint ? `${inputId}-message` : undefined}
        {...props}
      />
      {end}
    </FieldShell>
  );
}

export function PasswordField(props: Omit<TextFieldProps, "type" | "end">) {
  const [visible, setVisible] = useState(false);
  return (
    <TextField
      {...props}
      type={visible ? "text" : "password"}
      end={
        <button
          type="button"
          className="field__icon-button"
          aria-label={visible ? "Скрыть пароль" : "Показать пароль"}
          onClick={() => setVisible((value) => !value)}
        >
          {visible ? <Eye size={20} /> : <EyeOff size={20} />}
        </button>
      }
    />
  );
}

type NumberFieldProps = Omit<TextFieldProps, "value" | "onChange" | "end"> & {
  value: number | null;
  onValueChange: (value: number | null) => void;
  /** «₽» или функция, возвращающая окончание («месяцев») */
  suffix?: string | ((value: number) => string);
  maxDigits?: number;
};

/**
 * Числовое поле с разделителями разрядов и подписью справа.
 * Храним число, а показываем «1 300 000 ₽» — так пользователь не ошибётся в нулях.
 */
export function NumberField({ value, onValueChange, suffix, maxDigits = 12, ...props }: NumberFieldProps) {
  const suffixText = typeof suffix === "function" ? (value ? suffix(value) : "") : suffix;
  return (
    <TextField
      {...props}
      inputMode="numeric"
      autoComplete="off"
      value={formatNumber(value)}
      onChange={(event) => {
        const digits = digitsOnly(event.target.value).slice(0, maxDigits);
        onValueChange(digits ? Number(digits) : null);
      }}
      end={suffixText ? <span className="field__suffix">{suffixText}</span> : undefined}
    />
  );
}

export function MoneyField(props: Omit<NumberFieldProps, "suffix">) {
  return <NumberField {...props} suffix="₽" />;
}

export function TermField(props: Omit<NumberFieldProps, "suffix" | "maxDigits">) {
  return <NumberField {...props} maxDigits={3} suffix={(n) => plural(n, MONTH_FORMS)} />;
}

type TextAreaFieldProps = TextareaHTMLAttributes<HTMLTextAreaElement> & {
  label?: ReactNode;
  error?: string | null;
  hint?: ReactNode;
};

export function TextAreaField({ label, error, hint, className, id, ...props }: TextAreaFieldProps) {
  const autoId = useId();
  const inputId = id ?? autoId;
  return (
    <FieldShell id={inputId} label={label} error={error} hint={hint} className={className} textarea>
      <textarea
        id={inputId}
        className="field__input"
        aria-invalid={Boolean(error) || undefined}
        aria-describedby={error || hint ? `${inputId}-message` : undefined}
        {...props}
      />
    </FieldShell>
  );
}
