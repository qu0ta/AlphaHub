import { ArrowRight, FileStack, FolderCheck, FolderPlus } from "lucide-react";
import { Link, useNavigate } from "react-router-dom";
import { applicationsApi, companyApi } from "../../api/client";
import { Page } from "../../components/layout/Page";
import { Badge, StatusBadge } from "../../components/ui/Badge";
import { Button } from "../../components/ui/Button";
import { Card, Divider, InfoBlock, StateMessage } from "../../components/ui/Card";
import { usePolling, useResource } from "../../hooks/useResource";
import { formatDate, formatMoney, formatTerm } from "../../lib/format";
import { useAuth } from "../../state/auth";
import { useProducts } from "../../state/products";
import type { Application } from "../../types";

const IN_PROGRESS: Application["status"][] = ["SUBMITTED", "IN_REVIEW", "MANUAL", "RETENTION_REVIEW", "SIGNED"];

/** Текст под карточкой заявки — что происходит и что делать. */
function statusNote(app: Application): string {
  switch (app.status) {
    case "DRAFT":
      return "Данные сохранены. Вернитесь к заполнению в удобное время.";
    case "NEED_DATA":
      return "Банк запросил дополнительные данные. Добавьте их, чтобы продолжить рассмотрение.";
    case "OFFER":
    case "RETENTION_OFFER":
      return "Банк подготовил предложение. Примите или отклоните его.";
    case "RETENTION_REVIEW":
      return "Банк проверяет, можно ли улучшить условия.";
    case "SIGNED":
      return "Документы подписаны, деньги скоро поступят на счёт.";
    case "DISBURSED":
      return "Кредит выдан. Деньги зачислены на расчётный счёт.";
    case "REJECTED":
      return "Банк не может одобрить заявку по результатам проверки.";
    case "DECLINED":
      return "Заявка завершена по вашему решению.";
    default:
      return `Получена ${formatDate(app.submittedAt)}. Сообщим, когда появится решение.`;
  }
}

function actionFor(app: Application): { label: string; to: string } {
  if (app.status === "DRAFT") return { label: "Продолжить черновик", to: `/applications/${app.id}/edit` };
  if (app.status === "NEED_DATA") return { label: "Добавить данные", to: `/applications/${app.id}` };
  if (app.status === "OFFER" || app.status === "RETENTION_OFFER") return { label: "Посмотреть предложение", to: `/applications/${app.id}/offer` };
  return { label: "Открыть заявку", to: `/applications/${app.id}` };
}

/** Экран 06 «Мои заявки». */
export function ApplicationsPage() {
  const { token } = useAuth();
  const navigate = useNavigate();
  const applications = useResource(() => applicationsApi.list(token), [token]);
  const company = useResource(() => companyApi.get(token), [token]);
  const { products } = useProducts();

  const list = applications.data ?? [];
  // Пока какая-то заявка «в работе» у банка — периодически обновляем статусы
  usePolling(() => void applications.reload({ silent: true }), list.some((app) => IN_PROGRESS.includes(app.status)), 3000);

  const drafts = list.filter((app) => app.status === "DRAFT").length;
  const inReview = list.filter((app) => IN_PROGRESS.includes(app.status)).length;
  const summary = [inReview && `${inReview} на рассмотрении`, drafts && `${drafts} ${drafts === 1 ? "черновик" : "черновика"}`].filter(Boolean).join(" · ");
  const productName = (code: string) => products.find((item) => item.code === code)?.name ?? "Кредит";

  return (
    <Page
      crumbs={[{ label: "Мои заявки" }]}
      title="Мои заявки"
      subtitle="Следите за рассмотрением и продолжайте сохранённые заявки."
      actions={<Button onClick={() => navigate("/products")}>Новая заявка</Button>}
    >
      {applications.loading ? (
        <Card>Загружаем заявки…</Card>
      ) : applications.error ? (
        <Card>
          <StateMessage title="Не удалось загрузить заявки" text="Проверьте соединение и повторите попытку." action={<Button onClick={() => void applications.reload()}>Повторить</Button>} />
        </Card>
      ) : list.length === 0 ? (
        <Card>
          <StateMessage
            icon={<FileStack size={28} />}
            title="Заявок пока нет"
            text="Выберите кредитный продукт — заявка займёт около 10 минут, черновик сохраняется автоматически."
            action={<Button onClick={() => navigate("/products")}>Выбрать продукт</Button>}
          />
        </Card>
      ) : (
        <>
          <div className="list-summary">
            <span className="list-summary__title">Все заявки</span>
            <Badge>{list.length}</Badge>
            {summary && <span className="muted">{summary}</span>}
          </div>
          <div className="grid-2">
            {list.map((app) => {
              const action = actionFor(app);
              const amount = app.offers.find((offer) => offer.status === "ACCEPTED")?.amount ?? app.amount;
              return (
                <Card key={app.id} as="article">
                  <div className="card__header">
                    <span className="muted app-card__number">{app.status === "DRAFT" ? `Создан ${formatDate(app.createdAt)}` : `Заявка № ${app.number}`}</span>
                    <StatusBadge status={app.status} />
                  </div>
                  <div className="stack stack--8">
                    <h2 className="app-card__product">{productName(app.productCode)}</h2>
                    <p className="muted">{app.company?.name}</p>
                  </div>
                  <div className="app-card__params">
                    <div className="details__item">
                      <span className="details__label">Сумма кредита</span>
                      <span className="app-card__value">{amount ? formatMoney(amount) : "Не указана"}</span>
                    </div>
                    <div className="details__item">
                      <span className="details__label">Срок</span>
                      <span className="app-card__value">{app.term ? formatTerm(app.term) : "Не указан"}</span>
                    </div>
                  </div>
                  <Divider />
                  <p className="muted app-card__note">{statusNote(app)}</p>
                  <div>
                    <Button variant={action.label === "Открыть заявку" || app.status === "DRAFT" ? "secondary" : "primary"} onClick={() => navigate(action.to)}>
                      {action.label}
                    </Button>
                  </div>
                </Card>
              );
            })}
          </div>
        </>
      )}

      {company.data ? (
        <Card compact>
          <div className="company-head">
            <span className="icon-tile" aria-hidden="true">
              <FolderCheck size={24} />
            </span>
            <div className="company-head__text">
              <p className="strong">Компания добавлена</p>
              <p className="muted">
                {company.data.name}
                {" · Финансовую сводку загрузите в заявке"}
              </p>
            </div>
            <Link to="/company" className="text-link">
              К документам
              <ArrowRight size={20} aria-hidden="true" />
            </Link>
          </div>
        </Card>
      ) : (
        !company.loading && (
          <Card compact>
            <div className="company-head">
              <span className="icon-tile icon-tile--accent" aria-hidden="true">
                <FolderPlus size={24} />
              </span>
              <div className="company-head__text">
                <p className="strong">Добавьте компанию</p>
                <p className="muted">Без данных компании заявку отправить нельзя.</p>
              </div>
              <Link to="/onboarding/company" className="text-link">
                Заполнить
                <ArrowRight size={20} aria-hidden="true" />
              </Link>
            </div>
          </Card>
        )
      )}

      {drafts > 0 && (
        <InfoBlock title="Не нужно создавать заявку заново">Если вы уже начали заполнение, продолжите черновик — введённые данные сохранены.</InfoBlock>
      )}
    </Page>
  );
}
