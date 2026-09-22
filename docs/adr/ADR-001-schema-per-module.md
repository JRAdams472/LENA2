# ADR-001: One schema per module — rule and documented exceptions

## Status

Accepted (Phase 6 remediation, audit A1-01 / A1-05 / A1-18)

## Context

The architecture doc states each module owns exactly one database schema and
must not reach into a sibling domain's schema. The audit found three
violations:

- `recipe` SQL joined `mealplan.meal_slot`/`mealplan.meal_plan` to score
  recency for recommendations.
- `analytics.IngredientOverlapScores` joins `recipe.recipe_item` and
  `mealplan.*` for a Jaccard-similarity read model.
- `userprefs` owned no schema at all: its tables sat inside `inventory`,
  `wine`, and `recipe`.
- `recipeimport`'s tables live in the `recipe` schema while the service now
  lives under `internal/app/`.

## Decision

1. **The rule stands and is enforced by CI.** `internal/testutil`'s schema
   guard test parses every `internal/<d>/queries.sql` and fails on any
   schema-qualified reference outside an explicit allow-list.
2. **Recency scoring is composed at the application layer.** The recipe
   domain exposes `ListRatedAtLeast` and the mealplan domain exposes
   `LastPlannedDates`; the BFF combines them in Go. No cross-schema SQL.
3. **`analytics` is a documented read-model exception.** It denormalises
   from `recipe.*` and `mealplan.*` for reporting queries that would be
   expensive to reconstruct through domain APIs. This is the only such
   exception; anything else must go through domain services.
4. **`userprefs` owns a real `userprefs` schema.** `user_item`,
   `user_bottle`, and `user_recipe_preference` were migrated there
   (migration 0026); cross-schema foreign keys are allowed, and
   `inventory.user_item`'s `ON DELETE CASCADE` replaces the former manual
   cross-domain delete in `inventory.DeleteItem`.
5. **`recipeimport` keeps its tables in `recipe`** (`recipe.recipe_import`)
   since the service is an application service, not a domain (see ADR-002).

## Consequences

- New domains must add themselves to the guard test's allow-list with their
  own schema only; a foreign-schema reference fails CI.
- If analytics queries grow beyond the two documented join targets, prefer
  publishing domain events into an `analytics`-owned projection instead of
  widening the exception.
