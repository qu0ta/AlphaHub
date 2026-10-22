import { LoaderCircle } from "lucide-react";
import { useState, type ReactNode } from "react";
import { Navigate, useNavigate, useParams } from "react-router-dom";
import { ApiError, applicationsApi } from "../../api/client";
import { Page } from "../../components/layout/Page";
import { StatusBadge } from "../../components/ui/Badge";
import { Button } from "../../components/ui/Button";
import { Alert, Card, CardHeader } from "../../components/ui/Card";
import { TextAreaField } from "../../components/ui/Field";
import { Modal } from "../../components/ui/Modal";
import { Timeline } from "../../components/ui/Timeline";
import { FileRow, pdfError, UploadZone } from "../../components/ui/Upload";
import { usePolling, useResource } from "../../hooks/useResource";
import { annuityPayment, formatDate, formatFileSize, formatLongDate, formatMoney, formatRate, formatTerm } from "../../lib/format";
import { useAuth } from "../../state/auth";
import { useProducts } from "../../state/products";
import type { Application, Offer } from "../../types";

/** Статусы, в которых заявка меняется без участия клиента — их опрашиваем. */
const LIVE_STATUSES: Application["status"][] = ["SUBMITTED", "IN_REVIEW", "MANUAL", "RETENTION_REVIEW", "SIGNED"];

/**
 * Экраны 13, 14, 15, 18, 19, 20 — статус заявки.
 * Слева всегда «Ход заявки», справа — карточка, зависящая от статуса.
 */
export function ApplicationStatusPage() {
  const { id = "" } = useParams();
  const { token } = useAuth();
  const navigate = useNavigate();
  const { products } = useProducts();
  const resource = useResource(() => applicationsApi.get(token, id), [token, id]);
  const app = resource.data;

  usePolling(() => void resource.reload({ silent: true }), Boolean(app && LIVE_STATUSES.includes(app.status)), 2000);

  const crumbs = [{ label: "Мои заявки", to: "/applications" }, { label: app ? `Заявка № ${app.number}` : "Заявка" }];

  if (resource.loading && !app) {
    return (
      <Page crumbs={crumbs} title="Заявка">
        <Card>Загружаем статус…</Card>
      </Page>
    );
  }

  // Экран 20, вариант «Техническая ошибка»
  if (resource.error || !app) {
    const notFound = resource.error instanceof ApiError && resource.error.status === 404;
    return (
      <Page crumbs={crumbs} title="Заявка">
        <OutcomeCard
          tone="neutral"
          title={notFound ? "Заявка не найдена" : "Техническая ошибка"}
          text={notFound ? "Возможно, ссылка устарела." : "Не удалось получить актуальный статус."}
          note={notFound ? "Вернитесь к списку заявок." : "Обновите страницу или повторите позже."}
          action={notFound ? <Button variant="secondary" onClick={() => navigate("/applications")}>К заявкам</Button> : <Button onClick={() => void resource.reload()}>Повторить</Button>}
        />
      </Page>
    );
  }

  if (app.status === "DRAFT") return <Navigate to={`/applications/${app.id}/edit`} replace />;

  const product = products.find((item) => item.code === app.productCode);
  const accepted = app.offers.find((offer) => offer.status === "ACCEPTED");
  const amount = accepted?.amount ?? app.amount;
  const term = accepted?.term ?? app.term;

  return (
    <Page
      crumbs={crumbs}
      title={`Заявка № ${app.number}`}
      meta={
        <>
          <StatusBadge status={app.status} />
          <span>
            {product?.name ?? "Кредит"} · {formatMoney(amount)} · {formatTerm(term)}
          </span>
        </>
      }
    >
      {app.status === "REJECTED" || app.status === "DECLINED" ? (
        <div className="stack stack--24">
          {app.status === "REJECTED" ? (
            <OutcomeCard
              tone="danger"
              title="Отказ банка"
              text="Банк не может одобрить заявку по результатам проверки."
              note="Повторная заявка будет доступна позже."
              action={<Button variant="secondary" onClick={() => navigate("/applications")}>На главную</Button>}
            />
          ) : (
            <OutcomeCard
              tone="warning"
              title="Вы отказались от предложения"
              text="Заявка завершена по вашему решению."
              note="Причина сохранена в истории."
              action={<Button variant="secondary" onClick={() => navigate("/applications")}>На главную</Button>}
            />
          )}
          <Card>
            <CardHeader title="Ход заявки" />
            <Timeline events={app.history} />
          </Card>
        </div>
      ) : (
        <div className="columns">
          <Card className="status-history">
            <CardHeader title="Ход заявки" />
            <Timeline events={app.history} />
          </Card>
          <div className="columns__side">
            <SideCard app={app} token={token} onUpdated={(next) => resource.setData(next)} />
          </div>
        </div>
      )}
    </Page>
  );
}

