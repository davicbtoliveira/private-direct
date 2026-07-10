# ADR 0009: API Namespace

## Status

Accepted

## Context

The production React application and backend share one origin. Existing backend
routes occupy human-facing paths such as `/contacts`, which would collide with
client-side navigation. No production frontend or external API client exists
yet, so this is the lowest-cost point to establish a clear boundary.

## Decision

All product HTTP and WebSocket endpoints will move under `/api`. For example:

- `POST /api/login`
- `GET /api/contacts`
- `GET /api/ws`

`GET /health` remains at the root because it is an infrastructure endpoint.
The MVP will not retain aliases for the old unprefixed routes. Vite will proxy
`/api` to the Go process during development, including WebSocket upgrades.

All other production `GET` paths that do not name an asset will fall back to the
SPA entry point so the frontend owns human-facing navigation.

## Consequences

- API, assets, and client-side routes have unambiguous ownership.
- The backend public tests and existing documentation must update paths.
- The change is intentionally incompatible with the pre-frontend prototype.
- Development proxy configuration has one product-route prefix.

