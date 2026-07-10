# ADR 0018: Browser Support and Verification

## Status

Accepted

## Context

Private Direct depends on WebSocket and WebRTC behavior that differs more across
browsers than ordinary HTTP interfaces. Its responsive layout also changes
information architecture between desktop and mobile. A single component-test
suite cannot validate these boundaries.

## Decision

The MVP supports the two most recent stable releases of Chrome and Edge,
Firefox, and Safari. The frontend supports desktop and mobile browser layouts.

Verification includes:

- Go public-boundary tests for every changed HTTP and WebSocket contract
- Vitest and Testing Library for frontend state and component behavior
- a full two-peer Playwright workflow in Chromium
- authentication and layout smoke tests in Firefox and WebKit
- screenshot review at 1440 by 900 and 390 by 844 CSS pixels
- final manual WebRTC verification in Chromium, Firefox, and WebKit engines

Responsive screenshots must show no text overflow, incoherent overlap, clipped
controls, or hidden primary actions. Keyboard focus and reduced-motion behavior
are part of component and browser verification.

## Consequences

- WebRTC logic is exercised at both deterministic unit seams and a real browser
  boundary.
- Cross-engine smoke tests catch API and layout assumptions early.
- Safari support is approximated by Playwright WebKit in automation and still
  needs real-device validation before a production claim beyond MVP.
- Browser support moves forward with stable releases rather than freezing old
  versions.

