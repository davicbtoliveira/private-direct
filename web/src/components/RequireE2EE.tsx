import { Navigate, Outlet } from "react-router-dom";
import { useSession } from "../session/sessionContext";
import { hasRememberedDevice } from "../e2ee/matrixSession";

export default function RequireE2EE() {
  const { state } = useSession();
  if (state.user?.e2ee_ready === false) return <Navigate to="/setup-encryption" replace />;
  if (state.user?.e2ee_ready && state.user.protocol_version !== undefined && state.user.protocol_version !== 1) return <Navigate to="/update-encryption" replace />;
  if (state.user?.e2ee_ready && !hasRememberedDevice()) return <Navigate to="/recover-encryption" replace />;
  return <Outlet />;
}
