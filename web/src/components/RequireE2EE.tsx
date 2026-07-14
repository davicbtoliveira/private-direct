import { Navigate, Outlet } from "react-router-dom";
import { useSession } from "../session/sessionContext";

export default function RequireE2EE() {
  const { state } = useSession();
  if (state.user?.e2ee_ready === false) return <Navigate to="/setup-encryption" replace />;
  return <Outlet />;
}
