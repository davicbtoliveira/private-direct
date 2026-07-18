const FRAME_SIZE = 16 * 1024;
const BUF_HIGH = 1_048_576;
const BUF_LOW = 262_144;
const MAX_MEMORY_BUDGET = 256 * 1024 * 1024;

export type AttachmentState = "pending" | "ready" | "transferring" | "complete" | "failed";

export type AttachmentInfo = {
  id: string;
  index: number;
  filename: string;
  mime: string;
  size: number;
  blob?: Blob;
  objectUrl?: string;
  state: AttachmentState;
  error?: string;
  progress: number;
};

export type MediaManifest = {
  messageId: string;
  attachments: Array<{
    id: string;
    index: number;
    filename: string;
    mime: string;
    size: number;
  }>;
};

function chunkId(id: string): Uint8Array {
  const compact = id.replaceAll("-", "");
  if (!/^[0-9a-f]{32}$/i.test(compact)) throw new Error("invalid attachment id");
  return Uint8Array.from(compact.match(/.{2}/g)!, (byte) => Number.parseInt(byte, 16));
}

function frameId(bytes: Uint8Array): string {
  const hex = Array.from(bytes, (byte) => byte.toString(16).padStart(2, "0")).join("");
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
}

function binFrame(attachmentId: string, chunkIndex: number, data: ArrayBuffer): ArrayBuffer {
  const version = new Uint8Array([1]);
  const aId = chunkId(attachmentId);
  const headerLen = 1 + 16 + 4;
  const buf = new ArrayBuffer(headerLen + data.byteLength);
  const view = new Uint8Array(buf);
  view.set(version, 0);
  view.set(aId, 1);
  new DataView(buf).setUint32(17, chunkIndex, false);
  view.set(new Uint8Array(data), headerLen);
  return buf;
}

async function sha256(data: ArrayBuffer): Promise<string> {
  const hash = await crypto.subtle.digest("SHA-256", data);
  return Array.from(new Uint8Array(hash))
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("");
}

async function sha256Incremental(chunks: ArrayBuffer[]): Promise<string> {
  if (chunks.length === 1) return sha256(chunks[0]);
  const totalLen = chunks.reduce((s, c) => s + c.byteLength, 0);
  const combined = new Uint8Array(totalLen);
  let offset = 0;
  for (const c of chunks) {
    combined.set(new Uint8Array(c), offset);
    offset += c.byteLength;
  }
  return sha256(combined.buffer);
}

function readBinFrame(data: ArrayBuffer): { version: number; attachmentId: string; chunkIndex: number; payload: ArrayBuffer } | null {
  if (data.byteLength < 21) return null;
  const view = new Uint8Array(data);
  const version = view[0];
  if (version !== 1) return null;
  const aId = frameId(view.slice(1, 17));
  const idx = new DataView(data).getUint32(17, false);
  const payload = data.slice(21);
  return { version, attachmentId: aId, chunkIndex: idx, payload };
}

function imageUrl(blob: Blob): string {
  return URL.createObjectURL(blob);
}

function revokeUrl(url: string) {
  URL.revokeObjectURL(url);
}

async function validateMime(file: File): Promise<boolean> {
  const validMimes = ["image/jpeg", "image/png", "image/webp", "image/gif", "video/mp4", "video/webm"];
  if (!validMimes.includes(file.type)) return false;

  if (file.type.startsWith("image/")) {
    const header = await file.slice(0, 12).arrayBuffer();
    const bytes = new Uint8Array(header);
    if (file.type === "image/jpeg") {
      return bytes[0] === 0xFF && bytes[1] === 0xD8 && bytes[2] === 0xFF;
    }
    if (file.type === "image/png") {
      return bytes[0] === 0x89 && bytes[1] === 0x50 && bytes[2] === 0x4E && bytes[3] === 0x47;
    }
    if (file.type === "image/webp") {
      return bytes[0] === 0x52 && bytes[1] === 0x49 && bytes[2] === 0x46 && bytes[3] === 0x46;
    }
    if (file.type === "image/gif") {
      return bytes[0] === 0x47 && bytes[1] === 0x49 && bytes[2] === 0x46;
    }
    return true;
  }
  return true;
}

function sanitizeFilename(name: string): string {
  return name.replace(/[^a-zA-Z0-9._-]/g, "_").slice(0, 255);
}

export {
  FRAME_SIZE,
  BUF_HIGH,
  BUF_LOW,
  MAX_MEMORY_BUDGET,
  binFrame,
  readBinFrame,
  sha256,
  sha256Incremental,
  imageUrl,
  revokeUrl,
  validateMime,
  sanitizeFilename,
};
