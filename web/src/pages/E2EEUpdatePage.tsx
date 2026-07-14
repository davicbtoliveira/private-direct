import { Navigate } from "react-router-dom";
import { useSession } from "../session/sessionContext";
import AuthShell from "./AuthShell";
import styles from "./AuthShell.module.css";
export default function E2EEUpdatePage(){const{state}=useSession();if(!state.user)return null;if(state.user.protocol_version===1)return <Navigate to="/chat" replace/>;return <AuthShell title="Encryption update required" footerLabel="" footerTo="/" footerLink=""><div className={styles.form}><p role="alert">This account uses an incompatible encryption protocol. Update Private Direct before messaging.</p><p className={styles.hint}>Messaging is blocked. Plaintext fallback is never used.</p></div></AuthShell>}
