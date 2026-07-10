# ADR 0007: Browser Session

## Status

Accepted

## Context

The backend already authenticates protected HTTP routes and WebSocket signaling
with a 15-minute JWT access token. Refresh credentials use a 30-day HttpOnly
cookie. The frontend needs to restore a session and identify the current user
without making the JWT payload part of its application contract or exposing the
refresh token to JavaScript.

## Decision

The existing JWT access-token mechanism remains unchanged. Login and refresh
responses will add an explicit `user` object:

```json
{
  "access_token": "...",
  "token_type": "Bearer",
  "expires_in": 900,
  "user": {"id": 1, "username": "ana"}
}
```

The frontend keeps the access token in memory only. It does not create, decode,
or persist JWTs. The refresh token remains exclusively in the HttpOnly cookie.

On startup, the app calls `POST /refresh`. Success restores the authenticated
app; an authentication failure shows the login screen. While active, the app
refreshes shortly before access-token expiry and reconnects authenticated
realtime transports with the new access token when needed. WebSocket
authentication carries the opaque token in `Sec-WebSocket-Protocol`, never in a
URL.

## Consequences

- Reloading does not expose credentials through web storage.
- Frontend identity does not depend on JWT claim decoding.
- Login and refresh gain a backward-compatible response field.
- WebSocket reconnection must use the latest in-memory access token.
- A lost or invalid refresh cookie returns the user to authentication.
