# ADR 0019: Interface Language

## Status

Accepted

## Context

The product and repository use an English name and English technical
documentation, while implementation planning occurred in Portuguese. The MVP
needs one consistent interface language without prematurely adding localization
runtime and translation maintenance.

## Decision

All user-visible MVP interface copy is English. The product name remains
`Private Direct`.

User-facing strings are grouped by domain rather than scattered through state
or transport code, making future localization tractable. The MVP does not add
an internationalization library, locale selector, or translated copy.

## Consequences

- Authentication, contacts, conversation states, validation, and error messages
  use one consistent English vocabulary.
- Future localization requires a deliberate extraction into message catalogs.
- Transport error codes remain language-neutral and are mapped to English at
  the UI boundary.

