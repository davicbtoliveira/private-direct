import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Routes, Route } from "react-router-dom";
import { afterEach, beforeEach, describe, it, expect, vi } from "vitest";
import * as client from "../api/client";
import SessionProvider from "./SessionProvider";
import LoginPage from "../pages/LoginPage";

function setup() {
  return render(
    <MemoryRouter initialEntries={["/login"]}>
      <SessionProvider>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route path="/chat" element={<div>workspace</div>} />
        </Routes>
      </SessionProvider>
    </MemoryRouter>
  );
}

beforeEach(() => {
  vi.restoreAllMocks();
  client.setAccessToken(null);
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("login flow", () => {
  it("logs in and navigates to the workspace", async () => {
    const user = userEvent.setup();
    vi.spyOn(client.api, "refresh").mockRejectedValue(
      Object.assign(new Error("invalid_refresh"), { status: 401 })
    );
    vi.spyOn(client.api, "login").mockResolvedValue({
      access_token: "tok",
      token_type: "Bearer",
      expires_in: 900,
      user: { id: 1, username: "alice" },
    });

    setup();
    await waitFor(() => {
      expect(screen.getByRole("heading", { name: "Sign in" })).toBeInTheDocument();
    });

    await user.type(screen.getByLabelText("Username"), "alice");
    await user.type(screen.getByLabelText("Password"), "secret-password");
    await user.click(screen.getByRole("button", { name: "Sign in" }));

    await waitFor(() => {
      expect(screen.getByText("workspace")).toBeInTheDocument();
    });
    expect(client.getAccessToken()).toBe("tok");
  });

  it("shows a generic error without disclosing the username on bad credentials", async () => {
    const user = userEvent.setup();
    vi.spyOn(client.api, "refresh").mockRejectedValue(
      Object.assign(new Error("invalid_refresh"), { status: 401 })
    );
    vi.spyOn(client.api, "login").mockRejectedValue(
      Object.assign(new Error("invalid_credentials"), { status: 401 })
    );

    setup();
    await waitFor(() => {
      expect(screen.getByRole("heading", { name: "Sign in" })).toBeInTheDocument();
    });

    await user.type(screen.getByLabelText("Username"), "ghost");
    await user.type(screen.getByLabelText("Password"), "wrong-password");
    await user.click(screen.getByRole("button", { name: "Sign in" }));

    await waitFor(() => {
      expect(
        screen.getByText("Username or password is incorrect.")
      ).toBeInTheDocument();
    });
    expect(client.getAccessToken()).toBeNull();
  });
});
