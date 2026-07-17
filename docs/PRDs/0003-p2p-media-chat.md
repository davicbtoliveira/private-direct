# PRD 0003: P2P Chat with Image and Video Attachments

Issue label: `ready-for-agent`

## Problem Statement

Private Direct users can authenticate against the backend, but the current web
workspace does not yet complete the accepted-contact, presence, WebRTC, or chat
workflows described by the existing MVP decisions. Users therefore cannot
exchange acknowledged text through the product UI, and they cannot send images
or videos directly to a contact.

Users need a complete online-only conversation flow that preserves Private
Direct's defining privacy boundary. Text and media must travel peer-to-peer,
never through or into server storage. Image and video transfer must remain
usable under browser memory, WebRTC message-size, bandwidth, malformed-input,
and partial-failure constraints. Received media must be immediately accessible
from the ephemeral chat transcript without turning the product into a calling,
streaming, cloud-storage, or offline-messaging service.

## Solution

Complete the browser chat vertical slice defined by the accepted frontend ADRs:
accepted contacts, streamed presence, one peer connection per active contact,
acknowledged text messages, ephemeral per-contact transcripts, and truthful
connection and delivery states. Extend that peer protocol with image and video
attachments.

A message may contain optional plain text and up to 10 attachments. The user
adds files through a multiple-file picker or desktop drag-and-drop, reviews
previews, and sends the group to an online contact. The receiving browser
validates and automatically downloads at most 100 MiB per message over a
dedicated reliable WebRTC media data channel. The existing chat data channel
continues to carry text, manifests, control messages, and acknowledgements.

The conversation renders completed images and videos directly. Four or more
attachments use a four-tile grid, with a `+N` overlay on the fourth tile when
more items exist. Clicking a tile opens an in-app gallery with expanded images,
manual video playback, navigation, native controls, and fullscreen support.
Transfer, validation, cancellation, partial failure, retry, memory cleanup, and
unsupported-codec states remain explicit and honest.

## User Stories

