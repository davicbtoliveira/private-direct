import { useEffect, useState, type RefObject } from "react";
import { api } from "../api/client";
import Sheet from "../components/Sheet";
import styles from "./SecuritySheets.module.css";
import { matrixSession } from "./matrixSession";
import { useSession } from "../session/sessionContext";

type Device = { id: string; name: string; created_at: string; last_seen_at: string };
export default function DevicesSheet({ onClose, returnFocusRef }: { onClose: () => void; returnFocusRef: RefObject<HTMLElement> }) {
  const { state } = useSession();
  const [devices,setDevices]=useState<Device[]>([]); const [error,setError]=useState<string|null>(null);
  const load=async()=>{try{setDevices((await api.e2eeDevices()).devices)}catch{setError("Could not load authorized devices.")}};
  useEffect(()=>{void load()},[]);
  const revoke=async(id:string)=>{if(!confirm("Revoke this device? Downloaded history on it cannot be erased."))return;try{await api.revokeE2EEDevice(id);if(state.user)await (await matrixSession(state.user.username)).rotateFutureKeys();await load()}catch{setError("Could not revoke device.")}};
  return <Sheet title="Authorized devices" onClose={onClose} returnFocusRef={returnFocusRef}><div className={styles.body}><p className={styles.notice}>Maximum 10 devices. Revocation blocks future sync; it cannot erase history already downloaded.</p>{error&&<p role="alert" className={styles.error}>{error}</p>}<ul className={styles.list}>{devices.map(device=><li key={device.id} className={styles.item}><div><strong>{device.name}</strong><small>Added {new Date(device.created_at).toLocaleDateString()} · Last access {new Date(device.last_seen_at).toLocaleString()}</small></div><button type="button" onClick={()=>void revoke(device.id)}>Revoke</button></li>)}</ul></div></Sheet>;
}
