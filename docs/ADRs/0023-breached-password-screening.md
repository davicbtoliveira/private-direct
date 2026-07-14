# ADR 0023: Breached Password Screening

## Status

Proposed

## Context

The existing password policy checks only length. Registration should reject
passwords known to have appeared in public breach corpora without disclosing
the complete password or making a self-hosted instance depend completely on an
external service.

## Decision

Registration and password changes will query the free HIBP Pwned Passwords
range endpoint using its k-anonymity protocol:

- Compute the query SHA-1 digest only for this check and never persist it.
- Send only the first five hexadecimal digest characters.
- Request padded responses with `Add-Padding: true`.
- Compare returned suffixes locally and discard unrelated response data.
- Perform the lookup only after the complete password is submitted, never for
  each typed character.
- Reject a password when its complete digest appears in the returned range.
- Continue using the application's password KDF for authentication storage;
  SHA-1 is not used as a password-storage hash.

If HIBP is unavailable or returns an unusable response, registration continues
under the local password policy and the user sees a warning that breach
screening could not be completed. The warning must not expose password data.

## Consequences

- HIBP receives an instance/client IP address and a five-character hash prefix,
  but not the password or complete digest.
- A network or service failure weakens one registration check but does not make
  a self-hosted instance unavailable.
- Tests require a fake range service; automated tests must not call HIBP.
- Operators should be able to identify service failures without password,
  digest, prefix, or returned-suffix logging.

## Supersedes

Once accepted and implemented, this extends ADR 0011. Password and recovery-key
credentials remain separate as decided by ADR 0022.
