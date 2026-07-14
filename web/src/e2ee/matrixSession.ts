import {
  DecryptionSettings,
  DeviceId,
  DeviceLists,
  EncryptionSettings,
  OlmMachine,
  RequestType,
  RoomId,
  TrustRequirement,
  UserId,
} from "@matrix-org/matrix-sdk-crypto-wasm";
import { api } from "../api/client";
import { mnemonicToSeedSync, validateMnemonic } from "@scure/bip39";
import { wordlist } from "@scure/bip39/wordlists/english";
import { clearMasterKeys, loadMasterKey, saveMasterKey } from "./keyStore";
import { clearDraftStorage } from "./draftStore";

const DEVICE_KEY = "private-direct-device-id";
let session: MatrixSession | null = null;

type MatrixRequest = {
  id: string;
  type: RequestType;
  body: string;
  event_type?: string;
  txn_id?: string;
};

function matrixUser(username: string) { return new UserId(`@${username}:private-direct`); }
function roomFor(a: number, b: number) { const [low, high] = a < b ? [a,b] : [b,a]; return new RoomId(`!dm_${low}_${high}:private-direct`); }
function decode64(value: string) { return Uint8Array.from(atob(value), character => character.charCodeAt(0)); }
function encode64(value: Uint8Array) { let result = ""; value.forEach(byte => result += String.fromCharCode(byte)); return btoa(result); }

class MatrixSession {
  private cursor = "0";
  private rooms = new Set<string>();
  private constructor(private machine: OlmMachine, private deviceID: string, private username: string) {}

  static async open(username: string) {
    const deviceID = localStorage.getItem(DEVICE_KEY);
    if (!deviceID) throw new Error("e2ee_device_missing");
    const machine = await OlmMachine.initialize(matrixUser(username), new DeviceId(deviceID), `private-direct-${username}-${deviceID}`);
    const value = new MatrixSession(machine, deviceID, username);
    await value.flushOutgoing();
    await value.syncKeys();
    return value;
  }

  private async sendRequest(request: MatrixRequest): Promise<string> {
    const body = JSON.parse(request.body || "{}") as Record<string, unknown>;
    if (request.type === RequestType.KeysUpload) return JSON.stringify(await api.e2eeKeysUpload({ ...body, device_id: this.deviceID }));
    if (request.type === RequestType.KeysQuery) return JSON.stringify(await api.e2eeKeysQuery(body));
    if (request.type === RequestType.KeysClaim) return JSON.stringify(await api.e2eeKeysClaim(body));
    if (request.type === RequestType.ToDevice && request.event_type && request.txn_id) {
      return JSON.stringify(await api.e2eeToDevice(request.event_type, request.txn_id, body));
    }
    throw new Error(`unsupported_matrix_request_${request.type}`);
  }

  private async flushOutgoing() {
    for (const raw of await this.machine.outgoingRequests()) {
      const request = raw as unknown as MatrixRequest;
      if (![RequestType.KeysUpload, RequestType.KeysQuery, RequestType.KeysClaim, RequestType.ToDevice].includes(request.type)) continue;
      const response = await this.sendRequest(request);
      await this.machine.markRequestAsSent(request.id, request.type, response);
    }
  }

  async syncKeys() {
    const response = await api.e2eeSync(this.deviceID, this.cursor);
    this.cursor = response.next;
    await this.machine.receiveSyncChanges(
      JSON.stringify(response.events),
      new DeviceLists([], []),
      new Map(),
    );
    await this.flushOutgoing();
  }

  async encrypt(myID: number, contactID: number, contactUsername: string, content: Record<string, unknown>) {
    const contact = matrixUser(contactUsername);
    await this.machine.updateTrackedUsers([contact.clone()]);
    await this.flushOutgoing();
    const missing = await this.machine.getMissingSessions([contact.clone()]);
    if (missing) {
      const request = missing as unknown as MatrixRequest;
      await this.machine.markRequestAsSent(request.id, request.type, await this.sendRequest(request));
    }
    const room = roomFor(myID, contactID);
    this.rooms.add(room.toString());
    for (const raw of await this.machine.shareRoomKey(room, [contact], new EncryptionSettings())) {
      const request = raw as unknown as MatrixRequest;
      await this.machine.markRequestAsSent(request.id, request.type, await this.sendRequest(request));
    }
    const encrypted = JSON.parse(await this.machine.encryptRoomEvent(room, "m.room.message", JSON.stringify(content))) as Record<string, unknown>;
    void this.backupRoomKeys();
    return encrypted;
  }

