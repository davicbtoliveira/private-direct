# PRD 0002: Private Direct MVP Frontend

Issue label: `ready-for-agent`

## Problem Statement

Members of a small self-hosted private community have a backend that can create
accounts, establish contacts, report presence, and forward WebRTC signaling, but
they have no browser application through which to complete those workflows or
exchange messages. They cannot reliably tell who is reachable, establish a
direct channel, understand connection failures, or exchange text without using
the backend API manually.

The frontend must preserve Private Direct's defining boundary: chat messages
travel directly between two online contacts over WebRTC and never pass through,
persist on, queue at, or fall back to the server.

## Solution

Build the production MVP as an English-language React browser application served
from the same origin and deployable Go application as the backend. Invited users
will register, authenticate, find exact usernames, manage contact requests, see
streamed presence, negotiate direct peer connections, and exchange acknowledged
plain-text messages.

The application will expose truthful connection and delivery states. It will
keep per-contact transcripts only in the active tab's memory, reject offline
delivery, and make ephemeral behavior explicit. A carbon-and-amber responsive
workspace will provide a contact rail on desktop and separate contact and chat
views on mobile, with a functional direct-line trace communicating peer-channel
state.

Browser-critical backend gaps are part of this vertical slice. The work includes
the minimum HTTP, WebSocket, validation, presence, signaling, and static-serving
changes required for the frontend to present correct state.

## User Stories

