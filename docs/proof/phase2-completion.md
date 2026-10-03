# Phase 2 Proof of Completion — Backend Adoption (LEN-8)

**Branch:** `ingredient-layer-p2`
**PR:** https://github.com/JRAdams472/LENA2/pull/245
**Parent:** LEN-6 — Generic ingredient layer

## What shipped

### GraphQL schema (internal/bff/schema.graphqls)
- `ingredientId` on `RecipeItemInput`, `AddMealSlotItemInput`, `EventRecipeItemInput`, `AddGroceryItemInput`; `itemId` optional on all four
- `Item.ingredient` (catalog link) and `Item.householdIngredient` (household-override-aware)
- `GroceryListItem.ingredient` + `GroceryListItem.usualBrand`
- **Breaking:** `AddMealSlotItemInput.itemId` relaxed `ID!` → `ID` (flagged in PR body)

### Mutations
| Mutation | Auth | Behavior |
|---|---|---|
| `getOrCreateIngredient` | member | normalized-name dedupe; returns existing row on match |
| `mergeIngredient` | admin | single-transaction cross-schema repoint (ADR-004) |
| `setItemIngredient` | admin | global catalog link; null unlinks |
| `setHouseholdItemIngredient` | member | household override; null clears |
| `checkGroceryItemWithBrand` | member | binds brand, checks off, credits pantry, stores usual |

### Grocery flows
- `aggregateGroceryNeeds` keys needs by ingredient when the source line carries one; first preferred brand rides along as the `itemId` hint
- `householdPantryStock` resolves stocked items override → catalog → unlinked; ingredient needs net against any brand of that ingredient
- `checkCreditItem` picks bound item → usual brand → nil; `ToggleGroceryItemChecked` credits accordingly and refreshes the usual record

### Write paths
- Recipe / meal-slot / event / grocery mutations accept ingredient-only, item-only, or both; neither → `BAD_USER_INPUT`
- Server-side item→ingredient resolution fills `ingredientId` for item-only callers

### Downstream
- Nutrition rollup: `RepresentativeItemForIngredient` (usual → first linked) for ingredient-only lines
- AI pantry tool: resolved ingredient names in pantry rows for brand-agnostic availability matching
- Analytics: `EntityIngredient` events for ingredient-keyed groceries/recipes

### Batch SQLC queries
- `GetItemIngredientsForItems` (override-aware, one round trip)
- `ListUsualItemsForIngredients`

## Test evidence

| Suite | Result |
|---|---|
| `go test -short ./internal/... ./cmd/...` | **35/35 packages pass** |
| `go test` integration (testcontainers, real Postgres) | **14/14 packages pass** |
| `golangci-lint run ./...` | **0 issues** |
| `go vet` / `gofmt` / `go build` | clean |

### New tests this phase
- `TestResolver_Inventory_IngredientLinking` — all four linking mutations: happy paths, set+clear, admin-gating, unauthorized
- `TestResolver_ToggleGroceryItemChecked_UsualBrand` — usual-brand credit + usual refresh
- `TestResolver_ToggleGroceryItemChecked_NoUsualCreditsNothing` — no usual → no pantry write
- `TestResolver_CheckGroceryItemWithBrand` — full bind/check-off/credit/usual flow
- `TestResolver_GenerateGroceryList_IngredientNeeds` — ingredient aggregation + cross-brand stock netting
- `TestIntegrationItemIngredientBatchResolution` — batch resolution semantics vs real Postgres

## Live deployment smoke test (docker compose debug stack)

```
mutation { getOrCreateIngredient(input: {name: "Smoke Test Corn"}) { id name } }
→ {"id":"457","name":"smoke test corn"}                     # normalized

mutation { setItemIngredient(itemId: "1", ingredientId: "457") } → true
{ item(id: "1") { ingredient { name } householdIngredient { name } } }
→ ingredient: "smoke test corn", householdIngredient: "smoke test corn"  # catalog fall-through

mutation { setHouseholdItemIngredient(itemId: "1", ingredientId: "458") } → true
→ ingredient: "smoke test corn", householdIngredient: "smoke test jasmine rice"  # override wins

cleanup: overrides cleared, links removed, smoke ingredients deleted — verified via SQL
```

Introspection confirms all five mutations and both Item/GroceryListItem fields live on the rebuilt api container.

## Notes for reviewers
- The `is_from_recipe` slot override now suppresses recipe lines by **both** keys (ingredient and item) so a brand-only override still covers an ingredient-keyed recipe line naming the same item
- `checkGroceryItemWithBrand` runs bind + check-off + pantry credit + usual write inside one unit of work
- Analytics event payloads unchanged for existing item entities; ingredient entities only appear for ingredient-keyed lines
