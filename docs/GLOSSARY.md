# Private Direct Glossary

## Contact

Two registered users whose contact request was accepted. Only contacts may
exchange presence and WebRTC signaling events.

## Contact request

A pending invitation from one registered user to another exact username. The
recipient may accept or reject it.

## Contact invalidation

A streamed `contacts_changed` event telling a connected client to refetch
accepted contacts and incoming requests from the HTTP API.

## Data channel

The WebRTC transport used by two online contacts to exchange chat messages
directly. Chat payloads do not pass through the Private Direct server.

## Delivered

A message state reached only after the sending peer receives an acknowledgement
that the remote client accepted the message envelope. It does not mean read.

## Invite code

A single-use code created by the instance operator. A valid code is required to
register a user.

## Browser session

An authenticated browser lifecycle backed by an opaque JWT access token held
only in memory and a refresh token held only in an HttpOnly cookie. The frontend
does not decode or persist either token.

## API namespace

The `/api` path prefix that separates product HTTP and WebSocket endpoints from
human-facing SPA routes. The infrastructure health check remains `/health`.

## Online

A volatile state meaning that a user currently has an authenticated WebSocket
connection to the instance. It does not guarantee that a peer-to-peer
connection can be established.

## Presence snapshot

The initial set of accepted contacts currently online, sent when an
authenticated WebSocket connection opens. Subsequent changes arrive as streamed
presence events without a page refresh.

## Operator

The person running a self-hosted Private Direct instance. Operator actions are
authenticated separately from user actions.

## Peer connection

A WebRTC connection between two contacts. It must be negotiated while both
contacts are online and may still fail when network traversal is unavailable.

## Unread

A tab-local count of messages received for a conversation that is not currently
open. It is cleared when that conversation opens and is never synchronized.

## Polite peer

The contact that rolls back or accepts a colliding WebRTC offer during perfect
negotiation. The MVP assigns this role to the peer with the greater numeric user
ID.

## Signaling

The exchange of WebRTC offers, answers, and ICE candidates through the backend.
Signaling establishes a peer connection but never carries chat messages.

## Session transcript

Messages visible in the current browser session. The MVP does not persist or
synchronize message history. Switching contacts keeps it in memory, while
refreshing, closing the tab, or logging out clears it.

## Realtime connection

The single active MVP WebSocket connection for a user. Opening a newer
connection replaces the older one without invalidating its HTTP session.

## WebSocket subprotocol

The standard WebSocket handshake header used by the MVP to identify the
`private-direct` protocol and authenticate with the opaque in-memory access
token without putting that token in the URL.

## Username

A unique canonical lowercase handle containing 3 to 32 ASCII letters, digits,
dots, underscores, or hyphens. The UI prefixes it with `@`; storage does not.
