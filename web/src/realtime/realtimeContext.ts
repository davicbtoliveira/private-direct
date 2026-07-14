import { createContext, useContext } from "react";
import type { ContactRequestResponse, User } from "../api/types";
import type { PeerChannelState } from "./peerManager";

export type PresenceState = "connecting" | "online" | "offline";
export type RealtimeState = "connecting" | "connected" | "replaced";

type RealtimeContextValue = {
  contacts: User[];
  requests: ContactRequestResponse[];
  contactsLoading: boolean;
  requestsLoading: boolean;
  contactsError: string | null;
  requestsError: string | null;
  presence: Record<number, PresenceState>;
  peerChannels: Record<number, PeerChannelState>;
  realtimeState: RealtimeState;
  announcement: string;
  mailboxRevision: number;
  refreshContacts: () => Promise<void>;
  refreshRequests: () => Promise<void>;
  refreshContactState: () => Promise<void>;
  removeRequest: (id: number) => void;
  connectPeer: (userId: number) => void;
  sendToPeer: (userId: number, data: string) => boolean;
  onPeerMessage: ((userId: number, data: string) => void) | null;
  setPeerMessageHandler: (fn: ((userId: number, data: string) => void) | null) => void;
  retryPeer: (userId: number) => void;
};

export const RealtimeContext = createContext<RealtimeContextValue | null>(null);

export function useRealtime() {
  const context = useContext(RealtimeContext);
  if (!context) throw new Error("useRealtime must be used within RealtimeProvider");
  return context;
}
