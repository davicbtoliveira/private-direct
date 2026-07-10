# ADR 0005: MVP Session Transcript

## Status

Accepted for MVP

## Context

The server deliberately does not store chat messages. Client-side persistence
would introduce its own privacy, retention, multi-device, and migration choices
that are not defined for the MVP. Users still need to switch between contacts
without immediately losing the current conversation.

## Decision

For the MVP, each browser tab keeps a separate in-memory transcript per contact.
Switching contacts preserves those transcripts while the tab remains alive.
Refreshing, closing the tab, or logging out clears every transcript.

Chat messages and delivery state will not be written to `localStorage`,
`sessionStorage`, IndexedDB, the backend, or another persistence layer. The chat
interface will disclose this behavior once in context.

This is an MVP constraint, not a permanent product prohibition. Persistent
history requires a future decision covering local encryption, retention,
multi-device behavior, backup, and deletion semantics.

## Consequences

- Message content remains absent from server storage.
- A tab can retain useful context while the user moves between contacts.
- A reload is destructive to the transcript and must not be presented as a
  recovery mechanism.
- Future history support requires a new PRD and ADR.

