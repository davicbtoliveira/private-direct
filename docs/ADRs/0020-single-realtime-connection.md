# ADR 0020: Single Realtime Connection Per User

## Status

Accepted for MVP

## Context

The prototype broadcasts signaling events to every WebSocket connection owned
by the target user. If several tabs or devices answer one offer, the initiating
peer receives competing answers and ICE candidates. Correct multi-device
routing needs device identity and targeted signaling, both outside the MVP.

## Decision

The MVP allows one active realtime WebSocket connection per user. A new tab or
device connection replaces the existing connection. Before closing, the old
connection receives:

```json
{"type":"session_replaced"}
```

The replaced client displays `Messaging continued in another tab` and does not
automatically reconnect. Its ordinary HTTP session remains valid.

Replacing a connection does not emit an offline/online presence transition.
Contacts continue to see the user as online throughout the handoff.

Multi-tab and multi-device realtime messaging require a future protocol with
device identity and targeted signaling.

## Consequences

- Each offer has one receiving browser and one answer path.
- Opening a newer tab deliberately disables realtime messaging in the old tab.
- Presence remains stable during connection replacement.
- The presence hub can route to one active client per user.

