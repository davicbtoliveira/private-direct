import {
  DeviceId,
  OlmMachine,
  UserId,
} from "@matrix-org/matrix-sdk-crypto-wasm";
import { generateMnemonic, mnemonicToSeedSync } from "@scure/bip39";
import { wordlist } from "@scure/bip39/wordlists/english";
import { saveMasterKey } from "./keyStore";

export type E2EESetupPayload = {
  device_id: string;
  identity_keys: Record<string, unknown>;
  device_keys: Record<string, unknown>;
  wrapped_master_key: string;
  kdf_salt: string;
  protocol_version: 1;
};

function base64(bytes: Uint8Array): string {
  let binary = "";
  bytes.forEach((byte) => (binary += String.fromCharCode(byte)));
  return btoa(binary);
}

export function createRecoveryPhrase(): string {
  return generateMnemonic(wordlist, 256);
}

export async function createE2EESetup(
  username: string,
  phrase: string,
): Promise<E2EESetupPayload> {
  const deviceID = crypto.randomUUID();
  const userID = new UserId(`@${username}:private-direct`);
  const machine = await OlmMachine.initialize(
    userID,
    new DeviceId(deviceID),
    `private-direct-${username}-${deviceID}`,
  );
  try {
    const outgoing = await machine.outgoingRequests();
    const upload = outgoing.find((request) => "body" in request);
    if (!upload || !("body" in upload) || typeof upload.body !== "string") {
      throw new Error("matrix_device_keys_unavailable");
    }
    const deviceKeys = JSON.parse(upload.body) as Record<string, unknown>;
    const crossSigning = await machine.bootstrapCrossSigning(true);
    const identityKeys = JSON.parse(
      crossSigning.uploadSigningKeysRequest.body,
    ) as Record<string, unknown>;

    const salt = crypto.getRandomValues(new Uint8Array(32));
    const iv = crypto.getRandomValues(new Uint8Array(12));
    const masterKey = crypto.getRandomValues(new Uint8Array(32));
    const seed = new Uint8Array(mnemonicToSeedSync(phrase));
    const material = await crypto.subtle.importKey("raw", seed.buffer, "HKDF", false, [
      "deriveKey",
    ]);
    const wrappingKey = await crypto.subtle.deriveKey(
      { name: "HKDF", hash: "SHA-256", salt, info: new TextEncoder().encode("private-direct-account-v1") },
      material,
      { name: "AES-GCM", length: 256 },
      false,
      ["encrypt"],
    );
    const ciphertext = new Uint8Array(
      await crypto.subtle.encrypt({ name: "AES-GCM", iv }, wrappingKey, masterKey),
    );
    const storedMaster = await crypto.subtle.importKey("raw", masterKey, "AES-GCM", false, ["encrypt", "decrypt"]);
    await saveMasterKey(username, storedMaster);
    masterKey.fill(0);
    seed.fill(0);

    return {
      device_id: deviceID,
      identity_keys: identityKeys,
      device_keys: deviceKeys,
      wrapped_master_key: JSON.stringify({ iv: base64(iv), ciphertext: base64(ciphertext) }),
      kdf_salt: base64(salt),
      protocol_version: 1,
    };
  } finally {
    machine.close();
  }
}
