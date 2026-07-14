import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, expect, it, vi } from "vitest";
import App from "../App";
import * as client from "../api/client";
import FakeWebSocket from "../test/FakeWebSocket";

const cryptoSession = vi.hoisted(() => ({
  encrypt: vi.fn().mockResolvedValue({ algorithm: "m.megolm.v1.aes-sha2", ciphertext: "opaque" }),
  decrypt: vi.fn().mockResolvedValue({ content: { body: "persisted secret", sent_at: "2026-07-14T00:00:00Z" } }),
  createEventChain: vi.fn().mockResolvedValue({device_id:"device",index:1,previous_hash:"",event_hash:"hash",signature:"signature"}),
}));
vi.mock("./matrixSession", () => ({ matrixSession: vi.fn().mockResolvedValue(cryptoSession), hasRememberedDevice: () => true }));

beforeEach(() => {
  vi.clearAllMocks();
  localStorage.clear();
  FakeWebSocket.reset();
  client.setAccessToken(null);
  vi.spyOn(client.api, "refresh").mockResolvedValue({ access_token:"token",token_type:"Bearer",expires_in:900,user:{id:1,username:"alice",e2ee_ready:true} });
  vi.spyOn(client.api, "listContacts").mockResolvedValue({contacts:[{id:2,username:"bob"}]});
  vi.spyOn(client.api, "incomingRequests").mockResolvedValue({requests:[]});
  vi.spyOn(client.api, "listMessages").mockResolvedValue({messages:[{id:"550e8400-e29b-41d4-a716-446655440000",sequence:1,sender_id:2,recipient_id:1,ciphertext:{ciphertext:"opaque"},created_at:"2026-07-14T00:00:00Z",delivered:false}]});
  vi.spyOn(client.api, "markMessageDelivered").mockResolvedValue(undefined);
  vi.spyOn(client.api, "createMessage").mockResolvedValue({id:"id",sequence:2,created_at:"2026-07-14T00:01:00Z"});
  vi.spyOn(client.api, "contactIdentity").mockResolvedValue({username:"bob",identity_keys:{master:"bob-public"},protocol_version:1});
});

it("loads decrypted history and persists only encrypted outgoing content", async () => {
  render(<MemoryRouter initialEntries={["/chat/bob"]}><App /></MemoryRouter>);
  await waitFor(() => expect(FakeWebSocket.instances).toHaveLength(1));
  act(() => { FakeWebSocket.instances[0].open(); FakeWebSocket.instances[0].receive({type:"presence_snapshot",online_users:[]}); });
  expect(await screen.findByText("persisted secret")).toBeInTheDocument();

  act(() => FakeWebSocket.instances[0].receive({ type: "mailbox_changed", cursor: 2 }));
  await waitFor(() => expect(client.api.listMessages).toHaveBeenCalledTimes(2));

  const user=userEvent.setup();
  await user.type(screen.getByLabelText("Message"),"new secret");
  await user.click(screen.getByRole("button",{name:"Send message"}));
  await waitFor(()=>expect(client.api.createMessage).toHaveBeenCalled());
  const [, , ciphertext]=vi.mocked(client.api.createMessage).mock.calls[0];
  expect(ciphertext).toEqual({algorithm:"m.megolm.v1.aes-sha2",ciphertext:"opaque"});
  expect(JSON.stringify(ciphertext)).not.toContain("new secret");
  expect(await screen.findByText("sent")).toBeInTheDocument();
});
