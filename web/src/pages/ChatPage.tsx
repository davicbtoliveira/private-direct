import { useCallback, useEffect, useRef, useState, type KeyboardEvent } from "react";
import { NavLink, useLocation, useNavigate, useParams } from "react-router-dom";
import { ArrowLeft, Bell, Loader, LogOut, Paperclip, Plus, RefreshCw, Send, ShieldCheck, Smartphone, X } from "lucide-react";
import AddContactSheet from "../contacts/AddContactSheet";
import IncomingRequestsSheet from "../contacts/IncomingRequestsSheet";
import { useRealtime, type PresenceState } from "../realtime/realtimeContext";
import { useSession } from "../session/sessionContext";
import type { PeerChannelState } from "../realtime/peerManager";
import { api } from "../api/client";
import { matrixSession } from "../e2ee/matrixSession";
import { queueMessage, queuedMessages, removeQueued } from "../e2ee/outbox";
import styles from "./ChatPage.module.css";
import DevicesSheet from "../e2ee/DevicesSheet";
import Sheet from "../components/Sheet";
import securityStyles from "../e2ee/SecuritySheets.module.css";
import { identityFingerprint, knownIdentity, trustIdentity } from "../e2ee/identity";
import { loadDraft, saveDraft } from "../e2ee/draftStore";
import { validateEventPage } from "../e2ee/eventChain";
import { sanitizeFilename, type AttachmentInfo, type MediaManifest } from "../rtc/mediaTransfer";
import { useMedia } from "../rtc/useMedia";

type DeliveryState = "queued" | "sending" | "sent" | "delivered" | "not-delivered";

interface ChatMessage {
  id: string;
  sequence: number;
  direction: "sent" | "received";
  content: string;
  delivery: DeliveryState;
  timestamp: string;
}

interface MessageEnvelope {
  type: "message";
  id: string;
  content: string;
  timestamp: string;
}

interface AckEnvelope {
  type: "ack";
  message_id: string;
}

interface EncryptedMessageEnvelope {
  type: "encrypted_message";
  id: string;
  ciphertext: Record<string, unknown>;
  timestamp: string;
}

type PeerEnvelope = MessageEnvelope | EncryptedMessageEnvelope | AckEnvelope;

const COMPOSER_LIMIT = 4000;
const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
const MAX_ATTACHMENTS = 10;
const MAX_IMAGE_SIZE = 20 * 1024 * 1024;
const MAX_VIDEO_SIZE = 100 * 1024 * 1024;
const VALID_MEDIA_TYPES = new Set(["image/jpeg", "image/png", "image/webp", "image/gif", "video/mp4", "video/webm"]);

type SelectedFile = { file: File; preview: string; error?: string };