1. As an authenticated user, I want my accepted contacts loaded into the workspace, so that I can choose whom to message.
2. As an authenticated user, I want incoming contact changes reflected without refreshing, so that the workspace stays current.
3. As an authenticated user, I want an initial presence snapshot, so that I know which contacts can receive online-only messages.
4. As an authenticated user, I want presence changes streamed live, so that offline contacts are not presented as reachable.
5. As an authenticated user, I want online presence distinguished from peer-channel readiness, so that connectivity is represented truthfully.
6. As an online user, I want incoming peer offers handled while another conversation is open, so that I remain reachable.
7. As a contact, I want either peer to initiate negotiation, so that messaging does not depend on a fixed caller.
8. As a contact, I want simultaneous offers resolved deterministically, so that negotiation glare does not break chat.
9. As a contact, I want stale signaling ignored, so that an old connection attempt cannot corrupt a current conversation.
10. As a contact, I want one peer connection per contact that has initiated messaging, so that switching conversations does not repeatedly renegotiate.
11. As a sender, I want text to travel through the direct peer channel, so that the server never observes chat content.
12. As a sender, I want text marked `sending` until the receiving application accepts it, so that queued bytes are not called delivered.
13. As a sender, I want text marked `delivered` only after its acknowledgement, so that delivery has a precise meaning.
14. As a sender, I want failed text marked `not_delivered`, so that transport failure is visible.
15. As a sender, I want explicit retry for failed text, so that the app does not create a hidden offline queue.
16. As a recipient, I want retried message IDs deduplicated, so that retry never displays content twice.
17. As a user, I want separate in-memory transcripts per contact, so that switching conversations preserves current-tab context.
18. As a user, I want reload, tab close, and logout to clear transcripts, so that ephemeral behavior is not mistaken for history.
19. As a user, I want inactive-conversation messages counted as unread locally, so that I notice new activity.
20. As a user, I want opening a conversation to clear its local unread count, so that viewed state stays useful.
21. As a sender, I want a paperclip action to open a multiple-file picker, so that I can attach several files efficiently.
22. As a desktop sender, I want to drag files onto the conversation, so that attachment selection fits desktop workflows.
23. As a sender, I want only JPEG, PNG, WebP, GIF, MP4, and WebM files accepted, so that supported media has predictable handling.
24. As a sender, I want image files limited to 20 MiB, so that a single image cannot consume unreasonable session memory.
25. As a sender, I want video files limited to 100 MiB, so that short videos remain useful without unbounded transfer.
26. As a sender, I want all attachments in one message limited to 100 MiB total, so that a multi-file group remains bounded.
27. As a sender, I want at most 10 attachments in one message, so that transfer and layout remain manageable.
28. As a sender, I want optional plain text alongside attachments, so that one caption can describe the complete group.
29. As a sender, I want attachment text to retain the existing 4,000-character limit, so that media does not bypass composer bounds.
30. As a sender, I want attachment previews, filenames, and sizes shown before send, so that I can verify my selection.
31. As a sender, I want to remove an individual selected attachment before send, so that one mistake does not clear the group.
32. As a sender, I want selection order preserved, so that the recipient sees the intended sequence.
33. As a sender, I want invalid selections explained before transmission, so that failure is actionable and costs no bandwidth.
34. As a sender, I want filenames visible before send, so that disclosure of the original filename is not hidden.
35. As an offline sender, I want attachment sending disabled with no queue, so that media is never promised for later delivery.
36. As a sender, I want a manifest sent before binary content, so that the receiver can validate and reserve capacity first.
37. As a recipient, I want manifest count, type, individual size, and aggregate size validated, so that peers cannot bypass product limits.
38. As a recipient, I want incoming media accepted automatically after validation, so that online-only delivery does not wait for another interaction.
39. As a recipient, I want a manifest rejected before bytes when session capacity is insufficient, so that the transfer cannot exhaust the tab.
40. As a user, I want text and control messages kept on the chat data channel, so that media framing cannot corrupt chat envelopes.
41. As a user, I want binary chunks carried on a separate media data channel, so that large files do not occupy the chat stream.
42. As a user, I want media frames capped at 16 KiB and the negotiated transport maximum, so that transfer works across supported browser engines.
43. As a user, I want media sending paused when the browser buffer is high, so that queuing cannot grow without bound.
44. As a user, I want media sending resumed through the data-channel low-buffer event, so that transfer continues without polling loops.
45. As a recipient, I want each binary frame tied to an attachment ID and chunk index, so that assembly is deterministic.
46. As a recipient, I want attachment size verified after assembly, so that truncated or oversized content is rejected.
47. As a recipient, I want attachment SHA-256 verified incrementally, so that incorrectly assembled content is never presented as complete.
48. As a recipient, I want MIME type and binary signature validated, so that a renamed hostile file is not trusted as media.
49. As a recipient, I want filename extensions treated only as display metadata, so that extension spoofing does not control rendering.
50. As a recipient, I want an invalid attachment to fail independently, so that valid siblings may still complete.
51. As a sender, I want attachments transferred FIFO within their message, so that bandwidth and state remain predictable.
52. As a sender, I want a single aggregate byte percentage, so that progress remains readable for a multi-file group.
53. As a user, I want each attachment marked `waiting`, `transferring`, `complete`, or `failed`, so that partial state is visible.
54. As a recipient, I want incoming progress visible, so that an automatically downloaded video does not look frozen.
55. As a sender, I want completed sibling attachments preserved when one fails, so that successful work is not discarded.
56. As a sender, I want retry to resend only incomplete attachments, so that a partial failure does not repeat completed bytes.
57. As a recipient, I want retries to preserve message and attachment IDs, so that completed files can be acknowledged without duplication.
58. As a sender, I want the group marked `delivered` only after every attachment completes and the final acknowledgement arrives, so that partial receipt is not misrepresented.
59. As a sender, I want to cancel an in-progress attachment group, so that I can stop unwanted remaining transfer.
60. As a sender, I want cancellation to stop the active attachment and queued siblings, so that bandwidth use ends promptly.
61. As a sender, I want cancellation to state that already received files cannot be recalled, so that stopping transfer is not mistaken for deletion.
62. As a sender, I want a cancelled group marked `not_delivered`, so that terminal state remains truthful.
63. As a sender, I want retry after cancellation to restart incomplete attachments from their beginning, so that resumable-byte complexity is avoided.
64. As a sender, I want a peer disconnect to stop media transfer, so that bytes are not silently queued offline.
65. As a sender, I want media retry to remain explicit after reconnect handling, so that a large upload never restarts unexpectedly.
66. As a recipient, I want completed images visible directly in the conversation, so that viewing does not require another download step.
67. As a recipient, I want completed videos playable directly from the conversation, so that playback stays inside chat.
68. As a recipient, I want one to three attachments rendered directly, so that small groups use their available space.
69. As a recipient, I want four or more attachments shown in a four-tile grid, so that large groups remain compact.
70. As a recipient, I want a `+N` overlay on the fourth tile when more than four attachments exist, so that hidden item count is obvious.
71. As a recipient, I want clicking any tile to open the gallery at that attachment, so that navigation preserves context.
72. As a recipient, I want clicking `+N` to expose every attachment in the message, so that no file becomes inaccessible.
73. As a recipient, I want expanded image viewing in the gallery, so that image details are usable.
74. As a recipient, I want video controls and fullscreen support in the gallery, so that playback remains practical.
75. As a recipient, I want media never to autoplay, so that chat does not unexpectedly consume attention or produce sound.
76. As a recipient, I want media usable only after full transfer and validation, so that incomplete bytes are never decoded as a finished file.
77. As a recipient, I want an unsupported video codec represented as a non-playable file card, so that the failure is understandable.
78. As a recipient, I want an unsupported but accepted container saveable as its original file, so that the received bytes remain accessible.
79. As a user, I want tab media retained under a 256 MiB budget, so that accepted contacts cannot grow memory without bound.
80. As a user, I want a `Clear media` action, so that I can release tab memory without erasing text context.
81. As a user, I want cleared media represented by placeholders, so that transcript order and captions remain understandable.
82. As a user, I want object URLs revoked when media is cleared, the transcript is destroyed, or I log out, so that browser resources are released.
83. As a user, I want session-media limits explained with a recovery action, so that capacity rejection is not a dead end.
84. As a privacy-conscious user, I want media payloads absent from backend HTTP, WebSocket, logs, and storage, so that the server cannot inspect chat content.
85. As a privacy-conscious user, I want local filesystem paths never sent, so that attachments disclose only browser-provided metadata.
86. As a keyboard user, I want attachment actions, gallery navigation, cancellation, retry, and close controls keyboard-accessible, so that media chat does not require a pointer.
87. As a screen-reader user, I want transfer and validation state announced without stealing focus, so that asynchronous progress remains understandable.
88. As a mobile user, I want the composer, media grid, and gallery usable at narrow widths, so that primary actions remain reachable.
89. As a reduced-motion user, I want progress and gallery transitions to respect my motion preference, so that media UI remains comfortable.
90. As an instance operator, I want the web app and peer protocol deployed as one version, so that media capability negotiation with stale clients is unnecessary for this release.

