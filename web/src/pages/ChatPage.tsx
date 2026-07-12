import { useRef, useState } from "react";
import { NavLink, useNavigate, useParams } from "react-router-dom";
import { ArrowLeft, Bell, LogOut, Plus, RefreshCw } from "lucide-react";
import AddContactSheet from "../contacts/AddContactSheet";
import IncomingRequestsSheet from "../contacts/IncomingRequestsSheet";
import { useRealtime, type PresenceState } from "../realtime/realtimeContext";
import { useSession } from "../session/sessionContext";
import styles from "./ChatPage.module.css";

const presenceLabels: Record<PresenceState, string> = {
  connecting: "Connecting",
  online: "Online",
  offline: "Offline",
};

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
    realtimeState,
    announcement,
    refreshContacts,
    refreshRequests,
    refreshContactState,
    removeRequest,
  } = useRealtime();
  const navigate = useNavigate();
  const me = state.user;
  const [addOpen, setAddOpen] = useState(false);
  const [requestsOpen, setRequestsOpen] = useState(false);
  const addTriggerRef = useRef<HTMLButtonElement>(null);
  const requestsTriggerRef = useRef<HTMLButtonElement>(null);

  const onLogout = async () => {
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
                  return (
                    <NavLink
                      key={contact.id}
                      to={`/chat/${encodeURIComponent(contact.username)}`}
                      className={({ isActive }) =>
                        `${styles.contactLink} ${isActive ? styles.activeContact : ""}`
                      }
                      aria-label={`@${contact.username}, ${contactPresence}`}
                      onClick={() => setShowConversation(true)}
                    >
                      <span className={styles.contactHandle}>@{contact.username}</span>
                      <span className={styles.contactPresence}>
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
            </header>
            <div className={styles.conversationBody}>
              {username
                ? "Direct messaging is not connected."
                : "Select a contact to start a direct conversation."}
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
