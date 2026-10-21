import { CircleAlert, Info } from "lucide-react";
import type { CSSProperties, ReactNode } from "react";

export function Card({ children, compact = false, className = "", as: Tag = "section" }: { children: ReactNode; compact?: boolean; className?: string; as?: "section" | "div" | "article" }) {
  return <Tag className={`card ${compact ? "card--compact" : ""} ${className}`}>{children}</Tag>;
}

export function CardHeader({ title, subtitle, aside }: { title: ReactNode; subtitle?: ReactNode; aside?: ReactNode }) {
  return (
    <div className="card__header">
      <div>
        <h2 className="card__title">{title}</h2>
        {subtitle && <p className="card__subtitle">{subtitle}</p>}
      </div>
      {aside}
    </div>
  );
}

export function Divider() {
  return <div className="divider" role="presentation" />;
}

/** Сетка «подпись — значение» для реквизитов. */
export function Details({ items, columns = 3 }: { items: Array<{ label: string; value: ReactNode }>; columns?: number }) {
  return (
    <dl className="details" style={{ "--details-columns": columns } as CSSProperties}>
      {items.map((item) => (
        <div className="details__item" key={item.label}>
          <dt className="details__label">{item.label}</dt>
          <dd className="details__value">{item.value}</dd>
        </div>
      ))}
    </dl>
  );
}

export function InfoBlock({ title, children, accent = false, icon }: { title?: ReactNode; children?: ReactNode; accent?: boolean; icon?: ReactNode }) {
  return (
    <div className={`info-block ${accent ? "info-block--accent" : ""}`}>
      <span className="info-block__icon">{icon ?? <Info size={20} />}</span>
      <div className="info-block__body">
        {title && <p className="info-block__title">{title}</p>}
        {children && <div className="info-block__text">{children}</div>}
      </div>
    </div>
  );
}

export function Alert({ children, tone = "error" }: { children: ReactNode; tone?: "error" | "success" }) {
  return (
    <div className={`alert alert--${tone}`} role={tone === "error" ? "alert" : "status"}>
      <CircleAlert size={18} aria-hidden="true" />
      <span>{children}</span>
    </div>
  );
}

/** Пустое состояние или ошибка загрузки внутри страницы. */
export function StateMessage({ icon, title, text, action }: { icon?: ReactNode; title: string; text?: ReactNode; action?: ReactNode }) {
  return (
    <div className="state">
      {icon && <span className="state__icon">{icon}</span>}
      <p className="state__title">{title}</p>
      {text && <p className="state__text">{text}</p>}
      {action}
    </div>
  );
}
