# ADR 0016: MVP Registration Session

## Status

Accepted for MVP

## Context

Registration currently creates a user but does not issue a session. Sending a
new user back through the same credential form is needless friction, while
changing registration and authentication into one backend transaction expands
the server contract beyond what the MVP needs.

## Decision

For the MVP, after registration succeeds, the frontend immediately calls login
with the username and password already held by the active form. On login
success, it clears the form and opens the authenticated application.

If registration succeeds but login fails, the frontend opens login with only
the username prefilled and clearly states that the account was created. It does
not retain, repopulate, log, or persist the password.

This is an MVP orchestration choice. A future API may issue a session directly
from registration through a separately reviewed contract.

## Consequences

- New users enter the product without retyping credentials.
- Registration and login remain independently testable backend operations.
- Partial success must be represented explicitly because the invite is already
  consumed and the account already exists.
- The frontend briefly holds the submitted password only until the login request
  finishes.

