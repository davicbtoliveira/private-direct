import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, it, expect, vi } from "vitest";
import App from "./App";
import * as client from "./api/client";

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <App />
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

describe("App routes", () => {
  it("renders the login screen when refresh fails", async () => {
    vi.spyOn(client.api, "refresh").mockRejectedValue(
      Object.assign(new Error("invalid_refresh"), { status: 401 })
    );
    renderAt("/");
    await waitFor(() => {
      expect(screen.getByRole("heading", { name: "Sign in" })).toBeInTheDocument();
    });
  });

  it("renders the registration screen", async () => {
    vi.spyOn(client.api, "refresh").mockRejectedValue(
      Object.assign(new Error("invalid_refresh"), { status: 401 })
    );
    renderAt("/register");
    await waitFor(() => {
      expect(screen.getByRole("heading", { name: "Register" })).toBeInTheDocument();
    });
  });

  it("renders the workspace for a conversation with a dotted username", async () => {
    vi.spyOn(client.api, "refresh").mockResolvedValue({
      access_token: "tok",
      token_type: "Bearer",
      expires_in: 900,
      user: { id: 1, username: "alice" },
    });
    renderAt("/chat/alice.b");
    await waitFor(() => {
      expect(screen.getByTestId("workspace")).toBeInTheDocument();
      expect(screen.getByText("@alice.b")).toBeInTheDocument();
    });
  });
});
