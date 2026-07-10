# ADR 0010: Usernames

## Status

Accepted

## Context

The prototype accepts any non-empty, case-sensitive username. Exact lookup is a
privacy feature, but inconsistent case and unconstrained whitespace make an
identifier difficult to type, share, validate, and display reliably.

## Decision

An MVP username is a canonical lowercase handle with 3 to 32 ASCII characters.
It must match:

```text
[a-z0-9][a-z0-9._-]{2,31}
```

Registration, login, and exact lookup trim surrounding whitespace and normalize
input to lowercase before use. The backend enforces the rule and remains the
source of truth; the frontend mirrors it for immediate feedback.

The UI displays handles with a leading `@`, but the stored and transported value
does not include `@`. Display names, biographies, and avatars are out of scope
for the MVP.

## Consequences

- Usernames are unambiguous to type and compare.
- Existing prototype data with invalid usernames needs manual migration; no
  production users exist yet.
- Internationalized usernames and separate display names require future design.
- Exact lookup remains explicit and does not become directory search.

