import { Building2, LoaderCircle, Pencil, ShieldCheck, Trash2 } from "lucide-react";
import { useState } from "react";
import { Link, Navigate } from "react-router-dom";
import { ApiError, companyApi } from "../../api/client";
import { Page } from "../../components/layout/Page";
import { Badge } from "../../components/ui/Badge";
import { Button } from "../../components/ui/Button";
import { Card, CardHeader, Details, Divider, InfoBlock, StateMessage } from "../../components/ui/Card";
import { Modal } from "../../components/ui/Modal";
import { FileRow, pdfError, UploadZone } from "../../components/ui/Upload";
import { usePolling, useResource } from "../../hooks/useResource";
import { formatDate, formatDateAt, formatFileSize, plural } from "../../lib/format";
import { BUSINESS_TYPE_FULL, positionText } from "../../lib/labels";
import { useAuth } from "../../state/auth";
import type { CompanyDocument } from "../../types";

/** Экраны 04 и 05 «Компания и документы» — пустое и заполненное состояние. */
export function CompanyPage() {
  const { token } = useAuth();
  const company = useResource(() => companyApi.get(token), [token]);
  const documents = useResource(() => companyApi.listDocuments(token), [token]);
  const [uploadError, setUploadError] = useState<string | null>(null);
  const [uploading, setUploading] = useState(false);
  const [toDelete, setToDelete] = useState<CompanyDocument | null>(null);
  const [deleting, setDeleting] = useState(false);

  const docs = documents.data ?? [];
  const processing = docs.some((doc) => doc.status === "PROCESSING");
  // Пока документ обрабатывается — обновляем статус каждые 3 секунды
  usePolling(() => void documents.reload({ silent: true }), processing, 3000);

  if (company.loading) return <Page crumbs={[{ label: "Компания и документы" }]} title="Компания и документы"><Card>Загружаем данные компании…</Card></Page>;
  if (company.error) {
    return (
      <Page crumbs={[{ label: "Компания и документы" }]} title="Компания и документы">
        <Card>
          <StateMessage title="Не удалось загрузить данные" text="Проверьте соединение и повторите попытку." action={<Button onClick={() => void company.reload()}>Повторить</Button>} />
        </Card>
      </Page>
    );
  }
  // Компании ещё нет — сначала первичное заполнение
  if (!company.data) return <Navigate to="/onboarding/company" replace />;

  const data = company.data;

  const upload = async (file: File) => {
    const error = pdfError(file);
    setUploadError(error);
    if (error) return;
    setUploading(true);
    try {
      await companyApi.uploadDocument(token, file);
      await documents.reload({ silent: true });
    } catch (reason) {
      setUploadError(reason instanceof ApiError ? reason.serverText : "Не удалось загрузить файл");
    } finally {
      setUploading(false);
    }
  };

  const confirmDelete = async () => {
    if (!toDelete) return;
    setDeleting(true);
    try {
      await companyApi.deleteDocument(token, toDelete.id);
      await documents.reload({ silent: true });
      setToDelete(null);
    } finally {
      setDeleting(false);
    }
  };

  const doc = docs[0];

  return (
    <Page
      crumbs={[{ label: "Компания и документы" }]}
      title="Компания и документы"
      subtitle={doc ? "Реквизиты компании и актуальные документы для ваших заявок." : "Проверьте реквизиты и добавьте документ компании."}
    >
      <Card>
        <div className="company-head">
          <span className="icon-tile" aria-hidden="true">
            <Building2 size={24} />
          </span>
          <div className="company-head__text">
            <p className="company-head__name">{data.name}</p>
            <p className="muted">{BUSINESS_TYPE_FULL[data.businessType]}</p>
          </div>
          <Badge tone="success">Данные из реестра</Badge>
          <Link to="/company/edit" className="text-link">
            <Pencil size={16} aria-hidden="true" />
            Изменить
          </Link>
        </div>
        <Divider />
        <Details
          items={[
            { label: "ИНН", value: data.inn },
            { label: "Дата регистрации", value: formatDate(data.registrationDate) },
            { label: "Основной ОКВЭД", value: `${data.okvedCode} · ${data.okvedName}` },
            { label: "Представитель", value: data.representativeName },
            { label: "Должность", value: positionText(data.representativePosition, data.representativePositionOther) },
            { label: "Источник данных", value: `${data.source === "EGRUL" ? "ЕГРЮЛ" : "ЕГРИП"} · Только для чтения` },
          ]}
        />
      </Card>

      <Card>
        <CardHeader
          title="Документы компании"
          subtitle={doc ? "Бухгалтерская отчётность за последний завершённый год." : "Загрузите бухгалтерскую отчётность за последний завершённый год."}
          aside={<Badge>{docs.length ? `${docs.length} ${plural(docs.length, ["документ", "документа", "документов"])}` : "Нет документов"}</Badge>}
        />

        {doc ? (
          <>
            <FileRow
              name={doc.name}
              meta={`${formatFileSize(doc.sizeBytes)} · Загружен ${formatDateAt(doc.uploadedAt)}`}
              status={doc.status === "PROCESSING" ? <Badge tone="warning">Обрабатывается</Badge> : <Badge tone="success">Проверен</Badge>}
            />
            {uploadError && <p className="field__error">{uploadError}</p>}
            <div className="actions-row actions-row--between">
              <span className="inline-note">
                {doc.status === "PROCESSING" ? (
                  <>
                    <LoaderCircle size={16} className="spin" aria-hidden="true" />
                    Проверяем файл. Это займёт несколько минут.
                  </>
                ) : (
                  <>
                    <ShieldCheck size={16} aria-hidden="true" />
                    Файл проверен и будет использован в заявках.
                  </>
                )}
              </span>
              <div className="actions-row actions-row--wide">
                <label className="button button--secondary">
                  {uploading ? "Загружаем…" : "Заменить файл"}
                  <input
                    type="file"
                    accept="application/pdf,.pdf"
                    className="visually-hidden"
                    disabled={uploading}
                    onChange={(event) => {
                      const file = event.target.files?.[0];
                      if (file) void upload(file);
                      event.target.value = "";
                    }}
                  />
                </label>
                <button type="button" className="text-link" onClick={() => setToDelete(doc)}>
                  <Trash2 size={16} aria-hidden="true" />
                  Удалить
                </button>
              </div>
            </div>
          </>
        ) : (
          <>
            <UploadZone error={uploadError} disabled={uploading} title={uploading ? "Загружаем файл…" : undefined} onFile={(file) => void upload(file)} />
            <span className="inline-note">
              <ShieldCheck size={16} aria-hidden="true" />
              Файл будет доступен только вам и специалистам, которые рассматривают заявку.
            </span>
          </>
        )}
      </Card>

      {doc && (
        <InfoBlock title={doc.status === "PROCESSING" ? "Документ сохранён" : "Всё готово"}>
          {doc.status === "PROCESSING"
            ? "Можно продолжить работу в кабинете. Статус обновится после завершения обработки."
            : "Можно подавать заявку — реквизиты и документ подставятся автоматически."}
        </InfoBlock>
      )}

      {toDelete && (
        <Modal title="Удалить документ?" onClose={() => setToDelete(null)}>
          <p className="modal__text">Файл исчезнет из документов компании. При необходимости его можно загрузить снова.</p>
          <div className="modal__actions">
            <Button variant="secondary" onClick={() => setToDelete(null)}>
              Отмена
            </Button>
            <Button loading={deleting} onClick={() => void confirmDelete()}>
              Удалить
            </Button>
          </div>
        </Modal>
      )}
    </Page>
  );
}
