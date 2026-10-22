import { LoaderCircle, Trash2 } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { ApiError, applicationsApi, type ApplicationPatch } from "../../../api/client";
import { Badge } from "../../../components/ui/Badge";
import { Button } from "../../../components/ui/Button";
import { Alert, Card, CardHeader, InfoBlock } from "../../../components/ui/Card";
import { MoneyField } from "../../../components/ui/Field";
import { FileRow, pdfError, UploadZone } from "../../../components/ui/Upload";
import { digitsOnly, formatFileSize, formatNumber, formatPeriod } from "../../../lib/format";
import { FINANCE_COLUMNS } from "../../../lib/labels";
import type { Application, FinanceField, FinanceMonth } from "../../../types";
import { financeErrors } from "./validation";

/** Экраны 10 и 11: шаг 3 «Финансы и документы» — до и после загрузки PDF. */
export function StepFinance({
  app,
  token,
  showErrors,
  onChange,
  onReplace,
  onBack,
  onNext,
}: {
  app: Application;
  token: string | null;
  showErrors: boolean;
  onChange: (patch: ApplicationPatch) => void;
  onReplace: (app: Application) => void;
  onBack: () => void;
  onNext: () => void;
}) {
  const [uploadError, setUploadError] = useState<string | null>(null);
  const [uploading, setUploading] = useState(false);
  const errors = financeErrors(app);
  const processing = app.document?.status === "PROCESSING";

  // onReplace — новая функция при каждом рендере; храним в ref, чтобы не
  // перезапускать таймер опроса на каждое нажатие клавиши
  const replaceRef = useRef(onReplace);
  replaceRef.current = onReplace;

  // Пока документ распознаётся — раз в секунду спрашиваем результат
  useEffect(() => {
    if (!processing) return;
    const timer = window.setInterval(async () => {
      try {
        const fresh = await applicationsApi.get(token, app.id);
        if (fresh.document?.status !== "PROCESSING") replaceRef.current(fresh);
      } catch {
        // временная ошибка — спросим ещё раз на следующем тике
      }
    }, 1000);
    return () => window.clearInterval(timer);
  }, [app.id, processing, token]);

  const upload = async (file: File) => {
    const error = pdfError(file);
    setUploadError(error);
    if (error) return;
    setUploading(true);
    try {
      onReplace(await applicationsApi.uploadFinancialDocument(token, app.id, file));
    } catch (reason) {
      setUploadError(reason instanceof ApiError ? reason.serverText : "Не удалось загрузить файл");
    } finally {
      setUploading(false);
    }
  };

  const remove = async () => {
    onReplace(await applicationsApi.removeFinancialDocument(token, app.id));
  };

  const updateCell = (period: string, field: FinanceField, value: number | null) => {
    const financials = (app.financials ?? []).map((row): FinanceMonth => (row.period === period ? { ...row, [field]: value } : row));
    const key = `${period}.${field}`;
    const editedCells = app.editedCells.includes(key) ? app.editedCells : [...app.editedCells, key];
    onChange({ financials, editedCells });
  };

  const doc = app.document;

  return (
    <Card>
      <CardHeader title="Финансы и документы" subtitle="Загрузите финансовую сводку за 6 полных месяцев." />

      {!doc ? (
        <UploadZone
          hint="Один обязательный текстовый PDF, до 10 МБ"
          title={uploading ? "Загружаем файл…" : undefined}
          disabled={uploading}
          error={uploadError ?? (showErrors ? errors.document : null)}
          onFile={(file) => void upload(file)}
        />
      ) : (
        <FileRow
          compact
          name={doc.name}
          meta={formatFileSize(doc.sizeBytes)}
          status={
            doc.status === "PROCESSING" ? (
              <Badge tone="warning">Обрабатывается</Badge>
            ) : doc.status === "ERROR" ? (
              <Badge tone="danger">Не распознан</Badge>
            ) : (
              <Badge tone="success">Данные извлечены</Badge>
            )
          }
          actions={
            !processing && (
              <button type="button" className="text-link" onClick={() => void remove()}>
                <Trash2 size={16} aria-hidden="true" />
                Заменить
              </button>
            )
          }
        />
      )}

      {processing && (
        <span className="inline-note">
          <LoaderCircle size={16} className="spin" aria-hidden="true" />
          Извлекаем показатели из документа. Обычно это несколько секунд.
        </span>
      )}

      {doc?.status === "ERROR" && <Alert>{doc.error}</Alert>}

      {app.financials && (
        <FinanceTable rows={app.financials} edited={app.editedCells} missing={showErrors ? errors.missingCells : []} onCell={updateCell} />
      )}
      {showErrors && errors.financials && app.financials && <p className="field__error">{errors.financials}</p>}

      <div className="debt-row">
        <MoneyField
          className="debt-row__field"
          label="Текущий ежемесячный платёж по действующим кредитам"
          placeholder="0"
          value={app.debtPayment}
          error={showErrors ? errors.debt : null}
          hint="Если кредитов нет — укажите 0 ₽"
          onValueChange={(debtPayment) => onChange({ debtPayment })}
        />
        {app.financials && <span className="hint-text">Все извлечённые значения можно исправить.</span>}
      </div>

      {!app.financials && (
        <InfoBlock accent title="После загрузки система извлечёт показатели">
          Их можно будет проверить и исправить. Документ проверяется на формат и размер, совпадение ИНН и отчётный период.
        </InfoBlock>
      )}

      <div className="actions-row">
        <Button variant="secondary" className="wizard-button wizard-button--back" onClick={onBack}>
          Назад
        </Button>
        <Button className="wizard-button wizard-button--wide" disabled={processing} onClick={onNext}>
          {app.financials ? "Подтвердить данные" : "Продолжить"}
        </Button>
      </div>
    </Card>
  );
}

