import { ArrowUpRight, Check, LockKeyhole } from "lucide-react";
import { useEffect, type ReactNode } from "react";
import { Logo } from "../../components/ui/Logo";

const PROMO = {
  login: ["Ваш бизнес.", "Следующий шаг."],
  register: ["Большие планы.", "Простой старт."],
} as const;

/** Экран входа/регистрации: форма слева, промо-блок справа (экраны 01 и 02). */
export function AuthLayout({ mode, title, subtitle, children }: { mode: "login" | "register"; title: string; subtitle: string; children: ReactNode }) {
  useEffect(() => {
    document.title = `${mode === "login" ? "Вход" : "Регистрация"} — AlphaHub`;
  }, [mode]);

  return (
    <div className="auth">
      <section className="auth__panel">
        <Logo />
        <div className="auth__form">
          <div className="auth__heading">
            <h1 className="auth__title">{title}</h1>
            <p className="auth__subtitle">{subtitle}</p>
          </div>
          {children}
        </div>
        <footer className="auth__footer">
          <span>© AlphaHub, 2026</span>
          <span className="auth__secure">
            <LockKeyhole size={14} aria-hidden="true" />
            Защищённое соединение
          </span>
        </footer>
      </section>

      <aside className="promo" aria-label="О сервисе">
        <span className="promo__tag">Финансирование малого бизнеса</span>
        <div className="promo__lead">
          <h2 className="promo__title">
            {PROMO[mode][0]}
            <br />
            {PROMO[mode][1]}
          </h2>
          <p className="promo__text">Кредитные продукты, документы и заявки — в одном личном кабинете.</p>
        </div>
        <div className="promo__card">
          <div className="promo__card-head">
            <span>Оборотный кредит</span>
            <ArrowUpRight size={20} aria-hidden="true" />
          </div>
          <div className="promo__amount">
            <span>Для повседневных задач бизнеса</span>
            <strong>до 10 млн ₽</strong>
          </div>
          {/* Декоративный график роста — высоты столбцов из макета */}
          <div className="promo__chart" aria-hidden="true">
            {[40, 65, 58, 90, 110, 140].map((height) => (
              <i key={height} style={{ height }} />
            ))}
          </div>
          <div className="divider" />
          <p className="promo__note">На закупки, расчёты с поставщиками и новые возможности.</p>
        </div>
        <ul className="promo__benefits">
          {["Документы всегда под рукой", "Статусы заявок без звонков", "Всё для бизнеса в одном месте"].map((text) => (
            <li key={text}>
              <Check size={20} aria-hidden="true" />
              {text}
            </li>
          ))}
        </ul>
        <p className="promo__footnote">Сервис для предпринимателей и небольших компаний.</p>
      </aside>
    </div>
  );
}
