# PRD 0001: Private Direct MVP Backend

Issue label: `ready-for-agent`

## Problem Statement

Self-hosted users need a private direct messaging app where the server does not
store or relay chat messages. The MVP must let invited users register, find each
other, become contacts, see online availability, establish a peer-to-peer
WebRTC connection, and exchange text messages while both peers are online.

The initial delivery must focus on the Go backend. The React frontend is planned
but not the center of this PRD.

## Solution

Build the backend foundation for Private Direct.

The backend will own identity, invite-code registration, authentication,
contacts, volatile presence, WebSocket signaling, and ICE configuration. Text
messages will not pass through the backend. They will be sent by clients over
WebRTC DataChannel after signaling succeeds.

The MVP will be online-only. If the recipient is offline, a message cannot be
delivered. If WebRTC peer connectivity fails and no TURN configuration solves
it, the backend will not provide a server-relayed chat fallback.

## User Stories

1. As a self-hosted instance owner, I want registration gated by invite code, so that public signups are not accidentally enabled.
2. As a self-hosted instance owner, I want to create invite codes, so that I can decide who can join my instance.
3. As a self-hosted instance owner, I want invite codes to be consumed or invalidated, so that access can be controlled.
4. As a new user, I want to register with an invite code, username, and password, so that I can join a private instance.
5. As a new user, I want invalid invite codes rejected, so that only approved users can register.
6. As a new user, I want duplicate usernames rejected, so that identities are unambiguous.
7. As a registered user, I want to log in with username and password, so that I can access my account.
8. As a registered user, I want my session refreshed without retyping my password, so that normal usage is not interrupted.
9. As a registered user, I want to log out, so that my current session stops being usable.
10. As a registered user, I want short-lived access tokens, so that stolen bearer tokens have limited value.
11. As a registered user, I want refresh tokens kept out of JavaScript-accessible storage, so that browser token theft risk is reduced.
12. As a registered user, I want to search for another user by exact username, so that I can start a contact request.
13. As a registered user, I want unknown usernames to return no contact target, so that discovery stays explicit.
14. As a registered user, I want to send a contact request, so that another user can approve communication with me.
15. As a registered user, I want duplicate contact requests handled predictably, so that repeated clicks do not create duplicate state.
16. As a registered user, I want to see incoming contact requests, so that I can decide who may contact me.
17. As a registered user, I want to accept a contact request, so that signaling and messaging can begin with that user.
18. As a registered user, I want to reject a contact request, so that unwanted users cannot start communication with me.
19. As a registered user, I want accepted contacts listed, so that I can choose who to message.
20. As a registered user, I want only accepted contacts to signal me, so that non-contacts cannot initiate WebRTC sessions.
21. As a registered user, I want to connect to an authenticated WebSocket, so that the backend can track my online presence.
22. As a registered user, I want my online status to follow WebSocket connectivity, so that contacts see whether I am reachable now.
23. As a registered user, I want my presence cleared when I disconnect, so that stale online indicators are minimized.
24. As a registered user, I want to receive presence changes for accepted contacts, so that I know who can receive real-time messages.
25. As a registered user, I want ICE server configuration from the backend, so that my client can attempt WebRTC connectivity with the instance settings.
26. As a registered user, I want STUN configuration available, so that common NAT traversal cases can work.
27. As a self-hosted instance owner, I want TURN configuration supported, so that stricter networks can be handled when I provide TURN.
28. As a registered user, I want to send WebRTC offers to accepted online contacts, so that I can start a peer connection.
29. As a registered user, I want to receive WebRTC offers from accepted contacts, so that I can answer incoming peer connections.
30. As a registered user, I want to send WebRTC answers to accepted contacts, so that peer connection setup can complete.
31. As a registered user, I want to exchange ICE candidates with accepted contacts, so that connectivity checks can succeed.
32. As a registered user, I want signaling messages rejected when the target is not my accepted contact, so that the signaling channel does not bypass contact consent.
33. As a registered user, I want signaling messages rejected when the target is offline, so that the client can show that peer setup cannot proceed.
34. As a registered user, I want text messages sent over WebRTC DataChannel, so that the server does not store or relay chat content.
35. As a registered user, I want the backend to avoid chat message persistence, so that message history is not centralized on the server.
36. As a registered user, I want no offline queue in the MVP, so that delivery semantics are clear and real-time only.
37. As a developer, I want backend behavior covered at the public API and WebSocket boundary, so that tests match user-visible behavior.
38. As a developer, I want a small MVP with explicit deferred work, so that future media, offline delivery, and stronger E2E can be designed separately.

## Implementation Decisions

- Backend will be implemented in Go.
- Frontend will be React, but MVP implementation focus is backend.
- SQLite will be the MVP database.
- Registration will require an invite code.
- Users authenticate with username and password.
- Access tokens will be short-lived JWTs.
- Refresh tokens will be stored in HttpOnly cookies.
- WebSocket connections will be authenticated with the access token.
- Contacts are created through exact username lookup, outgoing request, incoming acceptance, or rejection.
- Only accepted contacts can exchange signaling messages.
- Presence is volatile and in memory.
- WebSocket connected means online. WebSocket disconnected means offline.
- The backend forwards WebRTC offers, answers, and ICE candidates between accepted online contacts.
- Text chat messages are sent over WebRTC DataChannel and do not traverse the backend.
- The backend will not persist, queue, or relay chat messages.
- STUN configuration is required.
- TURN configuration is optional but supported.
- If TURN is not configured and WebRTC connectivity fails, the MVP does not provide server fallback.
- The MVP relies on WebRTC transport encryption and does not add custom E2E identity/key verification yet.
- Primary backend domains are configuration, storage, invite management, authentication, contact management, presence, signaling, and HTTP/WebSocket transport.

## Testing Decisions

- Test external behavior, not implementation details.
- Prefer one high-level test seam: backend public boundary through HTTP API plus authenticated WebSocket signaling.
- Cover invite registration, login, refresh, logout, contact request flow, contact acceptance, presence updates, ICE config retrieval, and signaling authorization.
- Cover negative paths: invalid invite, duplicate username, bad password, unauthenticated WebSocket, signaling to non-contact, signaling to offline contact, and duplicate contact request.
- Use SQLite in tests to match the MVP storage engine.
- Use in-process server tests where possible so tests exercise routing, auth middleware, persistence, and WebSocket behavior together.
- Do not test WebRTC browser internals in backend tests. Backend tests only prove ICE config delivery and signaling message forwarding rules.
- No prior test patterns exist yet because the repository currently contains only documentation.

## Out of Scope

- Offline message delivery.
- Server-side chat message persistence.
- Server-side chat message relay fallback.
- Message history.
- Images, video, and audio messages.
- Custom end-to-end encryption identity layer.
- Key verification UX.
- Multi-device sync.
- Production React application.
- Mobile applications.
- Federation between self-hosted instances.
- Admin UI.
- Push notifications.

## Further Notes

- This PRD follows ADR 0001: MVP Architecture.
- The MVP intentionally favors a small server role: identity, contacts, presence, and signaling.
- Operators who need reliable connectivity across restrictive networks should configure TURN.
- Future work can add media messages, stronger cryptographic identity, and offline delivery as separate PRDs.
