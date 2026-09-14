# Remediation Phase 6 — Architecture boundaries & grocery feature

- **Branch:** `audit-review-phase6` (cut from latest `main` after the Phase 5 PR is approved)
- **Theme:** Architecture boundaries & grocery feature — restore the one-schema-per-module rule (or
  document its exceptions), decide `userprefs` ownership, move business logic out of the BFF, untangle
  `recipeimport`'s sibling-domain coupling, fix the platform → domain import, and finally implement (or
  remove) grocery-list generation with a dimensionally sound nutrition summary.
- **Source reports:** `audit/phase-1-architecture.md`, `audit/phase-3-domains.md`,
  `audit/phase-2-bff.md`, `audit/summary.md` (top-15 #10; theme "Domain boundaries are documented but
  not enforced")
- **Prerequisite:** Phase 4's `UnitOfWork` — grocery generation is a cross-domain write that must run in
  one transaction.

## Findings in scope

| ID | Severity | One-line summary | Target file(s) |
|---|---|---|---|
| A1-04 | High | `generateGroceryList` creates an empty list; aggregation was deferred to "the BFF" and never implemented | `internal/grocery/service.go:227-231`; `internal/bff/resolver_grocery.go:86-110` |
| A1-01 | High | Cross-domain SQL joins into `mealplan.*` / `recipe.*` violate the documented one-schema-per-module rule | `internal/recipe/queries.sql:129-150`; `internal/analytics/queries.sql:66-100` |
| A3-11 | Medium | `grocery.Generate` is a stub: creates an empty list; no layer implements aggregation (domain-report view of A1-04) | `internal/grocery/service.go:224-231` |
| A1-05 | Medium | `userprefs` owns no schema; its tables sit in `inventory`/`wine`/`recipe` and `inventory` also writes `inventory.user_item` | `migrations/0006_create_userprefs.up.sql`; `internal/userprefs/queries.sql`; `internal/inventory/queries.sql:165-167` |
| A1-07 | Medium | `recipeimport` imports sibling domains `recipe` + `inventory`, concrete OCR/Ollama clients, and `platform/config` | `internal/recipeimport/service.go:13-21`; `internal/recipeimport/catalog.go:11-14` |
| A1-09 | Medium | Domain logic (nutrient-type auto-creation, inbox file writes) implemented in BFF | `internal/bff/nutrition_ocr.go:20-77`; `internal/bff/recipe_scan.go:19-84` |
| A1-12 | Medium | Platform package imports domain `identity` | `internal/platform/testenv/testenv.go:129` |
| A2-06 | Medium | `Nutrition` multiplies basis-less `food_nutrient.amount` by quantities in arbitrary units; totals dimensionally meaningless | `internal/bff/resolver_mealplan.go:194-232`; `inventory.food_nutrient` schema |

**Roadmap corrections / additions.** All IDs supplied in the roadmap exist with the severities shown.
**A3-11 (Medium)** was not assigned to any phase in the roadmap; it is the domain-report counterpart of
A1-04 and is added here so both close with the grocery-generation change.

## Remediation steps

1. **A1-04 / A3-11 — implement grocery-list generation (or remove the mutation).**
   1. Decide explicitly (record in the PR): implement, or mark `generateGroceryList` `@deprecated` /
      remove it until implemented. The steps below assume "implement".
   2. Implement in the BFF (the only layer allowed to read `mealplan` + `recipe` + `userprefs`) inside
      one `UnitOfWork`: load the plan's slots, expand recipes via `recipe.ListRecipeItemsByRecipes`,
      aggregate by `(item_id, unit_id)`, subtract on-hand stock from `userprefs.ListUserItems`, then
      bulk-insert through a new `grocery.AddGroceryListItems(ctx, []GroceryListItem)`.
   3. Delete the empty-list stub in `grocery/service.go:224-231` or make it return
      `domainerr.ErrNotImplemented` if step 1 chose "remove".
   4. Add an integration test: plan with two recipes sharing an ingredient → one aggregated grocery line
      with the summed quantity minus pantry stock.
2. **A1-01 — no cross-domain SQL.**
   1. `recipe/queries.sql:129-150` (recency/rating scoring): expose
      `mealplan.LastPlannedDates(ctx, userID, recipeIDs) map[int64]time.Time` and
      `recipe.ListRatedAtLeast(ctx, userID, minRating)`; combine and score in the BFF (~10 lines of
      arithmetic).
   2. `analytics/queries.sql:66-100` (`IngredientOverlapScores`): either (a) accept `analytics` as a
      read-model that may denormalise from other domains and **document the exception** in
      `docs/adr/ADR-001` (co-located Low A1-18), or (b) publish `RecipeCreated`/`MealSlotAdded` events
      into an `analytics`-owned projection so the Jaccard query stays inside `analytics.*`.
   3. Add a guard test that parses each `internal/<d>/queries.sql`, extracts `schema.table` tokens and
      fails on any schema other than the package's own (allow-list for the documented exception). This
      turns the doc rule into CI.
3. **A1-05 — decide `userprefs` ownership.**
   1. Either create a `userprefs` schema and migrate `user_item`, `user_bottle`,
      `user_recipe_preference` into it (cross-schema FKs are fine), or explicitly re-assign ownership
      (`user_item` → `inventory`, `user_bottle` → `wine`, `user_recipe_preference` → `recipe`) and
      dissolve `userprefs` into per-domain "per-user" query files.
   2. Whichever is chosen, `inventory.DeleteItem` must go through the owning service or rely on `ON
      DELETE CASCADE` (aligns with the Phase 4 A3-07 decision); update the guard test allow-list from
      step 2.3.
4. **A1-09 — move business logic out of the BFF.**
   1. Move `processNutritionPhoto` into `inventory.Service.ApplyNutritionLabel(ctx, itemID, parsed
      []nutritionparse.Nutrient, by)` (Phase 2 already removed the reference-data creation; this step
      relocates what remains).
   2. Move the inbox file write from `recipe_scan.go:19-84` into
      `recipeimport.Service.Submit(ctx, mediaType, bytes, by)` so cleanup lives with the transaction;
      the BFF keeps only decode / size check / auth.
5. **A1-07 — narrow `recipeimport`'s dependencies.**
   1. Either (a) acknowledge it as an application service and move it to `internal/app/recipeimport`
      (or under `bff/`), or (b) keep it a domain but depend on locally defined narrow interfaces
      (`OCRExtractor`, `Structurer`, `RecipeWriter`, `CatalogReader`) and accept plain config values
      rather than `*config.Config`.
   2. Document the choice in `docs/adr/` (co-located A1-18).
6. **A1-12 — platform must not import a domain.**
   1. Move `testenv` to `internal/testutil` (outside `platform`), or have it seed users with raw SQL /
      accept a `UserSeeder` func from the caller. Update all `integration_test.go` imports.
7. **A2-06 — dimensionally sound nutrition totals.**
   1. Define the basis explicitly: add `per_quantity NUMERIC` + `per_unit_id` to
      `inventory.food_nutrient` (migration), or standardise on per-100 g and require item net weight.
   2. Do unit conversion in the `inventory`/`mealplan` domain and move the aggregation from
      `resolver_mealplan.go:194-232` into `mealplan.Service.NutritionSummary(...)` so it can be
      unit-tested with fixtures. Return `null`/a warning for items whose units cannot be converted rather
      than summing incompatible quantities.
   3. Update `schema.graphqls` documentation for `nutrition` to state the basis.

## Co-located Low policy

Fix a Low finding only if it lives in code already being changed for a High/Medium finding in this
phase; never edit code solely to fix a Low. Lows expected to be co-located here: A1-18 (ADRs — written
as the record of the A1-01/A1-05/A1-07 decisions), A1-16 (monolithic `Config` — only the `Import`
sub-struct if A1-07 option (b) is chosen), A2-20 (orphan inbox file — if not already closed in Phase 5,
closes with the A1-09 move), A4-12 (one container per test — only if `testenv` is being relocated
anyway and a `TestMain` per package is cheap to add).

## Verification

- `go build ./...` passes (after `sqlc generate` and any migration; commit regenerated code).
- `go test ./...` passes, including the new cross-schema guard test and grocery-generation test.
- `golangci-lint run ./...` and `go vet ./...` report no issues.
- Manual: `go list -deps ./internal/platform/... | grep internal/identity` returns nothing.
- Manual: `grep -n "mealplan\.\|recipe\." internal/recipe/queries.sql internal/analytics/queries.sql`
  shows only the package's own schema (or only the documented analytics exception).
- Manual (GraphQL): `generateGroceryList(mealPlanId)` for a plan containing two recipes that share
  "flour" returns a list with one flour line whose quantity equals the sum minus pantry stock; with the
  pantry fully stocked the line is omitted.
- Manual: `mealPlan.nutrition` for a plan whose items have incompatible units returns the documented
  fallback rather than a summed number.

## Closing instruction

Open a PR from `audit-review-phase6` into `main` summarising the changes above, then **stop**. Do not
begin Phase 7 until this PR has been reviewed and approved.