## Implementation Decisions

- Deliver one vertical slice: accepted contacts and streamed presence, WebRTC
  peer lifecycle, acknowledged text, ephemeral transcripts, then image/video
  attachments. Accepted ADRs for the frontend foundation remain authoritative.
- Keep the backend responsible only for identity, contacts, presence, ICE
  configuration, and signaling. It must not accept, relay, persist, queue, log,
  or inspect chat text, manifests, or binary media.
- Keep one reliable ordered `chat` data channel for text messages, media
  manifests, readiness results, completion controls, cancellation, errors, and
  acknowledgements.
- Add one reliable ordered `media` data channel per peer connection for binary
  frames. Attachments within a message transfer sequentially in selection
  order. Different contacts retain independent peer connections and queues.
- Extend a chat message with optional plain text and an attachment manifest.
  Message and attachment identities use UUIDs. Manifest entries carry identity,
  selection index, sanitized original filename, declared MIME type, and size.
- Allow up to 10 attachments and 100 MiB aggregate per message. Limit each
  image to 20 MiB and each video to 100 MiB. Keep text limited to 4,000 Unicode
  characters.
- Accept JPEG, PNG, WebP, GIF, MP4, and WebM. Do not transcode. Validate MIME
  type and binary file signature on both sender and receiver; never trust the
  extension as a render decision.
