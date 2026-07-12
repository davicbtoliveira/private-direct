import { useCallback, useEffect, useRef, useState, type KeyboardEvent } from "react";
import { NavLink, useNavigate, useParams } from "react-router-dom";
import { ArrowLeft, Bell, LogOut, Plus, RefreshCw, Send } from "lucide-react";
import AddContactSheet from "../contacts/AddContactSheet";
import IncomingRequestsSheet from "../contacts/IncomingRequestsSheet";
import { useRealtime, type PresenceState } from "../realtime/realtimeContext";
import { useSession } from "../session/sessionContext";
import type { PeerChannelState } from "../realtime/peerManager";
import styles from "./ChatPage.module.css";

type DeliveryState = "sending" | "delivered" | "not-delivered";

interface ChatMessage {
  id: string;
  direction: "sent" | "received";
  content: string;
  delivery: DeliveryState;
  timestamp: string;
}

interface MessageEnvelope {
  type: "message";
  id: string;
  content: string;
  timestamp: string;
}

interface AckEnvelope {
  type: "ack";
  message_id: string;
}

type PeerEnvelope = MessageEnvelope | AckEnvelope;

const COMPOSER_LIMIT = 4000;
const ACK_TIMEOUT_MS = 8000;
const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

const presenceLabels: Record<PresenceState, string> = {
  connecting: "Connecting",
  online: "Online",
  offline: "Offline",
};

const channelLabels: Record<PeerChannelState, string> = {
  idle: "Not connected",
  negotiating: "Negotiating",
  "directly-connected": "Directly connected",
  offline: "Contact offline",
  failed: "Connection failed",
};

function isValidUUID(value: unknown): value is string {
  return typeof value === "string" && UUID_RE.test(value);
}

function validEnvelope(data: unknown): PeerEnvelope | null {
  if (typeof data !== "object" || data === null) return null;
  const obj = data as Record<string, unknown>;
  if (obj.type !== "message" && obj.type !== "ack") return null;
  if (obj.type === "ack") {
    if (!isValidUUID(obj.message_id)) return null;
    return { type: "ack", message_id: obj.message_id as string };
  }
  if (!isValidUUID(obj.id)) return null;
  if (typeof obj.content !== "string" || typeof obj.timestamp !== "string") return null;
  if (obj.content.length === 0 || [...obj.content].length > COMPOSER_LIMIT) return null;
  return {
    type: "message",
    id: obj.id as string,
    content: obj.content as string,
    timestamp: obj.timestamp as string,
  };
}

function codePointLength(text: string): number {
  return [...text].length;
}

