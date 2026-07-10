import { createContext, useContext } from "react";
import type { User } from "../api/types";

export type SessionStatus = "restoring" | "authenticated" | "unauthenticated";

export type SessionState = {
  status: SessionStatus;
  user: User | null;
  error: string | null;
};

export type SessionAction =
  | { type: "restore_start" }
  | { type: "restore_success"; user: User }
  | { type: "restore_failure" }
  | { type: "login_success"; user: User }
  | { type: "logout" }
  | { type: "error"; message: string };

export const initialSessionState: SessionState = {
  status: "restoring",
  user: null,
  error: null,
};

export function sessionReducer(state: SessionState, action: SessionAction): SessionState {
  switch (action.type) {
    case "restore_start":
      return { status: "restoring", user: null, error: null };
    case "restore_success":
      return { status: "authenticated", user: action.user, error: null };
    case "restore_failure":
      return { status: "unauthenticated", user: null, error: null };
    case "login_success":
      return { status: "authenticated", user: action.user, error: null };
    case "logout":
      return { status: "unauthenticated", user: null, error: null };
    case "error":
      return { ...state, error: action.message };
    default:
      return state;
  }
}

type SessionContextValue = {
  state: SessionState;
  login: (username: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
};

export const SessionContext = createContext<SessionContextValue | null>(null);

export function useSession() {
  const ctx = useContext(SessionContext);
  if (!ctx) {
    throw new Error("useSession must be used within SessionProvider");
  }
  return ctx;
}
