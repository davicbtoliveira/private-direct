import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import * as client from "../api/client";
import LoginPage from "../pages/LoginPage";
import RegisterPage from "../pages/RegisterPage";
import SessionProvider from "./SessionProvider";

function Workspace() {
  const location = useLocation();
  const warning = (location.state as { warning?: string } | null)?.warning;
  return <div>{warning ?? "workspace"}</div>;
}

function setup() {
  return render(
    <MemoryRouter initialEntries={["/register"]}>
      <SessionProvider>
        <Routes>
          <Route path="/register" element={<RegisterPage />} />
          <Route path="/login" element={<LoginPage />} />
          <Route path="/chat" element={<Workspace />} />
        </Routes>
      </SessionProvider>
    </MemoryRouter>
  );
}

async function fillValidForm(password = "secret-passphrase") {
  const user = userEvent.setup();
  await user.type(screen.getByLabelText("Invite code"), "  invite-one  ");
  await user.type(screen.getByLabelText("Username"), "  Alice.B  ");
  await user.type(screen.getByLabelText("Password"), password);
  await user.type(screen.getByLabelText("Confirm password"), password);
  return user;
}

beforeEach(() => {
  vi.restoreAllMocks();
  client.setAccessToken(null);
  vi.spyOn(client.api, "refresh").mockRejectedValue(
    Object.assign(new Error("invalid_refresh"), { status: 401 })
  );
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("registration flow", () => {
  it("normalizes registration and logs the new user in", async () => {
    vi.spyOn(client.api, "register").mockResolvedValue({ id: 1, username: "alice.b" });
    vi.spyOn(client.api, "login").mockResolvedValue({
      access_token: "tok",
      token_type: "Bearer",
      expires_in: 900,
      user: { id: 1, username: "alice.b" },
    });
    setup();
    const user = await fillValidForm();

    await user.click(screen.getByRole("button", { name: "Register" }));

    await waitFor(() => expect(screen.getByText("workspace")).toBeInTheDocument());
    expect(client.api.register).toHaveBeenCalledWith(
      "invite-one",
      "alice.b",
      "secret-passphrase"
    );
    expect(client.api.login).toHaveBeenCalledWith("alice.b", "secret-passphrase");
    expect(client.getAccessToken()).toBe("tok");
  });

  it("blocks invalid local input with field-specific guidance", async () => {
    const register = vi.spyOn(client.api, "register");
    setup();
    const user = userEvent.setup();
    await user.type(screen.getByLabelText("Invite code"), "invite-one");
    await user.type(screen.getByLabelText("Username"), "@Al");
    await user.type(screen.getByLabelText("Password"), "short");
    await user.type(screen.getByLabelText("Confirm password"), "different");

    await user.click(screen.getByRole("button", { name: "Register" }));

    expect(
      screen.getByText(
        "Use 3-32 letters, numbers, dots, dashes, or underscores; start with a letter or number."
      )
    ).toBeInTheDocument();
    expect(screen.getByText("Use 12-72 UTF-8 bytes.")).toBeInTheDocument();
    expect(screen.getByText("Passwords do not match.")).toBeInTheDocument();
    expect(screen.getByLabelText("Username")).toHaveFocus();
    expect(register).not.toHaveBeenCalled();
  });

  it("clears immediate username feedback when corrected", async () => {
    setup();
    const user = userEvent.setup();
    const username = screen.getByLabelText("Username");
    await user.type(username, "@al");
    await user.tab();
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Use 3-32 letters, numbers, dots, dashes, or underscores"
    );

    await user.click(username);
    await user.clear(username);
    await user.type(username, "Alice.B");

    expect(
      screen.queryByText(
        "Use 3-32 letters, numbers, dots, dashes, or underscores; start with a letter or number."
      )
    ).not.toBeInTheDocument();
  });

  it.each([
    ["ab", false],
    ["abc", true],
    ["a".repeat(32), true],
    ["a".repeat(33), false],
  ])("enforces username length boundary for %s", async (username, accepted) => {
    const register = vi.spyOn(client.api, "register").mockResolvedValue({
      id: 1,
      username,
    });
    vi.spyOn(client.api, "login").mockResolvedValue({
      access_token: "tok",
      token_type: "Bearer",
      expires_in: 900,
      user: { id: 1, username },
    });
    setup();
    const user = userEvent.setup();
    await user.type(screen.getByLabelText("Invite code"), "invite-one");
    await user.type(screen.getByLabelText("Username"), username);
    await user.type(screen.getByLabelText("Password"), "secret-passphrase");
    await user.type(screen.getByLabelText("Confirm password"), "secret-passphrase");
    await user.click(screen.getByRole("button", { name: "Register" }));

    if (accepted) {
      await waitFor(() => expect(register).toHaveBeenCalled());
    } else {
      expect(register).not.toHaveBeenCalled();
      expect(screen.getByLabelText("Username")).toHaveFocus();
    }
  });

  it.each(["é".repeat(6), "é".repeat(36)])(
    "accepts a password at a UTF-8 byte boundary",
    async (password) => {
      vi.spyOn(client.api, "register").mockResolvedValue({
        id: 1,
        username: "alice.b",
      });
      vi.spyOn(client.api, "login").mockResolvedValue({
        access_token: "tok",
        token_type: "Bearer",
        expires_in: 900,
        user: { id: 1, username: "alice.b" },
      });
      setup();
      const user = await fillValidForm(password);

      await user.click(screen.getByRole("button", { name: "Register" }));

      await waitFor(() => expect(client.api.register).toHaveBeenCalled());
      expect(client.api.register).toHaveBeenCalledWith(
        "invite-one",
        "alice.b",
        password
      );
    }
  );

  it.each(["a".repeat(11), "a".repeat(73)])(
    "rejects a password outside the UTF-8 byte boundaries",
    async (password) => {
      const register = vi.spyOn(client.api, "register");
      setup();
      const user = await fillValidForm(password);

      await user.click(screen.getByRole("button", { name: "Register" }));

      expect(screen.getByText("Use 12-72 UTF-8 bytes.")).toBeInTheDocument();
      expect(register).not.toHaveBeenCalled();
    }
  );

  it.each([
    ["invalid_invite", "Invite code is not valid."],
    [
      "invite_used",
      "Invite code has already been used. Ask the operator for another.",
    ],
    ["username_exists", "Username is already registered."],
    [
      "invalid_username",
      "Use 3-32 letters, numbers, dots, dashes, or underscores; start with a letter or number.",
    ],
    ["invalid_password", "Use 12-72 UTF-8 bytes."],
  ])("maps %s to actionable guidance", async (code, message) => {
    vi.spyOn(client.api, "register").mockRejectedValue(
      Object.assign(new Error(code), { status: 400, code })
    );
    setup();
    const user = await fillValidForm();

    await user.click(screen.getByRole("button", { name: "Register" }));

    expect(await screen.findByText(message)).toBeInTheDocument();
  });

  it("recovers after registration succeeds but automatic login fails", async () => {
    vi.spyOn(client.api, "register").mockResolvedValue({ id: 1, username: "alice.b" });
    vi.spyOn(client.api, "login").mockRejectedValue(
      Object.assign(new Error("login_failed"), { status: 500, code: "login_failed" })
    );
    setup();
    const user = await fillValidForm();

    await user.click(screen.getByRole("button", { name: "Register" }));

    expect(await screen.findByText("Account created. Sign in to continue.")).toBeInTheDocument();
    expect(screen.getByLabelText("Username")).toHaveValue("alice.b");
    expect(screen.getByLabelText("Password")).toHaveValue("");
  });

  it("shows a warning after registration when breach screening was unavailable", async () => {
    vi.spyOn(client.api, "register").mockResolvedValue({
      id: 1,
      username: "alice.b",
      warning: "password_breach_check_unavailable",
    });
    vi.spyOn(client.api, "login").mockResolvedValue({
      access_token: "tok",
      token_type: "Bearer",
      expires_in: 900,
      user: { id: 1, username: "alice.b" },
    });
    setup();
    const user = await fillValidForm();

    await user.click(screen.getByRole("button", { name: "Register" }));

    expect(await screen.findByText(/created without that check/)).toBeInTheDocument();
  });

  it("maps a breached password to actionable guidance", async () => {
    vi.spyOn(client.api, "register").mockRejectedValue(
      Object.assign(new Error("password_breached"), {
        status: 400,
        code: "password_breached",
      })
    );
    setup();
    const user = await fillValidForm();

    await user.click(screen.getByRole("button", { name: "Register" }));

    expect(await screen.findByText(/has not appeared in a known breach/)).toBeInTheDocument();
  });
});
