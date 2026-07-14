# ADR 0022: E2EE Persistent Message History

## Status

Proposed

## Context

The MVP keeps a tab-local transcript and sends messages only through WebRTC.
Users now require Telegram-like durable history across browser sessions, server
restarts, and devices while preserving end-to-end confidentiality.

A browser-only application cannot keep cryptographic identity independently of
browser storage on every supported operating system. Cross-browser recovery
therefore requires an explicit user-held recovery secret rather than a native
component.

## Decision

Private Direct will provide persistent history for one-to-one text
conversations with these properties:

- Message content is always end-to-end encrypted. The server stores ciphertext
  and required routing metadata, but never plaintext message or recovery keys.
- WebRTC remains the preferred live delivery path when available. The server is
  an encrypted mailbox and canonical sync source, including offline delivery.
- Stable message IDs deduplicate delivery through WebRTC and server sync.
- New browsers recover account keys and complete history using a high-entropy
  recovery key held by the user.
- The database stores a wrapped account master key, KDF salt and parameters,
  and optionally a derived verifier. It never stores the recovery key.
- Loss of every authorized device and the recovery key permanently loses access
  to history.
- Recovery restores complete history. A party possessing both a database copy
  and the recovery key can decrypt that history; this is an accepted tradeoff.
- Account identity keys use trust on first use. An unexpected contact identity
  key change blocks messaging until the user confirms it. Authorized device
  additions do not change account identity.
- Device revocation prevents future sync and rotates keys protecting future
  messages. It cannot reliably erase content already downloaded by the revoked
  device.
- Read/unread state synchronizes between a user's devices as encrypted events.
- Users may delete a message locally or for both participants at any time.
  Delete-for-both produces a signed tombstone, removes active server ciphertext,
  and is applied by offline devices when they next synchronize.
- Operator backups may retain deleted ciphertext until their configured expiry;
  product and operator documentation must disclose this limitation.
- The first release covers text messages and one-to-one conversations only.
- The server assigns a monotonic sequence within each conversation; this is the
  canonical display order. Clients generate UUIDs used for idempotency and
  deduplication across HTTP and WebRTC.
- Sync uses a global per-user event cursor plus per-conversation sequences.
  HTTP performs durable paginated sync; WebSocket sends invalidations only.
- History loads in pages of 50 messages and is cached as encrypted IndexedDB
  data. Clearing browser storage requires recovery and a server resync.
- Delivery states are `queued`, `sending`, `sent`, `delivered`, and
  `not-delivered`. Queued messages upload automatically after reconnection;
  retry from `not-delivered` is explicit and preserves the UUID.
- A conversation has a default per-user ciphertext quota of 100 MiB, configurable
  or disableable by the operator. Exhaustion blocks persistence and never
  deletes history automatically.
- Any conversation participant may delete any message for both participants.
- Accounts support up to ten authorized devices. Devices do not expire
  automatically and are revoked manually.
- Password plus recovery key are both required to recover history in a new
  browser. Recovery uses a 24-word English BIP-39 phrase and supports rotation.
- Password reset is an operator action and never grants access to encrypted
  history without a recovery key or authorized device.
- Account deletion removes active ciphertext and key material. Deleted
  usernames remain reserved to prevent identity impersonation.
- A browser profile is one E2EE device. Multiple tabs coordinate a single
  realtime leader and share encrypted IndexedDB state.
- Private browsing is allowed with a clear warning about ephemeral storage.
- Normal logout preserves local device authorization and encrypted cache. A
  separate logout-and-remove-device action revokes it and clears local state.
- Search and synchronized drafts are out of scope. Drafts remain encrypted and
  local to a device.
- Incompatible clients cannot exchange messages; plaintext fallback is never
  allowed. Existing tab-local transcripts are not migrated.
- Plaintext remains limited to 4,000 code points and server envelopes to 24 KiB.
  Default message submission rate is 120 per minute per user with burst 30,
  configurable by the operator.
- E2EE events form signed hash chains. Clients reject gaps, regressions, and
  altered links, and authorized devices compare observed heads. Public key
  transparency and complete split-view prevention remain future work.
- Existing users must complete E2EE setup, save and confirm their recovery
  phrase, and register the current device before chat is enabled after upgrade.
- The browser crypto engine will use the production-oriented Matrix Rust SDK
  crypto state machine through its WASM bindings while retaining the existing
  custom Go transport and backend. Matrix federation/protocol compatibility is
  not a product goal.
- E2EE remains labeled beta until an external cryptographic review is complete.

## Consequences

- The product promise changes from strictly peer-to-peer transport to E2EE at
  all times with direct peer delivery when possible.
- The backend learns message routing metadata, timestamps, approximate sizes,
  and traffic patterns even though it cannot read content.
- Key recovery, rotation, device authorization, sync cursors, tombstones, and
  conflict handling become core protocol concerns.
- Cryptographic protocol and library selection require a separate reviewed
  decision. Custom cryptographic primitives are not acceptable.
- Attachments, groups, native clients, and read receipts sent to contacts remain
  out of scope.

## Supersedes

Once accepted and implemented, this decision supersedes the history and
transport constraints in ADR 0001 and ADR 0005. ADR 0004 remains relevant to
the WebRTC fast path but its delivery states must be extended for durable server
acceptance and multi-device delivery.

## Open Questions

- Authentication and recovery-key unlock boundaries
- Exact adaptation boundary around the Matrix crypto state machine
- Device authorization handshake and device-list transparency details
- Sync endpoint schemas and cursor-gap recovery
- Server compaction and backup guidance
- Rate limits for non-message sync events
- External security review scope and remediation gate
