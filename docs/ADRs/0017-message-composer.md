# ADR 0017: Message Composer

## Status

Accepted

## Context

The MVP carries text directly over a WebRTC DataChannel. Input behavior must be
predictable, bound payload size, distinguish online negotiation from offline
delivery, and never suggest that whitespace or an offline message was queued.

## Decision

Messages are plain text with a maximum of 4,000 Unicode characters. The composer
supports multiple lines. `Enter` sends and `Shift+Enter` inserts a line break.
Markdown and rich-text interpretation are out of scope.

Input containing only whitespace is invalid. Its send button is visibly muted,
semantically disabled, and unavailable to keyboard or pointer activation.

When presence is online but the direct channel is still negotiating, sending
may add the message to the in-memory transcript as `sending`. The client sends
it when the channel opens; failed negotiation changes it to `not_delivered`.
This pending state is memory-only.

When presence is offline, sending is disabled and no message is queued. Draft
text may remain in the visible composer until the user changes it or leaves the
tab.

## Consequences

- Keyboard and button behavior remain equivalent.
- Message rendering can stay literal and avoid an HTML/Markdown attack surface.
- The client needs a bounded per-peer pending-send collection during active
  negotiation.
- Online negotiation buffering does not become offline delivery.