function SideCard({ app, token, onUpdated }: { app: Application; token: string | null; onUpdated: (app: Application) => void }) {
  const navigate = useNavigate();
  const offer = app.offers.find((item) => item.status === "ACTIVE");

  switch (app.status) {
    case "NEED_DATA":
      return <InfoRequestCards app={app} token={token} onUpdated={onUpdated} />;

    case "OFFER":
    case "RETENTION_OFFER":
      return offer ? (
        <Card compact>
          <h2 className="card__title">{app.status === "RETENTION_OFFER" ? "Новое предложение банка" : "Предложение банка"}</h2>
          <div className="stack stack--12">
            <p className="strong">
              {formatMoney(offer.amount)} на {formatTerm(offer.term)}
            </p>
            <p className="muted">Ставка {formatRate(offer.rate)} годовых</p>
            <p className="muted">Платёж {formatMoney(offer.monthlyPayment)} в месяц</p>
            <p className="hint-text">Действует до {formatLongDate(offer.validUntil)}</p>
          </div>
          <Button block onClick={() => navigate(`/applications/${app.id}/offer`)}>
            Посмотреть предложение
          </Button>
        </Card>
      ) : null;

    case "RETENTION_REVIEW":
      return (
        <Card compact>
          <h2 className="card__title">Что дальше</h2>
          <p className="strong">Банк проверяет условия</p>
          <p className="muted">Количество улучшений зависит от доступного запаса.</p>
          <p className="muted">Если улучшение невозможно, заявка завершится.</p>
        </Card>
      );

    case "SIGNED":
      return (
        <Card compact>
          <h2 className="card__title">Выдаём кредит</h2>
          <p className="inline-note">
            <LoaderCircle size={16} className="spin" aria-hidden="true" />
            Документы подписаны, деньги поступят на расчётный счёт через несколько секунд.
          </p>
        </Card>
      );

    case "DISBURSED":
      return <DisbursedCard app={app} />;

    default:
      return (
        <Card compact>
          <h2 className="card__title">Что происходит</h2>
          <p className="strong">Решение ожидается</p>
          <p className="muted">Обычно проверка занимает до 2 рабочих дней.</p>
          <p className="muted">Ничего делать не нужно — статус обновится сам.</p>
        </Card>
      );
  }
}

/** Экран 14: запрос банка и форма дополнительных данных. */
function InfoRequestCards({ app, token, onUpdated }: { app: Application; token: string | null; onUpdated: (app: Application) => void }) {
  const [file, setFile] = useState<File | null>(null);
  const [fileError, setFileError] = useState<string | null>(null);
  const [explanation, setExplanation] = useState("");
  const [touched, setTouched] = useState(false);
  const [sending, setSending] = useState(false);
  const [error, setError] = useState("");
  const request = app.infoRequest;

  const explanationError = touched && !explanation.trim() ? "Добавьте пояснение для банка" : null;

  const send = async () => {
    setTouched(true);
    setError("");
    if (!explanation.trim()) return;
    setSending(true);
    try {
      onUpdated(await applicationsApi.respondToInfoRequest(token, app.id, { fileName: file?.name ?? null, explanation: explanation.trim() }));
    } catch {
      setError("Не удалось отправить данные. Повторите попытку.");
    } finally {
      setSending(false);
    }
  };

  return (
    <>
      <Card compact>
        <h2 className="card__title">Запрос банка</h2>
        {request && (
          <>
            <p className="strong">До {formatLongDate(request.dueDate).replace(/ \d{4}$/, "")}</p>
            <ul className="request-list">
              {request.items.map((item) => (
                <li key={item} className="muted">
                  {item}
                </li>
              ))}
            </ul>
          </>
        )}
      </Card>
      <Card compact>
        <h2 className="card__title card__title--sm">Дополнительные данные</h2>
        {file ? (
          <FileRow
            compact
            name={file.name}
            meta={formatFileSize(file.size)}
            actions={
              <button type="button" className="text-link" onClick={() => setFile(null)}>
                Убрать
              </button>
            }
          />
        ) : (
          <UploadZone
            compact
            title="Перетащите PDF или выберите файл"
            hint="Необязательно, до 10 МБ"
            error={fileError}
            onFile={(next) => {
              const problem = pdfError(next);
              setFileError(problem);
              if (!problem) setFile(next);
            }}
          />
        )}
        <TextAreaField label="Пояснение" placeholder="Например: пояснение цели кредита" rows={3} value={explanation} error={explanationError} onChange={(event) => setExplanation(event.target.value)} />
        {error && <Alert>{error}</Alert>}
        <Button block loading={sending} onClick={() => void send()}>
          Повторно отправить
        </Button>
      </Card>
    </>
  );
}

