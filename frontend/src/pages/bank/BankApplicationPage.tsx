import { useNavigate, useParams } from "react-router-dom";
import { bankApi } from "../../api/client";
import { Page } from "../../components/layout/Page";
import { StatusBadge } from "../../components/ui/Badge";
import { Button } from "../../components/ui/Button";
import { Card, StateMessage } from "../../components/ui/Card";
import { Timeline } from "../../components/ui/Timeline";
import { usePolling, useResource } from "../../hooks/useResource";
import { formatDateAt, formatMoney, formatRate, formatTerm } from "../../lib/format";
import { useProducts } from "../../state/products";
import { ChecksCard, CompanyCreditCard, FinanceCard, ScoringCard } from "./parts";

const DECISION_LABELS = { APPROVE: "Одобрено", REJECT: "Отказ", REQUEST_DATA: "Запрошены данные" } as const;

/** Экран 22 «Банк — карточка заявки». */
export function BankApplicationPage() {
  const { id = "" } = useParams();
  const navigate = useNavigate();
  const { products } = useProducts();
  const resource = useResource(() => bankApi.get(id), [id]);
  const app = resource.data;
  usePolling(() => void resource.reload({ silent: true }), Boolean(app && ["SUBMITTED", "IN_REVIEW", "RETENTION_REVIEW", "SIGNED"].includes(app.status)), 3000);

  const crumbs = [{ label: "Очередь заявок", to: "/bank/applications" }, { label: app ? `Заявка ${app.number}` : "Заявка" }];

  if (resource.loading && !app) {
    return (
      <Page crumbs={crumbs} title="Заявка">
        <Card>Загружаем заявку…</Card>
      </Page>
    );
  }
  if (resource.error || !app) {
    return (
      <Page crumbs={crumbs} title="Заявка">
        <Card>
          <StateMessage title="Не удалось загрузить заявку" action={<Button onClick={() => void resource.reload()}>Повторить</Button>} />
        </Card>
      </Page>
    );
  }

  const productName = products.find((item) => item.code === app.productCode)?.name ?? "Кредит";
  const lastOffer = app.offers.at(-1);

  return (
    <Page
      crumbs={crumbs}
      title={`Заявка ${app.number}`}
      subtitle={`${app.company?.name} · ИНН ${app.company?.inn} · ${productName}`}
      actions={<StatusBadge status={app.status} audience="bank" />}
    >
      <div className="columns">
        <CompanyCreditCard app={app} productName={productName} />
        <ScoringCard app={app} />
      </div>

      <FinanceCard app={app} />

      <div className="grid-2">
        <ChecksCard app={app} />
        <Card compact>
          <h2 className="card__title">Решение</h2>
          {app.status === "MANUAL" ? (
            <>
              <p className="muted">Заявка ждёт решения. Добавьте комментарий и выберите действие: одобрить, отказать или запросить данные.</p>
              <div>
                <Button onClick={() => navigate(`/bank/applications/${app.id}/decision`)}>Перейти к решению</Button>
              </div>
            </>
          ) : app.decision ? (
            <div className="stack stack--8">
              <p className="strong">
                {DECISION_LABELS[app.decision.type]} · {formatDateAt(app.decision.at)}
              </p>
              <p className="muted">«{app.decision.comment}» — {app.decision.employeeName}</p>
              {app.decision.type === "APPROVE" && lastOffer && (
                <p className="muted">
                  Последнее предложение: {formatMoney(lastOffer.amount)} · {formatTerm(lastOffer.term)} · {formatRate(lastOffer.rate)}
                </p>
              )}
            </div>
          ) : app.status === "SUBMITTED" || app.status === "IN_REVIEW" ? (
            <p className="muted">Идёт автоматическая проверка. Если балл окажется между порогами, заявка придёт на ручное решение.</p>
          ) : (
            <p className="muted">Решение приняла автоматическая система — ручная проверка не потребовалась.</p>
          )}
        </Card>
      </div>

      <Card compact>
        <h2 className="card__title">История заявки</h2>
        <Timeline events={app.history} />
      </Card>
    </Page>
  );
}
