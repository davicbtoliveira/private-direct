import { useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { ArrowLeft, Plus, Bell, LogOut } from "lucide-react";
import styles from "./ChatPage.module.css";
import { useSession } from "../session/sessionContext";

export default function ChatPage() {
  const { username } = useParams();
  const hasContact = Boolean(username);
  const [showConversation, setShowConversation] = useState(hasContact);
  const { state, logout } = useSession();
  const navigate = useNavigate();
  const me = state.user;

  const onLogout = async () => {
    await logout();
    navigate("/login", { replace: true });
  };

  return (
    <div
      className={`${styles.workspace} ${showConversation ? styles.showConversation : ""}`}
      data-testid="workspace"
    >
      <aside className={styles.rail} aria-label="Contacts">
        <div className={styles.railHeader}>
          <span className={styles.railBrand}>Private Direct</span>
          <button className={styles.iconBtn} aria-label="Add contact" title="Add contact">
            <Plus size={18} />
          </button>
          <button
            className={styles.iconBtn}
            aria-label="Incoming requests"
            title="Incoming requests"
          >
            <Bell size={18} />
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
        {me && (
          <div className={styles.me}>
            <span className={styles.handle}>@{me.username}</span>
          </div>
        )}
        <nav className={styles.contactList} aria-label="Contact list">
          <p className={styles.empty}>No contacts yet.</p>
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
            ? "Direct channel ready."
            : "Select a contact to start a direct conversation."}
        </div>
      </section>
    </div>
  );
}
