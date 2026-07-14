import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { expect, it, vi } from "vitest";
import { api } from "../api/client";
import E2EESetupPage from "../pages/E2EESetupPage";
import { SessionContext, type SessionState } from "../session/sessionContext";

const { phrase, payload } = vi.hoisted(() => ({
  phrase: Array.from({ length: 24 }, (_, index) => `word${index + 1}`).join(" "),
  payload: {
    device_id: "device-one",
    identity_keys: { master_key: "public" },
    device_keys: { device_keys: { "ed25519:device-one": "public" } },
    wrapped_master_key: "ciphertext",
    kdf_salt: "salt",
    protocol_version: 1 as const,
  },
}));

vi.mock("./setup", () => ({
  createRecoveryPhrase: () => phrase,
  createE2EESetup: vi.fn().mockResolvedValue(payload),
}));

it("requires recovery confirmation then completes first-device setup", async () => {
  const markE2EEReady = vi.fn();
  vi.spyOn(api, "setupE2EE").mockResolvedValue({ e2ee_ready: true });
  const state: SessionState = {
    status: "authenticated",
    user: { id: 1, username: "alice", e2ee_ready: false },
    error: null,
  };
  render(
    <SessionContext.Provider value={{ state, login: vi.fn(), logout: vi.fn(), markE2EEReady }}>
      <MemoryRouter initialEntries={["/setup-encryption"]}>
        <Routes>
          <Route path="/setup-encryption" element={<E2EESetupPage />} />
          <Route path="/chat" element={<div>chat ready</div>} />
        </Routes>
      </MemoryRouter>
    </SessionContext.Provider>,
  );
  const user = userEvent.setup();
  expect(screen.getByLabelText("Recovery phrase")).toHaveTextContent(phrase);
  await user.click(screen.getByRole("button", { name: /saved my recovery phrase/i }));
  expect(screen.getByRole("alert")).toHaveTextContent("does not match");

  await user.type(screen.getByLabelText("Confirm recovery phrase"), phrase);
  await user.click(screen.getByRole("button", { name: /saved my recovery phrase/i }));

  expect(await screen.findByText("chat ready")).toBeInTheDocument();
  expect(api.setupE2EE).toHaveBeenCalledWith(payload);
  expect(markE2EEReady).toHaveBeenCalledOnce();
});
