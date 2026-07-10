# ADR 0002: Frontend Delivery

## Status

Accepted

## Context

Private Direct is a self-hosted application. The MVP frontend needs HTTP API,
refresh-cookie, and WebSocket access to the Go backend. Shipping independent
origins would add CORS, cookie, origin-validation, reverse-proxy, and deployment
configuration to the MVP.

## Decision

The frontend will be a React and TypeScript application built with Vite.

During development, Vite will proxy backend HTTP and WebSocket traffic. In
production, the Go application will serve the built frontend from the same
origin as the API and WebSocket endpoint. The self-hosted release will therefore
remain a single deployable application.

The frontend MVP is a vertical product slice. It may extend the backend's
public HTTP or WebSocket contract when browser clients otherwise cannot present
correct state. Each contract change must be covered at the backend's public
boundary. Backend changes unrelated to a frontend workflow remain out of scope.

## Consequences

- Refresh cookies and WebSocket origin checks remain same-origin.
- Local development requires both the Vite and Go processes.
- The Go build and release process must include the frontend assets.
- Browser-critical API gaps may be fixed as part of frontend work.
- Client-side routes require an SPA fallback without shadowing backend routes.
- A separately deployed frontend is deferred until deployment requirements
  justify its extra configuration.
