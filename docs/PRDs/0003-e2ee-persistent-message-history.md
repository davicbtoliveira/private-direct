# PRD 0003: E2EE Persistent Message History

Issue label: `ready-for-agent`

## Problem Statement

Private Direct users lose every conversation transcript after refreshing or
closing a tab. They cannot read history from another browser, receive messages
while offline, or recover conversations after a server restart. This prevents
the self-hosted app from providing a durable, Telegram-like messaging
experience.

Users require persistent one-to-one text history without giving the server or
operator access to message plaintext or recovery secrets. Direct peer delivery
must remain available when possible, but it cannot be the only delivery path.

## Solution

Add an end-to-end encrypted mailbox and multi-device synchronization model.
Clients encrypt all message and state events before upload. The Go backend
stores, orders, quotas, and synchronizes ciphertext in durable SQLite storage.
WebRTC remains the preferred live path; HTTP sync is canonical and supplies
offline delivery. WebSocket invalidations tell clients when to sync.

Each account receives an E2EE identity and a user-held 24-word recovery phrase.
Authorized browser profiles recover complete history across supported operating
systems. The server stores only wrapped account keys and never stores the
recovery phrase. Users can revoke devices, rotate recovery credentials, delete
messages for themselves or both participants, and remove their account.

The release also screens passwords through the free HIBP Pwned Passwords range
API and adds scheduled, operator-key-encrypted SQLite backups.

## User Stories

