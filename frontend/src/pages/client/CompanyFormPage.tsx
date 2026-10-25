import { CircleAlert, CircleCheck, LoaderCircle, LockKeyhole, SearchX, ServerCrash } from "lucide-react";
import { useEffect, useRef, useState, type FormEvent } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { ApiError, companyApi } from "../../api/client";
import { Page } from "../../components/layout/Page";
import { Badge } from "../../components/ui/Badge";
import { Button } from "../../components/ui/Button";
import { Alert, Card, Details, Divider, InfoBlock } from "../../components/ui/Card";
import { TextField } from "../../components/ui/Field";
import { Select } from "../../components/ui/Select";
import { useResource } from "../../hooks/useResource";
import { digitsOnly, formatDate } from "../../lib/format";
import { BUSINESS_TYPE_SHORT, POSITION_LABELS } from "../../lib/labels";
import { clearAfterLogin } from "../../lib/session";
import { innError } from "../../lib/validation";
import { useAuth } from "../../state/auth";
import type { RegistryCompany, RepresentativePosition } from "../../types";

type Lookup =
  | { state: "idle" }
  | { state: "loading" }
  | { state: "found"; company: RegistryCompany }
  | { state: "invalid"; message: string }
  | { state: "notFound" }
  | { state: "unavailable" };

// Проверяем только формат: синтетические ИНН демо-компаний не проходят контрольную сумму
const lookupInnError = (value: string) => (/^(?:[0-9]{10}|[0-9]{12})$/.test(value) ? null : "Введите 10 или 12 цифр ИНН");

const POSITION_OPTIONS = (Object.keys(POSITION_LABELS) as RepresentativePosition[]).map((value) => ({ value, label: POSITION_LABELS[value] }));

/** Бизнес должен существовать минимум 6 месяцев (словарь данных, Miro). */
function isTooYoung(registrationDate: string) {
  const limit = new Date();
  limit.setMonth(limit.getMonth() - 6);
  return new Date(registrationDate) > limit;
}

/**
 * Экран 03 «Первичное заполнение компании».
 * Тот же экран открывается для изменения данных из раздела «Компания и документы».
 */
export function CompanyFormPage({ mode }: { mode: "onboarding" | "edit" }) {
  const { token, user } = useAuth();
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const next = params.get("next");

  const existing = useResource(() => companyApi.get(token), [token]);

  const [inn, setInn] = useState("");
  const [lookup, setLookup] = useState<Lookup>({ state: "idle" });
  const [name, setName] = useState(user?.name ?? "");
  const [position, setPosition] = useState<RepresentativePosition | null>("director");
  const [positionOther, setPositionOther] = useState("");
  const [touched, setTouched] = useState(false);
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState("");
  const lookupId = useRef(0);

  // Онбординг начался — флаг перенаправления после регистрации больше не нужен
  useEffect(() => {
    clearAfterLogin();
  }, []);

  // Подставляем уже сохранённые данные при редактировании
  useEffect(() => {
    const company = existing.data;
    if (!company) return;
    setInn(company.inn);
    setLookup({ state: "found", company });
    setName(company.representativeName || user?.name || "");
    setPosition(company.representativePosition);
    setPositionOther(company.representativePositionOther);
  }, [existing.data, user?.name]);

  const runLookup = async (value: string) => {
    const error = lookupInnError(value);
    if (error) {
      setLookup({ state: "invalid", message: error });
      return;
    }
    const id = ++lookupId.current;
    setLookup({ state: "loading" });
    try {
      const company = await companyApi.lookup(value, token);
      if (id === lookupId.current) setLookup({ state: "found", company });
    } catch (reason) {
      if (id !== lookupId.current) return;
      if (reason instanceof ApiError && reason.status === 404) setLookup({ state: "notFound" });
      else if (reason instanceof ApiError && reason.status === 400) setLookup({ state: "invalid", message: "Некорректный ИНН" });
      else setLookup({ state: "unavailable" });
    }
  };

  const onInnChange = (value: string) => {
    const digits = digitsOnly(value).slice(0, 12);
    setInn(digits);
    // Ищем автоматически, когда введено 10 или 12 цифр — без лишней кнопки.
    // На 10 цифрах ИП ещё может дописывать номер, поэтому 10-значный ИНН,
    // не прошедший проверку, не считаем ошибкой, пока не введено 12.
    if (digits.length === 12 || (digits.length === 10 && !lookupInnError(digits))) void runLookup(digits);
    else {
      lookupId.current += 1;
      setLookup({ state: "idle" });
    }
  };

  const found = lookup.state === "found" ? lookup.company : null;
  const tooYoung = found ? isTooYoung(found.registrationDate) : false;
  const nameError = touched && name.trim().split(/\s+/).length < 2 ? "Укажите ФИО полностью" : null;
  const positionOtherError = touched && position === "other" && !positionOther.trim() ? "Укажите должность" : null;
  const innFieldError = lookup.state === "invalid" ? lookup.message : touched && !found ? "Найдите компанию по ИНН" : null;

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setTouched(true);
    setSaveError("");
    if (!found || tooYoung || nameError || !name.trim() || !position || (position === "other" && !positionOther.trim())) return;
    setSaving(true);
    try {
      await companyApi.save(token, {
        ...found,
        representativeName: name.trim(),
        representativePosition: position,
        representativePositionOther: position === "other" ? positionOther.trim() : "",
      });
      navigate(next ?? (mode === "onboarding" ? "/products" : "/company"), { replace: true });
    } catch (reason) {
      setSaveError(reason instanceof ApiError ? reason.serverText : "Не удалось сохранить компанию. Повторите попытку.");
    } finally {
      setSaving(false);
    }
  };

  return (
    <Page
      crumbs={[{ label: mode === "onboarding" ? "Данные компании" : "Компания и документы", to: mode === "edit" ? "/company" : undefined }, ...(mode === "edit" ? [{ label: "Изменение данных" }] : [])]}
      title={mode === "onboarding" ? "Расскажите о компании" : "Изменение данных компании"}
      subtitle="Найдите компанию в базе сервиса по ИНН."
    >
      <div className="columns">
        <Card as="div">
          <form className="stack stack--24" onSubmit={submit} noValidate>
            <TextField
              label="ИНН компании или ИП"
              inputMode="numeric"
              autoComplete="off"
              placeholder="10 цифр для ООО или 12 для ИП"
              value={inn}
              error={innFieldError}
              onChange={(event) => onInnChange(event.target.value)}
              onBlur={() => inn && !found && lookup.state === "idle" && void runLookup(inn)}
              end={
                lookup.state === "loading" ? (
                  <LoaderCircle size={20} className="spin muted" aria-label="Проверяем ИНН" />
                ) : found ? (
                  <CircleCheck size={20} className="muted" aria-label="Компания найдена" />
                ) : null
              }
            />

            <LookupResult lookup={lookup} tooYoung={tooYoung} onRetry={() => void runLookup(inn)} />

            <div className="form-grid">
              <TextField label="ФИО представителя" autoComplete="name" value={name} error={nameError} readOnly onChange={(event) => setName(event.target.value)} />
              <Select label="Должность" value={position} options={POSITION_OPTIONS} onChange={setPosition} />
              {position === "other" && (
                <TextField
                  className="form-grid__wide"
                  label="Укажите должность"
                  value={positionOther}
                  error={positionOtherError}
                  onChange={(event) => setPositionOther(event.target.value)}
                />
              )}
            </div>

            {saveError && <Alert>{saveError}</Alert>}
            <Divider />
            <div className="actions-row actions-row--between">
              <span className="hint-text">
                {mode === "onboarding" ? "Данные можно уточнить позже" : "Изменения попадут в новые заявки"}
              </span>
              <div className="actions-row">
                {mode === "edit" && (
                  <Button variant="secondary" onClick={() => navigate("/company")}>
                    Отмена
                  </Button>
                )}
                <Button type="submit" loading={saving} disabled={tooYoung}>
                  Сохранить компанию
                </Button>
              </div>
            </div>
          </form>
        </Card>
        <div className="columns__side">
          <InfoBlock title="Зачем нужны данные компании">Они помогут подготовить документы и связать будущие заявки с вашим бизнесом.</InfoBlock>
          <InfoBlock title="Откуда берутся данные" icon={<LockKeyhole size={20} />}>
            Реквизиты загружаются из базы сервиса (реестр ЕГРЮЛ/ЕГРИП смоделирован). Изменить их вручную нельзя.
          </InfoBlock>
        </div>
      </div>
    </Page>
  );
}