1. As an invited user, I want to register with an invite code, username, and password, so that I can join my private instance.
2. As an invited user, I want invalid or consumed invite codes rejected clearly, so that I know I need a valid invitation.
3. As an invited user, I want my username normalized before registration, so that letter casing does not create ambiguous identities.
4. As an invited user, I want immediate username-format feedback, so that I can correct an invalid handle before submitting.
5. As an invited user, I want duplicate usernames rejected clearly, so that every contact identity remains unique.
6. As an invited user, I want password-length feedback based on the backend's actual limits, so that registration does not fail unexpectedly.
7. As an invited user, I want to confirm my password locally, so that a typing mistake does not consume my invite and lock me out.
8. As a newly registered user, I want the MVP to log me in automatically, so that I do not retype credentials I just submitted.
9. As a newly registered user whose automatic login fails, I want confirmation that my account exists and my username prefilled on login, so that I can recover without reusing my invite.
10. As a security-conscious user, I want my registration password cleared after use, so that it is not retained by the application.
11. As a registered user, I want to log in with username and password, so that I can access my contacts.
12. As a registered user, I want authentication errors to remain generic, so that the interface does not disclose whether another username exists.
13. As a returning user, I want my session restored through the refresh cookie, so that a reload does not require another password entry.
14. As a returning user without a valid session, I want to land directly on login, so that the application does not expose a broken workspace.
15. As an authenticated user, I want my access token held only in memory, so that browser storage does not retain a bearer credential.
16. As an authenticated user, I want the frontend to treat JWTs as opaque, so that token claims do not become a second identity contract.
17. As an authenticated user, I want the access token refreshed before expiry, so that normal use is not interrupted.
18. As an authenticated user, I want realtime transports to reconnect with the newest access token, so that a refreshed session remains usable.
19. As an authenticated user, I want to log out, so that my refresh session, peer connections, transcripts, drafts, and unread state are cleared.
20. As a privacy-conscious user, I want WebSocket authentication kept out of the URL, so that access tokens are not exposed in routine URL logs.
21. As an authenticated user, I want only one active realtime connection for my account in the MVP, so that competing tabs cannot answer the same peer offer.
22. As a user opening a newer tab or device, I want it to take over realtime messaging, so that the newest context is authoritative.
23. As a user in a replaced tab, I want to see that messaging continued elsewhere, so that I understand why realtime updates stopped.
24. As a user in a replaced tab, I want HTTP authentication to remain valid, so that realtime takeover does not unexpectedly log me out.
25. As an authenticated user, I want to find another user by exact username, so that discovery remains intentional and private.
26. As an authenticated user, I want username lookup to normalize casing and surrounding whitespace, so that valid handles are easy to enter.
27. As an authenticated user, I want unknown usernames to produce a specific empty result, so that I can correct the handle.
28. As an authenticated user, I want to send a contact request from an exact lookup result, so that the recipient can approve communication.
29. As an authenticated user, I want repeated contact-request clicks handled predictably, so that duplicate state is not created.
30. As a request recipient, I want a new incoming request to appear in realtime, so that I do not refresh or wait for polling.
31. As a request recipient, I want to review all pending incoming requests, so that I can control who may contact me.
32. As a request recipient, I want to accept a request, so that the requester becomes an approved contact.
33. As a request recipient, I want to reject a request, so that an unwanted user cannot initiate signaling.
34. As a party to a resolved request, I want contact state refreshed in realtime, so that both interfaces agree without a page refresh.
35. As a user whose contact was just accepted, I want the new contact's current presence immediately, so that the contact does not remain stuck in an unknown state.
36. As an authenticated user, I want my accepted contacts ordered consistently, so that the contact rail is predictable.
37. As an authenticated user, I want empty contact and request collections represented as empty lists, so that the interface can render proper empty states.
38. As an authenticated user, I want to see a connecting state while realtime presence initializes, so that unknown state is not mislabeled offline.
39. As an authenticated user, I want an initial snapshot of online accepted contacts, so that presence is correct immediately after connection.
40. As an authenticated user, I want subsequent presence changes streamed live, so that status never requires an F5 refresh.
41. As an authenticated user, I want only accepted contacts' presence, so that the server does not expose unrelated users' availability.
42. As an authenticated user, I want a contact marked offline when their realtime connection ends, so that I do not attempt impossible delivery.
43. As an authenticated user, I want connection replacement to avoid a false offline transition, so that tab takeover does not flicker contact presence.
44. As an authenticated user, I want online presence distinguished from a connected peer channel, so that reachability is not confused with successful WebRTC negotiation.
45. As an authenticated user, I want the instance's STUN and optional TURN configuration applied automatically, so that direct connectivity uses operator settings.
46. As an online contact, I want opening a conversation to begin peer negotiation, so that the direct channel is ready when I write.
47. As an online contact, I want sending the first message to begin negotiation when needed, so that a missing channel does not discard my intent.
48. As a contact, I want to initiate a conversation from either side, so that messaging does not depend on a fixed caller role.
49. As a contact opening a conversation simultaneously with another contact, I want offer collisions handled deterministically, so that glare does not break the conversation.
50. As an authenticated user, I want stale answers and ICE candidates ignored after reconnection, so that an old attempt cannot corrupt the active peer connection.
51. As an authenticated user, I want signaling failures correlated to the correct contact and connection attempt, so that one failing peer does not mark another conversation failed.
52. As an online contact, I want one bounded automatic reconnection attempt after unexpected peer loss, so that transient failures recover without an infinite loop.
53. As an online contact after repeated failure, I want an explicit Try again action, so that recovery remains under my control.
54. As an offline contact, I want reconnection attempts stopped until presence returns online, so that the browser does not loop against an unreachable peer.
55. As an authenticated user, I want incoming offers accepted for inactive conversations, so that I remain reachable while viewing another contact.
56. As an authenticated user, I want one peer connection per contact that has initiated messaging, so that switching conversations does not force needless renegotiation.
57. As an authenticated user, I want a reliable ordered data channel named chat, so that message order is stable.
58. As an authenticated user, I want simultaneous negotiation to produce one deterministic chat data channel, so that duplicate messages are not created by duplicate channels.
59. As a sender, I want to compose plain text up to 4,000 Unicode code points, so that messages are useful but bounded.
60. As a sender, I want Enter to send and Shift+Enter to insert a line break, so that keyboard behavior matches a messaging tool.
61. As a sender, I want a whitespace-only message to keep the send button visibly and semantically disabled, so that empty content cannot be sent.
62. As a sender, I want Markdown and HTML interpretation omitted, so that the MVP renders exactly the text I entered.
63. As a sender to an online contact whose channel is negotiating, I want a bounded in-memory pending state, so that my message can send when the channel opens.
64. As a sender to an offline contact, I want sending disabled with no queue, so that the interface never promises offline delivery.
65. As a sender, I want a new message labeled sending until the remote client acknowledges it, so that transport state is truthful.
66. As a sender, I want a message labeled delivered only after the receiving application accepts it, so that delivery has a precise meaning.
67. As a sender, I want a missing acknowledgement or closed channel to mark the message not delivered, so that failure is visible.
68. As a sender, I want to retry a not-delivered message explicitly, so that recovery does not create a hidden background queue.
69. As a recipient, I want retries deduplicated by message ID and acknowledged again, so that retry never displays the same message twice.
70. As a recipient, I want malformed, unsupported, or oversized peer envelopes ignored safely, so that invalid peer input does not break the transcript.
71. As a user, I want no read receipts in the MVP, so that delivered is not misrepresented as read.
72. As a user, I want separate in-memory transcripts per contact, so that switching conversations preserves current-tab context.
73. As a user, I want refreshing, closing the tab, or logging out to clear transcripts, so that the MVP does not imply persistent history.
74. As a user, I want one contextual disclosure that transcripts are temporary, so that ephemeral behavior is understood without repeated warnings.
75. As a user receiving a message in an inactive conversation, I want a local unread badge, so that I notice new activity.
76. As a user opening an unread conversation, I want its badge cleared locally, so that the rail reflects what I have viewed in this tab.
77. As a desktop user, I want contacts and the active conversation visible side by side, so that repeated messaging is efficient.
78. As a mobile user, I want contact list and conversation on separate full-width views, so that controls and text remain usable on a narrow screen.
79. As a mobile user, I want a familiar back action in a conversation, so that I can return to contacts quickly.
80. As an unauthenticated user, I want direct login and registration screens instead of a marketing landing page, so that I reach the product immediately.
81. As a user, I want contact search and incoming requests in focused sheets, so that secondary tasks do not permanently crowd the workspace.
82. As a user, I want connection, empty, offline, failure, and active states to have distinct actionable layouts, so that I always know what can happen next.
83. As a user, I want interface copy in consistent English, so that actions and outcomes use one vocabulary.
84. As a user, I want a dark carbon interface with restrained amber detail, so that the product has a focused identity without generic cyberpunk decoration.
85. As a user, I want online teal and fault coral reserved for semantic states, so that presence and failure remain distinguishable from amber interaction detail.
86. As a user, I want the direct-line trace to encode peer-channel state, so that the signature visual element also communicates useful information.
87. As a reduced-motion user, I want an equivalent static connection state, so that animation is never required to understand the interface.
88. As a keyboard user, I want visible focus and complete keyboard access, so that every primary workflow is operable without a pointer.
89. As a screen-reader user, I want semantic controls and announced status changes, so that realtime behavior remains understandable.
90. As a user with long handles or messages, I want text to wrap or truncate intentionally without overlap, so that the layout remains coherent.
91. As a self-hosted instance operator, I want frontend and backend on one origin, so that deployment avoids CORS and cross-origin cookie configuration.
92. As a self-hosted instance operator, I want one Go production artifact serving the built SPA, so that deployment remains small.
93. As a self-hosted instance operator, I want the health endpoint kept outside the product API namespace, so that infrastructure checks remain stable.
94. As a developer, I want product endpoints under /api and human-facing routes owned by the SPA, so that routing responsibilities do not collide.
95. As a developer, I want missing API and asset routes to return real 404 responses, so that the SPA fallback does not hide deployment errors.
96. As a developer, I want browser-critical backend changes tested at public HTTP and WebSocket boundaries, so that frontend assumptions match observable behavior.
97. As a developer, I want realtime state represented by explicit reducers and transport services, so that transitions are deterministic and testable.
98. As a developer, I want a full two-browser workflow test, so that API, WebSocket, WebRTC, and UI behavior are verified together.
99. As a developer, I want focused protocol tests for collision, early ICE, timeout, deduplication, and retry, so that rare timing paths remain deterministic.
100. As a developer, I want desktop and mobile screenshot review, so that overflow, overlap, focus, contrast, and responsive regressions are visible before release.