  async decrypt(myID: number, contactID: number, senderUsername: string, messageID: string, timestamp: string, ciphertext: Record<string, unknown>) {
    await this.syncKeys();
    const event = JSON.stringify({
      type: "m.room.encrypted",
      event_id: `$${messageID}`,
      sender: `@${senderUsername}:private-direct`,
      origin_server_ts: Date.parse(timestamp),
      content: ciphertext,
    });
    const room = roomFor(myID, contactID); this.rooms.add(room.toString());
    const decrypted = await this.machine.decryptRoomEvent(event, room, new DecryptionSettings(TrustRequirement.Untrusted));
    const result = JSON.parse(decrypted.event) as { content: Record<string, unknown> };
    void this.backupRoomKeys();
    return result;
  }

  async backupRoomKeys() {
    const key = await loadMasterKey(this.username);
    if (!key) return;
    const exported = await this.machine.exportRoomKeys(() => true);
    const iv = crypto.getRandomValues(new Uint8Array(12));
    const ciphertext = new Uint8Array(await crypto.subtle.encrypt({ name: "AES-GCM", iv }, key, new TextEncoder().encode(exported)));
    await api.saveKeyBackup(JSON.stringify({ iv: encode64(iv), ciphertext: encode64(ciphertext) }));
  }

  async rotateFutureKeys() { for (const id of this.rooms) await this.machine.invalidateGroupSession(new RoomId(id)); }
  async signTombstone(messageID:string,scope:"self"|"both",createdAt:string){const signed=`${messageID}|${scope}|${createdAt}`;const signatures=await this.machine.sign(signed);const raw=JSON.parse(signatures.asJSON()) as Record<string,Record<string,string>>;const signature=raw[`@${this.username}:private-direct`]?.[`ed25519:${this.deviceID}`];if(!signature)throw new Error("tombstone_signature_unavailable");return{device_id:this.deviceID,created_at:createdAt,signature}}
}

export function rememberDevice(deviceID: string) { localStorage.setItem(DEVICE_KEY, deviceID); }
export function hasRememberedDevice() { return localStorage.getItem(DEVICE_KEY) !== null; }
export function rememberedDeviceID() { return localStorage.getItem(DEVICE_KEY); }
export async function clearLocalDevice(username:string) { const id=rememberedDeviceID();localStorage.removeItem(DEVICE_KEY);session=null;clearDraftStorage();await clearMasterKeys();if(id)indexedDB.deleteDatabase(`private-direct-${username}-${id}`); }
export async function matrixSession(username: string) { session ??= await MatrixSession.open(username); return session; }

export async function recoverDevice(username: string, phrase: string) {
  if (!validateMnemonic(phrase.trim(), wordlist)) throw new Error("invalid_recovery_phrase");
  const recovery = await api.e2eeRecovery();
  if (recovery.protocol_version !== 1) throw new Error("unsupported_protocol_version");
  const wrapped = JSON.parse(recovery.wrapped_master_key) as { iv: string; ciphertext: string };
  const seed = new Uint8Array(mnemonicToSeedSync(phrase.trim()));
  const material = await crypto.subtle.importKey("raw", seed, "HKDF", false, ["deriveKey"]);
  const wrappingKey = await crypto.subtle.deriveKey({ name: "HKDF", hash: "SHA-256", salt: decode64(recovery.kdf_salt), info: new TextEncoder().encode("private-direct-account-v1") }, material, { name: "AES-GCM", length: 256 }, false, ["decrypt"]);
  let rawMaster: ArrayBuffer;
  try { rawMaster = await crypto.subtle.decrypt({ name: "AES-GCM", iv: decode64(wrapped.iv) }, wrappingKey, decode64(wrapped.ciphertext)); }
  catch { throw new Error("invalid_recovery_phrase"); }
  seed.fill(0);
  const master = await crypto.subtle.importKey("raw", rawMaster, "AES-GCM", false, ["encrypt", "decrypt"]);
  new Uint8Array(rawMaster).fill(0);

  const deviceID = crypto.randomUUID();
  const machine = await OlmMachine.initialize(matrixUser(username), new DeviceId(deviceID), `private-direct-${username}-${deviceID}`);
  try {
    const upload = (await machine.outgoingRequests()).find(request => "body" in request);
    if (!upload || !("body" in upload) || typeof upload.body !== "string") throw new Error("matrix_device_keys_unavailable");
    await api.registerRecoveryDevice(deviceID, JSON.parse(upload.body) as Record<string, unknown>);
    if (recovery.key_backup) {
      const backup = JSON.parse(recovery.key_backup) as { iv: string; ciphertext: string };
      const clear = await crypto.subtle.decrypt({ name: "AES-GCM", iv: decode64(backup.iv) }, master, decode64(backup.ciphertext));
      await machine.importExportedRoomKeys(new TextDecoder().decode(clear), () => undefined);
    }
    await saveMasterKey(username, master);
    rememberDevice(deviceID);
    session = null;
  } finally { machine.close(); }
}
