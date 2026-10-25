import { ArrowRight, Check } from "lucide-react";
import { useEffect, useState } from "react";
import { Link, Navigate, useNavigate, useParams } from "react-router-dom";
import { bankApi } from "../../api/client";
import { Page } from "../../components/layout/Page";
import { StatusBadge } from "../../components/ui/Badge";
import { Button } from "../../components/ui/Button";
import { Alert, Card, CardHeader, Divider, StateMessage } from "../../components/ui/Card";
import { MoneyField, TermField, TextAreaField, TextField } from "../../components/ui/Field";
import { useResource } from "../../hooks/useResource";
import { annuityPayment, formatMoney, formatTerm } from "../../lib/format";
import { useAuth } from "../../state/auth";
import { useProducts } from "../../state/products";
import type { DecisionType } from "../../types";
import { ChecksCard, ScoringCard } from "./parts";

/**
 * Экран 23 «Банк — решение по заявке».
 * Сотрудник сначала выбирает действие, затем пишет комментарий именно для него:
 * комментарий к одобрению видит клиент в предложении, список для запроса данных
 * становится пунктами запроса, причина отказа — итоговой причиной.
 * У каждого действия свой текст, поэтому комментарии не смешиваются.
 * При одобрении сотрудник может изменить условия: сумму, срок или ставку.
 */
/** Допустимая ставка в решении банка: ключевая ставка ЦБ ~14% + надбавка за риск малого бизнеса. */
const RATE_MIN = 15;
const RATE_MAX = 30;

/** Подписи поля комментария — свои для каждого действия. */
const ACTIONS: Record<DecisionType, { label: string; field: string; placeholder: string; hint: string; submit: string; required: string }> = {
  APPROVE: {
    label: "Одобрить",
    field: "Комментарий к одобрению",
    placeholder: "Например: одобрено с учётом стабильной выручки за полгода",
    hint: "Клиент увидит этот комментарий вместе с предложением банка.",
    submit: "Одобрить и отправить предложение",
    required: "Добавьте комментарий к одобрению",
  },
  REQUEST_DATA: {
    label: "Запросить данные",
    field: "Какие данные нужны",
    placeholder: "Выписка по расчётному счёту за сентябрь\nПояснение к снижению выручки в июле",
    hint: "Каждый пункт с новой строки — клиент получит их списком.",
    submit: "Запросить данные",
    required: "Перечислите, какие данные нужны",
  },
  REJECT: {
    label: "Отказать",
    field: "Причина отказа",
    placeholder: "Укажите причину отказа",
    hint: "Причина сохранится в заявке и будет показана клиенту.",
    submit: "Отказать",
    required: "Укажите причину отказа",
  },
};
const ACTION_ORDER: DecisionType[] = ["APPROVE", "REQUEST_DATA", "REJECT"];

