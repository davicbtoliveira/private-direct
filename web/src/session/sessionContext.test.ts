import { describe, it, expect } from "vitest";
import {
  initialSessionState,
  sessionReducer,
} from "../session/sessionContext";
import type { User } from "../api/types";

const alice: User = { id: 1, username: "alice" };

describe("sessionReducer", () => {
  it("starts in restoring", () => {
    expect(initialSessionState.status).toBe("restoring");
  });

  it("restores an authenticated user", () => {
    const next = sessionReducer(initialSessionState, {
      type: "restore_success",
      user: alice,
    });
    expect(next).toEqual({ status: "authenticated", user: alice, error: null });
  });

  it("returns to login on restore failure", () => {
    const next = sessionReducer(initialSessionState, { type: "restore_failure" });
    expect(next.status).toBe("unauthenticated");
  });

  it("records a login success", () => {
    const next = sessionReducer(
      { status: "unauthenticated", user: null, error: null },
      { type: "login_success", user: alice }
    );
    expect(next).toEqual({ status: "authenticated", user: alice, error: null });
  });

  it("clears everything on logout", () => {
    const next = sessionReducer(
      { status: "authenticated", user: alice, error: null },
      { type: "logout" }
    );
    expect(next).toEqual({ status: "unauthenticated", user: null, error: null });
  });

  it("records an error message without changing auth status", () => {
    const next = sessionReducer(
      { status: "authenticated", user: alice, error: null },
      { type: "error", message: "refresh_failed" }
    );
    expect(next.status).toBe("authenticated");
    expect(next.error).toBe("refresh_failed");
  });
});
