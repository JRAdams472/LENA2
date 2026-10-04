# LEN-21 — Allergy Backend: Proof of Completion

Phase 2 of LEN-16 (allergy tracking and risk detection). Backend resolvers,
services, and the batch-loaded conflict engine — everything the web/mobile
clients render in P3.

## What shipped

### Data access (sqlc + domain services)

- `internal/inventory/queries.sql` — `allergen` registry CRUD +
  `ListAllergens`, `GetAllergensByIDs`; `ingredient_allergen` /
  `item_allergen` flag upsert, delete, per-entity list, and batch
  `...ByIngredients` / `...ByItems` queries.
- `internal/userprefs/queries.sql` — `user_allergen` upsert/delete,
  per-member list, batch `ListUserAllergensByUsers`.
- `inventory.Service` — `Allergen` + `EntityAllergen` domain types,
  `AllergenFlagContains`/`AllergenFlagMayContain` kinds, full CRUD and
  batch reads; `domainerr` not-found semantics preserved (`:one` +
  `RETURNING` for update, `:execrows` for delete).
- `userprefs.Service` — `UserAllergen`, `MemberAllergyKindAllergy` /
  `MemberAllergyKindDietary`, kind validation in `SetUserAllergen`.
- Regenerated `sqlc` code for both packages (models for the new tables
  land in every package's `models.go` — sqlc emits the whole schema),
  plus `mockgen` mocks for `Querier` and the BFF service interfaces.

### Conflict engine (`internal/bff/resolver_allergy.go`)

- `allergyContext` batch-loads — inside the existing
  `itemChildren`/`recipeChildren`/`groceryChildren` preload paths — the
  flag maps, registry rows, household member roster, and member records.
- Entity allergen set = union of (a) the line's explicit ingredient
  flags, (b) the bound item's product-level flags, (c) the item's
  override-resolved ingredient flags. `contains` outranks `may_contain`
  when both kinds flag the same allergen.
- **Resolution order**: household `item → ingredient` override wins over
  the catalog link — catalog flags do not leak through a remap.
- Warnings = entity set ∩ member records; each warning carries the
  member, the allergen, `memberKind` (allergy|dietary), and `entityKind`
  (contains|may_contain), sorted by member name then allergen name.
- `householdID = 0` or nil member services (admin catalog browsing)
  yields flags with no warnings — no member queries issued.
- A resolver built with neither a preload nor a lazy `allergySource`
  **errors** on `allergens`/`allergyWarnings` rather than silently
  reporting zero conflicts. An empty flag set means "no allergen data" —
  never "known safe".

### GraphQL surface

- Fields: `allergens: [AllergenFlag!]!` + `allergyWarnings: [AllergyWarning!]!`
  on `Item`, `Ingredient`, `Recipe`, `MealSlot`, `EventRecipe`,
  `GroceryListItem` (plus `EventRecipeItem`/`RecipeItem` as applicable).
- Queries: `allergens` (registry), `myAllergies` (caller's records).
- Mutations: `setMyAllergy(allergenId, kind, on)` self-service member
  records; admin `createAllergen`, `updateAllergen`,
  `setIngredientAllergen`, `setItemAllergen` (kind null clears).
- `AllergyWarning` shape: `member`, `allergen`, `memberKind`,
  `entityKind`.

### Lazy-load parity

Resolvers constructed outside the preload paths (mutation responses,
nested child resolvers) carry `as *allergySource` and rebuild a
single-entity context on demand — warnings never degrade to empty just
because the parent didn't preload.

## Tests

11 new tests in `internal/bff/resolver_allergy_test.go`:

- Override wins over catalog link; catalog fallback when no override;
  unlinked item → product flags only.
- `lineSet` unions explicit ingredient + item + resolved ingredient.
- `contains` precedence over `may_contain`.
- Warning shape: member label, allergen, both kinds, sort order.
- No records → empty warnings, flags still render (no "safe" claim).
- `loadAllergyContext` batch coverage (flag maps, roster, records,
  registry) + no-household admin scope (zero member queries).
- Missing context → error, not silent zero-conflict.
- Recipe union across ingredient-keyed, item-bound, and manual lines.
- Meal slot empty → empty warnings, no error.
- Grocery line merges line ingredient + item + resolved ingredient.
- `myAllergies` returns caller's records only.
- `setMyAllergy` write / clear / invalid-kind propagation.

Six existing mock tests gained stubs for the two new batch queries.

## Verification

- `go test ./internal/bff` — all green.
- `go test ./...` — green (`cmd/lena`, `event`, `recipe` showed
  testcontainer-timing flakes on the first parallel run; pass clean
  with `-count=1`).
- `go build ./...`, `go vet`, `gofmt -l` — clean.
- `TestSchemaParses` — GraphQL schema binds to resolvers.

## Scope notes / follow-ups for P3

- No `setMemberAllergy` mutation — members manage their own records via
  `myAllergies`/`setMyAllergy`. If P3 wants a parent-edits-kid flow, add
  a household-member-admin mutation then.
- `allergyWarnings` labels conflicts; client copy must still distinguish
  "no flags" from "verified safe" — that is P3's job on the presentation
  layer.
