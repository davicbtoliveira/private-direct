import type {
  SessionResponse,
  User,
  LookupUser,
  ContactRequestResponse,
  IceServersResponse,
  RegistrationResponse,
} from "./types";

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