/**
 * Редактируемая таблица финансов: каждая ячейка — поле ввода.
 * Исправленные клиентом значения подсвечены — банк видит, что меняли
 * (в словаре данных: «источник значения и исправления сохраняются»).
 */
function FinanceTable({
  rows,
  edited,
  missing,
  onCell,
}: {
  rows: FinanceMonth[];
  edited: string[];
  missing: string[];
  onCell: (period: string, field: FinanceField, value: number | null) => void;
}) {
  return (
    <div className="stack stack--8">
      <div className="table-wrap">
        <table className="table table--dense finance-table">
          <thead>
            <tr>
              <th>Месяц</th>
              {FINANCE_COLUMNS.map((column) => (
                <th key={column.key} className="table__num">
                  {column.label}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr key={row.period}>
                <td className="finance-table__month">{formatPeriod(row.period)}</td>
                {FINANCE_COLUMNS.map((column) => {
                  const key = `${row.period}.${column.key}`;
                  const isMissing = missing.includes(key);
                  return (
                    <td key={column.key}>
                      <label className={`cell-input ${edited.includes(key) ? "cell-input--edited" : ""} ${isMissing ? "cell-input--error" : ""}`}>
                        <span className="visually-hidden">
                          {column.label}, {formatPeriod(row.period)}
                        </span>
                        <input
                          inputMode="numeric"
                          value={formatNumber(row[column.key])}
                          placeholder="—"
                          aria-invalid={isMissing || undefined}
                          onChange={(event) => {
                            const digits = digitsOnly(event.target.value).slice(0, 12);
                            onCell(row.period, column.key, digits ? Number(digits) : null);
                          }}
                        />
                        <span aria-hidden="true">₽</span>
                      </label>
                    </td>
                  );
                })}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {edited.length > 0 && (
        <span className="inline-note">
          <span className="legend-swatch" aria-hidden="true" />
          Исправлено вами — банк увидит, что значение изменено вручную
        </span>
      )}
    </div>
  );
}
