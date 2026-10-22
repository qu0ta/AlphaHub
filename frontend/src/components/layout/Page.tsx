import { ChevronRight } from "lucide-react";
import { Fragment, useEffect, type ReactNode } from "react";
import { Link } from "react-router-dom";
import { useAuth } from "../../state/auth";

export interface Crumb {
  label: string;
  to?: string;
}

/**
 * Страница кабинета: верхняя панель с хлебными крошками + заголовок + содержимое.
 * Одинаковая для всех экранов, чтобы отступы нигде не «плыли».
 */
export function Page({
  crumbs,
  title,
  subtitle,
  meta,
  actions,
  children,
}: {
  crumbs: Crumb[];
  title?: ReactNode;
  subtitle?: ReactNode;
  meta?: ReactNode;
  actions?: ReactNode;
  children: ReactNode;
}) {
  const { user } = useAuth();
  const root: Crumb = user?.role === "BANK_EMPLOYEE" ? { label: "Кабинет сотрудника", to: "/bank/applications" } : { label: "Личный кабинет", to: "/applications" };
  const all = [root, ...crumbs];

  // Заголовок вкладки браузера
  const lastLabel = crumbs[crumbs.length - 1]?.label;
  useEffect(() => {
    document.title = lastLabel ? `${lastLabel} — AlphaHub` : "AlphaHub";
  }, [lastLabel]);

  return (
    <>
      <header className="topbar">
        <nav aria-label="Навигационная цепочка">
          <ol className="breadcrumbs">
            {all.map((crumb, index) => {
              const isLast = index === all.length - 1;
              return (
                <Fragment key={`${crumb.label}-${index}`}>
                  <li className={`breadcrumbs__item ${isLast ? "breadcrumbs__item--current" : ""}`} aria-current={isLast ? "page" : undefined}>
                    {crumb.to && !isLast ? <Link to={crumb.to}>{crumb.label}</Link> : crumb.label}
                  </li>
                  {!isLast && (
                    <li className="breadcrumbs__item" aria-hidden="true">
                      <ChevronRight size={16} />
                    </li>
                  )}
                </Fragment>
              );
            })}
          </ol>
        </nav>
      </header>
      <div className="page">
        {(title || actions) && (
          <div className="page-header">
            <div className="page-header__text">
              {title && <h1 className="page-header__title">{title}</h1>}
              {subtitle && <p className="page-header__subtitle">{subtitle}</p>}
              {meta && <div className="page-header__meta">{meta}</div>}
            </div>
            {actions && <div className="page-header__actions">{actions}</div>}
          </div>
        )}
        {children}
      </div>
    </>
  );
}
