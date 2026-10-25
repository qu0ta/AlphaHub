import { Building2, Pencil } from "lucide-react";
import { Link, Navigate, useNavigate } from "react-router-dom";
import { companyApi } from "../../api/client";
import { Page } from "../../components/layout/Page";
import { Badge } from "../../components/ui/Badge";
import { Button } from "../../components/ui/Button";
import { Card, Details, Divider, InfoBlock, StateMessage } from "../../components/ui/Card";
import { useResource } from "../../hooks/useResource";
import { formatDate } from "../../lib/format";
import { BUSINESS_TYPE_FULL, positionText } from "../../lib/labels";
import { useAuth } from "../../state/auth";

/**
 * Экраны 04 и 05 «Компания».
 * Документы прикладываются к конкретной заявке (финансовая сводка на шаге 3 мастера),
 * отдельного хранилища документов компании на бэкенде нет.
 */
export function CompanyPage() {
  const { token } = useAuth();
  const navigate = useNavigate();
  const company = useResource(() => companyApi.get(token), [token]);

  if (company.loading) return <Page crumbs={[{ label: "Компания" }]} title="Компания"><Card>Загружаем данные компании…</Card></Page>;
  if (company.error) {
    return (
      <Page crumbs={[{ label: "Компания" }]} title="Компания">
        <Card>
          <StateMessage title="Не удалось загрузить данные" text="Проверьте соединение и повторите попытку." action={<Button onClick={() => void company.reload()}>Повторить</Button>} />
        </Card>
      </Page>
    );
  }
  // Компании ещё нет — сначала первичное заполнение
  if (!company.data) return <Navigate to="/onboarding/company" replace />;

  const data = company.data;

  return (
    <Page crumbs={[{ label: "Компания" }]} title="Компания" subtitle="Реквизиты компании подставляются во все ваши заявки.">
      <Card>
        <div className="company-head">
          <span className="icon-tile" aria-hidden="true">
            <Building2 size={24} />
          </span>
          <div className="company-head__text">
            <p className="company-head__name">{data.name}</p>
            <p className="muted">{BUSINESS_TYPE_FULL[data.businessType]}</p>
          </div>
          <Badge tone="success">Данные из реестра</Badge>
          <Link to="/company/edit" className="text-link">
            <Pencil size={16} aria-hidden="true" />
            Изменить
          </Link>
        </div>
        <Divider />
        <Details
          items={[
            { label: "ИНН", value: data.inn },
            { label: "Дата регистрации", value: formatDate(data.registrationDate) },
            { label: "Основной ОКВЭД", value: `${data.okvedCode} · ${data.okvedName}` },
            { label: "Представитель", value: data.representativeName },
            { label: "Должность", value: positionText(data.representativePosition, data.representativePositionOther) },
            { label: "Источник данных", value: `${data.source === "EGRUL" ? "ЕГРЮЛ" : "ЕГРИП"} · Только для чтения` },
          ]}
        />
      </Card>

      <InfoBlock title="Финансовые документы">
        Финансовая сводка за 6 месяцев загружается в каждой заявке на шаге «Финансы и документы» — так банк видит актуальные данные.
      </InfoBlock>
      <div>
        <Button onClick={() => navigate("/products")}>Выбрать кредит</Button>
      </div>
    </Page>
  );
}