## Implementation Decisions

- Build a React and TypeScript SPA with Vite. Bun owns dependency installation,
  scripts, and the lockfile; Bun is not required at production runtime.
- Use React Router, Lucide React, CSS Modules, global design tokens, React context
  and reducers, and a small typed fetch client. Do not add Redux, TanStack Query,
  Tailwind CSS, or a component kit for the MVP.
- Serve the production build from the Go application on the same origin. Vite
  proxies HTTP and WebSocket traffic to Go during development.
- Move every product HTTP and WebSocket endpoint under `/api`, including the
  operator invite endpoint and `/api/ws`. Keep `/health` at the root. Do not keep
  aliases for the pre-frontend prototype routes.
- Let the SPA own human-facing routes including `/login`, `/register`, and
  `/chat/:username`. Unknown API routes, missing assets, and non-GET requests
  must not receive the SPA fallback.
- Keep invite administration outside the UI. Operators continue to create invite
  codes through the authenticated API.
- Canonicalize usernames by trimming and lowercasing input. Enforce 3-32 ASCII
  characters matching `[a-z0-9][a-z0-9._-]{2,31}`. Display `@` only in the UI.
- Enforce registration passwords of 12-72 UTF-8 bytes, with no composition
  rules. Confirm passwords in the registration form. Login failures stay
  generic.
