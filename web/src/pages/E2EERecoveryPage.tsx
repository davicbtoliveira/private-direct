import { useState } from "react";
import { Navigate, useNavigate } from "react-router-dom";
import { recoverDevice, hasRememberedDevice } from "../e2ee/matrixSession";
import { useSession } from "../session/sessionContext";
import AuthShell from "./AuthShell";
import styles from "./AuthShell.module.css";

export default function E2EERecoveryPage() {
  const { state } = useSession();
  const navigate = useNavigate();
  const [phrase, setPhrase] = useState("");
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  if (!state.user) return null;
  if (!state.user.e2ee_ready) return <Navigate to="/setup-encryption" replace />;
  if (hasRememberedDevice()) return <Navigate to="/chat" replace />;

  const submit = async () => {
    setPending(true); setError(null);
    try { await recoverDevice(state.user!.username, phrase); navigate("/chat", { replace: true }); }
    catch (cause) {
      setError(cause instanceof Error && cause.message === "invalid_recovery_phrase" ? "Recovery phrase is invalid." : "Could not recover encrypted history.");
      setPending(false);
    }
  };

  return <AuthShell title="Recover encrypted history" footerLabel="" footerTo="/" footerLink="">
    <div className={styles.form}>
      <p>Enter your 24 recovery words to authorize this browser and restore message keys.</p>
      <label className={styles.field}><span className={styles.label}>Recovery phrase</span><textarea className={styles.input} value={phrase} onChange={event => setPhrase(event.target.value)} disabled={pending} autoComplete="off" /></label>
      {error && <p role="alert" className={styles.fieldError}>{error}</p>}
      <button type="button" className={styles.submit} disabled={pending || !phrase.trim()} onClick={() => void submit()}>{pending ? "Recovering…" : "Recover history"}</button>
      <p className={styles.hint}>The phrase is processed only on this device and is never sent to the server.</p>
    </div>
  </AuthShell>;
}