- Preserve original attachment bytes. This also preserves embedded file
  metadata; metadata stripping is not part of this release.
- Sanitize filenames for display and save behavior. The browser-provided path
  is unavailable and never enters the protocol.
- Have the receiver validate the manifest and reserve memory before binary
  transfer. Return an individual ready or rejected result for every attachment
  so one invalid sibling does not block valid ones.
- Use binary frames no larger than 16 KiB including a fixed header containing a
  protocol version, 16-byte attachment UUID, and 32-bit chunk index. Also obey
  the negotiated SCTP maximum message size.
- Apply media-channel backpressure: pause above 1 MiB buffered, set the low
  threshold to 256 KiB, and resume from the low-buffer event.
- Compute SHA-256 incrementally on both peers. A completion control carries the
  claimed size and digest. The receiver completes an attachment only after all
  expected bytes, size, signature, and digest agree.
- A completion control and binary chunks travel on different SCTP streams and
  may be observed in either cross-stream order. Finalization waits for both
  complete control metadata and expected binary content.
- Track status per attachment and derive one aggregate message percentage by
  bytes. Final message delivery requires all attachments and the final message
  acknowledgement.
- Preserve completed siblings after partial failure. Explicit retry keeps IDs,
  deduplicates completed attachments, and restarts only incomplete attachments
  from byte zero. No automatic offline queue, resumable offset, or background
  resend exists.
- Cancellation applies to the complete group. It stops active and queued
  incomplete attachments, cannot retract completed remote files, and leaves the
  message `not_delivered`.
- Automatically receive valid manifests while both peers remain online. Do not
  require a second recipient acceptance click.
- Enforce a 256 MiB tab-wide budget across completed and in-progress media.
  Reserve capacity before transfer. `Clear media` revokes object URLs and
  releases blobs while preserving text, ordering, and placeholders.
- Keep transcripts and media tab-local and memory-only. Reload, close, logout,
  or session teardown destroys them. Do not use browser persistence or backend
  storage.
- Add a composer paperclip with a multiple-file picker and desktop
  drag-and-drop. Show ordered previews, names, sizes, validation errors, and
  individual removal before send. Clipboard paste is deferred.
- Render one to three media items directly. At four or more, render a four-tile
  grid. With more than four, overlay the fourth tile with the count beyond the
  visible four.
- Open an in-app gallery at the clicked attachment. Support full attachment
  navigation, expanded images, manual video playback, native controls, and
  fullscreen. Never autoplay.
- Do not use progressive `MediaSource` playback. Enable viewing or playback only
  after full transfer and validation. If an accepted video container uses an
  unsupported codec, show a non-playable card and allow saving the original.
- Deploy the web client and peer protocol together. Do not add a feature or
  backward-compatibility handshake for older clients in this release.
- Preserve the existing carbon-and-amber visual language, responsive contact
  rail/conversation information architecture, English UI copy, visible keyboard
  focus, screen-reader status announcements, and reduced-motion behavior.

## Testing Decisions

- Test observable user and peer-protocol behavior, not component internals.
  Prefer the highest seam that proves a workflow and keep lower seams only for
  deterministic failure cases that real WebRTC makes slow or flaky.
- Primary seam: a two-peer Playwright workflow using two independent Chromium
  browser contexts against real Vite and Go processes. It proves login,
  accepted contacts, streamed presence, negotiation, acknowledged text, mixed
  image/video transfer, automatic receipt, four-tile `+N` layout, gallery
  navigation, delivery state, unread state, and transcript loss on reload.
