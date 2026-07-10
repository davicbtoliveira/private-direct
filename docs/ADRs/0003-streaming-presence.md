# ADR 0003: Streaming Presence

## Status

Accepted

## Context

The backend already broadcasts presence changes over WebSocket, but a newly
connected client does not know which contacts were online before it connected.
Treating every contact as offline until a future event produces false state.
Requiring a page refresh would also contradict real-time messaging behavior.

## Decision

An authenticated WebSocket connection will receive an initial presence event:

```json
{
  "type": "presence_snapshot",
  "online_users": [{"id": 2, "username": "ana"}]
}
```

The snapshot contains only accepted contacts that are currently online. After
the snapshot, the existing `presence` events stream every relevant online or
offline transition without a page refresh.

The client models presence as `connecting`, `online`, or `offline`. It returns
all contacts to `connecting` whenever the WebSocket connection is lost and only
derives `online` or `offline` after a new snapshot or streamed event.

A user is online while their single active MVP realtime WebSocket connection is
active. A newer connection replaces an older one without an offline transition.
Online presence indicates signaling availability, not a guarantee that WebRTC
negotiation will succeed.

## Consequences

- Presence becomes correct immediately after each WebSocket connection.
- Status changes appear live without polling or refreshing the page.
- Connection replacement does not produce a false offline transition.
- The backend WebSocket contract gains one outbound event type.
- Snapshot and delta ordering must be deterministic for a connecting client.
