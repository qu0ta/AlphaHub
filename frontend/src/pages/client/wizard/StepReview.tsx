import { useState } from "react";
import { applicationsApi, type ApplicationPatch } from "../../../api/client";
import { Button } from "../../../components/ui/Button";
import { Alert, Card } from "../../../components/ui/Card";
import { Checkbox } from "../../../components/ui/Checkbox";
import { formatFileSize, formatMoney, formatTerm } from "../../../lib/format";
import { PURPOSE_LABELS } from "../../../lib/labels";
import type { Application, Product } from "../../../types";

/** Экран 12: шаг 4 «Проверка и отправка». После отправки редактирование недоступно. */
export function StepReview({
  app,
  product,
  token,
  onChange,
  flush,
  onEdit,
  onSubmitted,
}: {
  app: Application;
  product: Product;
  token: string | null;
  onChange: (patch: ApplicationPatch) => void;
  flush: () => Promise<void>;
  onEdit: (step: number) => void;
  onSubmitted: (app: Application) => void;
}) {
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");
  const [touched, setTouched] = useState(false);

  const purpose = app.purposeCode === "other" ? `Другое: ${app.purposeComment}` : app.purposeCode ? PURPOSE_LABELS[app.purposeCode] : "—";
  const blocks = [
    { title: "Компания", text: `${app.company?.name} · ИНН ${app.company?.inn}`, step: 1, editLabel: "Изменить" },
    { title: "Кредит", text: `${product.name} · ${formatMoney(app.amount)} · ${formatTerm(app.term)} · ${purpose}`, step: 2, editLabel: "Изменить" },
    {
      title: "Финансы и документ",
      text: `${app.financials?.length ?? 0} месяцев · ${app.document?.name ?? "документ не загружен"}${app.document ? ` · ${formatFileSize(app.document.sizeBytes)}` : ""} · платёж по кредитам ${formatMoney(app.debtPayment)}`,
      step: 3,
      editLabel: "Изменить",
    },
  ];

  const submit = async () => {
    setTouched(true);
    setError("");
    if (!app.dataConfirmed) return;
    setSubmitting(true);
    try {
      // Сначала дожидаемся сохранения последних правок черновика
      await flush();
      onSubmitted(await applicationsApi.submit(token, app.id));
    } catch {
      setError("Не удалось отправить заявку. Проверьте, что все шаги заполнены, и повторите попытку.");
      setSubmitting(false);
    }
  };

  return (
    <div className="stack">
      {blocks.map((block) => (
        <Card compact key={block.title}>
          <div className="card__header">
            <h2 className="review-card__title">{block.title}</h2>
            <button type="button" className="text-link" onClick={() => onEdit(block.step)}>
              {block.editLabel}
            </button>
          </div>
          <p className="muted">{block.text}</p>
        </Card>
      ))}
      <Card compact>
        <Checkbox checked={app.dataConfirmed} onChange={(event) => onChange({ dataConfirmed: event.target.checked })}>
          Подтверждаю корректность указанных данных
        </Checkbox>
        {touched && !app.dataConfirmed && <span className="field__error">Подтвердите, что данные указаны верно</span>}
      </Card>
      {error && <Alert>{error}</Alert>}
      <p className="hint-text">Перед отправкой проверьте все блоки. После отправки редактирование недоступно.</p>
      <div className="actions-row">
        <Button variant="secondary" className="wizard-button wizard-button--back" onClick={() => onEdit(3)}>
          Назад
        </Button>
        <Button className="wizard-button wizard-button--wide" loading={submitting} onClick={() => void submit()}>
          Отправить заявку
        </Button>
      </div>
    </div>
  );
}
