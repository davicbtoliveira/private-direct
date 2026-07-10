import { useState, type FormEvent } from "react";
import { useNavigate } from "react-router-dom";
import AuthShell from "./AuthShell";
import styles from "./AuthShell.module.css";
import { useSession } from "../session/sessionContext";

export default function LoginPage() {
  const { login, state } = useSession();
  const navigate = useNavigate();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError(null);
    setPending(true);
    try {
      await login(username.trim().toLowerCase(), password);
      navigate("/chat", { replace: true });
    } catch {
      setError("Username or password is incorrect.");
    } finally {
      setPending(false);
    }
  };

  if (state.status === "restoring") {
    return <div aria-live="polite">Restoring session…</div>;
  }

  return (
    <AuthShell
      title="Sign in"
      footerLabel="No account?"
      footerTo="/register"
      footerLink="Register"
    >
      <form className={styles.form} aria-label="Sign in form" onSubmit={onSubmit}>
        <label className={styles.field}>
          <span className={styles.label}>Username</span>
          <input
            className={styles.input}
            name="username"
            autoComplete="username"
            aria-label="Username"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            disabled={pending}
          />
        </label>
        <label className={styles.field}>
          <span className={styles.label}>Password</span>
          <input
            className={styles.input}
            type="password"
            name="password"
            autoComplete="current-password"
            aria-label="Password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            disabled={pending}
          />
        </label>
        {error && (
          <p className={styles.error} role="alert">
            {error}
          </p>
        )}
        <button type="submit" className={styles.submit} disabled={pending}>
          {pending ? "Signing in…" : "Sign in"}
        </button>
      </form>
    </AuthShell>
  );
}
