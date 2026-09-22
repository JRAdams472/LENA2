# ADR-002: recipeimport is an application service

## Status

Accepted (Phase 6 remediation, audit A1-07 / A1-18)

## Context

`internal/recipeimport` orchestrates an OCR extraction client, an LLM
structuring client, the `recipe` domain writer, and the `inventory` catalog
reader. As a domain package it coupled itself to sibling domains and to
concrete platform clients, violating the domain-dependency rules.

## Decision

The package is an **application service** and lives at
`internal/app/recipeimport`. Application services may compose domain
services and platform clients — that is their job — so the recipe +
inventory + OCR/LLM dependencies are legitimate there. It still depends on
narrow interfaces (`OCR`, `LLM`, `Store`) for testability, and the BFF calls
it through `recipeimport.Service.Submit`/`Approve`.

## Consequences

- `internal/recipeimport` no longer exists; imports, `go:generate`, and CI
  paths point at `internal/app/recipeimport`.
- Its tables remain in the `recipe` schema (`recipe.recipe_import`), which
  is recorded as an allow-list entry in the schema guard test (ADR-001).
