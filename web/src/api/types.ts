export type User = {
  id: number;
  username: string;
  e2ee_ready?: boolean;
  protocol_version?: number;
};

export type SessionResponse = {
  access_token: string;
  token_type: string;
  expires_in: number;
  user: User;
};

export type LookupUser = User;

export type ContactRequestResponse = {
  id: number;
  username: string;
  status: string;
  created_at: string;
};

export type IceServer = {
  urls: string[];
  username?: string;
  credential?: string;
};

export type IceServersResponse = {
  ice_servers: IceServer[];
};

export type ApiError = {
  error: string;
};

export type RegistrationResponse = {
  id: number;
  username: string;
  warning?: "password_breach_check_unavailable";
};

export type SignalEvent = {
  type: "signal";
  from: User;
  signal_type: string;
  payload: Record<string, unknown>;
  connection_id?: string;
};

export type SignalError = {
  type: "error";
  error: string;
};
