import { useCallback, useEffect, useState } from "react";
import type { AttachmentInfo } from "../rtc/mediaTransfer";
import styles from "./Gallery.module.css";

type GalleryProps = {
  attachments: AttachmentInfo[];
  startIndex: number;
  onClose: () => void;
};

export default function Gallery({ attachments, startIndex, onClose }: GalleryProps) {
  const [index, setIndex] = useState(startIndex);

  const current = attachments[index];
  const hasPrev = index > 0;
  const hasNext = index < attachments.length - 1;
  const [canPlayVideo, setCanPlayVideo] = useState<boolean | null>(null);

  useEffect(() => {
    if (current?.mime.startsWith("video/") && current.objectUrl) {
      const video = document.createElement("video");
      const playable = video.canPlayType(current.mime);
      setCanPlayVideo(playable !== "");
    } else {
      setCanPlayVideo(null);
    }
  }, [current]);

  const goPrev = useCallback(() => { if (hasPrev) setIndex((i) => i - 1); }, [hasPrev]);
  const goNext = useCallback(() => { if (hasNext) setIndex((i) => i + 1); }, [hasNext]);

  useEffect(() => {
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
      if (e.key === "ArrowLeft") goPrev();
      if (e.key === "ArrowRight") goNext();
    };
    window.addEventListener("keydown", handleKey);
    return () => window.removeEventListener("keydown", handleKey);
  }, [onClose, goPrev, goNext]);

  if (!current) return null;

  const isVideo = current.mime.startsWith("video/");
  const canPlay = isVideo && current.objectUrl && canPlayVideo !== false;
  const unsupportedVideo = isVideo && (!current.objectUrl || canPlayVideo === false);

  return (
    <div
      className={styles.backdrop}
      onClick={onClose}
      role="dialog"
      aria-label="Media gallery"
      aria-modal="true"
    >
      <div className={styles.toolbar}>
        <button
          className={styles.navBtn}
          onClick={goPrev}
          disabled={!hasPrev}
          aria-label="Previous"
        >
          &#8249;
        </button>
        <span className={styles.counter}>
          {index + 1} / {attachments.length}
        </span>
        <button
          className={styles.navBtn}
          onClick={goNext}
          disabled={!hasNext}
          aria-label="Next"
        >
          &#8250;
        </button>
        <button className={styles.closeBtn} onClick={onClose} aria-label="Close gallery">
          &times;
        </button>
      </div>

      <div
        className={styles.content}
        onClick={(e) => e.stopPropagation()}
      >
        {canPlay && (
          <video
            src={current.objectUrl}
            controls
            autoPlay={false}
            className={styles.media}
          >
            <p>Your browser does not support video playback.</p>
          </video>
        )}
        {!isVideo && current.objectUrl && (
          <img
            src={current.objectUrl}
            alt={current.filename}
            className={styles.media}
          />
        )}
        {unsupportedVideo && (
          <div className={styles.unsupported}>
            <p>This video codec is not supported.</p>
            <p className={styles.filename}>{current.filename}</p>
            {current.blob && (
              <a
                href={current.objectUrl ?? URL.createObjectURL(current.blob)}
                download={current.filename}
                className={styles.downloadLink}
              >
                Save original file
              </a>
            )}
          </div>
        )}
        {!current.objectUrl && !isVideo && (
          <div className={styles.unsupported}>
            <p>Media unavailable.</p>
            <p className={styles.filename}>{current.filename}</p>
          </div>
        )}
      </div>

      <div className={styles.info}>
        <span className={styles.filename}>{current.filename}</span>
      </div>

      {hasPrev && (
        <button
          className={`${styles.sideBtn} ${styles.prevBtn}`}
          onClick={(e) => { e.stopPropagation(); goPrev(); }}
          aria-label="Previous attachment"
        />
      )}
      {hasNext && (
        <button
          className={`${styles.sideBtn} ${styles.nextBtn}`}
          onClick={(e) => { e.stopPropagation(); goNext(); }}
          aria-label="Next attachment"
        />
      )}
    </div>
  );
}
