# Phase 5 (LEN-11) — Plan close-out: docs, coverage, audit re-walk, UAT, wiki

**Plan:** `docs/ingredient-layer-plan.md` — all four phases shipped, plan marked shipped.
**Branch:** `LEN-11-close-out` (first branch under the new ticket-prefixed naming rule)

## Phase record

| Phase | Linear | PR | Merge commit | CI |
|---|---|---|---|---|
| P1 Schema + data | LEN-7 | #244 | `c5f28a3` | green |
| P2 Backend adoption | LEN-8 | #245 | `9069480` | green |
| P3 Web | LEN-9 | #246 | `82cb32e` | 6/6 green (run 37159353526) |
| P4 Mobile | LEN-10 | #247 | `427451e` | 6/6 green incl. mobile (run 37174963133) |

Merged phase branches deleted locally and remotely; `main` verified against `origin/main`.

## Documentation sweep

- `docs/ingredient-layer-plan.md` — status header added: shipped, phase→PR table, the `/inventory/ingredients` route deviation recorded
- `docs/newfeatures.md` — ingredient-layer entry appended
- `docs/graphql-schema.md` — `GroceryListItem.ingredient`/`usualBrand`, ingredient fields on `RecipeItemInput`/`AddGroceryItemInput`, `CreateIngredientInput`, `checkGroceryItemWithBrand`, merge + override mutations
- `docs/postgres-data-model.md` — `inventory.ingredient` promoted from "scaffolding" to the live generic-ingredient table; `item.ingredient_id` nullable semantics corrected (NOT NULL deferred to the curation backfill); `userprefs.household_item_ingredient` / `household_ingredient_item` tables documented
- `AGENTS.md` — branch/PR naming rule updated: `LEN-<N>-<slug>` branches, `LEN-<N>:` PR titles (per user direction)
- `clients/mobile/README.md` — grocery/scan screens described as ingredient-aware
- `clients/web/README.md`, `README.md` — reviewed; setup-focused, no stale ingredient claims

## Coverage check

No zero-coverage surfaces across P1–P4:

- Backend: resolver + sqlc-layer tests exercise ingredient queries and `checkGroceryItemWithBrand`
- Web: 527 Jest tests — new brand-picker flow, ingredients admin CRUD/merge, api-layer mutations
- Mobile: 96 tests — `needsBrandPick` predicate, `resolvedIngredientOf`, grocery usual-brand rendering, scan resolution
- e2e: spec updated for the `Ingredient or item` add-row label

## Audit re-walk (`audit/summary.md` — 111 findings: 0 critical / 15 high / 51 med / 45 low)

New ingredient queries checked against previously-corrected patterns — no regressions:

- `SetItemIngredient :execrows` (no silent success)
- `UpsertItemIngredientOverride`, `UpsertUsualItemForIngredient` use `ON CONFLICT` (no check-then-act race)
- Household scoping comes from context, never client input
- Global mutations (`mergeIngredient`, catalog `setItemIngredient`) behind `@admin`

## Full UAT loop (live API + DB verification)

1. **Import → approve** — seeded review-status import, approved via GraphQL; recipe persisted `ingredient.id=181, item=null`
2. **Meal plan → grocery generation** — slot added (dayOfWeek 0–6), list generated; paprika line retained remembered usual brand
3. **Usual-brand check-off → pantry credit** — Frontier paprika pantry `1.00 → 2.00`; `household_ingredient_item` mapping resolved the brand
4. **Dedupe** — `Carrots`/`carrots` normalized to ingredient 70; `carrot` created separate 459 as designed (exact-match index)
5. **Merge** — `mergeIngredient(459→70)`: refs repointed, 459 gone
6. **Household override** — item override made `householdIngredient=carrots` while catalog `ingredient=null`; cleared after
7. **Cleanup** — synthetic grocery line, meal slot, recipe, import row, and override all removed; pantry restored

## Wiki (`LENA2.wiki.git`, pushed as `c17e6db`)

- New **Ingredients** page — ingredient-vs-item model, preferred brands, usual brands, household overrides, admin merge
- Updated: Grocery-Lists (usual-brand section), Inventory-and-Pantry, Recipes, Mobile-Grocery-Lists, Mobile-Recipes-and-Meal-Plans, Mobile-Scanning, _Sidebar, Home index
- Screenshots refreshed on the isolated `lena2shots` stack: all canonical shots re-captured; new `grocery-brand-picker`, `ingredients-admin`, `recipe-detail-ingredients`; mobile emulator shots embedded on the mobile pages
- `tools/wiki-shots/` extended: `ing()` ingredient fixtures + usual-brand check-off in the seeder; `capture.mjs` gains the three new shots and a server-paginated items-page anchor
- Mobile pantry-scanner camera path remains a known follow-up (untestable on the emulator)

## Verification

- `flutter analyze`: clean · `flutter test`: 96 passing · web `tsc`/eslint/Jest: clean (527)
- CI on this PR: docs + tooling only
