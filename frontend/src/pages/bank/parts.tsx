import { Card, CardHeader, Details } from "../../components/ui/Card";
import { formatDate, formatMoney, formatPeriod, formatTerm } from "../../lib/format";
import { FINANCE_COLUMNS, PURPOSE_LABELS } from "../../lib/labels";
import type { Application } from "../../types";

/** Общие блоки карточки заявки для сотрудника банка (экраны 22 и 23). */

export function purposeText(app: Application) {
  if (!app.purposeCode) return "—";
  return app.purposeCode === "other" ? `Другое: ${app.purposeComment}` : PURPOSE_LABELS[app.purposeCode];
}

export function CompanyCreditCard({ app, productName }: { app: Application; productName: string }) {
  return (
    <Card compact>
      <CardHeader title="Компания и кредит" />
      <Details
        columns={2}
        items={[
          { label: "Организация", value: app.company?.name ?? "—" },
          { label: "ИНН", value: app.company?.inn ?? "—" },
          { label: "Дата регистрации", value: formatDate(app.company?.registrationDate) },
          { label: "ОКВЭД", value: app.company ? `${app.company.okvedCode} · ${app.company.okvedName}` : "—" },
          { label: "Продукт", value: productName },
          { label: "Сумма и срок", value: `${formatMoney(app.amount)} · ${formatTerm(app.term)}` },
          { label: "Цель", value: purposeText(app) },
          { label: "Платёж по текущим кредитам", value: formatMoney(app.debtPayment) },
        ]}
      />
    </Card>
  );
}

export function ScoringCard({ app }: { app: Application }) {
  return (
    <Card compact>
      <h2 className="card__title">Скоринг</h2>
      <p className="score">{app.score !== null ? `${app.score} / 100` : "—"}</p>
      <div className="stack stack--8">
        <p className="details__label">Основные факторы</p>
        <ul className="factors">
          {app.scoreFactors.map((factor) => (
            <li key={factor.text} className={factor.positive ? "factors__plus" : "factors__minus"}>
              {factor.positive ? "+" : "−"} {factor.text}
            </li>
          ))}
        </ul>
      </div>
    </Card>
  );
}

export function FinanceCard({ app }: { app: Application }) {
  const edited = new Set(app.editedCells);
  return (
    <Card compact>
      <CardHeader title="Финансы за 6 месяцев" aside={edited.size > 0 ? <span className="inline-note"><span className="legend-swatch" aria-hidden="true" />исправлено клиентом</span> : undefined} />
      <div className="table-wrap table-wrap--plain">
        <table className="table table--dense">
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
            {(app.financials ?? []).map((row) => (
              <tr key={row.period}>
                <td>{formatPeriod(row.period)}</td>
                {FINANCE_COLUMNS.map((column) => (
                  <td key={column.key} className={`table__num ${edited.has(`${row.period}.${column.key}`) ? "cell--edited" : ""}`}>
                    {formatMoney(row[column.key])}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </Card>
  );
}

export function ChecksCard({ app }: { app: Application }) {
  return (
    <Card compact>
      <h2 className="card__title">Проверки</h2>
      <ul className="checks">
        {app.checks.map((check) => (
          <li key={check.name}>
            <span className={`checks__dot ${check.ok ? "checks__dot--ok" : "checks__dot--warn"}`} aria-hidden="true" />
            {check.name}: {check.result}
          </li>
        ))}
        {app.document && (
          <li>
            <span className="checks__dot checks__dot--ok" aria-hidden="true" />
            PDF: {app.document.name}
          </li>
        )}
        {app.infoRequest?.response && (
          <li>
            <span className="checks__dot checks__dot--ok" aria-hidden="true" />
            Клиент дополнил данные: {app.infoRequest.response.explanation}
            {app.infoRequest.response.fileName ? ` · ${app.infoRequest.response.fileName}` : ""}
          </li>
        )}
      </ul>
    </Card>
  );
}
