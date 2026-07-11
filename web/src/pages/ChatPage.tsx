import { useCallback, useEffect, useRef, useState } from "react";
import { NavLink, useNavigate, useParams } from "react-router-dom";
import { ArrowLeft, Plus, Bell, LogOut, RefreshCw } from "lucide-react";
import styles from "./ChatPage.module.css";
import { useSession } from "../session/sessionContext";
import { api } from "../api/client";
import type { ContactRequestResponse, User } from "../api/types";
import AddContactSheet from "../contacts/AddContactSheet";
import IncomingRequestsSheet from "../contacts/IncomingRequestsSheet";

export default function ChatPage() {
  const { username } = useParams();
  const hasContact = Boolean(username);
  const [showConversation, setShowConversation] = useState(hasContact);
  const { state, logout } = useSession();
  const navigate = useNavigate();
  const me = state.user;
  const [contacts, setContacts] = useState<User[]>([]);
  const [requests, setRequests] = useState<ContactRequestResponse[]>([]);
  const [contactsLoading, setContactsLoading] = useState(true);
  const [requestsLoading, setRequestsLoading] = useState(true);
  const [contactsError, setContactsError] = useState<string | null>(null);
  const [requestsError, setRequestsError] = useState<string | null>(null);
  const [addOpen, setAddOpen] = useState(false);
  const [requestsOpen, setRequestsOpen] = useState(false);
  const addTriggerRef = useRef<HTMLButtonElement>(null);
  const requestsTriggerRef = useRef<HTMLButtonElement>(null);
  const contactsRequestRef = useRef(0);
  const requestsRequestRef = useRef(0);

  const refreshContacts = useCallback(async () => {
    const requestID = ++contactsRequestRef.current;
    setContactsLoading(true);
    setContactsError(null);
    try {
      const response = await api.listContacts();
      if (contactsRequestRef.current !== requestID) return;
      setContacts(
        [...response.contacts].sort((a, b) =>
          a.username < b.username ? -1 : a.username > b.username ? 1 : 0
        )
      );
    } catch {
      if (contactsRequestRef.current !== requestID) return;
      setContactsError("Could not load contacts. Try again.");
    } finally {
      if (contactsRequestRef.current === requestID) setContactsLoading(false);
    }
  }, []);

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

  useEffect(() => {
    void refreshContacts();
    void refreshRequests();
  }, [refreshContacts, refreshRequests]);

  const onLogout = async () => {
    await logout();
    navigate("/login", { replace: true });
  };

  const openRequests = () => {
    setRequestsOpen(true);
    void refreshRequests();
  };

  const completeRequest = async (resolvedID: number) => {
    setRequests((current) => current.filter((request) => request.id !== resolvedID));
    await Promise.all([refreshContacts(), refreshRequests()]);
  };

  const requestsLabel =
    requests.length > 0
      ? `Incoming requests, ${requests.length} pending`
      : "Incoming requests";

  return (
    <>
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
              contacts.map((contact) => (
                <NavLink
                  key={contact.id}
                  to={`/chat/${encodeURIComponent(contact.username)}`}
                  className={({ isActive }) =>
                    `${styles.contactLink} ${isActive ? styles.activeContact : ""}`
                  }
                  onClick={() => setShowConversation(true)}
                >
                  <span className={styles.contactHandle}>@{contact.username}</span>
                </NavLink>
              ))
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
