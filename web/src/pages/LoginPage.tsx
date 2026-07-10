import AuthShell from "./AuthShell";
import styles from "./AuthShell.module.css";

export default function LoginPage() {
  return (
    <AuthShell
      title="Sign in"
      footerLabel="No account?"
      footerTo="/register"
      footerLink="Register"
    >
      <form className={styles.form} aria-label="Sign in form">
        <label className={styles.field}>
          <span className={styles.label}>Username</span>
          <input
            className={styles.input}
            name="username"
            autoComplete="username"
            aria-label="Username"
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
          />
        </label>
        <button type="submit" className={styles.submit}>
          Sign in
        </button>
      </form>
    </AuthShell>
  );
}