/** Экран 19: кредит выдан + график платежей. */
function DisbursedCard({ app }: { app: Application }) {
  const [open, setOpen] = useState(false);
  const offer = app.offers.find((item) => item.status === "ACCEPTED");
  if (!offer) return null;
  const next = new Date(app.updatedAt);
  next.setMonth(next.getMonth() + 1);

  return (
    <>
      <Card compact>
        <h2 className="card__title">Кредит оформлен</h2>
        <p className="strong">{formatMoney(offer.amount)}</p>
        <p className="muted">
          Следующий платёж: {formatMoney(offer.monthlyPayment)} · {formatDate(next.toISOString())}
        </p>
        <p className="muted">График доступен по кнопке ниже.</p>
        <Button block onClick={() => setOpen(true)}>
          Открыть график
        </Button>
      </Card>
      {open && <ScheduleModal offer={offer} start={app.updatedAt} onClose={() => setOpen(false)} />}
    </>
  );
}

/** Аннуитетный график: платёж одинаковый, доля процентов уменьшается. */
function ScheduleModal({ offer, start, onClose }: { offer: Offer; start: string; onClose: () => void }) {
  const payment = annuityPayment(offer.amount, offer.term, offer.rate);
  const rate = offer.rate / 100 / 12;
  let balance = offer.amount;
  const rows = Array.from({ length: offer.term }, (_, index) => {
    const date = new Date(start);
    date.setMonth(date.getMonth() + index + 1);
    const interest = Math.round(balance * rate);
    const principal = index === offer.term - 1 ? balance : payment - interest;
    balance = Math.max(0, balance - principal);
    return { n: index + 1, date: formatDate(date.toISOString()), payment: principal + interest, principal, interest, balance };
  });

  return (
    <Modal title="График платежей" width={720} onClose={onClose}>
      <p className="modal__text">
        {formatMoney(offer.amount)} · {formatTerm(offer.term)} · {formatRate(offer.rate)} годовых. Расчёт демонстрационный.
      </p>
      <div className="table-wrap schedule">
        <table className="table table--dense">
          <thead>
            <tr>
              <th>№</th>
              <th>Дата</th>
              <th className="table__num">Платёж</th>
              <th className="table__num">Основной долг</th>
              <th className="table__num">Проценты</th>
              <th className="table__num">Остаток</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr key={row.n}>
                <td>{row.n}</td>
                <td>{row.date}</td>
                <td className="table__num">{formatMoney(row.payment)}</td>
                <td className="table__num">{formatMoney(row.principal)}</td>
                <td className="table__num">{formatMoney(row.interest)}</td>
                <td className="table__num">{formatMoney(row.balance)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </Modal>
  );
}

/** Экран 20: финальный исход заявки. */
function OutcomeCard({ tone, title, text, note, action }: { tone: "danger" | "warning" | "neutral"; title: string; text: string; note: string; action: ReactNode }) {
  return (
    <Card>
      <div className="outcome">
        <div className="stack stack--12">
          <p className="outcome__title">
            <span className={`outcome__dot outcome__dot--${tone}`} aria-hidden="true" />
            {title}
          </p>
          <p className="muted">{text}</p>
          <p className="hint-text">{note}</p>
        </div>
        {action}
      </div>
    </Card>
  );
}