- For the MVP, registration is followed by an automatic login call. If that
  second call fails, retain only the normalized username and explain that the
  account was created.
- Add the authenticated user object to login and refresh responses. Keep the
  existing 15-minute JWT access token opaque and in memory only. Keep refresh
  credentials in the existing HttpOnly cookie.
- Restore browser sessions with `POST /api/refresh`, refresh shortly before
  expiry, and close all realtime and in-memory conversation state on logout.
- Authenticate `/api/ws` with the `private-direct` WebSocket subprotocol plus the
  opaque access token in `Sec-WebSocket-Protocol`. Remove query-string token
  authentication.
- Allow one active realtime connection per user for the MVP. A newer connection
  sends `session_replaced` to the previous client, takes over signaling, and
  preserves continuous online presence.
- Send `presence_snapshot` as the first application event on each WebSocket.
  Model presence as connecting, online, or offline. Stream later presence deltas
  without polling or refresh.
- Send `contacts_changed` only after committed request changes. The client treats
  it as an invalidation and refetches contacts and incoming requests through
  HTTP. After acceptance, send each side the new contact's current presence.
- Normalize empty contact and incoming-request collections to JSON arrays rather
  than null.
- Maintain one peer connection per contact in the active realtime context.
  Accept valid incoming offers even when that conversation is inactive and keep
  established channels across client-side navigation.
- Implement WebRTC perfect negotiation. The greater user ID is polite and the
  lower user ID is impolite. Queue ICE received before a remote description.
- Use a per-attempt `connection_id` in signaling offers, answers, candidates, and
  errors. Adopt the winning offer's ID and ignore stale events from prior
  attempts.
- Use a negotiated, ordered, reliable DataChannel named `chat` with deterministic
  ID 0, preventing duplicate channels during simultaneous negotiation.
- Set peer negotiation timeout to 15 seconds. Attempt one automatic reconnect
  while the contact remains online; require explicit retry after that.
- Use JSON `message` and `ack` peer envelopes. Generate message IDs with
  `crypto.randomUUID()`. Treat timestamps as informative client timestamps.
- Validate inbound envelope type, required fields, UUIDs, and message size.
  Render message content as literal text, never HTML.
- Define the message limit as 4,000 Unicode code points. Bound the negotiation
  pending-send collection to 20 messages per contact.
- Start the acknowledgement timeout only after DataChannel send. Use an 8-second
  timeout. Retry with the same message ID; recipients deduplicate in memory and
  replay the acknowledgement without redisplaying content.
- Keep transcript, delivery, draft, deduplication, and unread state in memory per
  contact. Do not use localStorage, sessionStorage, IndexedDB, or backend storage
  for chat content.
- Use a desktop 280-pixel contact rail and flexible conversation pane. On mobile,
  render contact list and conversation as separate full-width views. Open
  contact search and requests as focused sheets.
- Use English interface copy grouped by domain, without an internationalization
  runtime in the MVP.
- Use the accepted visual tokens: Carbon `#10120F`, Panel `#191D18`, Text
  `#ECEFE7`, Amber `#F2B84B`, Online `#48B9A7`, and Fault `#EF6A5B`. Use no
  gradients and no radius above 6 pixels.