export default function ChatPage() {
  const { username } = useParams();
  const hasContact = Boolean(username);
  const [showConversation, setShowConversation] = useState(hasContact);
  const { state, logout } = useSession();
  const {
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
    setPeerMessageHandler,
    retryPeer,
  } = useRealtime();
  const navigate = useNavigate();
  const me = state.user;
  const [addOpen, setAddOpen] = useState(false);
  const [requestsOpen, setRequestsOpen] = useState(false);
  const addTriggerRef = useRef<HTMLButtonElement>(null);
  const requestsTriggerRef = useRef<HTMLButtonElement>(null);

  // Chat state: per-contact transcripts, drafts, unread, deduplication
  const [composer, setComposer] = useState("");
  const [composerHeight, setComposerHeight] = useState<string>("44px");

  const transcriptsRef = useRef<Record<string, ChatMessage[]>>({});
  const unreadsRef = useRef<Record<string, number>>({});
  const receivedIDsRef = useRef(new Set<string>());
  const draftsRef = useRef<Record<string, string>>({});
  const [, forceUpdate] = useState(0);

  const rerender = useCallback(() => forceUpdate((n) => n + 1), []);

  const contact = contacts.find((c) => c.username === username);

  // Get or initialize transcript for username
  const transcript = username ? transcriptsRef.current[username] ?? [] : [];
  const composerValue = username ? composer : "";

  // Load draft when changing conversations
  useEffect(() => {
    if (!username) return;
    setComposer(draftsRef.current[username] ?? "");
  }, [username]);

  const setComposerWithDraft = useCallback((value: string) => {
    if (codePointLength(value) > COMPOSER_LIMIT) return;
    setComposer(value);
    if (username) draftsRef.current[username] = value;
  }, [username]);
  useEffect(() => {
    if (!contact || !username || !me) return;
    const contactPresence = presence[contact.id];
    if (contactPresence === "online") {
      connectPeer(contact.id);
    }
  }, [connectPeer, contact, contact?.id, username, presence, me]);

  // Subscribe to incoming messages
  useEffect(() => {
    setPeerMessageHandler((fromUserId, data) => {
      const envelope = validEnvelope(data);
      if (!envelope) return;

      const fromContact = contacts.find((c) => c.id === fromUserId);
      if (!fromContact) return;

      const contactUsername = fromContact.username;

      if (envelope.type === "ack") {
        const messagesFor = transcriptsRef.current[contactUsername];
        if (!messagesFor) return;
        const found = messagesFor.find(
          (m) => m.id === envelope.message_id
        );
        if (found && found.delivery === "sending") {
          found.delivery = "delivered";
          rerender();
        }
        (window as any).__stopAckTimer?.(envelope.message_id);
        return;
      }

      if (envelope.type === "message") {
        if (receivedIDsRef.current.has(envelope.id)) {
          // Replay ack for duplicate
          const contactID = fromContact.id;
          sendToPeer(contactID, JSON.stringify({ type: "ack", message_id: envelope.id }));
          return;
        }
        receivedIDsRef.current.add(envelope.id);

        const message: ChatMessage = {
          id: envelope.id,
          direction: "received",
          content: envelope.content,
          delivery: "delivered",
          timestamp: envelope.timestamp,
        };

        if (!transcriptsRef.current[contactUsername]) {
          transcriptsRef.current[contactUsername] = [];
        }
        transcriptsRef.current[contactUsername].push(message);

        if (contactUsername !== username) {
          unreadsRef.current[contactUsername] =
            (unreadsRef.current[contactUsername] ?? 0) + 1;
        }

        // Send ack
        sendToPeer(fromUserId, JSON.stringify({ type: "ack", message_id: envelope.id }));
        rerender();
      }
    });

    return () => {
      setPeerMessageHandler(null);
    };
  }, [contacts, sendToPeer, setPeerMessageHandler, username]);

  // Clear unread when opening a conversation
  useEffect(() => {
    if (!username) return;
    unreadsRef.current[username] = 0;
    rerender();
  }, [username]);

  const onLogout = async () => {
    transcriptsRef.current = {};
    unreadsRef.current = {};
    receivedIDsRef.current = new Set();
    await logout();
    navigate("/login", { replace: true });
  };

  const openRequests = () => {
    setRequestsOpen(true);
    void refreshRequests();
  };

  const completeRequest = async (resolvedID: number) => {
    removeRequest(resolvedID);
    await refreshContactState();
  };

  const requestsLabel =
    requests.length > 0
      ? `Incoming requests, ${requests.length} pending`
      : "Incoming requests";

  const canSend = composerValue.trim().length > 0;

  const ackTimersRef = useRef<Record<string, () => void>>({});

  const sendMessage = () => {
    if (!canSend || !username || !contact) return;
    const contactId = contact.id;
    const content = composerValue;
    const id = crypto.randomUUID();
    const timestamp = new Date().toISOString();

    const message: ChatMessage = {
      id,
      direction: "sent",
      content,
      delivery: "sending",
      timestamp,
    };

    if (!transcriptsRef.current[username]) {
      transcriptsRef.current[username] = [];
    }
    transcriptsRef.current[username].push(message);

    setComposer("");
    if (username) draftsRef.current[username] = "";

    const envelope = JSON.stringify({ type: "message", id, content, timestamp });
    if (sendToPeer(contactId, envelope)) {
      // Start ack timeout
      const timer = window.setTimeout(() => {
        const messages = transcriptsRef.current[username];
        if (!messages) return;
        const found = messages.find((m) => m.id === id);
        if (found && found.delivery === "sending") {
          found.delivery = "not-delivered";
          rerender();
        }
      }, ACK_TIMEOUT_MS);

      (window as any).__stopAckTimer = (msgId: string) => {
        if (msgId === id) {
          window.clearTimeout(timer);
          delete (window as any).__stopAckTimer;
        }
      };
      ackTimersRef.current[id] = () => window.clearTimeout(timer);
    }

    rerender();
  };

  const handleKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      sendMessage();
    }
  };

  const retryMessage = (msgId: string) => {
    if (!username || !contact) return;
    const messages = transcriptsRef.current[username];
    if (!messages) return;
    const message = messages.find((m) => m.id === msgId);
    if (!message) return;
    message.delivery = "sending";
    rerender();

    const envelope = JSON.stringify({
      type: "message",
      id: message.id,
      content: message.content,
      timestamp: message.timestamp,
    });
    if (sendToPeer(contact.id, envelope)) {
      const timer = window.setTimeout(() => {
        const msgs = transcriptsRef.current[username!];
        if (!msgs) return;
        const found = msgs.find((m) => m.id === msgId);
        if (found && found.delivery === "sending") {
          found.delivery = "not-delivered";
          rerender();
        }
      }, ACK_TIMEOUT_MS);
      ackTimersRef.current[msgId] = () => window.clearTimeout(timer);
    }
  };

  const handleAutoGrow = (el: HTMLTextAreaElement) => {
    el.style.height = "auto";
    const h = Math.min(el.scrollHeight, 160);
    setComposerHeight(`${h}px`);
    el.style.height = `${h}px`;
  };

  const peerState = contact ? (peerChannels[contact.id] ?? "idle") : "idle";

  return (
    <>
      <div className={styles.workspaceFrame}>
        {realtimeState === "replaced" && (
          <div className={styles.takeoverNotice} role="alert">
            Messaging continued in another tab
          </div>
        )}
        <div
          className={`${styles.workspace} ${showConversation ? styles.showConversation : ""}`}
          data-testid="workspace"
        >
          <aside className={styles.rail} aria-label="Contacts">
            <div className={styles.railHeader}>
              <span className={styles.railBrand}>Private Direct</span>
              <div className={styles.railActions}>
                <button
                  ref={addTriggerRef}
                  className={styles.iconBtn}
                  aria-label="Add contact"
                  title="Add contact"
                  onClick={() => {
                    setAddOpen(true);
                    void refreshContacts();
                  }}
                >
                  <Plus size={18} />
                </button>
                <button
                  ref={requestsTriggerRef}
                  className={styles.iconBtn}
                  aria-label={requestsLabel}
                  title="Incoming requests"
                  onClick={openRequests}
                >
                  <Bell size={18} />
                  {requests.length > 0 && (
                    <span className={styles.requestBadge} aria-hidden="true">
                      {requests.length > 99 ? "99+" : requests.length}
                    </span>
                  )}
                </button>
                <button
                  className={styles.iconBtn}
                  aria-label="Sign out"
                  title="Sign out"
                  onClick={onLogout}
                >
                  <LogOut size={18} />
                </button>
              </div>
            </div>
            {me && (
              <div className={styles.me}>
                <span className={styles.handle}>@{me.username}</span>
                <span className={styles.realtimeStatus}>
                  {realtimeState === "connected"
                    ? "Realtime connected"
                    : realtimeState === "replaced"
                      ? "Realtime replaced"
                      : "Realtime connecting"}
                </span>
              </div>
            )}
            <nav className={styles.contactList} aria-label="Contact list">
              {contactsLoading && contacts.length === 0 ? (
                <p className={styles.empty} role="status">
                  Loading contacts…
                </p>
              ) : contactsError ? (
                <div className={styles.listError} role="alert">
                  <p>{contactsError}</p>
                  <button type="button" onClick={refreshContacts}>
                    <RefreshCw size={16} />
                    Reload contacts
                  </button>
                </div>
              ) : contacts.length === 0 ? (
                <p className={styles.empty}>
                  No contacts yet. Find someone by exact handle.
                </p>
              ) : (
                contacts.map((contact) => {
                  const contactPresence = presence[contact.id] ?? "connecting";
                  const contactUnread = unreadsRef.current[contact.username] ?? 0;
                  return (
                    <NavLink
                      key={contact.id}
                      to={`/chat/${encodeURIComponent(contact.username)}`}
                      className={({ isActive }) =>
                        `${styles.contactLink} ${isActive ? styles.activeContact : ""}`
                      }
                      aria-label={`@${contact.username}, ${contactPresence}${contactUnread > 0 ? `, ${contactUnread} unread` : ""}`}
                      onClick={() => setShowConversation(true)}
                    >
                      <span className={styles.contactHandle}>@{contact.username}</span>
                      <span className={styles.contactPresence}>
                        {contactUnread > 0 && (
                          <span className={styles.unreadBadge} aria-hidden="true">
                            {contactUnread > 99 ? "99+" : contactUnread}
                          </span>
                        )}
                        <span
                          className={`${styles.presenceDot} ${styles[contactPresence]}`}
                          aria-hidden="true"
                        />
                        {presenceLabels[contactPresence]}
                      </span>
                    </NavLink>
                  );
                })
              )}
            </nav>
          </aside>

          <section className={styles.conversation} aria-label="Conversation">
            <header className={styles.conversationHeader}>
              <button
                className={styles.back}
                aria-label="Back to contacts"
                onClick={() => setShowConversation(false)}
              >
                <ArrowLeft size={20} />
              </button>
              <span className={styles.handle}>
                {username ? `@${username}` : "No conversation selected"}
              </span>
              {username && (
                <span className={styles.channelState} aria-live="polite">
                  {channelLabels[peerState]}
                </span>
              )}
            </header>

            <div className={styles.conversationBody}>
              {!username ? (
                <p className={styles.placeholder}>
                  Select a contact to start a direct conversation.
                </p>
              ) : (
                <div className={styles.messagesArea}>
                  {transcript.length === 0 && peerState !== "directly-connected" && (
                    <p className={styles.ephemeralNotice}>
                      Messages are temporary and only visible in this tab.
                    </p>
                  )}
                  {transcript.map((msg) => (
                    <div
                      key={msg.id}
                      className={`${styles.messageRow} ${msg.direction === "sent" ? styles.sent : styles.received}`}
                    >
                      <div className={styles.messageBubble}>
                        <span className={styles.messageContent}>{msg.content}</span>
                        <span className={styles.messageMeta}>
                          {msg.direction === "sent" && (
                            <span className={styles.delivery} data-delivery={msg.delivery}>
                              {msg.delivery === "sending"
                                ? "sending"
                                : msg.delivery === "delivered"
                                  ? "delivered"
                                  : "not delivered"}
                            </span>
                          )}
                          {new Date(msg.timestamp).toLocaleTimeString([], {
                            hour: "2-digit",
                            minute: "2-digit",
                          })}
                        </span>
                        {msg.delivery === "not-delivered" && msg.direction === "sent" && (
                          <button
                            type="button"
                            className={styles.retryBtn}
                            onClick={() => retryMessage(msg.id)}
                            aria-label={`Retry sending message`}
                          >
                            Try again
                          </button>
                        )}
                      </div>
                    </div>
                  ))}
                </div>
              )}

              {username && (
                <div className={styles.composerArea}>
                  {peerState === "offline" && (
                    <p className={styles.offlineNotice}>
                      {contact?.username ?? username} is offline. Messages cannot be delivered.
                    </p>
                  )}
                  {peerState === "failed" && (
                    <div className={styles.failedBanner}>
                      <span>Connection failed.</span>
                      <button type="button" onClick={() => retryPeer(contact!.id)}>
                        Try again
                      </button>
                    </div>
                  )}
                  <div className={styles.composerRow}>
                    <textarea
                      className={styles.composer}
                      value={composerValue}
                      onChange={(e) => {
                        setComposerWithDraft(e.target.value);
                        handleAutoGrow(e.target);
                      }}
                      onKeyDown={handleKeyDown}
                      placeholder="Type a message…"
                      rows={1}
                      disabled={peerState === "offline"}
                      aria-label="Message"
                      style={{ height: composerHeight }}
                      ref={(el) => {
                        if (el) handleAutoGrow(el);
                      }}
                    />
                    <button
                      type="button"
                      className={styles.sendBtn}
                      onClick={sendMessage}
                      disabled={!canSend || peerState === "offline"}
                      aria-label="Send message"
                    >
                      <Send size={18} />
                    </button>
                  </div>
                </div>
              )}
            </div>
          </section>
        </div>
        <span className="sr-only" role="status" aria-live="polite">
          {announcement}
        </span>
      </div>

      {addOpen && (
        <AddContactSheet
          onClose={() => setAddOpen(false)}
          onComplete={refreshContacts}
          returnFocusRef={addTriggerRef}
        />
      )}
      {requestsOpen && (
        <IncomingRequestsSheet
          requests={requests}
          loading={requestsLoading}
          loadError={requestsError}
          onClose={() => setRequestsOpen(false)}
          onComplete={completeRequest}
          onReload={refreshRequests}
          returnFocusRef={requestsTriggerRef}
        />
      )}
    </>
  );
}
