import { ReactNode } from "react";
import { Link } from "react-router-dom";
import styles from "./AuthShell.module.css";

type AuthShellProps = {
  title: string;
  footerLabel: string;
  footerTo: string;
  footerLink: string;
  children: ReactNode;
};

export default function AuthShell({
  title,
  footerLabel,
  footerTo,
  footerLink,
  children,
}: AuthShellProps) {
  return (
    <div className={styles.shell} data-testid="auth-shell">
      <div className={styles.brandRow}>
        <span className={styles.brand}>Private Direct</span>
      </div>
      <div className={styles.center}>
        <div className={styles.card}>
          <h1 className={styles.cardTitle}>{title}</h1>
          {children}
          <p className={styles.link}>
            {footerLabel}{" "}
            <Link to={footerTo}>{footerLink}</Link>
          </p>
        </div>
      </div>
    </div>
  );
}
