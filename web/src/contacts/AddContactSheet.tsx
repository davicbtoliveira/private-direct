import { useState, type FormEvent, type RefObject } from "react";
import { Search, UserPlus } from "lucide-react";
import { api } from "../api/client";
import type { User } from "../api/types";
import Sheet from "../components/Sheet";
import styles from "./ContactSheets.module.css";

type Props = {
  onClose: () => void;
  onComplete: () => Promise<void>;
  returnFocusRef: RefObject<HTMLElement>;
};

type RequestState = "idle" | "sending" | "sent" | "unavailable";

const usernamePattern = /^[a-z0-9][a-z0-9._-]{2,31}$/;

function errorCode(error: unknown) {
  if (typeof error === "object" && error !== null && "code" in error) {
    return String(error.code);
  }
  return "request_failed";
}

export default function AddContactSheet({
  onClose,
  onComplete,
  returnFocusRef,
}: Props) {
  const [query, setQuery] = useState("");
  const [result, setResult] = useState<User | null>(null);
  const [searching, setSearching] = useState(false);
  const [requestState, setRequestState] = useState<RequestState>("idle");
  const [error, setError] = useState<string | null>(null);

  const search = async (event: FormEvent) => {
    event.preventDefault();
    const username = query.trim().toLowerCase();
    setQuery(username);
    setResult(null);
    setRequestState("idle");
    if (!usernamePattern.test(username)) {
      setError("Enter an exact handle using 3-32 letters, numbers, dots, dashes, or underscores.");
      return;
    }

    setSearching(true);
    setError(null);
    try {
      setResult(await api.lookupUser(username));
    } catch (requestError) {
      const code = errorCode(requestError);
      if (code === "user_not_found") {
        setError(`No user found for @${username}.`);
      } else if (code === "invalid_username" || code === "username_required") {
        setError("Enter a complete exact handle.");
      } else {
        setError("Could not search for that handle. Try again.");
      }
    } finally {
      setSearching(false);
    }
  };

  const sendRequest = async () => {
    if (!result || requestState !== "idle") return;
    setRequestState("sending");
    setError(null);
    try {
      const response = await api.createContactRequest(result.username);
      if (response.status !== "pending") {
        setError("A previous request was rejected. New requests are not available.");
        setRequestState("unavailable");
        return;
      }
      await onComplete();
      setRequestState("sent");
    } catch (requestError) {
      const code = errorCode(requestError);
      if (code === "already_contacts") {
        setError(`@${result.username} is already in your contacts.`);
        await onComplete();
      } else if (code === "incoming_request_exists") {
        setError(`@${result.username} already sent you a request. Review incoming requests.`);
      } else if (
        code === "contact_request_unavailable" ||
        code === "contact_request_exists"
      ) {
        setError(`A request involving @${result.username} already exists.`);
      } else {
        setError("Could not send the request. Try again.");
      }
      setRequestState("idle");
    }
  };

  return (
    <Sheet title="Add contact" onClose={onClose} returnFocusRef={returnFocusRef}>
      <form className={styles.searchForm} onSubmit={search} noValidate>
        <label className={styles.field}>
          <span className={styles.label}>Exact username</span>
          <input
            className={styles.input}
            value={query}
            onChange={(event) => {
              setQuery(event.target.value);
              setResult(null);
              setRequestState("idle");
              setError(null);
            }}
            autoCapitalize="none"
            autoComplete="off"
            spellCheck={false}
            disabled={searching}
            data-autofocus
          />
        </label>
        <button className={styles.primaryButton} type="submit" disabled={searching}>
          <Search size={18} />
          {searching ? "Searching…" : "Find contact"}
        </button>
      </form>

      {error && (
        <p className={styles.error} role="alert">
          {error}
        </p>
      )}

      {result && (
        <div className={styles.lookupResult}>
          <span className={styles.handle}>@{result.username}</span>
          <button
            className={styles.primaryButton}
            type="button"
            aria-label={`Send request to @${result.username}`}
            onClick={sendRequest}
            disabled={requestState !== "idle"}
          >
            <UserPlus size={18} />
            {requestState === "sending"
              ? "Sending…"
              : requestState === "sent"
                ? "Request sent"
                : requestState === "unavailable"
                  ? "Unavailable"
                  : "Send request"}
          </button>
        </div>
      )}

      {result && requestState === "sent" && (
        <p className={styles.status} role="status">
          Request sent to @{result.username}.
        </p>
      )}
    </Sheet>
  );
}