function mediaFileError(file: File): string | undefined {
  if (!VALID_MEDIA_TYPES.has(file.type)) return "Unsupported file type";
  const limit = file.type.startsWith("video/") ? MAX_VIDEO_SIZE : MAX_IMAGE_SIZE;
  return file.size > limit ? `File too large (max ${limit / 1024 / 1024} MiB)` : undefined;
}

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KiB`;
  return `${(bytes / 1024 / 1024).toFixed(1)} MiB`;
}

function waitForOpen(channel: RTCDataChannel): Promise<void> {
  if (channel.readyState === "open") return Promise.resolve();
  return new Promise((resolve, reject) => {
    const timer = window.setTimeout(() => reject(new Error("media channel timeout")), 15_000);
    channel.addEventListener("open", () => { window.clearTimeout(timer); resolve(); }, { once: true });
    channel.addEventListener("close", () => { window.clearTimeout(timer); reject(new Error("media channel closed")); }, { once: true });
  });
}

const presenceLabels: Record<PresenceState, string> = {
  connecting: "Connecting",
  online: "Online",
  offline: "Offline",
};

const channelLabels: Record<PeerChannelState, string> = {
  idle: "Not connected",
  negotiating: "Negotiating",
  "directly-connected": "Directly connected",
  offline: "Contact offline",
  failed: "Connection failed",
};

function isValidUUID(value: unknown): value is string {
  return typeof value === "string" && UUID_RE.test(value);
}

function validEnvelope(data: unknown): PeerEnvelope | null {
  if (typeof data !== "object" || data === null) return null;
  const obj = data as Record<string, unknown>;
  if (obj.type !== "message" && obj.type !== "encrypted_message" && obj.type !== "ack") return null;
  if (obj.type === "ack") {
    if (!isValidUUID(obj.message_id)) return null;
    return { type: "ack", message_id: obj.message_id as string };
  }
  if (obj.type === "encrypted_message") {
    if (!isValidUUID(obj.id) || typeof obj.timestamp !== "string" || typeof obj.ciphertext !== "object" || obj.ciphertext === null) return null;
    return { type: "encrypted_message", id: obj.id, timestamp: obj.timestamp, ciphertext: obj.ciphertext as Record<string, unknown> };
  }
  if (!isValidUUID(obj.id)) return null;
  if (typeof obj.content !== "string" || typeof obj.timestamp !== "string") return null;
  if (obj.content.length === 0 || [...obj.content].length > COMPOSER_LIMIT) return null;
  return {
    type: "message",
    id: obj.id as string,
    content: obj.content as string,
    timestamp: obj.timestamp as string,
  };
}

function codePointLength(text: string): number {
  return [...text].length;
}

export default function ChatPage() {
  const { username } = useParams();
  const hasContact = Boolean(username);
  const [showConversation, setShowConversation] = useState(hasContact);
  const { state, logout } = useSession();
  const {
    contacts,
    requests,
    contactsLoading,
    requestsLoading,
    contactsError,
    requestsError,
    presence,
    peerChannels,
    realtimeState,
    announcement,
    mailboxRevision,
    refreshContacts,
    refreshRequests,
    refreshContactState,
    removeRequest,
    connectPeer,
    sendToPeer,
    setPeerMessageHandler,
    openPeerMedia,
    setPeerMediaHandler,
    retryPeer,
  } = useRealtime();
  const navigate = useNavigate();
  const location = useLocation();
  const me = state.user;
  const [addOpen, setAddOpen] = useState(false);
  const [requestsOpen, setRequestsOpen] = useState(false);
  const registrationWarning = (location.state as { warning?: string } | null)?.warning;
  const addTriggerRef = useRef<HTMLButtonElement>(null);
  const requestsTriggerRef = useRef<HTMLButtonElement>(null);
  const devicesTriggerRef = useRef<HTMLButtonElement>(null);
  const identityTriggerRef = useRef<HTMLButtonElement>(null);
  const [devicesOpen,setDevicesOpen]=useState(false);
  const [identityOpen,setIdentityOpen]=useState(false);
  const [fingerprint,setFingerprint]=useState("");
  const [identityChanged,setIdentityChanged]=useState(false);
  const [protocolMismatch,setProtocolMismatch]=useState(false);
  const [ephemeralStorage,setEphemeralStorage]=useState(false);

  // Chat state: per-contact transcripts, drafts, unread, deduplication
  const [composer, setComposer] = useState("");
  const [composerHeight, setComposerHeight] = useState<string>("44px");
  const [selectedFiles, setSelectedFiles] = useState<SelectedFile[]>([]);
  const [sendingMedia, setSendingMedia] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const { attachments, sendAttachment, receiveManifest, receiveAttachmentComplete, receiveFrame, clearAttachments } = useMedia();

  const transcriptsRef = useRef<Record<string, ChatMessage[]>>({});
  const unreadsRef = useRef<Record<string, number>>({});
  const receivedIDsRef = useRef(new Set<string>());
  const draftsRef = useRef<Record<string, string>>({});
  const earliestRef = useRef<Record<string, number>>({});
  const hasOlderRef = useRef<Record<string, boolean>>({});
  const loadingOlderRef = useRef(false);
  const [, forceUpdate] = useState(0);

  const rerender = useCallback(() => forceUpdate((n) => n + 1), []);

  const contact = contacts.find((c) => c.username === username);

  useEffect(()=>{if(!contact)return;let cancelled=false;void api.contactIdentity(contact.username).then(async result=>{const next=await identityFingerprint(result.identity_keys);if(cancelled)return;setProtocolMismatch(result.protocol_version!==1);const known=knownIdentity(contact.username);if(known===null)trustIdentity(contact.username,next);setFingerprint(next);setIdentityChanged(known!==null&&known!==next)}).catch(()=>{if(!cancelled)setIdentityChanged(true)});return()=>{cancelled=true}},[contact]);

  // Get or initialize transcript for username
  const transcript = username ? transcriptsRef.current[username] ?? [] : [];
  const composerValue = username ? composer : "";

  // Load draft when changing conversations
  useEffect(() => {
    if (!username) return;
    if(draftsRef.current[username]!==undefined){setComposer(draftsRef.current[username]);return}
    if(me)void loadDraft(me.username,username).then(value=>{draftsRef.current[username]=value;setComposer(value)}).catch(()=>setEphemeralStorage(true));
  }, [me,username]);

  useEffect(()=>{void navigator.storage?.persisted().then(persisted=>setEphemeralStorage(!persisted)).catch(()=>setEphemeralStorage(true))},[]);

  useEffect(() => {
    if (!contact || !me || !username) return;
    let cancelled = false;
    void (async () => {
      try {
        const cryptoSession = await matrixSession(me.username);
        const response = await api.listMessages(contact.id);
        await validateEventPage([...response.messages].reverse());
        const loaded: ChatMessage[] = [];
        for (const stored of [...response.messages].reverse()) {
          const senderUsername = stored.sender_id === me.id ? me.username : contact.username;
          const decrypted = await cryptoSession.decrypt(
            me.id,
            contact.id,
            senderUsername,
            stored.id,
            stored.created_at,
            stored.ciphertext,
          );
          const body = decrypted.content;
          if (typeof body.body !== "string") continue;
          loaded.push({
            id: stored.id,
            sequence: stored.sequence,
            direction: stored.sender_id === me.id ? "sent" : "received",
            content: body.body,
            delivery: stored.sender_id === me.id && stored.delivered ? "delivered" : "sent",
            timestamp: typeof body.sent_at === "string" ? body.sent_at : stored.created_at,
          });
          if (stored.recipient_id === me.id) void api.markMessageDelivered(stored.id);
        }
        if (!cancelled) {
          const existing = transcriptsRef.current[username] ?? [];
          const deleted=new Set(response.deleted??[]);
          const merged = new Map(existing.map(message=>[message.id,message])); loaded.forEach(message=>merged.set(message.id,message));
          transcriptsRef.current[username] = [...merged.values()].filter(message=>!deleted.has(message.id)).sort((a,b)=>a.sequence-b.sequence);
          if(loaded.length){earliestRef.current[username]=Math.min(...loaded.map(message=>message.sequence));hasOlderRef.current[username]=response.messages.length===50;void api.markConversationRead(contact.id,Math.max(...loaded.map(message=>message.sequence))).catch(()=>undefined)}
          rerender();
        }
      } catch {
        // Keep current transcript when encrypted history is temporarily unavailable.
      }
    })();
    return () => { cancelled = true; };
  }, [contact, mailboxRevision, me, rerender, username]);

  useEffect(()=>{void api.unreadMessages().then(({unread})=>{for(const item of contacts)unreadsRef.current[item.username]=unread[String(item.id)]??0;rerender()}).catch(()=>undefined)},[contacts,mailboxRevision,rerender]);

  const loadOlder = async () => {
    if(!contact||!me||!username||loadingOlderRef.current||!hasOlderRef.current[username])return;
    loadingOlderRef.current=true;
    try{const response=await api.listMessages(contact.id,earliestRef.current[username]);const cryptoSession=await matrixSession(me.username);const older:ChatMessage[]=[];for(const stored of [...response.messages].reverse()){const decrypted=await cryptoSession.decrypt(me.id,contact.id,stored.sender_id===me.id?me.username:contact.username,stored.id,stored.created_at,stored.ciphertext);if(typeof decrypted.content.body!=="string")continue;older.push({id:stored.id,sequence:stored.sequence,direction:stored.sender_id===me.id?"sent":"received",content:decrypted.content.body,delivery:stored.sender_id===me.id&&stored.delivered?"delivered":"sent",timestamp:typeof decrypted.content.sent_at==="string"?decrypted.content.sent_at:stored.created_at})}const deleted=new Set(response.deleted??[]);const existing=(transcriptsRef.current[username]??[]).filter(message=>!deleted.has(message.id));const ids=new Set(existing.map(message=>message.id));transcriptsRef.current[username]=[...older.filter(message=>!ids.has(message.id)&&!deleted.has(message.id)),...existing].sort((a,b)=>a.sequence-b.sequence);if(older.length)earliestRef.current[username]=Math.min(...older.map(message=>message.sequence));hasOlderRef.current[username]=response.messages.length===50;rerender()}finally{loadingOlderRef.current=false}
  };

  useEffect(() => {
    if (!me || realtimeState !== "connected") return;
    void (async()=>{for(const item of await queuedMessages()){try{await api.createMessage(item.id,item.to,item.ciphertext,item.chain);await removeQueued(item.id);for(const messages of Object.values(transcriptsRef.current)){const found=messages.find(message=>message.id===item.id);if(found)found.delivery="sent"}}catch{continue}}rerender()})();
  },[mailboxRevision,me,realtimeState,rerender]);

  const setComposerWithDraft = useCallback((value: string) => {
    if (codePointLength(value) > COMPOSER_LIMIT) return;
    setComposer(value);
    if (username) draftsRef.current[username] = value;
    if(username&&me)void saveDraft(me.username,username,value).catch(()=>setEphemeralStorage(true));
  }, [me,username]);
  useEffect(() => {
    if (!contact || !username || !me) return;
    const contactPresence = presence[contact.id];
    if (contactPresence === "online") {
      connectPeer(contact.id);
    }
  }, [connectPeer, contact, contact?.id, username, presence, me]);

  // Subscribe to incoming messages
  useEffect(() => {
    setPeerMessageHandler((fromUserId, data) => {
      try {
        const media = JSON.parse(data) as Record<string, unknown>;
        if (media.type === "media_manifest" && typeof media.messageId === "string" && Array.isArray(media.attachments)) {
          receiveManifest(fromUserId, media as MediaManifest);
          rerender();
          return;
        }
        if (media.type === "attachment_complete") {
          receiveAttachmentComplete(fromUserId, media);
          return;
        }
      } catch {
        return;
      }
      const envelope = validEnvelope(data);
      if (!envelope) return;

      const fromContact = contacts.find((c) => c.id === fromUserId);
      if (!fromContact) return;

      const contactUsername = fromContact.username;

      if (envelope.type === "ack") {
        const messagesFor = transcriptsRef.current[contactUsername];
        if (!messagesFor) return;
        const found = messagesFor.find(
          (m) => m.id === envelope.message_id
        );
        if (found && found.delivery === "sending") {
          found.delivery = "delivered";
          rerender();
        }
        (window as any).__stopAckTimer?.(envelope.message_id);
        return;
      }

      if (envelope.type === "message") {
        if (receivedIDsRef.current.has(envelope.id)) {
          // Replay ack for duplicate
          const contactID = fromContact.id;
          sendToPeer(contactID, JSON.stringify({ type: "ack", message_id: envelope.id }));
          return;
        }
        receivedIDsRef.current.add(envelope.id);

        const message: ChatMessage = {
          id: envelope.id,
          sequence: Number.MAX_SAFE_INTEGER,
          direction: "received",
          content: envelope.content,
          delivery: "delivered",
          timestamp: envelope.timestamp,
        };

        if (!transcriptsRef.current[contactUsername]) {
          transcriptsRef.current[contactUsername] = [];
        }
        transcriptsRef.current[contactUsername].push(message);

        if (contactUsername !== username) {
          unreadsRef.current[contactUsername] =
            (unreadsRef.current[contactUsername] ?? 0) + 1;
        }

        // Send ack
        sendToPeer(fromUserId, JSON.stringify({ type: "ack", message_id: envelope.id }));
        rerender();
      }

      if (envelope.type === "encrypted_message" && me) {
        if (receivedIDsRef.current.has(envelope.id)) {
          sendToPeer(fromUserId, JSON.stringify({ type: "ack", message_id: envelope.id }));
          return;
        }
        receivedIDsRef.current.add(envelope.id);
        void (async()=>{try{const cryptoSession=await matrixSession(me.username);const decrypted=await cryptoSession.decrypt(me.id,fromContact.id,fromContact.username,envelope.id,envelope.timestamp,envelope.ciphertext);const body=decrypted.content;if(typeof body.body!=="string")return;if(!transcriptsRef.current[contactUsername])transcriptsRef.current[contactUsername]=[];transcriptsRef.current[contactUsername].push({id:envelope.id,sequence:Number.MAX_SAFE_INTEGER,direction:"received",content:body.body,delivery:"delivered",timestamp:typeof body.sent_at==="string"?body.sent_at:envelope.timestamp});sendToPeer(fromUserId,JSON.stringify({type:"ack",message_id:envelope.id}));rerender()}catch{receivedIDsRef.current.delete(envelope.id)}})();
      }
    });

    return () => {
      setPeerMessageHandler(null);
    };
  }, [contacts, me, receiveAttachmentComplete, receiveManifest, rerender, sendToPeer, setPeerMessageHandler, username]);

  useEffect(() => {
    setPeerMediaHandler(receiveFrame);
    return () => setPeerMediaHandler(null);
  }, [receiveFrame, setPeerMediaHandler]);

  // Clear unread when opening a conversation
  useEffect(() => {
    if (!username) return;
    unreadsRef.current[username] = 0;
    rerender();
  }, [username]);

  const onLogout = async () => {
    transcriptsRef.current = {};
    unreadsRef.current = {};
    receivedIDsRef.current = new Set();
    clearAttachments();
    await logout();
    navigate("/login", { replace: true });
  };

  const openRequests = () => {
    setRequestsOpen(true);
    void refreshRequests();
  };

  const completeRequest = async (resolvedID: number) => {
    removeRequest(resolvedID);
    await refreshContactState();
  };

  const requestsLabel =
    requests.length > 0
      ? `Incoming requests, ${requests.length} pending`
      : "Incoming requests";

  const processFiles = useCallback((files: FileList) => {
    const available = Math.max(0, MAX_ATTACHMENTS - selectedFiles.length);
    const next = Array.from(files).slice(0, available).map((file) => ({
      file,
      preview: file.type.startsWith("image/") ? URL.createObjectURL(file) : "",
      error: mediaFileError(file),
    }));
    setSelectedFiles((current) => [...current, ...next]);
  }, [selectedFiles.length]);

  const removeFile = (index: number) => {
    setSelectedFiles((current) => {
      const removed = current[index];
      if (removed?.preview) URL.revokeObjectURL(removed.preview);
      return current.filter((_, itemIndex) => itemIndex !== index);
    });
  };

  const canSend = (composerValue.trim().length > 0 || selectedFiles.some((item) => !item.error)) && !identityChanged && !protocolMismatch;

  const sendMessage = async () => {
    if (!canSend || !username || !contact) return;
    const contactId = contact.id;
    const content = composerValue;
    const files = selectedFiles.filter((item) => !item.error);
    const id = crypto.randomUUID();
    const timestamp = new Date().toISOString();

    const message: ChatMessage = {
      id,
      sequence: Number.MAX_SAFE_INTEGER,
      direction: "sent",
      content,
      delivery: "sending",
      timestamp,
    };

    if (!transcriptsRef.current[username]) {
      transcriptsRef.current[username] = [];
    }
    transcriptsRef.current[username].push(message);

    setComposer("");
    if (username) draftsRef.current[username] = "";
    setSelectedFiles([]);
    for (const selected of selectedFiles) if (selected.preview) URL.revokeObjectURL(selected.preview);

    try {
      const cryptoSession = await matrixSession(me!.username);
      const ciphertext = await cryptoSession.encrypt(me!.id, contactId, contact.username, { body: content, sent_at: timestamp });
      const chain=await cryptoSession.createEventChain(contactId,id,ciphertext);
      await queueMessage({ id, to: contactId, ciphertext,chain });
      message.delivery = "queued";
      rerender();
      await api.createMessage(id, contactId, ciphertext,chain);
      await removeQueued(id);
      message.delivery = "sent";
      sendToPeer(contactId, JSON.stringify({ type: "encrypted_message", id, ciphertext, timestamp }));
    } catch (error) {
      const status = typeof error === "object" && error && "status" in error ? Number(error.status) : 0;
      message.delivery = status >= 400 && status < 500 && status !== 429 ? "not-delivered" : "queued";
    }
    rerender();

    if (files.length > 0) {
      const manifest: MediaManifest = {
        messageId: id,
        attachments: files.map(({ file }, index) => ({
          id: crypto.randomUUID(),
          index,
          filename: sanitizeFilename(file.name),
          mime: file.type,
          size: file.size,
        })),
      };
      sendToPeer(contactId, JSON.stringify({ type: "media_manifest", ...manifest }));
      const mediaChannel = openPeerMedia(contactId);
      if (mediaChannel) {
        setSendingMedia(true);
        try {
          await waitForOpen(mediaChannel);
          for (let index = 0; index < files.length; index++) {
            const selected = files[index];
            await sendAttachment(
              contactId,
              (data) => sendToPeer(contactId, data),
              mediaChannel,
              id,
              selected.file,
              index,
              manifest.attachments[index].id,
            );
          }
        } catch {
          // Keep message; attachment state remains incomplete.
        } finally {
          setSendingMedia(false);
        }
      }
    }
  };

  const handleKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      void sendMessage();
    }
  };

  const retryMessage = async (msgId: string) => {
    if (!username || !contact) return;
    const messages = transcriptsRef.current[username];
    if (!messages) return;
    const message = messages.find((m) => m.id === msgId);
    if (!message) return;
    message.delivery = "sending";
    rerender();

    try {
      const cryptoSession = await matrixSession(me!.username);
      const ciphertext = await cryptoSession.encrypt(me!.id, contact.id, contact.username, { body: message.content, sent_at: message.timestamp });
      const chain=await cryptoSession.createEventChain(contact.id,message.id,ciphertext);
      await queueMessage({ id: message.id, to: contact.id, ciphertext,chain });
      message.delivery="queued";rerender();
      await api.createMessage(message.id, contact.id, ciphertext,chain);
      await removeQueued(message.id);
      message.delivery = "sent";
      sendToPeer(contact.id, JSON.stringify({ type: "encrypted_message", id: message.id, ciphertext, timestamp: message.timestamp }));
    } catch { message.delivery = "not-delivered"; }
    rerender();
  };

  const deleteMessage=async(messageID:string,scope:"self"|"both")=>{if(!username||!me)return;const warning=scope==="both"?"Delete for both? Revoked devices and operator backups may still retain ciphertext.":"Delete from all your authorized devices?";if(!confirm(warning))return;try{const createdAt=new Date().toISOString();const signed=await (await matrixSession(me.username)).signTombstone(messageID,scope,createdAt);await api.deleteMessage(messageID,{scope,...signed});transcriptsRef.current[username]=(transcriptsRef.current[username]??[]).filter(message=>message.id!==messageID);rerender()}catch{/* Keep message if signed deletion fails. */}};

  const handleAutoGrow = (el: HTMLTextAreaElement) => {
    el.style.height = "auto";
    const h = Math.min(el.scrollHeight, 160);
    setComposerHeight(`${h}px`);
    el.style.height = `${h}px`;
  };

  const peerState = contact ? (peerChannels[contact.id] ?? "idle") : "idle";

  const renderAttachment = (attachment: AttachmentInfo) => {
    if (attachment.state === "complete" && attachment.objectUrl) {
      return attachment.mime.startsWith("video/")
        ? <video src={attachment.objectUrl} controls className={styles.attachmentMedia} />
        : <img src={attachment.objectUrl} alt={attachment.filename} className={styles.attachmentMedia} />;
    }
    if (attachment.state === "failed") return <span className={styles.attachmentFailed}>{attachment.error ?? "Transfer failed"}</span>;
    return <div className={styles.attachmentTransferring}><span className={styles.attachmentFilename}>{attachment.filename}</span><div className={styles.progressBar}><div className={styles.progressFill} style={{ width: `${attachment.progress}%` }} /></div><span className={styles.attachmentProgress}>{attachment.progress}%</span></div>;
  };

  return (
    <>
      <div className={styles.workspaceFrame}>
        {registrationWarning && (
          <div className={styles.takeoverNotice} role="alert">
            {registrationWarning}
          </div>
        )}
        {ephemeralStorage && <div className={styles.takeoverNotice} role="status">Storage may be ephemeral. Private browsing can lose this device, queue, and drafts.</div>}
        {realtimeState === "replaced" && (
          <div className={styles.takeoverNotice} role="alert">
            Messaging continued in another tab
          </div>
        )}
        <div
          className={`${styles.workspace} ${showConversation ? styles.showConversation : ""}`}
          data-testid="workspace"
        >
          <aside className={styles.rail} aria-label="Contacts">
            <div className={styles.railHeader}>
              <span className={styles.railBrand}>Private Direct</span>
              <div className={styles.railActions}>
                <button
                  ref={addTriggerRef}
                  className={styles.iconBtn}
                  aria-label="Add contact"
                  title="Add contact"
                  onClick={() => {
                    setAddOpen(true);
                    void refreshContacts();
                  }}
                >
                  <Plus size={18} />
                </button>
                <button
                  ref={requestsTriggerRef}
                  className={styles.iconBtn}
                  aria-label={requestsLabel}
                  title="Incoming requests"
                  onClick={openRequests}
                >
                  <Bell size={18} />
                  {requests.length > 0 && (
                    <span className={styles.requestBadge} aria-hidden="true">
                      {requests.length > 99 ? "99+" : requests.length}
                    </span>
                  )}
                </button>
                <button
                  ref={devicesTriggerRef}
                  className={styles.iconBtn}
                  aria-label="Authorized devices"
                  title="Authorized devices"
                  onClick={() => setDevicesOpen(true)}
                >
                  <Smartphone size={18} />
                </button>
                <button
                  className={styles.iconBtn}
                  aria-label="Sign out"
                  title="Sign out"
                  onClick={onLogout}
                >
                  <LogOut size={18} />
                </button>
              </div>
            </div>
            {me && (
              <div className={styles.me}>
                <span className={styles.handle}>@{me.username}</span>
                <span className={styles.realtimeStatus}>
                  {realtimeState === "connected"
                    ? "Realtime connected"
                    : realtimeState === "replaced"
                      ? "Realtime replaced"
                      : "Realtime connecting"}
                </span>
              </div>
            )}
            <nav className={styles.contactList} aria-label="Contact list">
              {contactsLoading && contacts.length === 0 ? (
                <p className={styles.empty} role="status">
                  Loading contacts…
                </p>
              ) : contactsError ? (
                <div className={styles.listError} role="alert">
                  <p>{contactsError}</p>
                  <button type="button" onClick={refreshContacts}>
                    <RefreshCw size={16} />
                    Reload contacts
                  </button>
                </div>
              ) : contacts.length === 0 ? (
                <p className={styles.empty}>
                  No contacts yet. Find someone by exact handle.
                </p>
              ) : (
                contacts.map((contact) => {
                  const contactPresence = presence[contact.id] ?? "connecting";
                  const contactUnread = unreadsRef.current[contact.username] ?? 0;
                  return (
                    <NavLink
                      key={contact.id}
                      to={`/chat/${encodeURIComponent(contact.username)}`}
                      className={({ isActive }) =>
                        `${styles.contactLink} ${isActive ? styles.activeContact : ""}`
                      }
                      aria-label={`@${contact.username}, ${contactPresence}${contactUnread > 0 ? `, ${contactUnread} unread` : ""}`}
                      onClick={() => setShowConversation(true)}
                    >
                      <span className={styles.contactHandle}>@{contact.username}</span>
                      <span className={styles.contactPresence}>
                        {contactUnread > 0 && (
                          <span className={styles.unreadBadge} aria-hidden="true">
                            {contactUnread > 99 ? "99+" : contactUnread}
                          </span>
                        )}
                        <span
                          className={`${styles.presenceDot} ${styles[contactPresence]}`}
                          aria-hidden="true"
                        />
                        {presenceLabels[contactPresence]}
                      </span>
                    </NavLink>
                  );
                })
              )}
            </nav>
          </aside>

          <section className={styles.conversation} aria-label="Conversation">
            <header className={styles.conversationHeader}>
              <button
                className={styles.back}
                aria-label="Back to contacts"
                onClick={() => setShowConversation(false)}
              >
                <ArrowLeft size={20} />
              </button>
              <span className={styles.handle}>
                {username ? `@${username}` : "No conversation selected"}
              </span>
              {username && <button ref={identityTriggerRef} type="button" className={styles.identityBtn} onClick={()=>setIdentityOpen(true)} aria-label="Verify contact identity"><ShieldCheck size={17}/>{identityChanged?"Identity changed":"Verify"}</button>}
              {username && (
                <span className={styles.channelState} aria-live="polite">
                  {channelLabels[peerState]}
                </span>
              )}
            </header>

            <div className={styles.conversationBody}>
              {!username ? (
                <p className={styles.placeholder}>
                  Select a contact to start a direct conversation.
                </p>
              ) : (
                <div className={styles.messagesArea} onScroll={event=>{if(event.currentTarget.scrollTop===0)void loadOlder()}}>
                  {hasOlderRef.current[username] && <button type="button" className={styles.loadOlder} onClick={()=>void loadOlder()}>Load older messages</button>}
                  {transcript.length === 0 && peerState !== "directly-connected" && (
                    <p className={styles.ephemeralNotice}>
                      Messages are temporary and only visible in this tab.
                    </p>
                  )}
                  {transcript.map((msg) => (
                    <div
                      key={msg.id}
                      className={`${styles.messageRow} ${msg.direction === "sent" ? styles.sent : styles.received}`}
                    >
                      <div className={styles.messageBubble}>
                        {msg.content && <span className={styles.messageContent}>{msg.content}</span>}
                        {(attachments[msg.id] ?? []).length > 0 && <div className={styles.attachmentGrid}>{attachments[msg.id].map((attachment) => <div key={attachment.id} className={styles.attachmentTile}>{renderAttachment(attachment)}</div>)}</div>}
                        <span className={styles.messageMeta}>
                          {msg.direction === "sent" && (
                            <span className={styles.delivery} data-delivery={msg.delivery}>
                              {msg.delivery === "sending"
                                ? "sending"
                                : msg.delivery === "queued"
                                  ? "queued"
                                : msg.delivery === "sent"
                                  ? "sent"
                                : msg.delivery === "delivered"
                                  ? "delivered"
                                  : "not delivered"}
                            </span>
                          )}
                          {new Date(msg.timestamp).toLocaleTimeString([], {
                            hour: "2-digit",
                            minute: "2-digit",
                          })}
                        </span>
                        {msg.delivery === "not-delivered" && msg.direction === "sent" && (
                          <button
                            type="button"
                            className={styles.retryBtn}
                            onClick={() => void retryMessage(msg.id)}
                            aria-label={`Retry sending message`}
                          >
                            Try again
                          </button>
                        )}
                        <span className={styles.deleteActions}><button type="button" onClick={()=>void deleteMessage(msg.id,"self")}>Delete for me</button><button type="button" onClick={()=>void deleteMessage(msg.id,"both")}>Delete for both</button></span>
                      </div>
                    </div>
                  ))}
                </div>
              )}

              {username && (
                <div className={styles.composerArea}>
                  <input ref={fileInputRef} className={styles.fileInput} type="file" multiple accept={[...VALID_MEDIA_TYPES].join(",")} onChange={(event) => { if (event.target.files) processFiles(event.target.files); event.target.value = ""; }} />
                  {identityChanged && <div className={styles.identityWarning} role="alert">Contact identity changed. Verify safety number before sending.</div>}
                  {protocolMismatch && <div className={styles.identityWarning} role="alert">Contact uses an incompatible encryption protocol. They must update before messaging. Plaintext fallback is disabled.</div>}
                  {peerState === "offline" && (
                    <p className={styles.offlineNotice}>
                      {contact?.username ?? username} is offline. Messages cannot be delivered.
                    </p>
                  )}
                  {peerState === "failed" && (
                    <div className={styles.failedBanner}>
                      <span>Connection failed.</span>
                      <button type="button" onClick={() => retryPeer(contact!.id)}>
                        Try again
                      </button>
                    </div>
                  )}
                  {sendingMedia && <div className={styles.sendingOverlay}><Loader size={18} className={styles.sendingSpinner} /><span>Sending attachments…</span></div>}
                  {selectedFiles.length > 0 && <div className={styles.previewTray} aria-label="Selected files">{selectedFiles.map((selected, index) => <div key={`${selected.file.name}-${index}`} className={`${styles.previewItem} ${selected.error ? styles.previewInvalid : ""}`}>{selected.preview && <img src={selected.preview} alt="" className={styles.previewThumb} />}<div className={styles.previewInfo}><span className={styles.previewName}>{selected.file.name}</span><span className={styles.previewSize}>{formatFileSize(selected.file.size)}</span>{selected.error && <span className={styles.previewError}>{selected.error}</span>}</div><button type="button" className={styles.previewRemove} onClick={() => removeFile(index)} aria-label={`Remove ${selected.file.name}`}><X size={14} /></button></div>)}</div>}
                  <div className={styles.composerRow}>
                    <button type="button" className={styles.attachBtn} disabled={peerState !== "directly-connected" || sendingMedia} onClick={() => fileInputRef.current?.click()} aria-label="Attach files"><Paperclip size={18} /></button>
                    <textarea
                      className={styles.composer}
                      value={composerValue}
                      onChange={(e) => {
                        setComposerWithDraft(e.target.value);
                        handleAutoGrow(e.target);
                      }}
                      onKeyDown={handleKeyDown}
                      placeholder="Type a message…"
                      rows={1}
                      aria-label="Message"
                      style={{ height: composerHeight }}
                      ref={(el) => {
                        if (el) handleAutoGrow(el);
                      }}
                    />
                    <button
                      type="button"
                      className={styles.sendBtn}
                      onClick={() => void sendMessage()}
                      disabled={!canSend || sendingMedia}
                      aria-label="Send message"
                    >
                      <Send size={18} />
                    </button>
                  </div>
                </div>
              )}
            </div>
          </section>
        </div>
        <span className="sr-only" role="status" aria-live="polite">
          {announcement}
        </span>
      </div>

      {addOpen && (
        <AddContactSheet
          onClose={() => setAddOpen(false)}
          onComplete={refreshContacts}
          returnFocusRef={addTriggerRef}
        />
      )}
      {requestsOpen && (
        <IncomingRequestsSheet
          requests={requests}
          loading={requestsLoading}
          loadError={requestsError}
          onClose={() => setRequestsOpen(false)}
          onComplete={completeRequest}
          onReload={refreshRequests}
          returnFocusRef={requestsTriggerRef}
        />
      )}
      {devicesOpen && <DevicesSheet onClose={()=>setDevicesOpen(false)} returnFocusRef={devicesTriggerRef} onRemoveCurrent={onLogout}/>} 
      {identityOpen && <Sheet title={`Verify @${username}`} onClose={()=>setIdentityOpen(false)} returnFocusRef={identityTriggerRef}><div className={securityStyles.body}><p>Compare this safety number through another trusted channel.</p><p className={securityStyles.number}>{fingerprint || "Identity unavailable"}</p>{identityChanged&&<button type="button" onClick={()=>{trustIdentity(username!,fingerprint);setIdentityChanged(false);setIdentityOpen(false)}}>I confirmed this new identity</button>}</div></Sheet>}
    </>
  );
}
