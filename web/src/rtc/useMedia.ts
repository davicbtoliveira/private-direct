import { useCallback, useRef, useState } from "react";
import {
  type AttachmentInfo,
  type AttachmentState,
  type MediaManifest,
  FRAME_SIZE,
  BUF_HIGH,
  BUF_LOW,
  MAX_MEMORY_BUDGET,
  binFrame,
  readBinFrame,
  sha256Incremental,
  imageUrl,
  revokeUrl,
  validateMime,
  sanitizeFilename,
} from "./mediaTransfer";

export function useMedia() {
  const [attachments, setAttachments] = useState<Record<string, AttachmentInfo[]>>({});
  const receivingChunks = useRef<Record<string, { chunks: ArrayBuffer[]; info: AttachmentInfo }>>({});
  const budgetRef = useRef(0);
  const cancelledRef = useRef<Set<string>>(new Set());
  const pendingHashesRef = useRef<Set<string>>(new Set());

  const isCancelled = useCallback((messageId: string) => {
    return cancelledRef.current.has(messageId);
  }, []);

  const reserveBudget = useCallback((size: number): boolean => {
    if (budgetRef.current + size > MAX_MEMORY_BUDGET) return false;
    budgetRef.current += size;
    return true;
  }, []);

  const releaseBudget = useCallback((size: number) => {
    budgetRef.current = Math.max(0, budgetRef.current - size);
  }, []);

  const getAggregateProgress = useCallback((messageId: string): number => {
    const list = attachments[messageId];
    if (!list || list.length === 0) return 0;
    const totalBytes = list.reduce((s, a) => s + a.size, 0);
    if (totalBytes === 0) return 0;
    const doneBytes = list.reduce((s, a) => s + Math.round(a.size * a.progress / 100), 0);
    return Math.round(doneBytes / totalBytes * 100);
  }, [attachments]);

  const cancelAttachments = useCallback((messageId: string) => {
    cancelledRef.current.add(messageId);
    setAttachments((prev) => {
      const list = prev[messageId];
      if (!list) return prev;
      const next = { ...prev };
      next[messageId] = list.map((a) => {
        if (a.state === "transferring" || a.state === "pending" || a.state === "ready") {
          return { ...a, state: "failed" as AttachmentState, error: "Cancelled" };
        }
        return a;
      });
      return next;
    });
  }, []);

  const clearCancelled = useCallback((messageId: string) => {
    cancelledRef.current.delete(messageId);
  }, []);

  const receiveFrame = useCallback((contactId: number, data: ArrayBuffer) => {
    const frame = readBinFrame(data);
    if (!frame) return;
    const entry = receivingChunks.current[`${contactId}:${frame.attachmentId}`];
    if (entry) entry.chunks.push(frame.payload);
  }, []);

  const sendAttachment = useCallback(
    async (
      _contactId: number,
      sendControl: (data: string) => boolean,
      mediaChannel: RTCDataChannel,
      messageId: string,
      file: File,
      index: number,
      attachmentId?: string,
    ): Promise<AttachmentInfo> => {
      const id = attachmentId ?? crypto.randomUUID();
      const info: AttachmentInfo = {
        id,
        index,
        filename: sanitizeFilename(file.name),
        mime: file.type,
        size: file.size,
        state: "pending",
        progress: 0,
      };

      if (cancelledRef.current.has(messageId)) {
        info.state = "failed";
        info.error = "Cancelled";
        return info;
      }

      const valid = await validateMime(file);
      if (!valid) {
        info.state = "failed";
        info.error = "Invalid file type";
        return info;
      }

      info.blob = file;
      info.objectUrl = imageUrl(file);
      info.state = "ready";
      setAttachments((prev) => ({
        ...prev,
        [messageId]: [...(prev[messageId] ?? []), info],
      }));

      let chunkIndex = 0;
      let offset = 0;
      const chunks: ArrayBuffer[] = [];

      while (offset < file.size) {
        if (cancelledRef.current.has(messageId)) {
          info.state = "failed";
          info.error = "Cancelled";
          if (info.objectUrl) revokeUrl(info.objectUrl);
          delete info.blob;
          delete info.objectUrl;
          return info;
        }

        const end = Math.min(offset + FRAME_SIZE, file.size);
        const blobSlice = file.slice(offset, end);
        const chunk = await blobSlice.arrayBuffer();
        chunks.push(chunk);

        const frame = binFrame(id, chunkIndex, chunk);

        while (mediaChannel.bufferedAmount > BUF_HIGH) {
          await new Promise<void>((resolve) => {
            const check = () => {
              if (mediaChannel.bufferedAmount <= BUF_LOW) {
                resolve();
              } else {
                requestAnimationFrame(check);
              }
            };
            check();
          });
        }

        mediaChannel.send(frame);
        chunkIndex++;
        offset = end;

        info.progress = Math.round((offset / file.size) * 100);
        info.state = "transferring";
      }

      if (cancelledRef.current.has(messageId)) {
        info.state = "failed";
        info.error = "Cancelled";
        if (info.objectUrl) revokeUrl(info.objectUrl);
        delete info.blob;
        delete info.objectUrl;
        return info;
      }

      const digest = await sha256Incremental(chunks);
      const completeMsg = JSON.stringify({
        type: "attachment_complete",
        message_id: messageId,
        attachment_id: id,
        size: file.size,
        digest,
      });

      sendControl(completeMsg);

      info.state = "complete";
      info.progress = 100;
      setAttachments((prev) => ({ ...prev, [messageId]: [...(prev[messageId] ?? [])] }));
      return info;
    },
    [],
  );

  const receiveAttachmentComplete = useCallback(
    (contactId: number, data: Record<string, unknown>) => {
      const messageId = data.message_id as string;
      const attachmentId = data.attachment_id as string;
      const size = data.size as number;
      const digest = data.digest as string;

      if (cancelledRef.current.has(messageId)) return false;

      const key = `${contactId}:${attachmentId}`;
      const entry = receivingChunks.current[key];
      if (!entry) return false;

      const totalBytes = entry.chunks.reduce((s, c) => s + c.byteLength, 0);
      if (totalBytes !== size) return false;

      pendingHashesRef.current.add(key);
      sha256Incremental(entry.chunks).then((computed) => {
        pendingHashesRef.current.delete(key);
        if (computed !== digest) return;
        if (cancelledRef.current.has(messageId)) return;

        const blob = new Blob(entry.chunks, { type: entry.info.mime });
        const url = imageUrl(blob);

        const info: AttachmentInfo = {
          id: attachmentId,
          index: entry.info.index,
          filename: entry.info.filename,
          mime: entry.info.mime,
          size,
          blob,
          objectUrl: url,
          state: "complete",
          progress: 100,
        };

        setAttachments((prev) => ({
          ...prev,
          [messageId]: [...(prev[messageId] ?? []), info],
        }));

        delete receivingChunks.current[key];
      });

      return true;
    },
    [],
  );

  const receiveManifest = useCallback(
    (contactId: number, manifest: MediaManifest) => {
      if (cancelledRef.current.has(manifest.messageId)) return;

      const totalSize = manifest.attachments.reduce((s, a) => s + a.size, 0);
      if (!reserveBudget(totalSize)) {
        sendCancelControl(contactId, manifest.messageId, "Session memory full. Clear media and retry.");
        return;
      }

      for (const att of manifest.attachments) {
        const key = `${contactId}:${att.id}`;
        const info: AttachmentInfo = { ...att, state: "transferring", progress: 0 };
        receivingChunks.current[key] = { chunks: [], info };
      }
      setAttachments((prev) => ({
        ...prev,
        [manifest.messageId]: manifest.attachments.map((att) => ({ ...att, state: "transferring", progress: 0 })),
      }));
    },
    [reserveBudget],
  );

  function sendCancelControl(contactId: number, messageId: string, reason: string) {
    // Exposed for the caller to send via the control channel
    // The caller checks this via getPendingCancel
    pendingCancelsRef.current.push({ contactId, messageId, reason });
  }

  const pendingCancelsRef = useRef<Array<{ contactId: number; messageId: string; reason: string }>>([]);

  const flushCancelControls = useCallback((sendControl: (contactId: number, data: string) => boolean) => {
    const pending = pendingCancelsRef.current.splice(0);
    for (const p of pending) {
      sendControl(p.contactId, JSON.stringify({ type: "cancel_manifest", message_id: p.messageId, reason: p.reason }));
    }
  }, []);

  const receiveCancelManifest = useCallback((_contactId: number, messageId: string) => {
    cancelAttachments(messageId);
  }, [cancelAttachments]);

  const clearAttachments = useCallback((messageId?: string) => {
    if (messageId) {
      setAttachments((prev) => {
        const list = prev[messageId] ?? [];
        for (const a of list) {
          if (a.objectUrl) revokeUrl(a.objectUrl);
          releaseBudget(a.size);
        }
        const next = { ...prev };
        delete next[messageId];
        return next;
      });
    } else {
      setAttachments((prev) => {
        for (const list of Object.values(prev)) {
          for (const a of list) {
            if (a.objectUrl) revokeUrl(a.objectUrl);
            releaseBudget(a.size);
          }
        }
        return {};
      });
      budgetRef.current = 0;
      cancelledRef.current.clear();
    }
  }, [releaseBudget]);

  const clearMediaPreservingText = useCallback(() => {
    setAttachments((prev) => {
      const next: Record<string, AttachmentInfo[]> = {};
      for (const [messageId, list] of Object.entries(prev)) {
        next[messageId] = list.map((a) => {
          if (a.objectUrl) revokeUrl(a.objectUrl);
          releaseBudget(a.size);
          return { ...a, blob: undefined, objectUrl: undefined, state: "complete" as AttachmentState };
        });
      }
      return next;
    });
  }, [releaseBudget]);

  const initReceive = useCallback((contactId: number, attachmentId: string, size: number) => {
    const key = `${contactId}:${attachmentId}`;
    receivingChunks.current[key] = {
      chunks: [],
      info: { id: attachmentId, index: 0, filename: "", mime: "", size, state: "transferring", progress: 0 },
    };
  }, []);

  const retryAttachment = useCallback(
    async (
      contactId: number,
      sendControl: (data: string) => boolean,
      mediaChannel: RTCDataChannel,
      messageId: string,
      file: File,
      index: number,
      attachmentId: string,
    ): Promise<AttachmentInfo> => {
      clearCancelled(messageId);
      return sendAttachment(contactId, sendControl, mediaChannel, messageId, file, index, attachmentId);
    },
    [clearCancelled, sendAttachment],
  );

  const cleanupAll = useCallback(() => {
    for (const key of Object.keys(receivingChunks.current)) {
      delete receivingChunks.current[key];
    }
    for (const key of pendingHashesRef.current) {
      pendingHashesRef.current.delete(key);
    }
    cancelledRef.current.clear();
    budgetRef.current = 0;
    setAttachments({});
  }, []);

  return {
    attachments,
    sendAttachment,
    receiveManifest,
    receiveAttachmentComplete,
    receiveFrame,
    clearAttachments,
    clearMediaPreservingText,
    initReceive,
    cancelAttachments,
    clearCancelled,
    retryAttachment,
    receiveCancelManifest,
    flushCancelControls,
    isCancelled,
    cleanupAll,
    getAggregateProgress,
    budgetRef,
  };
}
