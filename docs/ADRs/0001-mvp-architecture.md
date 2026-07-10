# ADR 0001: MVP Architecture

## Status

Accepted

## Context

Private Direct is a self-hosted direct messaging app. Backend will be written in
Go. Frontend will be written in React, but initial development focuses on the
backend.

The MVP supports text messages only. Future versions may add images, video, and
audio.

The product goal is private, peer-to-peer direct messaging with minimal server
responsibility.

## Decision

The MVP will use an online-only messaging model.

Users must be online at the same time to exchange messages. Text messages will
travel through WebRTC DataChannel between peers. The backend will not persist,
relay, or queue messages.

The backend will provide:

- invite-code based registration
- username and password authentication
- short-lived JWT access tokens
- refresh token stored in an HttpOnly cookie
- contact requests by exact username
- manual contact acceptance
- authenticated WebSocket signaling
- volatile in-memory presence
- WebRTC offer, answer, and ICE candidate forwarding between accepted contacts
- ICE server configuration delivery to clients

SQLite will be used as the MVP database.

STUN configuration is required. TURN configuration is optional but supported.
If peer connectivity fails without TURN, the MVP will not fall back to
server-relayed chat messages.

The MVP will rely on WebRTC transport encryption. It will not implement an
additional end-to-end encryption layer yet.

## Consequences

The MVP stays small and aligned with the P2P requirement.

The server has a limited role: identity, contacts, presence, and signaling. It
does not become a message store or message relay.

Offline delivery is not supported. If the recipient is offline, the sender
cannot deliver the message.

Message history is not available unless a later client-side or encrypted sync
design is added.

Some networks will fail peer connectivity without TURN. Operators who need
better reliability should configure TURN.

SQLite keeps self-hosted deployment simple. A future migration to Postgres may
be needed if deployment requirements grow.

## Deferred

- offline message delivery
- server-side message persistence
- images, video, and audio messages
- custom end-to-end encryption identity layer
- key verification UX
- multi-device sync
- production React application
- server-side fallback relay for chat messages
