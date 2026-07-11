import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";
import App from "../App";
import * as client from "../api/client";

const alice = { id: 1, username: "alice" };
const bob = { id: 2, username: "bob" };
const request = {
  id: 7,
  username: "carol",
  status: "pending",
  created_at: "2026-07-11T12:00:00Z",
};

function setup() {
  return render(
    <MemoryRouter initialEntries={["/chat"]}>
      <App />
    </MemoryRouter>
  );
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise;
    reject = rejectPromise;
  });
  return { promise, resolve, reject };
}

beforeEach(() => {
  vi.restoreAllMocks();
  client.setAccessToken(null);
  vi.spyOn(client.api, "refresh").mockResolvedValue({
    access_token: "tok",
    token_type: "Bearer",
    expires_in: 900,
    user: alice,
  });
  vi.spyOn(client.api, "listContacts").mockResolvedValue({ contacts: [] });
  vi.spyOn(client.api, "incomingRequests").mockResolvedValue({ requests: [] });
});

describe("contact consent flow", () => {
  it("renders directed empty states", async () => {
    setup();

    expect(
      await screen.findByText("No contacts yet. Find someone by exact handle.")
    ).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Incoming requests" }));
    expect(
      await screen.findByText("No incoming requests. New requests will appear here.")
    ).toBeInTheDocument();
  });

  it("normalizes exact lookup and sends one predictable request", async () => {
    vi.spyOn(client.api, "lookupUser").mockResolvedValue(bob);
    vi.spyOn(client.api, "createContactRequest").mockResolvedValue({
      id: 9,
      username: "bob",
      status: "pending",
      created_at: "2026-07-11T12:00:00Z",
    });
    setup();
    const user = userEvent.setup();
    const trigger = await screen.findByRole("button", { name: "Add contact" });

    await user.click(trigger);
    await waitFor(() => expect(client.api.listContacts).toHaveBeenCalledTimes(2));
    const dialog = screen.getByRole("dialog", { name: "Add contact" });
    const input = within(dialog).getByLabelText("Exact username");
    await waitFor(() => expect(input).toHaveFocus());
    await user.type(input, "  BoB  ");
    await user.click(within(dialog).getByRole("button", { name: "Find contact" }));

    await waitFor(() => expect(client.api.lookupUser).toHaveBeenCalledWith("bob"));
    const send = await within(dialog).findByRole("button", {
      name: "Send request to @bob",
    });
    await user.dblClick(send);

    expect(await within(dialog).findByText("Request sent to @bob.")).toBeInTheDocument();
    expect(client.api.createContactRequest).toHaveBeenCalledTimes(1);
    expect(client.api.createContactRequest).toHaveBeenCalledWith("bob");
    expect(client.api.listContacts).toHaveBeenCalledTimes(3);

    await user.keyboard("{Escape}");
    expect(screen.queryByRole("dialog", { name: "Add contact" })).not.toBeInTheDocument();
    expect(trigger).toHaveFocus();
  });

  it("accepts an incoming request and refetches both authoritative lists", async () => {
    vi.mocked(client.api.incomingRequests)
      .mockResolvedValueOnce({ requests: [request] })
      .mockResolvedValueOnce({ requests: [request] })
      .mockResolvedValue({ requests: [] });
    vi.mocked(client.api.listContacts)
      .mockResolvedValueOnce({ contacts: [] })
      .mockResolvedValue({ contacts: [{ id: 3, username: "carol" }] });
    vi.spyOn(client.api, "acceptRequest").mockResolvedValue(undefined);
    setup();
    const user = userEvent.setup();

    const trigger = await screen.findByRole("button", {
      name: "Incoming requests, 1 pending",
    });
    await user.click(trigger);
    const dialog = screen.getByRole("dialog", { name: "Incoming requests" });
    await user.click(within(dialog).getByRole("button", { name: "Accept @carol" }));

    expect(client.api.acceptRequest).toHaveBeenCalledWith(7);
    expect(
      await within(dialog).findByText("No incoming requests. New requests will appear here.")
    ).toBeInTheDocument();
    expect(await screen.findByRole("link", { name: "@carol" })).toBeInTheDocument();
    expect(client.api.incomingRequests).toHaveBeenCalledTimes(3);
    expect(client.api.listContacts).toHaveBeenCalledTimes(2);
  });

  it("rejects an incoming request without creating a contact", async () => {
    vi.mocked(client.api.incomingRequests)
      .mockResolvedValueOnce({ requests: [request] })
      .mockResolvedValueOnce({ requests: [request] })
      .mockResolvedValue({ requests: [] });
    vi.spyOn(client.api, "rejectRequest").mockResolvedValue(undefined);
    setup();
    const user = userEvent.setup();

    await user.click(
      await screen.findByRole("button", { name: "Incoming requests, 1 pending" })
    );
    const dialog = screen.getByRole("dialog", { name: "Incoming requests" });
    await user.click(within(dialog).getByRole("button", { name: "Reject @carol" }));

    expect(client.api.rejectRequest).toHaveBeenCalledWith(7);
    expect(
      await within(dialog).findByText("No incoming requests. New requests will appear here.")
    ).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "@carol" })).not.toBeInTheDocument();
    expect(client.api.listContacts).toHaveBeenCalledTimes(2);
  });

  it("renders accepted contacts in stable username order", async () => {
    vi.mocked(client.api.listContacts).mockResolvedValue({
      contacts: [
        { id: 4, username: "zoe" },
        { id: 2, username: "amy" },
      ],
    });
    setup();

    const list = await screen.findByRole("navigation", { name: "Contact list" });
    await waitFor(() => {
      expect(within(list).getAllByRole("link").map((link) => link.textContent)).toEqual([
        "@amy",
        "@zoe",
      ]);
    });
  });

  it("keeps the newest incoming-request refresh when responses arrive out of order", async () => {
    const older = deferred<{ requests: (typeof request)[] }>();
    const newer = deferred<{ requests: (typeof request)[] }>();
    vi.mocked(client.api.incomingRequests)
      .mockImplementationOnce(() => older.promise)
      .mockImplementationOnce(() => newer.promise);
    setup();
    const user = userEvent.setup();
    const trigger = await screen.findByRole("button", { name: "Incoming requests" });
    await waitFor(() => expect(client.api.incomingRequests).toHaveBeenCalledTimes(1));

    await user.click(trigger);
    await waitFor(() => expect(client.api.incomingRequests).toHaveBeenCalledTimes(2));
    await act(async () => newer.resolve({ requests: [request] }));
    expect(
      await screen.findByRole("button", { name: "Accept @carol" })
    ).toBeInTheDocument();

    await act(async () => older.resolve({ requests: [] }));
    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Accept @carol" })).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: "Incoming requests, 1 pending" })
      ).toBeInTheDocument();
    });
  });

  it("shows an exclusive incoming-request error and retries in place", async () => {
    vi.mocked(client.api.incomingRequests)
      .mockRejectedValueOnce(new Error("offline"))
      .mockRejectedValueOnce(new Error("offline"))
      .mockResolvedValue({ requests: [] });
    setup();
    const user = userEvent.setup();

    await user.click(await screen.findByRole("button", { name: "Incoming requests" }));
    const dialog = screen.getByRole("dialog", { name: "Incoming requests" });
    expect(
      await within(dialog).findByText("Could not load incoming requests. Try again.")
    ).toBeInTheDocument();
    expect(
      within(dialog).queryByText("No incoming requests. New requests will appear here.")
    ).not.toBeInTheDocument();

    await user.click(within(dialog).getByRole("button", { name: "Retry" }));
    expect(
      await within(dialog).findByText("No incoming requests. New requests will appear here.")
    ).toBeInTheDocument();
    expect(
      within(dialog).queryByText("Could not load incoming requests. Try again.")
    ).not.toBeInTheDocument();
  });

  it("keeps a committed action non-actionable when its refetch fails", async () => {
    vi.mocked(client.api.incomingRequests)
      .mockResolvedValueOnce({ requests: [request] })
      .mockResolvedValueOnce({ requests: [request] })
      .mockRejectedValueOnce(new Error("offline"))
      .mockResolvedValue({ requests: [] });
    vi.mocked(client.api.listContacts)
      .mockResolvedValueOnce({ contacts: [] })
      .mockResolvedValue({ contacts: [{ id: 3, username: "carol" }] });
    vi.spyOn(client.api, "acceptRequest").mockResolvedValue(undefined);
    setup();
    const user = userEvent.setup();

    await user.click(
      await screen.findByRole("button", { name: "Incoming requests, 1 pending" })
    );
    const dialog = screen.getByRole("dialog", { name: "Incoming requests" });
    await user.click(within(dialog).getByRole("button", { name: "Accept @carol" }));

    const completion = await within(dialog).findByText("Accepted @carol.");
    expect(completion).toHaveFocus();
    expect(within(dialog).queryByRole("button", { name: "Accept @carol" })).not.toBeInTheDocument();
    expect(
      within(dialog).getByText("Could not load incoming requests. Try again.")
    ).toBeInTheDocument();

    await user.click(within(dialog).getByRole("button", { name: "Retry" }));
    expect(
      await within(dialog).findByText("No incoming requests. New requests will appear here.")
    ).toBeInTheDocument();
    expect(
      within(dialog).queryByText("Could not load incoming requests. Try again.")
    ).not.toBeInTheDocument();
  });
});
