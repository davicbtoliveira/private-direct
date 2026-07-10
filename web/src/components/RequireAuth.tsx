import { Navigate, Outlet, useLocation } from "react-router-dom";
import { useSession } from "../session/sessionContext";

export default function RequireAuth() {
  const { state } = useSession();
  const location = useLocation();
  if (state.status === "restoring") {
    return <div aria-live="polite">Restoring session…</div>;
  }
  if (state.status === "unauthenticated") {
    return <Navigate to="/login" state={{ from: location.pathname }} replace />;
  }
  return <Outlet />;
}
