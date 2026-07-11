import { useRef, useState, type RefObject } from "react";
import { Check, RefreshCw, X } from "lucide-react";
import { api } from "../api/client";
import type { ContactRequestResponse } from "../api/types";
import Sheet from "../components/Sheet";
import styles from "./ContactSheets.module.css";

type Props = {
  requests: ContactRequestResponse[];
  loading: boolean;
  loadError: string | null;
  onClose: () => void;
  onComplete: (resolvedID: number) => Promise<void>;
  onReload: () => Promise<void>;
  returnFocusRef: RefObject<HTMLElement>;
};

export default function IncomingRequestsSheet({
  requests,
  loading,
  loadError,
  onClose,
  onComplete,
  onReload,
  returnFocusRef,
}: Props) {
  const [pending, setPending] = useState<{
    id: number;
    action: "accept" | "reject";
  } | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [completion, setCompletion] = useState<string | null>(null);
  const completionRef = useRef<HTMLParagraphElement>(null);

  const resolve = async (request: ContactRequestResponse, accept: boolean) => {
    if (pending !== null) return;
    setPending({ id: request.id, action: accept ? "accept" : "reject" });
    setActionError(null);
    setCompletion(null);
    try {
      if (accept) await api.acceptRequest(request.id);
      else await api.rejectRequest(request.id);
      setCompletion(`${accept ? "Accepted" : "Rejected"} @${request.username}.`);
      await onComplete(request.id);
      completionRef.current?.focus();
    } catch {
      setActionError(`Could not ${accept ? "accept" : "reject"} @${request.username}. Try again.`);
    } finally {
      setPending(null);
    }
  };

  return (
    <Sheet
      title="Incoming requests"
      onClose={onClose}
      returnFocusRef={returnFocusRef}
    >
      {loadError && requests.length === 0 ? (
        <div className={styles.loadError} role="alert">
          <p>{loadError}</p>
          <button className={styles.retryButton} type="button" onClick={onReload}>
            <RefreshCw size={16} />
            Retry
          </button>
        </div>
      ) : loading && requests.length === 0 ? (
        <p className={styles.empty} role="status">
          Loading requests…
        </p>
      ) : requests.length === 0 ? (
        <p className={styles.empty} role="status">
          No incoming requests. New requests will appear here.
        </p>
      ) : (
        <ul className={styles.requestList}>
          {requests.map((request, index) => (
            <li
              key={request.id}
              className={styles.requestItem}
              aria-busy={pending?.id === request.id}
            >
              <span className={styles.handle}>@{request.username}</span>
              <div className={styles.requestActions}>
                <button
                  className={styles.acceptButton}
                  type="button"
                  aria-label={
                    pending?.id === request.id && pending.action === "accept"
                      ? `Accepting @${request.username}`
                      : `Accept @${request.username}`
                  }
                  onClick={() => resolve(request, true)}
                  disabled={pending !== null}
                  data-autofocus={index === 0 ? "true" : undefined}
                >
                  <Check size={17} />
                  {pending?.id === request.id && pending.action === "accept"
                    ? "Accepting…"
                    : "Accept"}
                </button>
                <button
                  className={styles.rejectButton}
                  type="button"
                  aria-label={
                    pending?.id === request.id && pending.action === "reject"
                      ? `Rejecting @${request.username}`
                      : `Reject @${request.username}`
                  }
                  onClick={() => resolve(request, false)}
                  disabled={pending !== null}
                >
                  <X size={17} />
                  {pending?.id === request.id && pending.action === "reject"
                    ? "Rejecting…"
                    : "Reject"}
                </button>
              </div>
            </li>
          ))}
        </ul>
      )}
      {loadError && requests.length > 0 && (
        <div className={styles.loadError} role="alert">
          <p>{loadError}</p>
          <button className={styles.retryButton} type="button" onClick={onReload}>
            <RefreshCw size={16} />
            Retry
          </button>
        </div>
      )}
      {actionError && (
        <p className={styles.error} role="alert">
          {actionError}
        </p>
      )}
      {completion && (
        <p
          ref={completionRef}
          className={styles.status}
          role="status"
          tabIndex={-1}
        >
          {completion}
        </p>
      )}
    </Sheet>
  );
}
