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

class MatrixSession {
  private cursor = "0";
  private constructor(private machine: OlmMachine, private deviceID: string) {}

  static async open(username: string) {
    const deviceID = localStorage.getItem(DEVICE_KEY);
    if (!deviceID) throw new Error("e2ee_device_missing");
    const machine = await OlmMachine.initialize(matrixUser(username), new DeviceId(deviceID), `private-direct-${username}-${deviceID}`);
    const value = new MatrixSession(machine, deviceID);
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
    for (const raw of await this.machine.shareRoomKey(room, [contact], new EncryptionSettings())) {
      const request = raw as unknown as MatrixRequest;
      await this.machine.markRequestAsSent(request.id, request.type, await this.sendRequest(request));
    }
    return JSON.parse(await this.machine.encryptRoomEvent(room, "m.room.message", JSON.stringify(content))) as Record<string, unknown>;
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
    const decrypted = await this.machine.decryptRoomEvent(event, roomFor(myID, contactID), new DecryptionSettings(TrustRequirement.Untrusted));
    return JSON.parse(decrypted.event) as { content: Record<string, unknown> };
  }
}

export function rememberDevice(deviceID: string) { localStorage.setItem(DEVICE_KEY, deviceID); }
export async function matrixSession(username: string) { session ??= await MatrixSession.open(username); return session; }
