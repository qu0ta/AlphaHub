import { Building2, Check, Repeat2 } from "lucide-react";
import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { applicationsApi, companyApi } from "../../api/client";
import { Page } from "../../components/layout/Page";
import { Badge } from "../../components/ui/Badge";
import { Button } from "../../components/ui/Button";
import { Alert, Card, Divider, InfoBlock, StateMessage } from "../../components/ui/Card";
import { useResource } from "../../hooks/useResource";
import { formatMoney, formatTerm } from "../../lib/format";
import { useAuth } from "../../state/auth";
import { useProducts } from "../../state/products";
import type { Product } from "../../types";

/** Что покажем в карточке недоступного продукта вместо параметров */
const UPCOMING_FEATURES: Record<string, string[]> = {
  INVESTMENT: ["Покупка оборудования и техники", "Открытие новых направлений", "Долгосрочное развитие компании"],
};

/** Экран 07 «Кредитные продукты». */
export function ProductsPage() {
  const { token } = useAuth();
  const navigate = useNavigate();
  const { products, loading, error, retry } = useProducts();
  const company = useResource(() => companyApi.get(token), [token]);
  const [starting, setStarting] = useState<string | null>(null);
  const [startError, setStartError] = useState("");

  /**
   * «Выбрать»: если уже есть черновик по этому продукту — открываем его,
   * чтобы не плодить дубли. Иначе создаём новый черновик.
   */
  const choose = async (product: Product) => {
    if (!company.data) {
      navigate("/onboarding/company?next=/products");
      return;
    }
    setStarting(product.code);
    setStartError("");
    try {
      const existing = (await applicationsApi.list(token)).find((app) => app.status === "DRAFT" && app.productCode === product.code);
      const app = existing ?? (await applicationsApi.create(token, product.code));
      navigate(`/applications/${app.id}/edit`);
    } catch {
      setStartError("Не удалось создать заявку. Повторите попытку.");
      setStarting(null);
    }
  };

  return (
    <Page crumbs={[{ label: "Кредитные продукты" }]} title="Финансирование для вашего бизнеса" subtitle="Выберите продукт под текущие задачи и планы компании.">
      {loading ? (
        <Card>Загружаем продукты…</Card>
      ) : error ? (
        <Card>
          <StateMessage title="Не удалось загрузить продукты" action={<Button onClick={() => void retry()}>Повторить</Button>} />
        </Card>
      ) : (
        <div className="grid-2">
          {products.map((product) => (
            <Card key={product.code} as="article" className="product-card">
              <div className="card__header">
                <span className={`icon-tile icon-tile--lg ${product.active ? "icon-tile--accent" : ""}`} aria-hidden="true">
                  {product.active ? <Repeat2 size={28} /> : <Building2 size={28} />}
                </span>
                {product.active ? <Badge tone="success">Доступен</Badge> : <Badge>Скоро будет доступно</Badge>}
              </div>
              <div className="stack stack--12">
                <h2 className="product-card__name">{product.name}</h2>
                <p className="product-card__text">{product.description}</p>
              </div>
              <Divider />
              {product.active ? (
                <dl className="product-card__params">
                  <div>
                    <dt>Сумма</dt>
                    <dd>до {formatMoney(product.maxAmount)}</dd>
                  </div>
                  <div>
                    <dt>Срок</dt>
                    <dd>до {formatTerm(product.maxTerm)}</dd>
                  </div>
                  <div>
                    <dt>Погашение</dt>
                    <dd>Ежемесячно</dd>
                  </div>
                </dl>
              ) : (
                <ul className="product-card__features">
                  {(UPCOMING_FEATURES[product.code] ?? []).map((text) => (
                    <li key={text}>
                      <Check size={18} aria-hidden="true" />
                      {text}
                    </li>
                  ))}
                </ul>
              )}
              <div className="stack stack--12 product-card__footer">
                <Button block disabled={!product.active} loading={starting === product.code} onClick={() => void choose(product)}>
                  {product.active ? "Выбрать" : "Скоро будет доступно"}
                </Button>
                <p className="hint-text">
                  {product.active ? "Индивидуальные условия определяются после рассмотрения заявки." : "Подать заявку пока нельзя. Продукт появится в каталоге позже."}
                </p>
              </div>
            </Card>
          ))}
        </div>
      )}

      {startError && <Alert>{startError}</Alert>}

      {company.data ? (
        <InfoBlock title="Компания уже добавлена">Реквизиты {company.data.name} сохранятся в кабинете — их не придётся вводить повторно.</InfoBlock>
      ) : (
        !company.loading && <InfoBlock title="Сначала добавьте компанию">После выбора продукта попросим ИНН — остальные реквизиты найдём в реестре сами.</InfoBlock>
      )}

      <p className="hint-text">Информация носит ознакомительный характер и не является публичной офертой. Решение о финансировании принимается индивидуально.</p>
    </Page>
  );
}
