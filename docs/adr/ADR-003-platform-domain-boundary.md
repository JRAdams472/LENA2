# ADR-003: platform must not import domain packages

## Status

Accepted (Phase 6 remediation, audit A1-12 / A1-18)

## Context

`internal/platform/testenv` imported the `identity` domain to seed test
users. Platform is the bottom of the dependency stack (config, dbtx, ocr
client, ollama client); a platform → domain edge inverts the layering.

## Decision

Test helpers that need domain services moved to `internal/testutil`, outside
`platform`. `internal/platform` now contains only infrastructure with no
domain imports, verified by
`go list -deps ./internal/platform/... | grep internal/` returning nothing.

## Consequences

- All integration tests import `internal/testutil` instead of
  `internal/platform/testenv`.
- New shared test helpers belong in `internal/testutil` (or alongside the
  domain they exercise); nothing under `platform/` may import a domain
  package.
