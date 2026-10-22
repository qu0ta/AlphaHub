import type { ReactNode } from "react";
import type { ApplicationStatus } from "../../types";

export type BadgeTone = "neutral" | "warning" | "success" | "danger";

export function Badge({ tone = "neutral", children }: { tone?: BadgeTone; children: ReactNode }) {
  return <span className={`badge badge--${tone}`}>{children}</span>;
}

/** Как статус называется для клиента. Клиенту не важны внутренние этапы банка. */
const CLIENT_STATUS: Record<ApplicationStatus, [string, BadgeTone]> = {
  DRAFT: ["Черновик", "neutral"],
  SUBMITTED: ["На рассмотрении", "warning"],
  IN_REVIEW: ["На рассмотрении", "warning"],
  MANUAL: ["На рассмотрении", "warning"],
  NEED_DATA: ["Нужны данные", "danger"],
  OFFER: ["Предложение готово", "success"],
  RETENTION_REVIEW: ["Повторное рассмотрение", "warning"],
  RETENTION_OFFER: ["Новое предложение", "success"],
  SIGNED: ["Подписание", "warning"],
  DISBURSED: ["Деньги выданы", "success"],
  REJECTED: ["Отказ банка", "danger"],
  DECLINED: ["Вы отказались", "neutral"],
};

/** Как статус называется для сотрудника банка. */
const BANK_STATUS: Record<ApplicationStatus, [string, BadgeTone]> = {
  DRAFT: ["Черновик", "neutral"],
  SUBMITTED: ["Автопроверка", "neutral"],
  IN_REVIEW: ["Автопроверка", "neutral"],
  MANUAL: ["На проверке", "warning"],
  NEED_DATA: ["Нужны данные", "neutral"],
  OFFER: ["Решение готово", "success"],
  RETENTION_REVIEW: ["Улучшение условий", "warning"],
  RETENTION_OFFER: ["Решение готово", "success"],
  SIGNED: ["Подписание", "success"],
  DISBURSED: ["Выдан", "success"],
  REJECTED: ["Отказ", "danger"],
  DECLINED: ["Клиент отказался", "neutral"],
};

export function clientStatusLabel(status: ApplicationStatus) {
  return CLIENT_STATUS[status][0];
}

export function StatusBadge({ status, audience = "client" }: { status: ApplicationStatus; audience?: "client" | "bank" }) {
  const [label, tone] = (audience === "bank" ? BANK_STATUS : CLIENT_STATUS)[status];
  return <Badge tone={tone}>{label}</Badge>;
}
