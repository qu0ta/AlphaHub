import type { ReactNode } from "react";
import { Navigate, Route, Routes } from "react-router-dom";
import { AppLayout } from "./components/layout/AppLayout";
import { Button } from "./components/ui/Button";
import { LoginPage } from "./pages/auth/LoginPage";
import { RegisterPage } from "./pages/auth/RegisterPage";
import { BankApplicationPage } from "./pages/bank/BankApplicationPage";
import { DecisionPage } from "./pages/bank/DecisionPage";
import { QueuePage } from "./pages/bank/QueuePage";
import { ApplicationsPage } from "./pages/client/ApplicationsPage";
import { ApplicationStatusPage } from "./pages/client/ApplicationStatusPage";
import { CompanyFormPage } from "./pages/client/CompanyFormPage";
import { CompanyPage } from "./pages/client/CompanyPage";
import { OfferPage } from "./pages/client/OfferPage";
import { ProductsPage } from "./pages/client/ProductsPage";
import { ApplicationWizard } from "./pages/client/wizard/ApplicationWizard";
import { getAfterLogin } from "./lib/session";
import { useAuth } from "./state/auth";
import type { Role } from "./types";

const homeFor = (role: Role) => (role === "BANK_EMPLOYEE" ? "/bank/applications" : "/applications");

const afterLogin = (role: Role) => getAfterLogin() ?? homeFor(role);

/**
 * Защищённый маршрут: без входа — на /login, с чужой ролью — на свою главную.
 * Скрытие экранов по роли — удобство интерфейса; права проверяет бэкенд.
 */
function RequireRole({ role, children }: { role?: Role; children: ReactNode }) {
  const { user } = useAuth();
  if (!user) return <Navigate to="/login" replace />;
  if (role && user.role !== role) return <Navigate to={homeFor(user.role)} replace />;
  return children;
}

export function App() {
  const { user, initializing, sessionError, retrySession, logout } = useAuth();

  if (initializing) return <div className="app-loading">Проверяем сессию…</div>;

  if (sessionError) {
    return (
      <div className="app-loading">
        <div className="session-error">
          <h1>Сервис временно недоступен</h1>
          <p className="muted">Не удалось проверить сессию. Повторите попытку.</p>
          <div className="actions-row actions-row--center">
            <Button onClick={() => void retrySession()}>Повторить</Button>
            <Button variant="secondary" onClick={logout}>
              Выйти
            </Button>
          </div>
        </div>
      </div>
    );
  }

  return (
    <Routes>
      <Route path="/login" element={user ? <Navigate to={afterLogin(user.role)} replace /> : <LoginPage />} />
      <Route path="/register" element={user ? <Navigate to={afterLogin(user.role)} replace /> : <RegisterPage />} />

      <Route
        element={
          <RequireRole>
            <AppLayout />
          </RequireRole>
        }
      >
        <Route path="/onboarding/company" element={<RequireRole role="CLIENT"><CompanyFormPage mode="onboarding" /></RequireRole>} />
        <Route path="/company" element={<RequireRole role="CLIENT"><CompanyPage /></RequireRole>} />
        <Route path="/company/edit" element={<RequireRole role="CLIENT"><CompanyFormPage mode="edit" /></RequireRole>} />
        <Route path="/products" element={<RequireRole role="CLIENT"><ProductsPage /></RequireRole>} />
        <Route path="/applications" element={<RequireRole role="CLIENT"><ApplicationsPage /></RequireRole>} />
        <Route path="/applications/:id" element={<RequireRole role="CLIENT"><ApplicationStatusPage /></RequireRole>} />
        <Route path="/applications/:id/edit" element={<RequireRole role="CLIENT"><ApplicationWizard /></RequireRole>} />
        <Route path="/applications/:id/offer" element={<RequireRole role="CLIENT"><OfferPage /></RequireRole>} />

        <Route path="/bank/applications" element={<RequireRole role="BANK_EMPLOYEE"><QueuePage /></RequireRole>} />
        <Route path="/bank/applications/:id" element={<RequireRole role="BANK_EMPLOYEE"><BankApplicationPage /></RequireRole>} />
        <Route path="/bank/applications/:id/decision" element={<RequireRole role="BANK_EMPLOYEE"><DecisionPage /></RequireRole>} />
      </Route>

      <Route path="*" element={<Navigate to={user ? homeFor(user.role) : "/login"} replace />} />
    </Routes>
  );
}
