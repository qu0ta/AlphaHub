import { Archive, Building2, ClipboardCheck, Files, Inbox, LogOut, Wallet, type LucideIcon } from "lucide-react";
import { NavLink, Outlet, useLocation, useNavigate } from "react-router-dom";
import { useAuth } from "../../state/auth";
import { Logo } from "../ui/Logo";

interface NavItem {
  to: string;
  label: string;
  icon: LucideIcon;
  /** Дополнительные пути, при которых пункт считается активным */
  match?: (pathname: string, search: string) => boolean;
}

const CLIENT_NAV: NavItem[] = [
  { to: "/applications", label: "Мои заявки", icon: Files },
  { to: "/products", label: "Кредитные продукты", icon: Wallet },
  { to: "/company", label: "Компания", icon: Building2, match: (path) => path.startsWith("/onboarding") },
];

const BANK_NAV: NavItem[] = [
  {
    to: "/bank/applications",
    label: "Очередь заявок",
    icon: Inbox,
    match: (path, search) => path.startsWith("/bank/applications") && !/view=(mine|archive)/.test(search),
  },
  { to: "/bank/applications?view=mine", label: "Мои решения", icon: ClipboardCheck, match: (_, search) => search.includes("view=mine") },
  { to: "/bank/applications?view=archive", label: "Архив", icon: Archive, match: (_, search) => search.includes("view=archive") },
];

/** Каркас кабинета: боковое меню слева, страница справа. */
export function AppLayout() {
  const { user, logout } = useAuth();
  const navigate = useNavigate();
  const { pathname, search } = useLocation();
  const isBank = user?.role === "BANK_EMPLOYEE";
  const items = isBank ? BANK_NAV : CLIENT_NAV;

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div className="sidebar__top">
          <div className="sidebar__brand">
            <Logo compact />
            <span className="sidebar__caption">{isBank ? "Кабинет сотрудника" : "Кабинет бизнеса"}</span>
          </div>
          <nav className="sidebar__nav" aria-label="Разделы">
            {items.map(({ to, label, icon: Icon, match }) => {
              // Для пунктов с ?view=... стандартная проверка NavLink не подходит,
              // поэтому активность считаем сами
              const active = match ? match(pathname, search) || (!to.includes("?") && pathname.startsWith(to)) : pathname.startsWith(to);
              return (
                <NavLink key={to} to={to} className={`sidebar__link ${active ? "sidebar__link--active" : ""}`} aria-current={active ? "page" : undefined}>
                  <Icon size={20} aria-hidden="true" />
                  <span>{label}</span>
                </NavLink>
              );
            })}
          </nav>
        </div>
        <div className="sidebar__profile">
          <span className="sidebar__user" title={user?.name}>
            {user?.name}
          </span>
          <button
            type="button"
            className="sidebar__logout"
            onClick={() => {
              logout();
              navigate("/login", { replace: true });
            }}
          >
            <LogOut size={16} aria-hidden="true" />
            Выйти
          </button>
        </div>
      </aside>
      <main className="workspace">
        <Outlet />
      </main>
    </div>
  );
}
