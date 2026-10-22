import { Link } from "react-router-dom";
import { Button } from "../../../components/ui/Button";
import { Card, CardHeader } from "../../../components/ui/Card";
import { TextField } from "../../../components/ui/Field";
import { formatDate } from "../../../lib/format";
import { BUSINESS_TYPE_SHORT, positionText } from "../../../lib/labels";
import type { Application } from "../../../types";

/** Экран 08: шаг 1 «Компания» — данные из профиля, только чтение. */
export function StepCompany({ app, onNext }: { app: Application; onNext: () => void }) {
  const company = app.company;
  return (
    <Card>
      <CardHeader title="Данные компании" subtitle="Данные подставлены из профиля и недоступны для изменения здесь." />
      {company ? (
        <div className="form-grid">
          <TextField label="ИНН" value={company.inn} readOnly />
          <TextField label="Организация" value={company.name} readOnly />
          <TextField label="Форма бизнеса" value={BUSINESS_TYPE_SHORT[company.businessType]} readOnly />
          <TextField label="Дата регистрации" value={formatDate(company.registrationDate)} readOnly />
          <TextField className="form-grid__wide" label="Основной ОКВЭД" value={`${company.okvedCode} — ${company.okvedName}`} readOnly />
          <TextField label="Представитель" value={company.representativeName} readOnly />
          <TextField label="Должность" value={positionText(company.representativePosition, company.representativePositionOther)} readOnly />
        </div>
      ) : (
        <p className="field__error">Данные компании не найдены. Заполните их в разделе «Компания и документы».</p>
      )}
      <p>
        Изменить данные можно в разделе{" "}
        <Link className="text-link" to={`/company/edit?next=${encodeURIComponent(`/applications/${app.id}/edit?step=1`)}`}>
          «Компания и документы»
        </Link>
        .
      </p>
      <div className="actions-row">
        <Button className="wizard-button" disabled={!company} onClick={onNext}>
          Продолжить
        </Button>
      </div>
    </Card>
  );
}