1. As a user, I want conversation history after refresh, so that navigation does not destroy context.
2. As a user, I want history after closing and reopening my browser, so that conversations are durable.
3. As a user, I want history after a server restart, so that self-hosting maintenance does not lose messages.
4. As a user, I want history in a newly authorized browser, so that I can use multiple devices.
5. As a user, I want messages sent while my contact is offline, so that simultaneous presence is unnecessary.
6. As a user, I want every message end-to-end encrypted, so that the server cannot read it.
7. As a user, I want live messages sent directly over WebRTC when possible, so that peer delivery remains part of the product.
8. As a user, I want server mailbox fallback, so that WebRTC failure does not lose durable messages.
9. As a user, I want duplicate WebRTC and mailbox delivery deduplicated, so that each message appears once.
10. As a user, I want canonical message ordering, so that all my devices show the same transcript.
11. As a user, I want recent messages loaded quickly, so that large histories do not block conversation opening.
12. As a user, I want older messages loaded while scrolling upward, so that complete history remains accessible.
13. As a user, I want unread state synchronized across my devices, so that reading once clears it everywhere.
14. As a user, I want delivery states that distinguish queued, sending, sent, delivered, and failed messages, so that status claims are precise.
15. As a user, I want offline outgoing messages queued locally, so that they send after connectivity returns.
16. As a user, I want automatic retry for queued messages, so that temporary connectivity failures need no action.
17. As a user, I want explicit retry for not-delivered messages, so that permanent failures stay under my control.
18. As a user, I want retries deduplicated, so that retrying never creates duplicate transcript entries.
19. As a user, I want to delete a message only for myself, so that I can clean my own history.
20. As a conversation participant, I want to delete any message for both participants, so that shared history can be removed.
21. As a user, I want deletions applied to devices that were offline, so that they converge after sync.
22. As a user, I want clear disclosure that operator backups can temporarily retain deleted ciphertext, so that erasure limits are honest.
23. As a new user, I want a recovery phrase during E2EE setup, so that browser storage loss does not destroy history.
24. As a user, I want recovery to require password plus recovery phrase, so that either credential alone is insufficient.
25. As a user, I want the recovery phrase never stored by the server, so that a DB breach does not expose it.
26. As a user, I want complete history restored by my recovery phrase, so that a new browser can reproduce my conversations.
27. As a user, I want clear warning that losing all devices and recovery phrase loses history permanently, so that I protect it.
28. As a user, I want to rotate my recovery phrase, so that a suspected disclosure can be contained for current server state.
29. As a user, I want to confirm a new recovery phrase before the old one is invalidated, so that rotation cannot lock me out accidentally.
30. As a user, I want a list of authorized devices, so that I can recognize account access.
31. As a user, I want no more than ten authorized devices, so that unexpected fan-out and abuse are bounded.
32. As a user, I want devices to remain authorized until manual revocation, so that occasional use does not force recovery.
33. As a user, I want to revoke a device, so that it cannot receive future history.
34. As a user, I want revocation to rotate future message protection, so that removed devices cannot decrypt later messages.
35. As a user, I want honest disclosure that revocation cannot erase already downloaded history, so that guarantees remain accurate.
36. As a user, I want an unexpected contact identity change to block messaging, so that silent key substitution is not accepted.
37. As a user, I want a safety number or QR verification path, so that I can authenticate a contact identity.
38. As a user, I want normal logout to preserve my authorized device, so that returning does not require recovery.
39. As a user, I want “sign out and remove this device,” so that I can revoke and wipe local state explicitly.
40. As a private-browsing user, I want a clear ephemeral-storage warning, so that I understand recovery and orphan-device consequences.
41. As a user with multiple tabs, I want them treated as one device, so that they share state without consuming device slots.
42. As a user, I want drafts encrypted locally, so that unsent text is not exposed or synchronized.
43. As an existing user, I want guided E2EE setup after upgrade, so that my account gains identity and recovery safely.
44. As an existing user, I want chat blocked until E2EE setup finishes, so that no plaintext fallback occurs.
45. As a user, I want incompatible contacts told to update, so that protocol failures are understandable.
46. As a user, I want plaintext fallback prohibited, so that version mismatch cannot silently weaken security.
47. As a user, I want password reset separated from message recovery, so that an operator reset cannot decrypt history.
48. As a user, I want operator-assisted password reset, so that I can regain account authentication without surrendering recovery secrets.
49. As a user, I want account deletion, so that active ciphertext, wrapped keys, sessions, and devices are removed.
50. As a former user, I want my deleted username reserved, so that another person cannot impersonate my prior identity.
51. As a registrant, I want known breached passwords rejected, so that common compromised credentials are not accepted.
52. As a registrant, I want registration to continue with a warning when HIBP is unavailable, so that an external outage does not block a self-hosted instance.
53. As a registrant, I want my complete password and complete query digest kept away from HIBP, so that breach screening minimizes disclosure.
54. As an operator, I want ciphertext storage quotas, so that persistent history cannot exhaust disk silently.
55. As an operator, I want quota exhaustion to block new persistence rather than delete old history, so that retention is predictable.
56. As an operator, I want configurable message rate limits, so that abuse cannot overwhelm the instance.
57. As an operator, I want message acknowledgement only after durable commit, so that `sent` survives process or power failure.
58. As an operator, I want automated consistent backups, so that database loss can be recovered.
59. As an operator, I want backups encrypted to my public `age` recipient, so that the running server lacks backup-decryption authority.
60. As an operator, I want daily backup scheduling and retention, so that recovery points are maintained automatically.
61. As an operator, I want backup integrity validation, so that corrupt snapshots are never published as successful.
62. As an operator, I want backup failure reported as degraded health without stopping chat, so that availability and recoverability are separately visible.
63. As an operator, I want a documented restore command and test process, so that backup recovery can be verified safely.
64. As a security reviewer, I want signed event hash chains, so that gaps, alteration, replay, and rollback become detectable by clients.
65. As a security reviewer, I want E2EE labeled beta until external review completes, so that security claims match evidence.
66. As a developer, I want deterministic crypto test vectors and fake external services, so that tests are repeatable and never depend on HIBP.

## Implementation Decisions

