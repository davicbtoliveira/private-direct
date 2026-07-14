import type {
  SessionResponse,
  User,
  LookupUser,
  ContactRequestResponse,
  IceServersResponse,
  RegistrationResponse,
} from "./types";
import type { E2EESetupPayload } from "../e2ee/setup";

const BASE = "/api";

let accessToken: string | null = null;

export function setAccessToken(token: string | null) {
  accessToken = token;
}

export function getAccessToken() {
  return accessToken;
}

type FetchOptions = {
  method?: string;
  body?: unknown;
  headers?: Record<string, string>;
  signal?: AbortSignal;
};

async function request<T>(path: string, opts: FetchOptions = {}): Promise<T> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...opts.headers,
  };
  if (accessToken) {
    headers["Authorization"] = `Bearer ${accessToken}`;
  }
  const res = await fetch(`${BASE}${path}`, {
    method: opts.method ?? "GET",
    headers,
    body: opts.body === undefined ? undefined : JSON.stringify(opts.body),
    credentials: "same-origin",
    signal: opts.signal,
  });
  return parseResponse<T>(res);
}

async function parseResponse<T>(res: Response): Promise<T> {
  if (res.status === 204) {
    return undefined as T;
  }
  const text = await res.text();
  if (text === "") {
    return undefined as T;
  }
  const body = text ? JSON.parse(text) : undefined;
  if (!res.ok) {
    const error = body?.error ?? "request_failed";
    throw Object.assign(new Error(error), { status: res.status, code: error });
  }
  return body as T;
}

export const api = {
  login: (username: string, password: string) =>
    request<SessionResponse>("/login", { method: "POST", body: { username, password } }),
  register: (inviteCode: string, username: string, password: string) =>
    request<RegistrationResponse>("/register", {
      method: "POST",
      body: { invite_code: inviteCode, username, password },
    }),
  refresh: () => request<SessionResponse>("/refresh", { method: "POST", body: {} }),
  logout: () => request<void>("/logout", { method: "POST", body: {} }),
  setupE2EE: (body: E2EESetupPayload) =>
    request<{ e2ee_ready: true }>("/e2ee/setup", { method: "POST", body }),
  e2eeKeysUpload: (body: Record<string, unknown>) => request<Record<string, unknown>>("/e2ee/keys/upload", { method: "POST", body }),
  e2eeKeysQuery: (body: Record<string, unknown>) => request<Record<string, unknown>>("/e2ee/keys/query", { method: "POST", body }),
  e2eeKeysClaim: (body: Record<string, unknown>) => request<Record<string, unknown>>("/e2ee/keys/claim", { method: "POST", body }),
  e2eeToDevice: (eventType: string, txnID: string, body: Record<string, unknown>) => request<Record<string, unknown>>(`/e2ee/to-device/${encodeURIComponent(eventType)}/${encodeURIComponent(txnID)}`, { method: "POST", body }),
  e2eeSync: (deviceID: string, since: string) => request<{ next: string; events: Record<string, unknown>[] }>(`/e2ee/sync?device_id=${encodeURIComponent(deviceID)}&since=${encodeURIComponent(since)}`),
  createMessage: (id: string, to: number, ciphertext: Record<string, unknown>) => request<{ id: string; sequence: number; created_at: string }>("/messages", { method: "POST", body: { id, to, ciphertext } }),
  listMessages: (contactID: number, before?: number) => request<{ messages: Array<{ id: string; sequence: number; sender_id: number; recipient_id: number; ciphertext: Record<string, unknown>; created_at: string; delivered: boolean }> }>(`/messages?contact_id=${contactID}&limit=50${before ? `&before=${before}` : ""}`),
  markMessageDelivered: (id: string) => request<void>(`/messages/${encodeURIComponent(id)}/delivered`, { method: "POST", body: {} }),
  lookupUser: (username: string) =>
    request<LookupUser>(`/users/lookup?username=${encodeURIComponent(username)}`),
  createContactRequest: (username: string) =>
    request<ContactRequestResponse>("/contacts/requests", {
      method: "POST",
      body: { username },
    }),
  incomingRequests: () =>
    request<{ requests: ContactRequestResponse[] }>("/contacts/requests/incoming"),
  acceptRequest: (id: number) =>
    request<void>(`/contacts/requests/${id}/accept`, { method: "POST", body: {} }),
  rejectRequest: (id: number) =>
    request<void>(`/contacts/requests/${id}/reject`, { method: "POST", body: {} }),
  listContacts: () => request<{ contacts: User[] }>("/contacts"),
  iceServers: () => request<IceServersResponse>("/ice-servers"),
};
