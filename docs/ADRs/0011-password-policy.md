# ADR 0011: MVP Password Policy

## Status

Accepted

## Context

The prototype accepts any non-empty password and hashes it with bcrypt. Very
short passwords are weak, while bcrypt rejects inputs beyond 72 bytes. Character
composition rules tend to reduce usability without providing a useful strength
model for this small MVP.

## Decision

Passwords must contain 12 through 72 UTF-8 bytes. The backend enforces both
limits. The registration form mirrors them and asks the user to confirm the
password locally before submission.

There are no uppercase, lowercase, number, or symbol composition requirements.
Authentication failures remain generic and do not disclose whether a username
exists.

Password change and account recovery are out of scope for the MVP.

## Consequences

- Password input respects bcrypt's hard technical limit.
- Users may use long passphrases without arbitrary composition rules.
- Unicode characters consume a variable number of the 72 available bytes; the
  UI must validate encoded byte length, not JavaScript string length.
- Losing a password requires operator intervention until recovery is designed.

