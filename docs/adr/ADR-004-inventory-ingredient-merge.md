# ADR-004: inventory owns the cross-schema ingredient merge

## Status

Accepted (ingredient-layer Phase 1)

## Context

The generic-ingredient layer (migrations 0045–0046) makes
`inventory.ingredient` the canonical entity recipes, meal plans, events,
grocery lists, and user preferences reference. Merging a duplicate
ingredient into a canonical one must repoint foreign keys in **every**
schema that can hold an `ingredient_id` — `recipe`, `mealplan`, `event`,
`grocery`, and `userprefs` — inside a single transaction. A merge that
updates some schemas but not others leaves dangling references or loses
true duplicates.

The alternatives were worse:

- **Per-schema merge queries orchestrated by the inventory service** —
  splits one atomic operation across five packages' queriers. The
  transaction would still be shared (every package's `sqlc.Querier` accepts
  the same `pgx.Tx`), so atomicity survives, but the merge logic — the
  ordering, the collision handling (demote item-hint rows, delete true
  duplicates) — would be scattered across five `queries.sql` files that
  must be kept consistent forever. Nobody reading `recipe/queries.sql`
  would know its `MergeRecipeItemIngredients` exists solely for
  inventory's merge.
- **Cross-schema orchestration from an app service** — moves the same
  problem up a layer without removing it.

## Decision

`internal/inventory` owns `MergeIngredients` and its queries may reference
`recipe`, `mealplan`, `event`, `grocery`, and `userprefs` schemas — the
exact set of schemas that hold `ingredient_id` references. The exception is
recorded in the schema-guard allow-list.

The scope is deliberately narrow: only the merge-time repoint/delete
queries and the resolution queries that must join `userprefs` household
overrides cross the boundary. Day-to-day per-domain ingredient usage
(recipe items, meal-slot items, grocery lines) stays in its owning schema
— a recipe still reads its items via `recipe/queries.sql`.

## Consequences

- `allowedSchemas["inventory"]` in
  `internal/testutil/schema_guard_test.go` gains the five referenced
  schemas, with a comment pointing here.
- Adding a new `ingredient_id` column to any table obligates the author to
  extend the merge queries in `internal/inventory/queries.sql` — the
  integration test `TestIntegrationMergeIngredients` exercises every
  repoint path and will catch a missed reference.
