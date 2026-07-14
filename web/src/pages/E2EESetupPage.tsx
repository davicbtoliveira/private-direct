import { useMemo, useState } from "react";
import { Navigate, useNavigate } from "react-router-dom";
import { api } from "../api/client";
import { createE2EESetup, createRecoveryPhrase } from "../e2ee/setup";
import { rememberDevice } from "../e2ee/matrixSession";
import { useSession } from "../session/sessionContext";
import AuthShell from "./AuthShell";
import styles from "./AuthShell.module.css";

export default function E2EESetupPage() {
  const { state, markE2EEReady } = useSession();
  const navigate = useNavigate();
  const phrase = useMemo(createRecoveryPhrase, []);
  const [confirmation, setConfirmation] = useState("");
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);

  if (state.user?.e2ee_ready) return <Navigate to="/chat" replace />;
  if (!state.user) return null;
  const username = state.user.username;

  const submit = async () => {
    if (confirmation.trim() !== phrase) {
      setError("Recovery phrase does not match.");
      return;
    }
    setPending(true);
    setError(null);
    try {
      const payload = await createE2EESetup(username, phrase);
      await api.setupE2EE(payload);
      rememberDevice(payload.device_id);
      markE2EEReady();
      setConfirmation("");
      navigate("/chat", { replace: true });
    } catch (cause) {
      console.error("E2EE setup failed", cause);
      setError("Could not complete encrypted messaging setup.");
      setPending(false);
    }
  };

  return (
    <AuthShell title="Protect your messages" footerLabel="" footerTo="/" footerLink="">
      <div className={styles.form}>
        <p>Save these 24 English recovery words. Losing every device and this phrase permanently loses history.</p>
        <output aria-label="Recovery phrase" className={styles.hint}>{phrase}</output>
        <label className={styles.field}>
          <span className={styles.label}>Confirm recovery phrase</span>
          <textarea
            className={styles.input}
            aria-label="Confirm recovery phrase"
            value={confirmation}
            onChange={(event) => setConfirmation(event.target.value)}
            disabled={pending}
          />
        </label>
        {error && <p role="alert" className={styles.fieldError}>{error}</p>}
        <button type="button" className={styles.primaryButton} disabled={pending} onClick={() => void submit()}>
          {pending ? "Protecting…" : "I saved my recovery phrase"}
        </button>
        <p className={styles.hint}>End-to-end encryption is beta pending external security review.</p>
      </div>
    </AuthShell>
  );
}
