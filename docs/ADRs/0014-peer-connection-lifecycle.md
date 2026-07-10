# ADR 0014: Peer Connection Lifecycle

## Status

Accepted

## Context

An accepted contact may initiate a direct conversation while the recipient is
viewing another contact. Restricting WebRTC negotiation to the visible route
would make a user appear online yet unable to receive messages. Closing every
connection on navigation would also create needless renegotiation.

## Decision

The active realtime browser tab may hold one peer connection per accepted
contact. A connection is created only when either side initiates. Incoming
offers from accepted contacts are handled even when their conversation is not
visible.

Once established, a peer connection remains active while both contacts are
online, unless it fails or either browser closes it. Switching the visible
conversation does not close the channel.

Messages received for an inactive conversation are appended to that contact's
in-memory session transcript and increase a local unread badge. Opening the
conversation clears its badge. System notifications and push notifications are
out of scope for the MVP.

## Consequences

- An online user can receive direct messages regardless of the visible route.
- Navigation does not repeatedly negotiate established channels.
- Peer, transcript, delivery, and unread state are keyed by contact ID.
- Resource use grows with contacts that establish channels during one tab
  session, acceptable for the MVP's small private instances.
