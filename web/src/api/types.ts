export type User = {
  id: number;
  username: string;
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
