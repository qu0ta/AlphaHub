import { Inbox } from "lucide-react";
import { useState, type FormEvent } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { bankApi, type QueueView } from "../../api/client";
import { Page } from "../../components/layout/Page";
import { StatusBadge } from "../../components/ui/Badge";
import { Button } from "../../components/ui/Button";
import { StateMessage } from "../../components/ui/Card";
import { TextField } from "../../components/ui/Field";
import { Select } from "../../components/ui/Select";
import { usePolling, useResource } from "../../hooks/useResource";
import { digitsOnly, formatMoney, formatTerm } from "../../lib/format";
import { useAuth } from "../../state/auth";
import type { Application } from "../../types";

const VIEWS: Record<QueueView, { title: string; subtitle: string; empty: string }> = {
  queue: { title: "Очередь заявок", subtitle: "Ручная проверка", empty: "Новых заявок на проверку нет." },
  mine: { title: "Мои решения", subtitle: "Заявки, по которым вы приняли решение", empty: "Вы ещё не принимали решений." },
  archive: { title: "Архив", subtitle: "Завершённые заявки", empty: "Завершённых заявок пока нет." },
};

type StatusFilter = "all" | "MANUAL" | "NEED_DATA" | "AUTO" | "DONE";

const STATUS_OPTIONS: Array<{ value: StatusFilter; label: string }> = [
  { value: "all", label: "Статус: все" },
  { value: "MANUAL", label: "На проверке" },
  { value: "NEED_DATA", label: "Нужны данные" },
  { value: "AUTO", label: "Автопроверка" },
  { value: "DONE", label: "Решение готово" },
];

function matchesStatus(app: Application, filter: StatusFilter) {
  switch (filter) {
    case "MANUAL":
      return app.status === "MANUAL";
    case "NEED_DATA":
      return app.status === "NEED_DATA";
    case "AUTO":
      return app.status === "SUBMITTED" || app.status === "IN_REVIEW";
    case "DONE":
      return app.decision !== null || app.status === "OFFER";
    default:
      return true;
  }
}

/** Экран 21 «Банк — очередь ручной проверки» (+ разделы «Мои решения» и «Архив»). */
export function QueuePage() {
  const { user, token } = useAuth();
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const view = (["mine", "archive"].includes(params.get("view") ?? "") ? params.get("view") : "queue") as QueueView;
  const [innInput, setInnInput] = useState("");
  const [inn, setInn] = useState("");
  const [status, setStatus] = useState<StatusFilter>("all");

  const resource = useResource(() => bankApi.list(token, { view, inn, employeeName: user?.name ?? "" }), [token, view, inn, user?.name]);
  // Новые заявки от клиентов появляются сами — обновляем список раз в 5 секунд
  usePolling(() => void resource.reload({ silent: true }), true, 5000);

  const rows = (resource.data ?? []).filter((app) => matchesStatus(app, status));
  const meta = VIEWS[view];

  const search = (event: FormEvent) => {
    event.preventDefault();
    setInn(innInput);
  };

  return (
    <Page crumbs={[{ label: meta.title }]} title={meta.title} subtitle={meta.subtitle}>
      <form className="toolbar" onSubmit={search} role="search">
        <TextField
          className="toolbar__search"
          aria-label="Поиск по ИНН"
          placeholder="Поиск по ИНН"
          inputMode="numeric"
          value={innInput}
          onChange={(event) => {
            const value = digitsOnly(event.target.value).slice(0, 12);
            setInnInput(value);
            if (!value) setInn("");
          }}
        />
        <Select className="toolbar__filter" value={status} options={STATUS_OPTIONS} onChange={setStatus} />
        <Button type="submit" className="toolbar__button">
          Найти
        </Button>
      </form>

      <div className="table-wrap">
        {resource.loading && !resource.data ? (
          <p className="table-message">Загружаем заявки…</p>
        ) : resource.error ? (
          <StateMessage title="Не удалось загрузить очередь" action={<Button onClick={() => void resource.reload()}>Повторить</Button>} />
        ) : rows.length === 0 ? (
          <StateMessage icon={<Inbox size={28} />} title={inn || status !== "all" ? "Ничего не найдено" : "Пусто"} text={inn || status !== "all" ? "Измените условия поиска." : meta.empty} />
        ) : (
          <table className="table queue-table">
            <thead>
              <tr>
                <th>№</th>
                <th>ИНН</th>
                <th>Сумма</th>
                <th>Срок</th>
                <th>Скоринг</th>
                <th>Статус</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((app) => (
                <tr key={app.id} className="table__row-link" onClick={() => navigate(`/bank/applications/${app.id}`)}>
                  <td>
                    {/* Ссылка нужна для клавиатуры и открытия в новой вкладке */}
                    <Link to={`/bank/applications/${app.id}`} className="queue-table__number" onClick={(event) => event.stopPropagation()}>
                      {app.number}
                    </Link>
                    <span className="queue-table__company">{app.company?.name}</span>
                  </td>
                  <td>{app.company?.inn}</td>
                  <td>{formatMoney(app.amount)}</td>
                  <td>{formatTerm(app.term)}</td>
                  <td>{app.score !== null ? `${app.score}/100` : "—"}</td>
                  <td>
                    <StatusBadge status={app.status} audience="bank" />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </Page>
  );
}

