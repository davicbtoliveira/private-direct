import { useCallback, useRef, useState } from "react";
import {
  type AttachmentInfo,
  type MediaManifest,
  FRAME_SIZE,
  BUF_HIGH,
  BUF_LOW,
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

      const key = `${contactId}:${attachmentId}`;
      const entry = receivingChunks.current[key];
      if (!entry) return false;

      const expectedSize = size;
      const totalBytes = entry.chunks.reduce((s, c) => s + c.byteLength, 0);

      if (totalBytes !== expectedSize) return false;

      sha256Incremental(entry.chunks).then((computed) => {
        if (computed !== digest) return;

        const blob = new Blob(entry.chunks, { type: entry.info.mime });
        const url = imageUrl(blob);

        const info: AttachmentInfo = {
          id: attachmentId,
          index: entry.info.index,
          filename: entry.info.filename,
          mime: entry.info.mime,
          size: expectedSize,
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
      for (const att of manifest.attachments) {
        const key = `${contactId}:${att.id}`;
        const info: AttachmentInfo = { ...att, state: "transferring", progress: 0 };
        receivingChunks.current[key] = { chunks: [], info };
      }
      setAttachments((prev) => ({ ...prev, [manifest.messageId]: manifest.attachments.map((att) => ({ ...att, state: "transferring", progress: 0 })) }));
    },
    [],
  );

  const clearAttachments = useCallback((messageId?: string) => {
    if (messageId) {
      setAttachments((prev) => {
        const list = prev[messageId] ?? [];
        for (const a of list) {
          if (a.objectUrl) revokeUrl(a.objectUrl);
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
          }
        }
        return {};
      });
    }
  }, []);

  const initReceive = useCallback((contactId: number, attachmentId: string, size: number) => {
    const key = `${contactId}:${attachmentId}`;
    receivingChunks.current[key] = { chunks: [], info: { id: attachmentId, index: 0, filename: "", mime: "", size, state: "transferring", progress: 0 } };
  }, []);

  return {
    attachments,
    sendAttachment,
    receiveManifest,
    receiveAttachmentComplete,
    receiveFrame,
    clearAttachments,
    initReceive,
  };
}
