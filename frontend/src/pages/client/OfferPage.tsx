import { useState } from "react";
import { Navigate, useNavigate, useParams } from "react-router-dom";
import { applicationsApi } from "../../api/client";
import { Page } from "../../components/layout/Page";
import { Badge } from "../../components/ui/Badge";
import { Button } from "../../components/ui/Button";
import { Alert, Card, InfoBlock, StateMessage } from "../../components/ui/Card";
import { TextAreaField } from "../../components/ui/Field";
import { Modal } from "../../components/ui/Modal";
import { useResource } from "../../hooks/useResource";
import { atLeast } from "../../lib/timing";
import { formatLongDate, formatMoney, formatRate, formatTerm } from "../../lib/format";
import { DECLINE_REASON_LABELS } from "../../lib/labels";
import { useAuth } from "../../state/auth";
import { useProducts } from "../../state/products";
import type { DeclineReason, Offer } from "../../types";

/** Экраны 16 «Предложение банка» и 17 «Отклонение предложения». */
export function OfferPage() {
  const { id = "" } = useParams();
  const { token } = useAuth();
  const navigate = useNavigate();
  const { products } = useProducts();
  const resource = useResource(() => applicationsApi.get(token, id), [token, id]);
  const [declineOpen, setDeclineOpen] = useState(false);
  const [accepting, setAccepting] = useState(false);
  const [error, setError] = useState("");

  const app = resource.data;
  const crumbs = [{ label: "Мои заявки", to: "/applications" }, { label: app ? `Заявка № ${app.number}` : "Заявка", to: app ? `/applications/${app.id}` : undefined }, { label: "Предложение банка" }];

  if (resource.loading) {
    return (
      <Page crumbs={crumbs} title="Предложение банка">
        <Card>Загружаем предложение…</Card>
      </Page>
    );
  }
  if (resource.error || !app) {
    return (
      <Page crumbs={crumbs} title="Предложение банка">
        <Card>
          <StateMessage title="Не удалось загрузить предложение" action={<Button onClick={() => void resource.reload()}>Повторить</Button>} />
        </Card>
      </Page>
    );
  }

  const offer = app.offers.find((item) => item.status === "ACTIVE");
  // Предложение уже принято или отклонено — показываем статус заявки
  if (!offer || (app.status !== "OFFER" && app.status !== "RETENTION_OFFER")) return <Navigate to={`/applications/${app.id}`} replace />;

  const previous = app.offers.filter((item) => item.status === "DECLINED").at(-1);
  const productName = products.find((item) => item.code === app.productCode)?.name ?? "Кредит";

  const accept = async () => {
    setAccepting(true);
    setError("");
    try {
      await atLeast(applicationsApi.acceptOffer(token, app.id), 1000);
      navigate(`/applications/${app.id}`, { replace: true });
    } catch {
      setError("Не удалось принять предложение. Повторите попытку.");
      setAccepting(false);
    }
  };

  return (
    <Page crumbs={crumbs} title={previous ? "Новое предложение банка" : "Предложение банка"} subtitle={`Заявка № ${app.number} · действует до ${formatLongDate(offer.validUntil)}`}>
      <Card>
        <div className="card__header">
          <p className="offer__product">{productName}</p>
          {previous && <Badge tone="success">Условия улучшены</Badge>}
        </div>
        <p className="offer__amount">
          {formatMoney(offer.amount)}
          {previous && previous.amount !== offer.amount && <s className="offer__old">{formatMoney(previous.amount)}</s>}
        </p>
        <dl className="offer__grid">
          <OfferValue label="Срок" value={formatTerm(offer.term)} old={previous && previous.term !== offer.term ? formatTerm(previous.term) : null} />
          <OfferValue label="Ставка" value={`${formatRate(offer.rate)} годовых`} old={previous && previous.rate !== offer.rate ? formatRate(previous.rate) : null} />
          <OfferValue
            label="Ежемесячный платёж"
            value={formatMoney(offer.monthlyPayment)}
            old={previous && previous.monthlyPayment !== offer.monthlyPayment ? formatMoney(previous.monthlyPayment) : null}
          />
          <OfferValue label="Выдача" value="После подписания" />
        </dl>
        {/* Комментарий сотрудника к одобрению — только при одобрении, не путается с запросом данных */}
        {app.decision?.type === "APPROVE" && app.decision.comment && <InfoBlock title="Комментарий банка">{app.decision.comment}</InfoBlock>}
        <InfoBlock>Это итоговое предложение банка. После принятия запустится симуляция подписания и выдачи.</InfoBlock>
        {error && <Alert>{error}</Alert>}
        <div className="actions-row">
          <Button variant="secondary" className="offer__button" onClick={() => setDeclineOpen(true)}>
            Отклонить
          </Button>
          <Button className="offer__button offer__button--wide" loading={accepting} onClick={() => void accept()}>
            Принять предложение
          </Button>
        </div>
      </Card>

      {declineOpen && (
        <DeclineModal
          offer={offer}
          onClose={() => setDeclineOpen(false)}
          onDecline={async (reason, comment) => {
            await atLeast(applicationsApi.declineOffer(token, app.id, reason, comment), 1000);
            navigate(`/applications/${app.id}`, { replace: true });
          }}
        />
      )}
    </Page>
  );
}

