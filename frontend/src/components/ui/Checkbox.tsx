import { Check } from "lucide-react";
import type { InputHTMLAttributes, ReactNode } from "react";

export function Checkbox({ children, ...props }: Omit<InputHTMLAttributes<HTMLInputElement>, "type"> & { children: ReactNode }) {
  return (
    <label className="checkbox">
      <input type="checkbox" className="checkbox__input" {...props} />
      <span className="checkbox__box" aria-hidden="true">
        <Check size={14} strokeWidth={3} />
      </span>
      <span className="checkbox__label">{children}</span>
    </label>
  );
}
