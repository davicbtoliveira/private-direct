import { describe, expect, it } from "vitest";
import {
  binFrame,
  readBinFrame,
  sha256,
  validateMime,
  sanitizeFilename,
  FRAME_SIZE,
  MAX_MEMORY_BUDGET,
} from "./mediaTransfer";

describe("binFrame / readBinFrame", () => {
  it("round-trips a frame", () => {
    const id = "550e8400-e29b-41d4-a716-446655440000";
    const payload = new Uint8Array([1, 2, 3, 4]).buffer;
    const frame = binFrame(id, 0, payload);
    const result = readBinFrame(frame);
    expect(result).not.toBeNull();
    expect(result!.version).toBe(1);
    expect(result!.attachmentId).toBe(id);
    expect(result!.chunkIndex).toBe(0);
    expect(new Uint8Array(result!.payload)).toEqual(new Uint8Array([1, 2, 3, 4]));
  });

  it("rejects frames smaller than header", () => {
    const buf = new ArrayBuffer(20);
    expect(readBinFrame(buf)).toBeNull();
  });

  it("rejects unknown version", () => {
    const id = "550e8400-e29b-41d4-a716-446655440000";
    const frame = binFrame(id, 0, new ArrayBuffer(4));
    const view = new Uint8Array(frame);
    view[0] = 99;
    expect(readBinFrame(frame)).toBeNull();
  });

  it("rejects invalid attachment id", () => {
    expect(() => binFrame("not-a-uuid", 0, new ArrayBuffer(4))).toThrow("invalid attachment id");
  });

  it("handles large payload at FRAME_SIZE boundary", () => {
    const id = "550e8400-e29b-41d4-a716-446655440000";
    const big = new Uint8Array(FRAME_SIZE);
    for (let i = 0; i < big.length; i++) big[i] = i & 0xFF;
    const frame = binFrame(id, 0, big.buffer);
    const result = readBinFrame(frame);
    expect(result).not.toBeNull();
    expect(result!.payload.byteLength).toBe(FRAME_SIZE);
  });
});

describe("sha256", () => {
  it("computes hex digest", async () => {
    const data = new TextEncoder().encode("hello").buffer;
    const digest = await sha256(data);
    expect(digest).toBe("2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824");
  });
});

describe("validateMime", () => {
  it("accepts a valid JPEG", async () => {
    const file = new File([new Uint8Array([0xFF, 0xD8, 0xFF, 0xE0, 0, 0x10, 0x4A, 0x46, 0x49, 0x46, 0, 1])], "test.jpg", { type: "image/jpeg" });
    await expect(validateMime(file)).resolves.toBe(true);
  });

  it("rejects a wrong signature", async () => {
    const file = new File([new Uint8Array([0x00, 0x00, 0x00, 0x00])], "test.jpg", { type: "image/jpeg" });
    await expect(validateMime(file)).resolves.toBe(false);
  });

  it("accepts a valid PNG", async () => {
    const file = new File([new Uint8Array([0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A])], "test.png", { type: "image/png" });
    await expect(validateMime(file)).resolves.toBe(true);
  });

  it("rejects unsupported MIME type", async () => {
    const file = new File([], "test.pdf", { type: "application/pdf" });
    await expect(validateMime(file)).resolves.toBe(false);
  });
});

describe("sanitizeFilename", () => {
  it("replaces unsafe characters", () => {
    expect(sanitizeFilename("hello/world:test.txt")).toBe("hello_world_test.txt");
  });

  it("truncates long names", () => {
    const long = "a".repeat(300);
    expect(sanitizeFilename(long).length).toBe(255);
  });

  it("preserves safe names", () => {
    expect(sanitizeFilename("photo_2024-01-01.jpg")).toBe("photo_2024-01-01.jpg");
  });
});

describe("MAX_MEMORY_BUDGET", () => {
  it("is 256 MiB", () => {
    expect(MAX_MEMORY_BUDGET).toBe(256 * 1024 * 1024);
  });
});
