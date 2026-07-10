# ADR 0006: WebRTC Negotiation

## Status

Accepted

## Context

Either contact may open a conversation first, and both may do so at nearly the
same time. A caller-only model cannot guarantee that the other peer initiates,
while unmanaged simultaneous offers create signaling glare.

## Decision

Peer connections will be created lazily when a user opens an online
conversation or attempts to send its first message. Either peer may initiate.

Clients will implement WebRTC perfect negotiation. For a contact pair, the user
with the greater numeric user ID is the polite peer and the user with the lower
ID is the impolite peer. This stable role assignment handles simultaneous
offers without another server-side coordination concept.

The first unexpected connection loss while both users remain online triggers
one automatic reconnection attempt. Further failure leaves the conversation
disconnected and presents an explicit `Try again` action. Going offline stops
reconnection until streamed presence reports the contact online again.

## Consequences

- Both contacts can initiate a conversation without a call-request workflow.
- Simultaneous offers have deterministic collision behavior.
- Perfect negotiation state and ICE candidates need focused unit tests.
- Reconnection remains bounded and visible instead of looping indefinitely.

