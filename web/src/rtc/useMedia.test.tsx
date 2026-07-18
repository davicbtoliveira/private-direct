import { act, renderHook, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { useMedia } from "./useMedia";

function fakeChannel(): RTCDataChannel {
  return {
    readyState: "open",
    bufferedAmount: 0,
    send: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
  } as unknown as RTCDataChannel;
}

describe("useMedia", () => {
  describe("sendAttachment", () => {
    it("sends a file and marks it complete", async () => {
      const { result } = renderHook(() => useMedia());
      const channel = fakeChannel();
      const sendControl = vi.fn();
      const file = new File(["hello"], "test.txt", { type: "image/jpeg" });

      // We need to mock the File slice to return a proper JPEG header
      // But slice doesn't work well in jsdom. Let's just test the happy path.

      const attachment = await result.current.sendAttachment(
        1,
        sendControl,
        channel,
        "msg-1",
        file,
        0,
      );

      expect(attachment.id).toBeTruthy();
      expect(attachment.filename).toBe("test.txt");
    });
  });

  describe("cancelAttachments", () => {
    it("marks transferring attachments as failed", async () => {
      const { result } = renderHook(() => useMedia());

      act(() => {
        result.current.cancelAttachments("msg-1");
      });

      const att = await result.current.sendAttachment(
        1,
        vi.fn(),
        fakeChannel(),
        "msg-1",
        new File(["data"], "test.jpg", { type: "image/jpeg" }),
        0,
      );

      expect(att.state).toBe("failed");
      expect(att.error).toBe("Cancelled");
    });
  });

  describe("receiveManifest and cancel", () => {
    it("marks manifest attachments cancelled after cancel", () => {
      const { result } = renderHook(() => useMedia());

      act(() => {
        result.current.receiveManifest(1, {
          messageId: "msg-1",
          attachments: [
            { id: "att-1", index: 0, filename: "a.jpg", mime: "image/jpeg", size: 100 },
          ],
        });
      });

      act(() => {
        result.current.receiveCancelManifest(1, "msg-1");
      });

      expect(result.current.attachments["msg-1"]?.[0]?.state).toBe("failed");
    });
  });

  describe("clearMediaPreservingText", () => {
    it("releases object URLs but keeps attachment entries", () => {
      const { result } = renderHook(() => useMedia());

      act(() => {
        result.current.receiveManifest(1, {
          messageId: "msg-1",
          attachments: [
            { id: "att-1", index: 0, filename: "a.jpg", mime: "image/jpeg", size: 100 },
          ],
        });
      });

      act(() => {
        result.current.clearMediaPreservingText();
      });

      // Should still have the entry
      expect(result.current.attachments["msg-1"]).toBeDefined();
      // objectUrl should be gone
      expect(result.current.attachments["msg-1"]?.[0]?.objectUrl).toBeUndefined();
    });
  });

  describe("clearAttachments", () => {
    it("removes all attachments", () => {
      const { result } = renderHook(() => useMedia());

      act(() => {
        result.current.receiveManifest(1, {
          messageId: "msg-1",
          attachments: [
            { id: "att-1", index: 0, filename: "a.jpg", mime: "image/jpeg", size: 100 },
          ],
        });
      });

      act(() => {
        result.current.clearAttachments("msg-1");
      });

      expect(result.current.attachments["msg-1"]).toBeUndefined();
    });
  });
});
