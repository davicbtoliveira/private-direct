# ADR 0022: Chat Media Attachments

## Status

Proposed

## Context

Private Direct currently defines direct, online-only, peer-to-peer text
messages. Users also need to exchange image and video files without turning the
product into a calling or live-streaming application.

Media must preserve the existing privacy boundary: chat payloads do not pass
through, persist on, queue at, or fall back to the server. The browser-session
transcript remains ephemeral.

## Decision

Images and videos will be modeled as media attachments in direct chat. One
message may contain optional plain text plus up to 10 media attachments. The
text keeps the existing 4,000 Unicode character limit and describes the group,
not an individual attachment. Attachments in one message may total at most 100
MiB. Each image may be at most 20 MiB and each video at most 100 MiB. Received
images are viewable inline in the conversation, and received videos are
playable inline.

Accepted formats are JPEG, PNG, WebP, GIF, MP4, and WebM. The application does
not transcode media. If the receiving browser cannot decode a codec inside an
accepted container, the conversation shows a non-playable attachment card and
allows the user to save the original file.

The existing reliable, ordered `chat` DataChannel continues to carry JSON text,
attachment manifests, control envelopes, and acknowledgements. A separate
DataChannel named `media` carries chunked binary attachment content so large
transfers do not occupy the chat control stream.

Each attachment has its own transfer state. Successfully received attachments
remain available if another attachment in the same message fails, and retry
resends only failed attachments. The containing message remains `sending` until
every attachment has been received and validated; only then does its final
acknowledgement move the message to `delivered`.

The sender may cancel the whole message transfer. Cancellation stops the active
attachment and all queued attachments, marks the message `not_delivered`, and
does not retract attachments already received by the peer. A later retry sends
only incomplete attachments again from their beginning.

After validating a manifest against type, count, and the 100 MiB aggregate
limit, the receiving client automatically accepts and downloads the
attachments. Incoming progress is visible. Completed media is immediately
accessible from the conversation without another acceptance step.

The conversation renders one to three attachments directly. Four or more use a
four-tile grid. If a message has more than four attachments, the fourth tile
shows a `+N` overlay for the attachments not represented in the grid.

Clicking a media tile or the `+N` overlay opens an in-app gallery at the clicked
item. The user can navigate every attachment in the message. Images are shown
at an expanded size. Videos use manual playback with native controls and
fullscreen support; media never autoplays.

Transfer progress is calculated from aggregate bytes across the message and
shown as one percentage. Each attachment tile separately shows one discrete
state: `waiting`, `transferring`, `complete`, or `failed`.

Attachments within one message transfer sequentially in selection order. Text
and control traffic remain independent on `chat`. Different contacts retain
their independent peer connections and transfer queues.

Both sender and receiver validate the declared MIME type and binary file
signature against the accepted-format allowlist. Filename extensions are not
trusted. An invalid attachment fails independently and does not prevent valid
siblings from transferring.

Sender and receiver compute SHA-256 incrementally while chunking and assembling
each attachment. The receiver marks an attachment `complete` only when both its
expected byte size and digest match, avoiding a second whole-file buffer for
verification.

Binary frames are at most 16 KiB, including their application header, and never
exceed the negotiated `RTCSctpTransport.maxMessageSize`. The sender pauses when
`media.bufferedAmount` exceeds 1 MiB, sets `bufferedAmountLowThreshold` to 256
KiB, and resumes on `bufferedamountlow`.

One tab retains at most 256 MiB of completed and in-progress media. A manifest
that would exceed the remaining budget is rejected before binary transfer. The
UI explains the session limit and offers `Clear media`, which revokes object
URLs and releases attachment blobs while preserving text and message
placeholders.

Consistent with ADRs 0004 and 0014, a broken transfer becomes
`not_delivered` after peer reconnection handling. Media does not automatically
resume or resend. Explicit retry restarts only incomplete attachments from
their beginning.

The web application and peer protocol deploy together from the same instance.
The media MVP does not negotiate backward-compatible capabilities with older
clients.

Implementation includes the currently missing browser chat foundation required
by media: contacts, presence, peer-connection lifecycle, acknowledged text
messages, and then attachment transfer. Existing accepted ADRs remain the
source of truth for those prerequisite behaviors.

The attachment manifest exposes the sanitized original filename, declared MIME
type, and byte size to the receiving contact. Local filesystem paths are never
available to or sent by the browser. The composer shows filenames before send
so this disclosure is visible to the sender.

A media message begins on `chat` as the existing `message` envelope extended
with an `attachments` manifest. Message and attachment IDs are UUIDs. Each
manifest entry contains attachment ID, selection index, sanitized filename,
MIME type, and byte size. The receiver validates the manifest, reserves its
memory budget, and returns `media_ready` with a ready or rejected result for
each attachment.

For every ready attachment, `media` carries ordered binary frames with a fixed
header containing protocol version, 16-byte attachment UUID, and 32-bit chunk
index. Payload fills the remainder of the 16 KiB frame limit. After its final
chunk, the sender sends `attachment_complete` on `chat` with byte size and
SHA-256 digest. The receiver may observe this control envelope before the last
binary frame, so it finalizes only after both the envelope and all expected
bytes exist.

The receiver responds with an attachment acknowledgement or error. Once every
attachment is complete, it emits the existing final message acknowledgement.
Cancellation and explicit retry preserve message and attachment IDs, allowing
the receiver to acknowledge completed attachments again without displaying or
storing duplicates.

The composer accepts attachments through a paperclip button backed by a
multiple-file picker. Desktop users may also drag files onto the conversation.
Clipboard paste is out of scope. Before send, the composer shows previews,
filenames, and sizes, lets the user remove individual items, and preserves
selection order.

Media is not progressively decoded or played while transferring. A tile shows
progress until the complete file passes size and SHA-256 validation; only then
does inline viewing or playback become available. `MediaSource` streaming is
out of scope.

Live calls, live streaming, and realtime camera or microphone tracks are out of
scope.

## Consequences

- Media belongs to the chat-message domain, not the calling domain.
- Delivery and display must preserve each attachment's membership in its
  containing message.
- The frontend needs attachment selection, transfer state, inline rendering,
  and media-object lifecycle management.
- The peer protocol needs to coordinate control envelopes on `chat` with
  bounded binary transfer on `media`.
- The server remains unaware of attachment content.
- Reloading, closing the tab, or logging out still destroys the transcript and
  its media.