- Bundle Chivo for brand/headings, Atkinson Hyperlegible for UI/conversation, and
  IBM Plex Mono for handles, times, and connection state.
- Use one functional direct-line trace as the signature visual. Animate it once
  on connection, and provide a static reduced-motion equivalent.
- Support the two most recent stable Chrome/Edge, Firefox, and Safari releases,
  with desktop and responsive mobile layouts.

## Testing Decisions

- Test observable behavior rather than component or handler implementation
  details. Prefer the highest seam that can prove a user workflow.
- The primary seam is one Playwright scenario using two independent browser
  contexts against real Vite and Go processes. It covers registration or login,
  contact request and acceptance, streamed presence, bidirectional WebRTC text,
  acknowledgements, inactive-conversation unread state, offline blocking, and
  transcript loss on reload.
- Retain the backend's established public-boundary seam: in-process Go HTTP and
  authenticated WebSocket tests backed by SQLite. Extend it for `/api` routing,
  validation, session identity, empty arrays, subprotocol authentication,
  connection replacement, ordered presence snapshot/deltas, contact
  invalidation, presence after acceptance, signaling correlation, authorization,
  and proof that chat payloads are rejected by the server.
- Use focused Vitest and Testing Library tests only where the browser seam would
  be slow or nondeterministic: session refresh, workspace reducers, composer
  keyboard/disabled behavior, perfect-negotiation glare, early ICE, stale
  signaling, timeout, bounded pending sends, acknowledgement, deduplication,
  retry, unread counts, and teardown.
- Run the full two-peer workflow in Chromium. Run authentication and responsive
  layout smoke tests in Firefox and WebKit.
- Capture and review screenshots at 1440x900 and 390x844. Verify no incoherent
  overlap, clipped actions, text overflow, unreadable contrast, or layout shift.
- Verify keyboard focus, semantic disabled controls, screen-reader status
  announcements, and `prefers-reduced-motion` behavior.
- Required gates are Go formatting and tests, frontend unit/component tests,
  frontend production build, Playwright tests, and manual WebRTC verification in
  Chromium, Firefox, and WebKit engines.

## Out of Scope

- Operator or invite-administration UI.
- Offline delivery, offline queues, push notifications, or background sync.
- Server-side chat relay, fallback transport, persistence, indexing, moderation,
  or access to chat payloads.
- Persistent local history, encrypted history, backups, synchronized history,
  retention controls, or message deletion. These are deferred beyond MVP rather
  than permanently prohibited.
- Images, files, audio, video, calls, reactions, replies, forwarding, editing,
  deletion, Markdown, rich text, or link previews.
- Read receipts, typing indicators, last-seen timestamps, or delivery across a
  closed browser.
- Display names, avatars, profiles, biographies, presence messages, or broad
  user-directory search.
- Outgoing-request management, cancellation, blocking, unblocking, removing
  contacts, or retrying a previously rejected request.
- Password change, forgotten-password recovery, email identity, MFA, passkeys,
  or account deletion.
- Multiple active realtime tabs, device identity, targeted device signaling,
  multi-device sync, or concurrent delivery to several devices.
- Custom end-to-end identity keys, key verification UX, safety numbers, or an
  encryption layer beyond WebRTC transport encryption.
- Federation, native mobile applications, desktop wrappers, and separately
  deployed frontend origins.
- Runtime localization, a language selector, or non-English interface copy.
- Analytics, telemetry, crash reporting, marketing pages, or onboarding tours.
- TURN provisioning or administration; the frontend only consumes operator
  configuration returned by the backend.

## Further Notes

- This PRD advances the production React application previously deferred by the
  backend-only PRD and architecture ADR. The online-only, no-server-message
  architecture remains unchanged.
- Accepted ADRs 0002 through 0021 and the domain glossary define the detailed
  decisions summarized here. When this PRD and an ADR differ, the narrower ADR
  governs.
- Success requires two real browser contexts to become accepted contacts, show
  correct streamed presence, establish one direct channel, and exchange
  acknowledged text in both directions without any chat payload reaching the Go
  server.
- Failed or offline connectivity must never display delivered state. Reloading,
  closing, or logging out must never imply that a transcript can be recovered.
- The release remains self-hosted and single-origin. The frontend build must be
  produced before compiling the Go binary that embeds its assets.
