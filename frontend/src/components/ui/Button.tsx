import { LoaderCircle } from "lucide-react";
import type { ButtonHTMLAttributes } from "react";

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: "primary" | "secondary" | "ghost";
  block?: boolean;
  /** Показывает спиннер и блокирует повторное нажатие, пока идёт запрос */
  loading?: boolean;
};

export function Button({ variant = "primary", block = false, loading = false, className = "", children, disabled, type = "button", ...props }: ButtonProps) {
  return (
    <button
      type={type}
      className={`button button--${variant} ${block ? "button--block" : ""} ${className}`}
      disabled={disabled || loading}
      aria-busy={loading || undefined}
      {...props}
    >
      {loading && <LoaderCircle size={18} className="spin" aria-hidden="true" />}
      {children}
    </button>
  );
}
