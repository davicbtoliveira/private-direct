import { Routes, Route, Navigate } from "react-router-dom";
import SessionProvider from "./session/SessionProvider";
import LoginPage from "./pages/LoginPage";
import RegisterPage from "./pages/RegisterPage";
import ChatPage from "./pages/ChatPage";
import RequireAuth from "./components/RequireAuth";
import RealtimeProvider from "./realtime/RealtimeProvider";

export default function App() {
  return (
    <SessionProvider>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/register" element={<RegisterPage />} />
        <Route element={<RequireAuth />}>
          <Route element={<RealtimeProvider />}>
            <Route path="/chat/:username" element={<ChatPage />} />
            <Route path="/chat" element={<ChatPage />} />
          </Route>
        </Route>
        <Route path="*" element={<Navigate to="/login" replace />} />
      </Routes>
    </SessionProvider>
  );
}