export function DecisionPage() {
  const { id = "" } = useParams();
  const { user } = useAuth();
  const navigate = useNavigate();
  const { products } = useProducts();
  const resource = useResource(() => bankApi.get(id), [id]);
  const app = resource.data;
  const product = products.find((item) => item.code === app?.productCode);

  const [action, setAction] = useState<DecisionType>("APPROVE");
  // Отдельный текст для каждого действия: при переключении ничего не теряется и не смешивается
  const [comments, setComments] = useState<Record<DecisionType, string>>({ APPROVE: "", REQUEST_DATA: "", REJECT: "" });
  const [amount, setAmount] = useState<number | null>(null);
  const [term, setTerm] = useState<number | null>(null);
  const [rate, setRate] = useState("");
  const [touched, setTouched] = useState(false);
  const [sending, setSending] = useState(false);
  const [error, setError] = useState("");

  // Условия по умолчанию — как запросил клиент, ставка — базовая по продукту
  useEffect(() => {
    if (!app || !product) return;
    setAmount((current) => current ?? app.amount);
    setTerm((current) => current ?? app.term);
    setRate((current) => current || String(product.rate).replace(".", ","));
  }, [app, product]);

  const crumbs = [
    { label: "Очередь заявок", to: "/bank/applications" },
    { label: app ? `Заявка ${app.number}` : "Заявка", to: app ? `/bank/applications/${app.id}` : undefined },
    { label: "Решение" },
  ];

  if (resource.loading && !app) {
    return (
      <Page crumbs={crumbs} title="Решение по заявке">
        <Card>Загружаем заявку…</Card>
      </Page>
    );
  }
  if (resource.error || !app) {
    return (
      <Page crumbs={crumbs} title="Решение по заявке">
        <Card>
          <StateMessage title="Не удалось загрузить заявку" action={<Button onClick={() => void resource.reload()}>Повторить</Button>} />
        </Card>
      </Page>
    );
  }
  // Решение уже принято — возвращаем в карточку
  if (app.status !== "MANUAL") return <Navigate to={`/bank/applications/${app.id}`} replace />;

  const rateNumber = Number(rate.replace(",", "."));
  const comment = comments[action];
  const config = ACTIONS[action];
  const commentError = touched && !comment.trim() ? config.required : null;
  const amountError = touched && product && (!amount || amount < product.minAmount || amount > product.maxAmount) ? `От ${formatMoney(product.minAmount)} до ${formatMoney(product.maxAmount)}` : null;
  const termError = touched && product && (!term || term < product.minTerm || term > product.maxTerm) ? `От ${product.minTerm} до ${formatTerm(product.maxTerm)}` : null;
  const rateError = touched && (!rateNumber || rateNumber < RATE_MIN || rateNumber > RATE_MAX) ? `Ставка от ${RATE_MIN} до ${RATE_MAX}%` : null;
  const payment = amount && term && rateNumber ? annuityPayment(amount, term, rateNumber) : null;

  const decide = async () => {
    setTouched(true);
    setError("");
    if (!comment.trim()) return;
    const conditionsInvalid = !amount || !term || !rateNumber || (product && (amount < product.minAmount || amount > product.maxAmount || term < product.minTerm || term > product.maxTerm)) || rateNumber < RATE_MIN || rateNumber > RATE_MAX;
    if (action === "APPROVE" && conditionsInvalid) return;
    setSending(true);
    try {
      await bankApi.decide(app.id, { type: action, comment: comment.trim(), employeeName: user?.name ?? "Сотрудник банка" }, action === "APPROVE" ? { amount: amount!, term: term!, rate: rateNumber } : undefined);
      navigate(`/bank/applications/${app.id}`, { replace: true });
    } catch {
      setError("Не удалось сохранить решение. Повторите попытку.");
      setSending(false);
    }
  };

  return (
    <Page
      crumbs={crumbs}
      title={`Решение по заявке ${app.number}`}
      subtitle={`${app.company?.name} · ИНН ${app.company?.inn} · ${product?.name ?? "Кредит"}`}
      actions={<StatusBadge status={app.status} audience="bank" />}
    >
      <div className="columns">
        <Card compact>
          <CardHeader title={app.company?.name} subtitle={`ИНН ${app.company?.inn}`} />
          <Divider />
          <div className="stack stack--8">
            <p>{product?.name}</p>
            <p>
              {formatMoney(app.amount)} · {formatTerm(app.term)}
            </p>
          </div>
          <Link to={`/bank/applications/${app.id}`} className="text-link">
            Вернуться к карточке заявки
            <ArrowRight size={16} aria-hidden="true" />
          </Link>
        </Card>
        <ScoringCard app={app} />
      </div>

      <div className="columns">
        <Card compact>
          <CardHeader title="Решение" subtitle="Выберите действие и добавьте комментарий к нему." />
          <fieldset className="radio-list radio-list--row">
            <legend className="visually-hidden">Действие по заявке</legend>
            {ACTION_ORDER.map((value) => (
              <label key={value} className="radio">
                <input
                  type="radio"
                  name="decision-action"
                  value={value}
                  checked={action === value}
                  onChange={() => {
                    setAction(value);
                    setTouched(false);
                  }}
                />
                {ACTIONS[value].label}
              </label>
            ))}
          </fieldset>
          <TextAreaField
            label={config.field}
            rows={5}
            maxLength={2000}
            placeholder={config.placeholder}
            hint={config.hint}
            value={comment}
            error={commentError}
            onChange={(event) => setComments((current) => ({ ...current, [action]: event.target.value }))}
          />

          {action === "APPROVE" && (
          <div className="stack stack--12">
            <p className="strong">Условия предложения при одобрении</p>
            <div className="conditions-grid">
              <MoneyField label="Сумма" value={amount} error={amountError} onValueChange={setAmount} />
              <TermField label="Срок" value={term} error={termError} onValueChange={setTerm} />
              <TextField
                label="Ставка"
                inputMode="decimal"
                value={rate}
                error={rateError}
                onChange={(event) => setRate(event.target.value.replace(/[^\d,.]/g, "").slice(0, 5))}
                end={<span className="field__suffix">%</span>}
              />
            </div>
            <p className="hint-text">{payment ? `Ежемесячный платёж ≈ ${formatMoney(payment)}. ` : ""}Клиент запрашивал {formatMoney(app.amount)} на {formatTerm(app.term)}.</p>
          </div>
          )}

          {error && <Alert>{error}</Alert>}
          <Divider />
          <div className="actions-row actions-row--end">
            <Button variant={action === "REJECT" ? "secondary" : "primary"} loading={sending} onClick={() => void decide()}>
              {config.submit}
            </Button>
          </div>
          <p className="hint-text">До выбора действия заявка остаётся на проверке.</p>
        </Card>

        <div className="columns__side">
          <Card compact>
            <h2 className="card__title">Путь заявки</h2>
            <ol className="path">
              <li className="path__item path__item--done">
                <span className="path__marker">
                  <Check size={16} strokeWidth={3} />
                </span>
                Очередь заявок
              </li>
              <li className="path__item path__item--done">
                <span className="path__marker">
                  <Check size={16} strokeWidth={3} />
                </span>
                Карточка заявки
              </li>
              <li className="path__item path__item--current">
                <span className="path__marker">3</span>
                Решение
              </li>
            </ol>
          </Card>
          <ChecksCard app={app} />
        </div>
      </div>
    </Page>
  );
}
