import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { Outlet } from "react-router-dom";
import { api, getAccessToken } from "../api/client";
import type { ContactRequestResponse, User } from "../api/types";
import { useSession } from "../session/sessionContext";
import {
  RealtimeContext,
  type PresenceState,
  type RealtimeState,
} from "./realtimeContext";
import { PeerManager, type PeerChannelState } from "./peerManager";

const reconnectDelayMS = 1_000;

function websocketURL() {
  const url = new URL("/api/ws", window.location.href);
  url.protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
  url.search = "";
  return url.toString();
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function parseUser(value: unknown): User | null {
  if (!isRecord(value)) return null;
  if (typeof value.id !== "number" || typeof value.username !== "string") return null;
  return { id: value.id, username: value.username };
}

export default function RealtimeProvider() {
  const { state: session } = useSession();
  const [contacts, setContacts] = useState<User[]>([]);
  const [requests, setRequests] = useState<ContactRequestResponse[]>([]);
  const [contactsLoading, setContactsLoading] = useState(true);
  const [requestsLoading, setRequestsLoading] = useState(true);
  const [contactsError, setContactsError] = useState<string | null>(null);
  const [requestsError, setRequestsError] = useState<string | null>(null);
  const [presence, setPresence] = useState<Record<number, PresenceState>>({});
  const [realtimeState, setRealtimeState] = useState<RealtimeState>("connecting");
  const [announcement, setAnnouncement] = useState("Realtime connecting.");
  const [peerChannels, setPeerChannels] = useState<Record<number, PeerChannelState>>({});
  const contactsRef = useRef<User[]>([]);
  const onlineUserIDsRef = useRef(new Set<number>());
  const realtimeStateRef = useRef<RealtimeState>("connecting");
  const contactsRequestRef = useRef(0);
  const requestsRequestRef = useRef(0);
  const peerManagerRef = useRef<PeerManager | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const peerMessageHandlerRef = useRef<((userId: number, data: string) => void) | null>(null);
  const presenceRef = useRef<Record<number, PresenceState>>({});

  const setConnectionState = useCallback((next: RealtimeState) => {
    realtimeStateRef.current = next;
    setRealtimeState(next);
  }, []);

  const syncPresenceForContacts = useCallback((nextContacts: User[]) => {
    const connected = realtimeStateRef.current === "connected";
    const next: Record<number, PresenceState> = {};
    for (const contact of nextContacts) {
      next[contact.id] = connected
        ? onlineUserIDsRef.current.has(contact.id)
          ? "online"
          : "offline"
        : "connecting";
    }
    setPresence(next);
  }, []);

  const refreshContacts = useCallback(async () => {
    const requestID = ++contactsRequestRef.current;
    setContactsLoading(true);
    setContactsError(null);
    try {
      const response = await api.listContacts();
      if (contactsRequestRef.current !== requestID) return;
      const next = [...response.contacts].sort((a, b) =>
        a.username < b.username ? -1 : a.username > b.username ? 1 : 0
      );
      contactsRef.current = next;
      setContacts(next);
      syncPresenceForContacts(next);
    } catch {
      if (contactsRequestRef.current !== requestID) return;
      setContactsError("Could not load contacts. Try again.");
    } finally {
      if (contactsRequestRef.current === requestID) setContactsLoading(false);
    }
  }, [syncPresenceForContacts]);

  const refreshRequests = useCallback(async () => {
    const requestID = ++requestsRequestRef.current;
    setRequestsLoading(true);
    setRequestsError(null);
    try {
      const response = await api.incomingRequests();
      if (requestsRequestRef.current !== requestID) return;
      setRequests(response.requests);
    } catch {
      if (requestsRequestRef.current !== requestID) return;
      setRequestsError("Could not load incoming requests. Try again.");
    } finally {
      if (requestsRequestRef.current === requestID) setRequestsLoading(false);
    }
  }, []);

  const refreshContactState = useCallback(async () => {
    await Promise.all([refreshContacts(), refreshRequests()]);
  }, [refreshContacts, refreshRequests]);

  const removeRequest = useCallback((id: number) => {
    setRequests((current) => current.filter((request) => request.id !== id));
  }, []);

  const sendPeerSignal = useCallback((toUserId: number, signalType: string, payload: Record<string, unknown>) => {
    const ws = wsRef.current;
    if (!ws || ws.readyState !== WebSocket.OPEN) return;
    ws.send(JSON.stringify({ type: "signal", to_user_id: toUserId, signal_type: signalType, payload }));
  }, []);

  const connectPeer = useCallback((userId: number) => {
    peerManagerRef.current?.connect(userId);
  }, []);

  const sendToPeer = useCallback((userId: number, data: string): boolean => {
    return peerManagerRef.current?.sendMessage(userId, data) ?? false;
  }, []);

  const retryPeer = useCallback((userId: number) => {
    peerManagerRef.current?.retryConnection(userId);
  }, []);

  const setPeerMessageHandler = useCallback((fn: ((userId: number, data: string) => void) | null) => {
    peerMessageHandlerRef.current = fn;
  }, []);

  useEffect(() => {
    presenceRef.current = presence;
  }, [presence]);

  useEffect(() => {
    void refreshContactState();
  }, [refreshContactState]);

  useEffect(() => {
    if (!session.user) return;
    let cancelled = false;

    const mgr = new PeerManager();
    mgr.init(
      session.user.id,
      [],
      sendPeerSignal,
      (userId, state) => {
        setPeerChannels((prev) => ({ ...prev, [userId]: state }));
      },
      (userId, data) => {
        peerMessageHandlerRef.current?.(userId, data);
      },
      (userId) => presenceRef.current[userId] === "online"
    );
    peerManagerRef.current = mgr;

    (async () => {
      try {
        const iceConfig = await api.iceServers();
        if (cancelled) return;
        mgr.init(
          session.user!.id,
          iceConfig.ice_servers,
          sendPeerSignal,
          (userId, state) => {
            setPeerChannels((prev) => ({ ...prev, [userId]: state }));
          },
          (userId, data) => {
            peerMessageHandlerRef.current?.(userId, data);
          },
          (userId) => presenceRef.current[userId] === "online"
        );
      } catch {
        // peer manager works without ICE config
      }
    })();

    return () => {
      cancelled = true;
      mgr.disconnectAll();
      peerManagerRef.current = null;
    };
  }, [sendPeerSignal, session.user]);

  useEffect(() => {
    if (!session.user) return;

    let stopped = false;
    let replaced = false;
    let activeSocket: WebSocket | null = null;
    let retryTimer: number | null = null;

    const markConnecting = () => {
      onlineUserIDsRef.current.clear();
      setConnectionState("connecting");
      syncPresenceForContacts(contactsRef.current);
      setAnnouncement("Realtime reconnecting.");
    };

    const scheduleReconnect = () => {
      if (stopped || replaced || retryTimer !== null) return;
      retryTimer = window.setTimeout(() => {
        retryTimer = null;
        connect();
      }, reconnectDelayMS);
    };

      const connect = () => {
      if (stopped || replaced) return;
      const token = getAccessToken();
      if (!token) {
        markConnecting();
        scheduleReconnect();
        return;
      }

      let socket: WebSocket;
      try {
        socket = new WebSocket(websocketURL(), ["private-direct", token]);
      } catch {
        markConnecting();
        scheduleReconnect();
        return;
      }

      activeSocket = socket;
      wsRef.current = socket;
      let snapshotReceived = false;
      let acceptsEvents = true;

      const closeSocket = (code: number, reason: string) => {
        acceptsEvents = false;
        socket.close(code, reason);
      };

      socket.onopen = () => {
        if (socket.protocol !== "private-direct") {
          closeSocket(1002, "invalid_protocol");
        }
      };

      socket.onmessage = (event) => {
        if (stopped || !acceptsEvents || activeSocket !== socket) return;
        let message: unknown;
        try {
          message = JSON.parse(String(event.data));
        } catch {
          closeSocket(1002, "invalid_json");
          return;
        }
        if (!isRecord(message) || typeof message.type !== "string") {
          closeSocket(1002, "invalid_event");
          return;
        }

        if (!snapshotReceived) {
          if (message.type !== "presence_snapshot" || !Array.isArray(message.online_users)) {
            closeSocket(1002, "snapshot_required");
            return;
          }
          const onlineUsers = message.online_users.map(parseUser);
          if (onlineUsers.some((onlineUser) => onlineUser === null)) {
            closeSocket(1002, "invalid_snapshot");
            return;
          }
          snapshotReceived = true;
          onlineUserIDsRef.current = new Set(
            onlineUsers.map((onlineUser) => (onlineUser as User).id)
          );
          setConnectionState("connected");
          syncPresenceForContacts(contactsRef.current);
          setAnnouncement("Realtime connected.");
          return;
        }

        if (message.type === "presence") {
          const eventUser = parseUser(message.user);
          if (!eventUser || typeof message.online !== "boolean") return;
          if (message.online) onlineUserIDsRef.current.add(eventUser.id);
          else onlineUserIDsRef.current.delete(eventUser.id);
          setPresence((current) => ({
            ...current,
            [eventUser.id]: message.online ? "online" : "offline",
          }));
          setAnnouncement(`@${eventUser.username} is ${message.online ? "online" : "offline"}.`);
          return;
        }

        if (message.type === "contacts_changed") {
          void refreshContactState();
          return;
        }

        if (message.type === "session_replaced") {
          replaced = true;
          onlineUserIDsRef.current.clear();
          setConnectionState("replaced");
          syncPresenceForContacts(contactsRef.current);
          setAnnouncement("");
          closeSocket(1000, "session_replaced");
          return;
        }

        if (message.type === "signal") {
          const signalFrom = parseUser(message.from);
          if (!signalFrom || typeof message.signal_type !== "string") return;
          peerManagerRef.current?.handleSignal(
            signalFrom.id,
            String(message.signal_type),
            (message.payload as Record<string, unknown>) ?? {}
          );
          return;
        }

        if (message.type === "error") {
          peerManagerRef.current?.handleSignalError(String(message.error));
          return;
        }
      };

      socket.onerror = () => closeSocket(1000, "socket_error");
      socket.onclose = () => {
        if (activeSocket !== socket) return;
        activeSocket = null;
        wsRef.current = null;
        if (stopped || replaced) return;
        markConnecting();
        scheduleReconnect();
      };
    };

    setConnectionState("connecting");
    syncPresenceForContacts(contactsRef.current);
    setAnnouncement("Realtime connecting.");
    connect();

    return () => {
      stopped = true;
      if (retryTimer !== null) window.clearTimeout(retryTimer);
      activeSocket?.close(1000, "workspace_closed");
      wsRef.current = null;
    };
  }, [
    refreshContactState,
    session.user,
    setConnectionState,
    syncPresenceForContacts,
  ]);

  const value = useMemo(
    () => ({
      contacts,
      requests,
      contactsLoading,
      requestsLoading,
      contactsError,
      requestsError,
      presence,
      peerChannels,
      realtimeState,
      announcement,
      refreshContacts,
      refreshRequests,
      refreshContactState,
      removeRequest,
      connectPeer,
      sendToPeer,
      onPeerMessage: peerMessageHandlerRef.current,
      setPeerMessageHandler,
      retryPeer,
    }),
    [
      announcement,
      contacts,
      contactsError,
      contactsLoading,
      peerChannels,
      presence,
      realtimeState,
      refreshContactState,
      refreshContacts,
      refreshRequests,
      removeRequest,
      connectPeer,
      sendToPeer,
      setPeerMessageHandler,
      retryPeer,
      requests,
      requestsError,
      requestsLoading,
    ]
  );

  return (
    <RealtimeContext.Provider value={value}>
      <Outlet />
    </RealtimeContext.Provider>
  );
}
