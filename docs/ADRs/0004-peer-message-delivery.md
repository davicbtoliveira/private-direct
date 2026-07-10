# ADR 0004: Peer Message Delivery

## Status

Accepted

## Context

Calling `RTCDataChannel.send` only queues bytes locally. It does not prove that
the remote application received and processed a message. The UI must not claim
delivery when peer negotiation or transport fails.

## Decision

Peers will use an ordered, reliable WebRTC DataChannel named `chat`. The channel
will carry JSON envelopes.

A text message has this shape:

```json
{
  "type": "message",
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "text": "Meet at 18:00?",
  "sent_at": "2026-07-10T20:00:00.000Z"
}
```

After validating and accepting the message, the receiving client responds:

```json
{
  "type": "ack",
  "message_id": "550e8400-e29b-41d4-a716-446655440000",
  "received_at": "2026-07-10T20:00:00.120Z"
}
```

Message IDs are generated with `crypto.randomUUID()`. Timestamps are informative
client timestamps, not trusted server time. Text is rendered as text, never as
HTML.

The sender shows `sending` until the matching acknowledgement arrives. It shows
`delivered` after acknowledgement. If the channel closes or acknowledgement
does not arrive within the client timeout, it shows `not_delivered` and offers
an explicit retry. Read receipts are out of scope.

## Consequences

- Delivery status means the remote application processed the envelope.
- Acknowledgements and messages remain peer-to-peer; the server does not observe
  either payload.
- Retries must preserve or deliberately replace message identity so duplicate
  display can be prevented.
- Delivery state disappears with the non-persistent session transcript.

