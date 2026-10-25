import type { ApplicationPatch } from "../../../api/client";
import { Button } from "../../../components/ui/Button";
import { Card, CardHeader } from "../../../components/ui/Card";
import { MoneyField, TermField, TextAreaField, TextField } from "../../../components/ui/Field";
import { Select } from "../../../components/ui/Select";
import { annuityPayment, formatMoney, formatNumber, formatTerm } from "../../../lib/format";
import { PURPOSE_LABELS } from "../../../lib/labels";
import type { Application, Product, PurposeCode } from "../../../types";
import { creditErrors } from "./validation";

const PURPOSE_OPTIONS = (Object.keys(PURPOSE_LABELS) as PurposeCode[]).map((value) => ({ value, label: PURPOSE_LABELS[value] }));

/** Экран 09: шаг 2 «Параметры кредита». */
export function StepCredit({
  app,
  product,
  showErrors,
  onChange,
  onBack,
  onNext,
}: {
  app: Application;
  product: Product;
  showErrors: boolean;
  onChange: (patch: ApplicationPatch) => void;
  onBack: () => void;
  onNext: () => void;
}) {
  const errors = creditErrors(app, product);
  const show = (key: keyof typeof errors) => (showErrors ? errors[key] : null);
  // Ошибку границ показываем сразу, как только значение введено — не дожидаясь «Продолжить»
  const liveAmountError = app.amount ? errors.amount : show("amount");
  const liveTermError = app.term ? errors.term : show("term");
  const payment = !errors.amount && !errors.term ? annuityPayment(app.amount ?? 0, app.term ?? 0, product.rate) : null;

  return (
    <Card>
      <CardHeader title="Параметры кредита" />
      <div className="form-grid">
        <TextField label="Продукт" value={product.name} readOnly />
        <MoneyField
          label="Сумма"
          placeholder={`от ${formatNumber(product.minAmount)}`}
          value={app.amount}
          error={liveAmountError}
          onValueChange={(amount) => onChange({ amount })}
        />
        <TermField label="Срок" placeholder={`${product.minTerm}–${product.maxTerm}`} value={app.term} error={liveTermError} onValueChange={(term) => onChange({ term })} />
        <Select
          label="Цель кредита"
          placeholder="Выберите цель"
          value={app.purposeCode}
          options={PURPOSE_OPTIONS}
          error={show("purpose")}
          onChange={(purposeCode) => onChange({ purposeCode, purposeComment: purposeCode === "other" ? app.purposeComment : "" })}
        />
        {/* Комментарий нужен только для цели «Другое» — в остальных случаях поля нет совсем */}
        {app.purposeCode === "other" && (
          <TextAreaField
            className="form-grid__wide"
            label="Комментарий к цели"
            rows={2}
            maxLength={2000}
            placeholder="Например: ремонт склада"
            value={app.purposeComment}
            error={show("comment")}
            onChange={(event) => onChange({ purposeComment: event.target.value })}
          />
        )}
      </div>

      <div className="estimate">
        <div className="stack stack--8">
          <span className="strong">Предварительный расчёт</span>
          <span className="estimate__value">{payment ? `≈ ${formatMoney(payment)} в месяц` : "Укажите сумму и срок"}</span>
        </div>
        <div className="stack stack--8 estimate__side">
          <span className="muted">
            Сумма: {formatNumber(product.minAmount)}–{formatMoney(product.maxAmount)} · Срок: {product.minTerm}–{formatTerm(product.maxTerm)}
          </span>
          <span className="hint-text">Ставка от {String(product.rate).replace(".", ",")}% годовых. Расчёт не является одобрением кредита.</span>
        </div>
      </div>

      <div className="actions-row">
        <Button variant="secondary" className="wizard-button wizard-button--back" onClick={onBack}>
          Назад
        </Button>
        <Button className="wizard-button" onClick={onNext}>
          Продолжить
        </Button>
      </div>
    </Card>
  );
}