- Preserve one-to-one text conversations. Groups, attachments, native apps, and contact-visible read receipts remain excluded.
- Replace tab-local transcript authority with an encrypted mailbox. WebRTC is the preferred live fast path; HTTP is the canonical durable path.
- Use the Matrix Rust SDK crypto state machine through browser WASM. Keep the custom Go backend and transport; Matrix federation and wire compatibility are not product goals. Do not invent cryptographic primitives.
- Store only ciphertext message/state envelopes, routing metadata, canonical sequences, cursors, quotas, device records, wrapped account keys, and required public key material.
- Give each account a stable identity key. Use trust on first use, safety-number/QR verification, and blocking on unexpected identity change. Authorized device addition does not replace account identity.
- Generate 24 English BIP-39 recovery words from 256 bits of client randomness. Require confirmation. Never send or store the phrase.
- Derive a key-encryption key locally and store only the wrapped account master key plus salt and KDF parameters. Recovery needs account password and recovery phrase.
- Allow recovery-key rotation. A prior DB backup plus prior phrase may still reveal history present in that backup; disclose this limitation.
- Register one authorized device per browser profile, maximum ten. Share its IndexedDB-backed state across tabs and elect one realtime leader through browser coordination.
- Revocation blocks future sync and triggers future-key rotation. It cannot guarantee deletion from a previously authorized device.
- Use client UUID message IDs for idempotency. Assign a monotonic sequence per conversation after commit for canonical ordering.
- Use a monotonic global per-user sync cursor across encrypted messages, tombstones, read state, device events, and key changes.
- Implement HTTP batch upload/download with cursor and conversation pagination. Use the existing authenticated WebSocket only for `mailbox_changed` invalidation. Always sync from the cursor after reconnect.
- Load 50 recent messages initially, then fetch older pages. Store local ciphertext/cache state in IndexedDB; do not retain plaintext after logout.
- Use delivery states `queued`, `sending`, `sent`, `delivered`, and `not-delivered`. `sent` requires server commit. `delivered` requires a recipient device decryption acknowledgement. Preserve UUID during retries.
- Synchronize own-account read/unread marker as an encrypted event. Do not send read receipts to contacts in this release.
- Model delete-for-both as a signed tombstone. Either participant can delete any shared message at any time. Apply tombstones after offline sync and remove active server ciphertext.
- Form signed hash chains for E2EE events. Reject altered links, gaps, and regressions; compare heads across devices. Public key transparency and complete split-view protection remain deferred.
- Keep drafts encrypted and local. Do not add history search.
- Keep the 4,000-code-point composer limit. Reject server envelopes above 24 KiB.
- Default to 100 MiB ciphertext quota per user, configurable or unlimited. Never auto-delete on quota exhaustion.
- Default to 120 message submissions per minute per user with burst 30. Treat `429` as queued/backoff, not permanent failure.
- Configure SQLite WAL, `synchronous=FULL`, foreign keys, and busy timeout. Return `sent` only after commit. Require persistent container volume configuration.
- Version the E2EE protocol. Refuse incompatible clients and never fall back to plaintext. Do not migrate existing in-memory transcripts.
- Block existing users from chat after upgrade until E2EE identity, recovery phrase confirmation, and first-device registration finish.
- Keep normal logout and remove-device logout separate. Allow private browsing with an explicit warning.
- Password changes require the current password. Forgotten-password reset is operator-assisted and does not unlock history. Alert authorized devices to reset events.
- Account deletion removes active ciphertext, wrapped keys, devices, and sessions; sends deletion state to contacts; permanently reserves the username.
- Continue using irreversible Argon2id password hashes with unique salts. Hash refresh tokens and invites. Never encrypt recoverable password plaintext.
- Query the free HIBP Pwned Passwords range endpoint from the backend after complete password submission. Send only the five-character SHA-1 prefix, request `Add-Padding: true`, compare locally, discard unrelated suffixes, and never log query material. Reject matches. On timeout/failure, continue local validation and return a visible warning.
- Add configurable automated backups to a local/mounted directory. Create snapshots using the SQLite online backup mechanism, validate with `PRAGMA integrity_check`, encrypt to a required public `age` recipient, write atomically, and publish a manifest containing ciphertext SHA-256, size, time, and schema version.
- Run backup once after first configuration, then daily at 03:00 in configured timezone. Prevent overlaps. Retain seven daily and four weekly valid backups; prune only after a new valid backup.
- Keep restore manual because the server lacks the private `age` identity. Report backup `ok`, `stale`, or `failed` through detailed health, structured logs, and metrics without leaking sensitive values. Backup failure does not stop messaging.
- Treat E2EE as beta until threat model, protocol ADR, deterministic vectors, remediation, and external cryptographic review are complete.

