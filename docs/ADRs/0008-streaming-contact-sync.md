# ADR 0008: Streaming Contact Synchronization

## Status

Accepted

## Context

Contact requests and resolutions currently change persistent state without
notifying connected browsers. Manual refresh is unsuitable for a realtime app,
while polling creates recurring traffic and duplicate synchronization logic.

## Decision

The WebSocket will send this invalidation event to every affected online user
after contact-request state changes:

```json
{"type":"contacts_changed"}
```

On receipt, the frontend refetches both the accepted-contact list and incoming
contact-request list through their HTTP endpoints. HTTP remains the source of
truth; the WebSocket event deliberately carries no duplicated contact payload.

Creating a request notifies the recipient. Accepting or rejecting notifies both
the requester and recipient. A disconnected user receives current state through
the normal HTTP bootstrap after reconnecting.

## Consequences

- Contact changes appear without polling or page refresh.
- One event can invalidate both related HTTP resources.
- Duplicate events are harmless because refetching is idempotent.
- The backend must publish only after the database transaction commits.