- Extend the primary workflow with bounded fixtures rather than 100 MiB files.
  Product limits are verified deterministically below the browser seam; the
  browser workflow proves transport and rendering without making the suite
  needlessly slow.
- Use Vitest and Testing Library at the workspace boundary for composer
  selection/removal/order, count and byte limits, MIME/signature validation,
  progress, cancellation, partial failure, retry, deduplication, memory-budget
  rejection, `Clear media`, object URL cleanup, unsupported-codec fallback,
  grid/gallery behavior, keyboard access, and status announcements.
- Use a deterministic fake peer/data-channel seam around the browser transport
  APIs to test manifest readiness, 16 KiB framing, frame headers, cross-stream
  control ordering, buffered-amount backpressure, disconnects, duplicate
  chunks, truncation, wrong sizes, digest mismatch, per-attachment ACK/error,
  and final-message ACK without testing browser internals.
- Retain Go's established in-process HTTP plus authenticated-WebSocket public
  seam. Regression tests prove contacts, presence, ICE config, signaling,
  authorization, and that unsupported chat/media payloads cannot turn the
  backend into a relay.
- Run the complete two-peer flow in Chromium. Run authentication, negotiation,
  attachment selection, and responsive media-layout smoke tests in Firefox and
  WebKit, consistent with the existing browser-support ADR.
- Review desktop and mobile screenshots at the established 1440x900 and 390x844
  sizes. Check grid cropping, `+N`, progress, long sanitized filenames, gallery,
  error recovery, focus visibility, clipped controls, and reduced motion.
- Required gates: Go formatting and tests, frontend typecheck, frontend unit and
  component tests, production build, Playwright suites, and final manual
  two-browser WebRTC verification in Chromium, Firefox, and WebKit engines.

## Out of Scope

- Audio attachments, generic documents, archives, or arbitrary file transfer.
- Voice calls, video calls, live camera or microphone tracks, screen sharing,
  live streaming, or recording inside the app.
- Progressive video playback, adaptive bitrate streaming, thumbnails generated
  on the server, transcoding, remuxing, compression, or codec conversion.
- Clipboard paste, camera capture, media editing, cropping, annotations, filters,
  or attachment reordering after selection.
- Server-side chat/media relay, storage, moderation, scanning, indexing,
  thumbnails, CDN delivery, or offline queues.
- Persistent local history, IndexedDB or filesystem-backed media cache,
  encrypted archive, backup, synchronized history, or multi-device delivery.
- Resuming an interrupted attachment from a byte offset, automatic resend after
  reconnect, background transfer, service-worker transfer, or push notification.
- Recall, remote deletion, message deletion, editing, forwarding, reactions,
  replies, read receipts, or typing indicators.
- Image or video metadata stripping, EXIF/GPS removal, watermarking, or content
  sanitization beyond format signature validation.
- Playback guarantees for every codec that can appear inside MP4 or WebM. An
  unsupported codec uses the file fallback.
- Media feature negotiation with old web clients. The instance deploys one web
  and peer-protocol version.
- Native mobile or desktop applications and separately deployed frontend
  origins.

## Further Notes

- ADR 0022 captures the media-specific decisions. Accepted ADRs 0002 through
  0021 and the domain glossary continue to govern the prerequisite browser chat
  foundation. When this PRD and a narrower ADR differ, the narrower ADR governs.
- `Delivered` continues to mean that the remote application accepted and
  validated the complete message. It does not mean read.
- `Online` continues to mean signaling availability, not guaranteed peer
  connectivity or media-delivery success.
- Media remains part of the ephemeral session transcript. Saving a received
  fallback file is an explicit browser action outside transcript persistence.
- Sending original bytes can disclose embedded metadata such as capture time,
  device details, or location. Because metadata stripping is out of scope, UI
  copy should avoid implying that file contents are anonymized.
- Success requires two real browser contexts to exchange acknowledged text and
  mixed media in both directions while backend tests prove chat payloads remain
  outside server transport and storage.
