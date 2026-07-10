import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { describe, it, expect } from "vitest";
import App from "./App";

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <App />
    </MemoryRouter>
  );
}

describe("App routes", () => {
  it("renders the login screen by default", () => {
    renderAt("/");
    expect(screen.getByRole("heading", { name: "Sign in" })).toBeInTheDocument();
  });

  it("renders the registration screen", () => {
    renderAt("/register");
    expect(screen.getByRole("heading", { name: "Register" })).toBeInTheDocument();
  });

  it("renders the workspace for a conversation with a dotted username", () => {
    renderAt("/chat/alice.b");
    expect(screen.getByTestId("workspace")).toBeInTheDocument();
    expect(screen.getByText("@alice.b")).toBeInTheDocument();
  });
});
