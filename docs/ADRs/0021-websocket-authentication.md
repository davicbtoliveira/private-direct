# ADR 0021: WebSocket Authentication

## Status

Accepted for MVP

## Context

The browser WebSocket API cannot set an `Authorization` header. The prototype
therefore accepts the JWT access token in the WebSocket query string, but URLs
are commonly retained in browser history, access logs, and reverse-proxy logs.
The frontend must keep the existing access token opaque and in memory without
placing it in a URL.

## Decision

The browser opens `/api/ws` with two values in the standard
`Sec-WebSocket-Protocol` request header: the application protocol
`private-direct` and an access-token value derived from the current in-memory
JWT. The Go server extracts and authenticates the token during the upgrade and
selects only `private-direct` as the negotiated response protocol.

Query-string access tokens are removed. The frontend never decodes, stores, or
logs the token. Reconnection uses the latest refreshed in-memory token.

This is the MVP transport. A future protocol may replace it with a short-lived,
single-use WebSocket ticket if infrastructure compatibility requires one.

## Consequences

- WebSocket URLs contain no credential.
- Access-token secrecy still depends on TLS and careful header logging.
- Browser and backend tests must cover missing, malformed, and invalid
  subprotocol credentials.
- Generic non-browser WebSocket clients must use the same protocol contract.

