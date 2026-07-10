# ADR 0015: Frontend Toolchain

## Status

Accepted

## Context

The repository has no existing JavaScript conventions. The frontend needs
routing, icons, exact visual control, unit and component tests, and a real
two-browser WebRTC test without introducing state or styling frameworks before
their complexity is justified.

## Decision

The frontend uses:

- React and TypeScript built by Vite
- Bun for dependency installation, lockfile, and frontend scripts
- React Router for browser navigation
- Lucide React for interface icons
- CSS Modules plus a small global token stylesheet
- React context and reducers for session and realtime state
- a small typed `fetch` client for server state
- Vitest and Testing Library for unit and component tests
- Playwright for browser workflow tests

Redux, TanStack Query, Tailwind CSS, and component kits are not part of the MVP.
They may be introduced later only for demonstrated complexity. Bun is a build
and development tool; the production application still runs as static assets
inside the Go process and has no Bun runtime dependency.

## Consequences

- The frontend dependency and abstraction surface stays small.
- Realtime state transitions remain explicit and testable through reducers.
- Visual implementation is not constrained by a generic component kit.
- The repository gains `bun.lock` as its frontend lockfile.
- Production builds must run the Bun frontend build before compiling embedded
  Go assets.

