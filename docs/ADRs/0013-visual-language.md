# ADR 0013: Visual Language

## Status

Accepted

## Context

Private Direct needs a distinctive operational interface for repeated private
conversation. Generic dark SaaS gradients, cyberpunk terminal styling, and
decorative network diagrams would obscure the product's simple promise. The
interface should feel like a small direct-connection panel while remaining
human and readable.

## Decision

The MVP uses a dark neutral palette with amber detail:

- Carbon: `#10120F`
- Panel: `#191D18`
- Text: `#ECEFE7`
- Amber: `#F2B84B`
- Online: `#48B9A7`
- Fault: `#EF6A5B`

Borders derive from translucent text rather than adding another dominant hue.
There are no gradients. Corners use at most a 6-pixel radius.

Chivo is the restrained brand and heading face. Atkinson Hyperlegible is the UI
and conversation face. IBM Plex Mono is reserved for handles, timestamps, and
connection status. Font assets are bundled with the self-hosted frontend.

The signature element is one functional direct-line trace connecting the active
contact context to the composer. Amber communicates an active direct channel,
a broken or muted trace communicates negotiation or disconnection, and fault
coral communicates failure. Online teal is reserved for presence. A single
short connection animation is allowed; reduced-motion users receive an
equivalent static state.

The rest of the interface remains quiet: exact dividers, restrained density,
message-first whitespace, no decorative diagrams, and no marketing hero.

## Consequences

- Connection state becomes a memorable part of the product identity without
  becoming decoration.
- Semantic teal and coral prevent the amber theme from becoming one-note.
- Dark-theme contrast, keyboard focus, text overflow, and mobile layout require
  screenshot and accessibility verification.
- Typography adds local assets to the production bundle.

