import { X } from "lucide-react";
import { useEffect, useId, useRef, type CSSProperties, type ReactNode } from "react";
import { createPortal } from "react-dom";

/**
 * Модальное окно.
 * createPortal выносит его в конец <body>, чтобы окно не обрезалось
 * родителями с overflow и было поверх всего интерфейса.
 */
export function Modal({ title, children, onClose, width }: { title: ReactNode; children: ReactNode; onClose: () => void; width?: number }) {
  const titleId = useId();
  const dialogRef = useRef<HTMLDivElement>(null);
  // onClose часто приходит новой функцией при каждом рендере. Храним её в ref,
  // чтобы эффект ниже выполнился один раз и не сбивал фокус с полей ввода.
  const onCloseRef = useRef(onClose);
  onCloseRef.current = onClose;

  useEffect(() => {
    const previous = document.activeElement as HTMLElement | null;
    dialogRef.current?.focus();
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") onCloseRef.current();
    };
    document.addEventListener("keydown", onKey);
    // Запрещаем прокрутку страницы под окном
    const overflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.removeEventListener("keydown", onKey);
      document.body.style.overflow = overflow;
      previous?.focus();
    };
  }, []);

  return createPortal(
    <div className="modal-backdrop" onMouseDown={(event) => event.target === event.currentTarget && onClose()}>
      <div
        ref={dialogRef}
        className="modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        tabIndex={-1}
        style={width ? ({ "--modal-width": `${width}px` } as CSSProperties) : undefined}
      >
        <div className="modal__header">
          <h2 className="modal__title" id={titleId}>
            {title}
          </h2>
          <button type="button" className="modal__close" aria-label="Закрыть" onClick={onClose}>
            <X size={20} />
          </button>
        </div>
        {children}
      </div>
    </div>,
    document.body,
  );
}
