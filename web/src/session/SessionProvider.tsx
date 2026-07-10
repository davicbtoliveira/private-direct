import { useCallback, useEffect, useMemo, useReducer, useRef, type ReactNode } from "react";
import { api, setAccessToken } from "../api/client";
import {
  SessionContext,
  initialSessionState,
  sessionReducer,
} from "./sessionContext";

const REFRESH_LEAD_MS = 60_000;
const REFRESH_POLL_MS = 15_000;

type Props = { children: ReactNode };

export default function SessionProvider({ children }: Props) {
  const [state, dispatch] = useReducer(sessionReducer, initialSessionState);
  const expiresInRef = useRef<number | null>(null);

  const scheduleRefresh = useCallback((expiresIn: number) => {
    const ms = Math.max(1_000, expiresIn * 1000 - REFRESH_LEAD_MS);
    expiresInRef.current = window.setTimeout(() => {
      void doRefresh();
    }, ms);
  }, []);

  const doRefresh = useCallback(async () => {
    try {
      const res = await api.refresh();
      setAccessToken(res.access_token);
      scheduleRefresh(res.expires_in);
    } catch {
      setAccessToken(null);
      dispatch({ type: "restore_failure" });
    }
  }, [scheduleRefresh]);

  useEffect(() => {
    let cancelled = false;
    let poll: number | null = null;
    (async () => {
      try {
        const res = await api.refresh();
        if (cancelled) return;
        setAccessToken(res.access_token);
        scheduleRefresh(res.expires_in);
        dispatch({ type: "restore_success", user: res.user });
      } catch {
        if (cancelled) return;
        setAccessToken(null);
        dispatch({ type: "restore_failure" });
        poll = window.setInterval(async () => {
          try {
            const res = await api.refresh();
            if (cancelled) return;
            setAccessToken(res.access_token);
            scheduleRefresh(res.expires_in);
            dispatch({ type: "restore_success", user: res.user });
            if (poll) window.clearInterval(poll);
          } catch {
            // keep waiting for a valid cookie
          }
        }, REFRESH_POLL_MS);
      }
    })();
    return () => {
      cancelled = true;
      if (poll) window.clearInterval(poll);
      if (expiresInRef.current) window.clearTimeout(expiresInRef.current);
    };
  }, [scheduleRefresh]);

  const login = useCallback(
    async (username: string, password: string) => {
      const res = await api.login(username, password);
      setAccessToken(res.access_token);
      scheduleRefresh(res.expires_in);
      dispatch({ type: "login_success", user: res.user });
    },
    [scheduleRefresh]
  );

  const logout = useCallback(async () => {
    try {
      await api.logout();
    } finally {
      setAccessToken(null);
      if (expiresInRef.current) window.clearTimeout(expiresInRef.current);
      dispatch({ type: "logout" });
    }
  }, []);

  const value = useMemo(() => ({ state, login, logout }), [state, login, logout]);

  return <SessionContext.Provider value={value}>{children}</SessionContext.Provider>;
}
