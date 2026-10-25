import { Check, CircleAlert, LoaderCircle } from "lucide-react";
import { useState } from "react";
import { Navigate, useNavigate, useParams, useSearchParams } from "react-router-dom";
import { applicationsApi } from "../../../api/client";
import { Page } from "../../../components/layout/Page";
import { Button } from "../../../components/ui/Button";
import { Alert, Card, StateMessage } from "../../../components/ui/Card";
import { Stepper } from "../../../components/ui/Stepper";
import { useResource } from "../../../hooks/useResource";
import { useAuth } from "../../../state/auth";
import { useProducts } from "../../../state/products";
import { StepCompany } from "./StepCompany";
import { StepCredit } from "./StepCredit";
import { StepFinance } from "./StepFinance";
import { StepReview } from "./StepReview";
import { useDraft, type SaveState } from "./useDraft";
import { creditErrors, financeErrors, hasErrors } from "./validation";

const STEPS = ["Компания", "Параметры кредита", "Финансы и документы", "Проверка"];

/**
 * Экраны 08–12: мастер заявки из 4 шагов.
 * Номер шага хранится в адресе (?step=2) — его можно обновить и не потерять место.
 */
export function ApplicationWizard() {
  const { id = "" } = useParams();
  const { token } = useAuth();
  const navigate = useNavigate();
  const [params, setParams] = useSearchParams();
  const step = Math.min(4, Math.max(1, Number(params.get("step")) || 1));
  const [showErrors, setShowErrors] = useState(false);
  const [integrationError, setIntegrationError] = useState("");

  const loaded = useResource(() => applicationsApi.get(token, id), [token, id]);
  const { products } = useProducts();
  const { app, setApp, change, flush, saveState } = useDraft(token, loaded.data);

  if (loaded.loading || (!app && !loaded.error)) {
    return (
      <Page crumbs={[{ label: "Мои заявки", to: "/applications" }, { label: "Новая заявка" }]} title="Новая заявка">
        <Card>Загружаем заявку…</Card>
      </Page>
    );
  }

  if (loaded.error || !app) {
    return (
      <Page crumbs={[{ label: "Мои заявки", to: "/applications" }, { label: "Заявка" }]} title="Заявка не найдена">
        <Card>
          <StateMessage title="Не удалось открыть заявку" text="Возможно, она удалена или принадлежит другому пользователю." action={<Button onClick={() => navigate("/applications")}>К заявкам</Button>} />
        </Card>
      </Page>
    );
  }

  // Отправленную заявку редактировать нельзя — показываем её статус
  if (app.status !== "DRAFT") return <Navigate to={`/applications/${app.id}`} replace />;

  const product = products.find((item) => item.code === app.productCode);

  /**
   * Ответ сервера после загрузки PDF: берём из него документ, финансы и платёж
   * по кредитам — всё это DS извлекает из файла. Остальные поля (сумма, срок, цель)
   * могли измениться локально и ещё не сохраниться — их не перетираем.
   * Платёж из PDF подставляем, только если он распознан; иначе оставляем введённый.
   */
  function mergeDocument(fresh: typeof app) {
    setApp((current) =>
      current && fresh
        ? {
            ...current,
            document: fresh.document,
            financials: fresh.financials,
            // После удаления PDF платёж тоже сбрасывается на бэкенде — оставляем введённый на экране
            debtPayment: fresh.debtPayment ?? current.debtPayment,
            editedCells: fresh.editedCells,
            dataConfirmed: false,
          }
        : current,
    );
  }

  const goTo = async (next: number) => {
    await flush();
    setShowErrors(false);
    setParams({ step: String(next) });
    window.scrollTo({ top: 0 });
  };

  /** «Продолжить»: проверяем текущий шаг, при ошибках подсвечиваем поля. */
  const proceed = async () => {
    const invalid = (step === 2 && product && hasErrors(creditErrors(app, product))) || (step === 3 && hasErrors(financeErrors(app)));
    if (invalid) {
      setShowErrors(true);
      return;
    }
    setIntegrationError("");
    try {
      if (step === 3 && app.financials && app.debtPayment !== null) {
        await flush();
        const saved = await applicationsApi.saveFinancials(token, app.id, app.debtPayment, app.financials);
        const confirmed = await applicationsApi.confirmFinancials(token, app.id);
        setApp((current) => (current ? { ...current, ...saved, ...confirmed } : current));
      }
      await goTo(step + 1);
    } catch (reason) {
      setIntegrationError(reason instanceof Error ? reason.message : "Не удалось сохранить данные на сервере");
    }
  };

  return (
    <Page
      crumbs={[{ label: "Мои заявки", to: "/applications" }, { label: step === 4 ? "Проверка и отправка" : "Новая заявка" }]}
      title={step === 4 ? "Проверка и отправка" : "Новая заявка"}
      meta={
        <>
          <span>Шаг {step} из 4</span>
          <SaveIndicator state={saveState} />
        </>
      }
      actions={
        <Button
          variant="secondary"
          onClick={async () => {
            await flush();
            navigate("/applications");
          }}
        >
          Сохранить и выйти
        </Button>
      }
    >
      <Stepper steps={STEPS} current={step} onSelect={(target) => void goTo(target)} />
      {integrationError && <Alert>{integrationError}</Alert>}

      {step === 1 && <StepCompany app={app} onNext={() => void goTo(2)} />}
      {step === 2 && product && <StepCredit app={app} product={product} showErrors={showErrors} onChange={change} onBack={() => void goTo(1)} onNext={() => void proceed()} />}
      {step === 3 && <StepFinance app={app} token={token} showErrors={showErrors} onChange={change} onReplace={mergeDocument} onBack={() => void goTo(2)} onNext={() => void proceed()} />}
      {step === 4 && product && (
        <StepReview
          app={app}
          product={product}
          token={token}
          onChange={change}
          flush={flush}
          onEdit={(target) => void goTo(target)}
          onSubmitted={(submitted) => navigate(`/applications/${submitted.id}`, { replace: true, state: { justSubmitted: true } })}
        />
      )}
    </Page>
  );
}

function SaveIndicator({ state }: { state: SaveState }) {
  if (state === "saving")
    return (
      <span className="save-indicator">
        <LoaderCircle size={14} className="spin" aria-hidden="true" />
        Сохраняем черновик…
      </span>
    );
  if (state === "saved")
    return (
      <span className="save-indicator save-indicator--ok">
        <Check size={14} aria-hidden="true" />
        Черновик сохранён
      </span>
    );
  if (state === "error")
    return (
      <span className="save-indicator save-indicator--error">
        <CircleAlert size={14} aria-hidden="true" />
        Не удалось сохранить — повторим автоматически
      </span>
    );
  return null;
}
