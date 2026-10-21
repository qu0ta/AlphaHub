import { Check } from "lucide-react";
import { Fragment } from "react";

/**
 * Индикатор шагов мастера заявки.
 * Пройденные шаги кликабельны — можно вернуться и поправить данные.
 */
export function Stepper({ steps, current, onSelect }: { steps: string[]; current: number; onSelect?: (step: number) => void }) {
  return (
    <ol className="stepper" aria-label="Шаги заявки">
      {steps.map((label, index) => {
        const step = index + 1;
        const done = step < current;
        const state = done ? "done" : step === current ? "current" : "pending";
        const content = (
          <>
            <span className="stepper__marker">{done ? <Check size={14} strokeWidth={3} /> : step}</span>
            <span>{label}</span>
          </>
        );
        return (
          <Fragment key={label}>
            <li className={`stepper__item stepper__item--${state}`} aria-current={step === current ? "step" : undefined}>
              {done && onSelect ? (
                <button type="button" className="stepper__link" onClick={() => onSelect(step)}>
                  {content}
                </button>
              ) : (
                content
              )}
            </li>
            {index < steps.length - 1 && <li className={`stepper__line ${done ? "stepper__line--done" : ""}`} aria-hidden="true" />}
          </Fragment>
        );
      })}
    </ol>
  );
}
