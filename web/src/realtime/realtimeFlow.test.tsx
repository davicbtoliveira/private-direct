import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";
import App from "../App";
import * as client from "../api/client";
import FakeWebSocket from "../test/FakeWebSocket";

const alice = { id: 1, username: "alice" };
const bob = { id: 2, username: "bob" };
const charlie = { id: 3, username: "charlie" };

function setup() {
  return render(
    <MemoryRouter initialEntries={["/chat"]}>
      <App />
    </MemoryRouter>
  );
}

beforeEach(() => {
  vi.restoreAllMocks();
  FakeWebSocket.reset();
  client.setAccessToken(null);
  vi.spyOn(client.api, "refresh").mockResolvedValue({
    access_token: "first-token",
    token_type: "Bearer",
    expires_in: 900,
    user: alice,
  });
  vi.spyOn(client.api, "listContacts").mockResolvedValue({
    contacts: [bob, charlie],
  });
  vi.spyOn(client.api, "incomingRequests").mockResolvedValue({ requests: [] });
});

describe("realtime workspace", () => {
  it("uses header protocols and applies snapshot then live presence", async () => {
    setup();
    await waitFor(() => expect(FakeWebSocket.instances).toHaveLength(1));
    const socket = FakeWebSocket.instances[0];
    const url = new URL(socket.url);
    expect(url.protocol).toBe("ws:");
    expect(url.host).toBe(window.location.host);
    expect(url.pathname).toBe("/api/ws");
    expect(url.search).toBe("");
    expect(socket.url).not.toContain("first-token");
    expect(socket.requestedProtocols).toEqual(["private-direct", "first-token"]);

    await screen.findByRole("link", { name: "@bob, connecting" });
    act(() => {
      socket.open();
      socket.receive({ type: "presence_snapshot", online_users: [bob] });
    });
    expect(screen.getByRole("link", { name: "@bob, online" })).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "@charlie, offline" })
    ).toBeInTheDocument();

    act(() => {
      socket.receive({ type: "presence", user: charlie, online: true });
    });
    expect(
      screen.getByRole("link", { name: "@charlie, online" })
    ).toBeInTheDocument();
  });

  it("returns presence to connecting and reconnects with the latest token", async () => {
    setup();
    await waitFor(() => expect(FakeWebSocket.instances).toHaveLength(1));
    const first = FakeWebSocket.instances[0];
    act(() => {
      first.open();
      first.receive({ type: "presence_snapshot", online_users: [bob] });
      first.serverClose();
    });
    expect(screen.getByRole("link", { name: "@bob, connecting" })).toBeInTheDocument();

    client.setAccessToken("refreshed-token");
    await waitFor(() => expect(FakeWebSocket.instances).toHaveLength(2), {
      timeout: 1_500,
    });
    expect(FakeWebSocket.instances[1].requestedProtocols).toEqual([
      "private-direct",
      "refreshed-token",
    ]);
  });

  it("stops reconnecting when another tab replaces the session", async () => {
    setup();
    await waitFor(() => expect(FakeWebSocket.instances).toHaveLength(1));
    const socket = FakeWebSocket.instances[0];
    act(() => {
      socket.open();
      socket.receive({ type: "presence_snapshot", online_users: [] });
      socket.receive({ type: "session_replaced" });
    });

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Messaging continued in another tab"
    );
    await new Promise((resolve) => window.setTimeout(resolve, 1_100));
    expect(FakeWebSocket.instances).toHaveLength(1);
  });

  it("ignores queued events after replacement begins closing", async () => {
    setup();
    await waitFor(() => expect(FakeWebSocket.instances).toHaveLength(1));
    const socket = FakeWebSocket.instances[0];
    socket.deferClose = true;
    act(() => {
      socket.open();
      socket.receive({ type: "presence_snapshot", online_users: [] });
      socket.receive({ type: "session_replaced" });
      socket.receive({ type: "presence", user: bob, online: true });
    });

    expect(screen.getByRole("link", { name: "@bob, connecting" })).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "@bob, online" })).not.toBeInTheDocument();
    act(() => socket.completeClose());
  });

  it("refetches contact state idempotently on duplicate invalidations", async () => {
    vi.mocked(client.api.listContacts).mockResolvedValue({ contacts: [bob] });
    setup();
    await waitFor(() => expect(FakeWebSocket.instances).toHaveLength(1));
    const socket = FakeWebSocket.instances[0];
    act(() => {
      socket.open();
      socket.receive({ type: "presence_snapshot", online_users: [] });
      socket.receive({ type: "contacts_changed" });
      socket.receive({ type: "contacts_changed" });
    });

    await waitFor(() => {
      expect(client.api.listContacts).toHaveBeenCalledTimes(3);
      expect(client.api.incomingRequests).toHaveBeenCalledTimes(3);
    });
    expect(screen.getAllByRole("link", { name: "@bob, offline" })).toHaveLength(1);
  });

  it("rejects application events received before the snapshot", async () => {
    setup();
    await waitFor(() => expect(FakeWebSocket.instances).toHaveLength(1));
    const socket = FakeWebSocket.instances[0];
    act(() => {
      socket.open();
      socket.receive({ type: "presence", user: bob, online: true });
    });

    expect(socket.closeCode).toBe(1002);
    expect(screen.getByRole("link", { name: "@bob, connecting" })).toBeInTheDocument();
  });

  it("rejects a server that negotiates the wrong application protocol", async () => {
    setup();
    await waitFor(() => expect(FakeWebSocket.instances).toHaveLength(1));
    const socket = FakeWebSocket.instances[0];

    act(() => socket.open("other-protocol"));

    expect(socket.closeCode).toBe(1002);
  });

  it("keeps one realtime connection while navigating conversations", async () => {
    setup();
    const user = userEvent.setup();
    await waitFor(() => expect(FakeWebSocket.instances).toHaveLength(1));

    await user.click(
      await screen.findByRole("link", { name: "@bob, connecting" })
    );

    expect(
      within(screen.getByRole("region", { name: "Conversation" })).getByText("@bob")
    ).toBeInTheDocument();
    expect(FakeWebSocket.instances).toHaveLength(1);
  });
});