function OfferValue({ label, value, old }: { label: string; value: string; old?: string | null }) {
  return (
    <div className="details__item">
      <dt className="details__label">{label}</dt>
      <dd className="offer__value">
        {value}
        {old && <s className="offer__old">{old}</s>}
      </dd>
    </div>
  );
}

const REASONS = Object.keys(DECLINE_REASON_LABELS) as DeclineReason[];

/** Экран 17: причина отказа обязательна — по ней банк ищет запас условий. */
function DeclineModal({ offer, onClose, onDecline }: { offer: Offer; onClose: () => void; onDecline: (reason: DeclineReason, comment: string) => Promise<void> }) {
  const [reason, setReason] = useState<DeclineReason | null>(null);
  const [comment, setComment] = useState("");
  const [touched, setTouched] = useState(false);
  const [sending, setSending] = useState(false);
  const [error, setError] = useState("");

  const reasonError = touched && !reason ? "Выберите причину" : null;
  const commentError = touched && reason === "other" && !comment.trim() ? "Опишите причину" : null;

  const submit = async () => {
    setTouched(true);
    if (!reason || (reason === "other" && !comment.trim())) return;
    setSending(true);
    setError("");
    try {
      await onDecline(reason, comment.trim());
    } catch {
      setError("Не удалось отправить ответ. Повторите попытку.");
      setSending(false);
    }
  };

  return (
    <Modal title="Почему вы отклоняете предложение?" width={640} onClose={onClose}>
      <p className="modal__text">
        {formatMoney(offer.amount)} · {formatTerm(offer.term)} · {formatRate(offer.rate)}. Причина обязательна — она поможет банку проверить запас условий.
      </p>
      <fieldset className="radio-list" aria-invalid={Boolean(reasonError) || undefined}>
        <legend className="visually-hidden">Причина отказа</legend>
        {REASONS.map((value) => (
          <label key={value} className="radio">
            <input type="radio" name="decline-reason" value={value} checked={reason === value} onChange={() => setReason(value)} />
            {DECLINE_REASON_LABELS[value]}
          </label>
        ))}
      </fieldset>
      {reasonError && <span className="field__error">{reasonError}</span>}
      <TextAreaField
        label={reason === "other" ? "Комментарий" : "Комментарий (необязательно)"}
        rows={3}
        maxLength={1000}
        value={comment}
        error={commentError}
        onChange={(event) => setComment(event.target.value)}
      />
      {error && <Alert>{error}</Alert>}
      <div className="modal__actions">
        <Button variant="secondary" onClick={onClose}>
          Отмена
        </Button>
        <Button loading={sending} onClick={() => void submit()}>
          Отклонить
        </Button>
      </div>
    </Modal>
  );
}
