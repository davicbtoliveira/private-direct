# ADR 0012: Frontend Information Architecture

## Status

Accepted

## Context

Private Direct is a repeated-use messaging tool, not a marketing site. Its core
workflow must keep contacts, live availability, connection state, and the active
conversation close together without turning secondary contact-management tasks
into permanent visual clutter.

## Decision

The first screen is the usable application. Unauthenticated routes are
`/login` and `/register`; there is no landing page.

On desktop, the authenticated workspace has a fixed 280-pixel contact rail and
a flexible conversation area. The rail contains the current user, add-contact
action, incoming-request action and count, and the accepted-contact list with
live presence. Search and incoming requests open as focused sheets rather than
dedicated pages.

Conversation routes use `/chat/:username`. On mobile, the contact rail and
conversation become separate full-width views. The conversation header provides
a standard back action to the contact list. Contact-management sheets remain
available from the list view.

## Consequences

- Desktop users can scan contacts while keeping conversation context visible.
- Mobile avoids compressing a two-pane tool into unusable narrow columns.
- Secondary workflows remain reachable without becoming permanent panels.
- The router must support SPA fallback and direct conversation URLs.
- Empty, connecting, offline, failed, and active conversation states need
  first-class layouts in the conversation area.

