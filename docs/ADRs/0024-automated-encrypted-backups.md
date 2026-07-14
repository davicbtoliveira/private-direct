# ADR 0024: Automated Encrypted Backups

## Status

Proposed

## Context

Persistent message history makes the SQLite database operationally critical.
Self-hosted operators need consistent automatic backups without giving the
running server access to backup decryption keys. The database contains E2EE
ciphertext but also operational metadata, password hashes, contacts, and
sessions.

## Decision

- Backups target a configurable local directory, normally an external mounted
  volume. S3, WebDAV, and other remote targets are initially out of scope.
- A SQLite online backup creates a consistent snapshot. The implementation must
  not copy live database files directly.
- The snapshot passes `PRAGMA integrity_check` before encryption.
- Backups are encrypted to a required operator-provided public `age` recipient.
  The server never has the corresponding private key.
- Output is written to a temporary file and atomically renamed only after the
  snapshot, integrity check, encryption, and manifest creation succeed.
- A manifest records ciphertext SHA-256, size, creation time, and schema
  version. It contains no account data, private path, or secret material.
- The default schedule is daily at 03:00 in the configured timezone. A first
  backup runs on the first startup after backup configuration. Overlapping runs
  are prohibited.
- Default retention is seven daily and four weekly backups, configurable by the
  operator. Pruning occurs only after a new valid backup is published. Failed or
  invalid files do not count toward retention.
- Restore testing is a manual operator action because it requires the offline
  private key. Restore must use a dedicated command and must never happen
  automatically during normal server startup.
- Backup failure does not stop messaging. Detailed health reports `ok`, `stale`,
  or `failed`, last success, and next scheduled attempt. Failures also produce a
  structured log and metric without exposing paths, hashes, recipients, or
  account data through public health output.
- Operator documentation must explain that backups may retain ciphertext and
  records removed from the active database until retention expires.

## Consequences

- The operator must create, protect, and test an `age` identity separately from
  the server.
- A local destination still needs an external mount or copy policy to survive
  host loss.
- Automated integrity checks establish snapshot consistency but cannot prove a
  successful decrypt-and-restore; operators must test that separately.
- Account passwords remain irreversibly hashed with Argon2id, refresh tokens
  and invites are hashed, message content remains E2EE ciphertext, account
  master keys remain wrapped, and recovery keys are never stored. Operational
  metadata remains plaintext and should be protected with volume encryption.
