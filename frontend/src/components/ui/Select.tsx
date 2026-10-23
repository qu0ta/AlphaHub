import { Check, ChevronDown } from "lucide-react";
import { useEffect, useId, useRef, useState, type KeyboardEvent } from "react";
import { FieldShell } from "./Field";

export interface SelectOption<T extends string> {
  value: T;
  label: string;
}

/**
 * Выпадающий список в стиле UI-kit (нативный <select> нельзя так оформить).
 * Поддерживает клавиатуру: стрелки, Enter, Escape — как настоящий список.
 */
export function Select<T extends string>({
  label,
  value,
  options,
  onChange,
  placeholder = "Выберите",
  error,
  className,
}: {
  label?: string;
  value: T | null;
  options: SelectOption<T>[];
  onChange: (value: T) => void;
  placeholder?: string;
  error?: string | null;
  className?: string;
}) {
  const id = useId();
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(0);
  const rootRef = useRef<HTMLDivElement>(null);
  const selected = options.find((option) => option.value === value);

  // Закрываем список кликом мимо него
  useEffect(() => {
    if (!open) return;
    const onDown = (event: MouseEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) setOpen(false);
    };
    document.addEventListener("mousedown", onDown);
    return () => document.removeEventListener("mousedown", onDown);
  }, [open]);

  const openList = () => {
    setActive(Math.max(0, options.findIndex((option) => option.value === value)));
    setOpen(true);
  };

  const choose = (index: number) => {
    onChange(options[index].value);
    setOpen(false);
  };

  const onKeyDown = (event: KeyboardEvent<HTMLButtonElement>) => {
    if (!open && ["ArrowDown", "ArrowUp", "Enter", " "].includes(event.key)) {
      event.preventDefault();
      openList();
      return;
    }
    if (!open) return;
    if (event.key === "ArrowDown") {
      event.preventDefault();
      setActive((index) => Math.min(options.length - 1, index + 1));
    } else if (event.key === "ArrowUp") {
      event.preventDefault();
      setActive((index) => Math.max(0, index - 1));
    } else if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      choose(active);
    } else if (event.key === "Escape" || event.key === "Tab") {
      setOpen(false);
    }
  };

  return (
    <div ref={rootRef} className={`select ${open ? "select--open" : ""} ${className ?? ""}`}>
      <FieldShell id={id} label={label} error={error}>
        <button
          id={id}
          type="button"
          className="select__trigger field__input"
          aria-haspopup="listbox"
          aria-expanded={open}
          aria-controls={`${id}-list`}
          onClick={() => (open ? setOpen(false) : openList())}
          onKeyDown={onKeyDown}
        >
          <span className={`select__value ${selected ? "" : "select__value--placeholder"}`}>{selected?.label ?? placeholder}</span>
          <ChevronDown size={20} className="select__chevron" aria-hidden="true" />
        </button>
      </FieldShell>
      {open && (
        <ul className="select__list" id={`${id}-list`} role="listbox" aria-labelledby={id}>
          {options.map((option, index) => {
            const isSelected = option.value === value;
            return (
              <li
                key={option.value}
                role="option"
                aria-selected={isSelected}
                className={`select__option ${isSelected ? "select__option--selected" : ""} ${index === active && !isSelected ? "select__option--active" : ""}`}
                onMouseEnter={() => setActive(index)}
                onMouseDown={(event) => {
                  event.preventDefault();
                  choose(index);
                }}
              >
                {option.label}
                {isSelected && <Check size={16} aria-hidden="true" />}
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}
