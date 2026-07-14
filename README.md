# Private Direct

Self-hosted, privacy-first direct messaging. The server handles identity,
contact consent, and online presence. Chat messages travel peer-to-peer over
WebRTC data channels and never touch the server.

## Architecture

```
Browser (Alice)                 Browser (Bob)
  React SPA                      React SPA
  WebSocket (presence)           WebSocket (presence)
  WebRTC DataChannel (chat)      WebRTC DataChannel (chat)
        |                            |
        v                            v
       Go backend — single binary, SQLite
       Invite-gated auth, contacts, presence, signaling
```

The Go binary serves the API, the WebSocket endpoint, and the embedded
React SPA from a single origin. Messages are never stored, relayed, or
queued by the server.

## Quick start

```sh
PRIVATE_DIRECT_OPERATOR_TOKEN=change-me \
PRIVATE_DIRECT_JWT_SECRET=change-me-too \
PRIVATE_DIRECT_STUN_URLS=stun:stun.l.google.com:19302 \
go run ./cmd/privatedirect
```

Open `http://127.0.0.1:8080`. Create an invite code, share it, and have
contacts register through the UI.

### Configuration

| Variable | Required | Description |
|---|---|---|
| `PRIVATE_DIRECT_ADDR` | — | Listen address (default `:8080`) |
| `PRIVATE_DIRECT_DB` | — | SQLite path (default `private-direct.db`) |
| `PRIVATE_DIRECT_OPERATOR_TOKEN` | Yes | Bearer token for invite creation |
| `PRIVATE_DIRECT_JWT_SECRET` | Yes | HS256 signing secret for access tokens |
| `PRIVATE_DIRECT_STUN_URLS` | Yes | Comma-separated STUN URLs |
| `PRIVATE_DIRECT_TURN_URLS` | — | Comma-separated TURN URLs |
| `PRIVATE_DIRECT_TURN_USERNAME` | — | TURN credential username |
| `PRIVATE_DIRECT_TURN_CREDENTIAL` | — | TURN credential password |
| `PRIVATE_DIRECT_MESSAGE_QUOTA_BYTES` | — | Per-user ciphertext quota (default 100 MiB) |
| `PRIVATE_DIRECT_MESSAGE_RATE_PER_MINUTE` | — | Sustained message rate (default 120/min) |
| `PRIVATE_DIRECT_MESSAGE_RATE_BURST` | — | Message rate burst (default 30) |

### Production build

```sh
cd web && bun install && bun run build && cd ..
CGO_ENABLED=0 go build -o privatedirect ./cmd/privatedirect
```

The single `privatedirect` binary contains the API, the WebSocket server,
and the built SPA. No Node.js or Bun runtime is needed in production.

### Creating invite codes

```sh
curl -X POST http://127.0.0.1:8080/api/operator/invites \
  -H "Content-Type: application/json" \
  -H "X-Operator-Token: change-me" \
  -d '{"code":"my-invite"}'
```

## Development

### Backend

```sh
go test ./...
go run ./cmd/privatedirect
```

### Frontend

```sh
cd web
bun install
bun run dev      # Vite dev server with /api proxy to :8080
bun run test     # Vitest unit + component tests
bun run typecheck
bun run build    # Production build into dist/
```

The Vite dev server proxies `/api` to `http://127.0.0.1:8080`, so start
the Go backend first, then `bun run dev` for hot-reload frontend work.

### End-to-end tests

```sh
cd web
bun run build         # embeddable SPA must exist
npx playwright test
```

Playwright starts the Go server automatically and runs browser-based
workflow tests in Chromium, Firefox, and WebKit (configurable in
`playwright.config.ts`).

## Project structure

```
cmd/privatedirect/     Go entry point
internal/app/          HTTP, WebSocket, auth, contacts, presence, signaling
web/
  src/                 React + TypeScript SPA source
  dist/                Vite build output (embedded by Go)
  e2e/                 Playwright test specs
  spa.go               Go embed directive
docs/                  ADRs, PRDs, glossary
```