/** Результат поиска по ИНН: найдено / загрузка / ошибки (состояния из макета 03). */
function LookupResult({ lookup, tooYoung, onRetry }: { lookup: Lookup; tooYoung: boolean; onRetry: () => void }) {
  if (lookup.state === "found") {
    const company = lookup.company;
    return (
      <div className="lookup lookup--found">
        <div className="card__header">
          <p className="lookup__name">{company.name}</p>
          <Badge tone={tooYoung ? "danger" : "success"}>{tooYoung ? "Не подходит" : "Найдена в реестре"}</Badge>
        </div>
        <Details
          columns={2}
          items={[
            { label: "Форма организации", value: BUSINESS_TYPE_SHORT[company.businessType] },
            { label: "Дата регистрации", value: formatDate(company.registrationDate) },
            { label: "Основной ОКВЭД", value: `${company.okvedCode} · ${company.okvedName}` },
          ]}
        />
        {tooYoung ? (
          <p className="field__error">Бизнес зарегистрирован меньше 6 месяцев назад — заявку на кредит пока подать нельзя.</p>
        ) : (
          <span className="inline-note">
            <LockKeyhole size={13} aria-hidden="true" />
            Данные реестра доступны только для чтения
          </span>
        )}
      </div>
    );
  }

  if (lookup.state === "loading") {
    return (
      <div className="lookup">
        <p className="lookup__title">
          <LoaderCircle size={20} className="spin lookup__icon--accent" aria-hidden="true" />
          Проверяем ИНН
        </p>
        <p className="muted">Получаем сведения из ЕГРЮЛ / ЕГРИП. Обычно это занимает несколько секунд.</p>
      </div>
    );
  }

  if (lookup.state === "notFound") {
    return (
      <div className="lookup">
        <p className="lookup__title">
          <SearchX size={20} aria-hidden="true" />
          Компания не найдена
        </p>
        <p className="muted">Проверьте ИНН и повторите поиск. Данные новой компании могут появиться позже.</p>
        <button type="button" className="text-link" onClick={onRetry}>
          Повторить поиск
        </button>
      </div>
    );
  }

  if (lookup.state === "unavailable") {
    return (
      <div className="lookup">
        <p className="lookup__title">
          <ServerCrash size={20} aria-hidden="true" />
          Реестр временно недоступен
        </p>
        <p className="muted">Не удалось получить сведения. Попробуйте ещё раз через минуту.</p>
        <button type="button" className="text-link" onClick={onRetry}>
          Повторить поиск
        </button>
      </div>
    );
  }

  if (lookup.state === "invalid") {
    return (
      <div className="lookup">
        <p className="lookup__title">
          <CircleAlert size={20} className="lookup__icon--accent" aria-hidden="true" />
          Неверный ИНН
        </p>
        <p className="muted">Введите 10 цифр для ООО или 12 для ИП.</p>
      </div>
    );
  }

  return null;
}
