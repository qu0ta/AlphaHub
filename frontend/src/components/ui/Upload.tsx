import { FileText, Upload as UploadIcon } from "lucide-react";
import { useId, useRef, useState, type DragEvent, type ReactNode } from "react";
import { formatFileSize } from "../../lib/format";

export const MAX_PDF_BYTES = 10 * 1024 * 1024;

/** Проверка до отправки: формат и размер. Бэкенд проверяет повторно. */
export function pdfError(file: File): string | null {
  const isPdf = file.type === "application/pdf" || file.name.toLowerCase().endsWith(".pdf");
  if (!isPdf) return "Неверный формат: загрузите файл PDF";
  if (file.size > MAX_PDF_BYTES) return `Файл ${formatFileSize(file.size)} — больше 10 МБ`;
  return null;
}

/**
 * Зона загрузки PDF: перетаскивание или выбор файла.
 * Сам input скрыт, кнопка «Выбрать файл» открывает системный диалог.
 */
export function UploadZone({
  title = "Перетащите PDF сюда или выберите файл",
  hint = "Один файл в формате PDF, до 10 МБ",
  error,
  compact = false,
  disabled = false,
  onFile,
}: {
  title?: string;
  hint?: ReactNode;
  error?: string | null;
  compact?: boolean;
  disabled?: boolean;
  onFile: (file: File) => void;
}) {
  const inputId = useId();
  const inputRef = useRef<HTMLInputElement>(null);
  const [dragover, setDragover] = useState(false);

  const onDrop = (event: DragEvent) => {
    event.preventDefault();
    setDragover(false);
    const file = event.dataTransfer.files?.[0];
    if (file && !disabled) onFile(file);
  };

  return (
    <div
      className={`upload ${compact ? "upload--compact" : ""} ${dragover ? "upload--dragover" : ""} ${error ? "upload--error" : ""}`}
      onDragOver={(event) => {
        event.preventDefault();
        if (!disabled) setDragover(true);
      }}
      onDragLeave={(event) => {
        if (!event.currentTarget.contains(event.relatedTarget as Node | null)) setDragover(false);
      }}
      onDrop={onDrop}
    >
      <UploadIcon size={28} className="upload__icon" aria-hidden="true" />
      <p className="upload__title">{title}</p>
      <p className="upload__hint">{hint}</p>
      {error && (
        <p className="upload__error" role="alert">
          {error}
        </p>
      )}
      <label htmlFor={inputId} className="visually-hidden">
        Выбрать PDF
      </label>
      <input
        ref={inputRef}
        id={inputId}
        type="file"
        accept="application/pdf,.pdf"
        className="visually-hidden"
        disabled={disabled}
        onChange={(event) => {
          const file = event.target.files?.[0];
          if (file) onFile(file);
          // Сбрасываем, чтобы можно было выбрать тот же файл ещё раз
          event.target.value = "";
        }}
      />
      <button type="button" className="button button--secondary upload__button" disabled={disabled} onClick={() => inputRef.current?.click()}>
        Выбрать файл
      </button>
    </div>
  );
}

export function FileRow({ name, meta, status, actions, compact = false }: { name: string; meta: ReactNode; status?: ReactNode; actions?: ReactNode; compact?: boolean }) {
  return (
    <div className={`file-row ${compact ? "file-row--compact" : ""}`}>
      <span className="file-row__type" aria-hidden="true">
        <FileText size={20} />
        PDF
      </span>
      <div className="file-row__body">
        <p className="file-row__name" title={name}>
          {name}
        </p>
        <p className="file-row__meta">{meta}</p>
      </div>
      {status}
      {actions}
    </div>
  );
}
