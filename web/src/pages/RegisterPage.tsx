import AuthShell from "./AuthShell";
import styles from "./AuthShell.module.css";

export default function RegisterPage() {
  return (
    <AuthShell
      title="Register"
      footerLabel="Already registered?"
      footerTo="/login"
      footerLink="Sign in"
    >
      <form className={styles.form} aria-label="Registration form">
        <label className={styles.field}>
          <span className={styles.label}>Invite code</span>
          <input className={styles.input} name="invite_code" aria-label="Invite code" />
        </label>
        <label className={styles.field}>
          <span className={styles.label}>Username</span>
          <input className={styles.input} name="username" aria-label="Username" />
        </label>
        <label className={styles.field}>
          <span className={styles.label}>Password</span>
          <input
            className={styles.input}
            type="password"
            name="password"
            aria-label="Password"
          />
        </label>
        <label className={styles.field}>
          <span className={styles.label}>Confirm password</span>
          <input
            className={styles.input}
            type="password"
            name="confirm_password"
            aria-label="Confirm password"
          />
        </label>
        <button type="submit" className={styles.submit}>
          Register
        </button>
      </form>
    </AuthShell>
  );
}