## Testing Decisions

- Test externally observable behavior, not internal table layouts, crypto helper calls, or React implementation details.
- Primary seam: a two-user, multi-browser-context Playwright workflow against the real Go server and temporary SQLite DB. It covers E2EE onboarding, contact verification, send, WebRTC/mailbox deduplication, offline delivery, restart recovery, pagination, multi-device recovery, unread sync, deletion, revocation, incompatible-client blocking, logout variants, and account deletion.
- Extend existing Playwright two-peer patterns and existing responsive Chromium/Firefox/WebKit coverage. Run real-device Safari verification before claiming iOS support.
- Backend public-boundary tests supplement the primary seam for deterministic HTTP/WebSocket contracts: idempotent upload, authorization, canonical ordering, cursor gaps, pagination, quotas, rate limits, tombstones, restart durability, health degradation, and account/device lifecycle.
- Frontend Vitest tests supplement browser workflows for crypto-state adapters, delivery-state transitions, IndexedDB recovery, leader election, duplicate invalidations, queued retry/backoff, private-mode warnings, and no-plaintext-fallback behavior.
- Use deterministic test vectors around the Matrix crypto adapter. Test ciphertext interoperability and state restoration without asserting private implementation state.
- Use a fake HIBP range server. Cover breached, clean, padded, timeout, malformed, and unavailable responses. Automated tests never call HIBP.
- Use temporary SQLite databases and restart the in-process server to prove committed history survives. Include power-loss-oriented transaction tests where practical.
- Test backup through its operator-visible boundary: consistent snapshot while writes occur, integrity failure, age encryption, atomic publication, manifest, schedule, overlap prevention, retention, degraded health, and restore command with a test identity.
- Add property/fuzz tests for untrusted envelope parsing, cursor processing, signed hash-chain validation, duplicate events, malformed ciphertext, and size boundaries.
- Security tests prove plaintext messages, passwords, recovery phrases, complete HIBP digests, and private backup keys never appear in DB rows, API responses, logs, manifests, or browser persistence.
- Accessibility and UI tests cover recovery confirmation, safety-number warning/block, delivery labels, retry action, quota failure, backup-independent user messaging, private browsing warning, and keyboard/screen-reader behavior.
- External cryptographic review is a release gate for removing the beta label; passing functional tests alone is insufficient.

## Out of Scope

- Group conversations.
- Images, video, audio, files, thumbnails, and object storage.
- Native mobile or desktop applications.
- A native cross-browser key agent.
- Federation between instances.
- Matrix server adoption or Matrix client interoperability.
- Contact-visible read receipts.
- Full-text history search.
- Synchronized drafts.
- S3, WebDAV, or other remote backup targets.
- Automatic restore.
- Public key transparency log and complete split-view prevention.
- Migration of tab-local pre-upgrade transcripts.
- Plaintext compatibility fallback.
- Automated device expiration.
- Guaranteed deletion from revoked/offline devices or expired operator backups.
- Removal of the E2EE beta label before external review.

## Further Notes

- This PRD follows proposed ADRs 0022, 0023, and 0024. ADRs 0001 and 0005 are superseded only after ADR 0022 is accepted and implemented.
- The server still learns participants, traffic timing, approximate size, and routing metadata. E2EE does not hide these properties.
- Recovery of complete history intentionally trades away strict historical forward secrecy: DB ciphertext plus the recovery phrase can recover that history.
- Backups require operator volume planning. A local backup on the same lost disk is not disaster recovery.
- External security review must examine the Matrix crypto adaptation, account identity/recovery construction, device authorization, signed event chains, WebRTC/mailbox deduplication, and deletion semantics.
