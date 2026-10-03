# LEN-7 Proof of Completion — Generic Ingredient Layer, Phase 1 (Schema + Data)

**Branch:** `ingredient-layer-p1` · **PR:** https://github.com/JRAdams472/LENA2/pull/244 · **CI run:** https://github.com/JRAdams472/LENA2/actions/runs/37147283838

## What shipped

| Area | Deliverable |
|---|---|
| Schema | Migration `0045_item_ingredient_link` — `item.ingredient_id`, `household_item_ingredient` (override), `household_ingredient_item` (usual brand), nullable `item_id` on `recipe_item` + `event_recipe_item`, FKs + indexes |
| Data | Migration `0046_ingredient_dedupe_seed` — normalized unique index on ingredient name + **413 seeded ingredients** with canonical default units (verified live: `SELECT count(*)` → 413) |
| Service | `internal/inventory`: `GetOrCreateIngredient` (lowercase-normalized, race-safe insert), ingredient CRUD/search/count, catalog link + household override + usual-brand CRUD, `ResolveItemIngredient` (override → catalog → none), `MergeIngredients` (single transaction; repoints recipe/mealplan/event/grocery/userprefs refs; demotes colliding item-hint rows; deletes true duplicates; deletes source last) |
| Downstream | nullable `item_id` adapted in `recipe`, `event`, `mealplan`/`grocery` expansion, `recipeembed`, AI tools (`ItemNamer` + `GetIngredientsByIDs`), BFF resolvers (`item` nullable, `itemId`/`ingredientId` inputs) |
| Import review | `itemKind` carried end-to-end (matcher → `review_json` → GraphQL → web); web ingredient chips bind the ingredient (fixes a latent bug where a picked "ingredient" suggestion's id was written to `item_id`) |
| Curation | `cmd/ingredientcurate` — `propose` (LLM → reviewable JSON, zero DB writes) / `apply` (deterministic, idempotent, `-dry-run`); artifact: `docs/ingredient-curation.json` |
| Docs | ADR-004 (cross-schema merge exception), `docs/ingredient-layer-plan.md`, workflow rules in `AGENTS.md` |

## Test evidence

### GitHub CI — run 37147283838 (all green)

| Job | Result | Duration |
|---|---|---|
| go | success | 4m09s |
| web | success | 1m27s |
| mobile | success | 1m05s |
| e2e | success | 3m32s |
| ocr-import | success | 1m21s |
| lint | success | 35s |

### Local verification logs

```
gofmt -l ./cmd ./internal          → 0 files
go vet ./...                       → clean
go build ./...                     → clean
golangci-lint run ./cmd/... ./internal/... → 0 issues
go test -short ./...               → 35 packages ok, 0 failures
go test ./internal/inventory/      → ok 7.500s  (unit + Testcontainers integration)
go test ./internal/recipe/ ./internal/event/ ./internal/grocery/ ./internal/mealplan/
                                   → ok 7.772s / 7.635s / 7.699s / 7.475s
go test ./internal/bff/            → ok 8.836s  (TestBFF_Integration incl. auth/e2e/events)
go test ./internal/ai/... ./internal/app/... ./internal/notifier/ ./internal/analytics/
        ./internal/ocrimport/ ./cmd/...        → all ok (recipeimport 35.8s e2e)
web: npx tsc --noEmit              → clean
web: npx jest                      → 45 suites, 512 tests, 0 failures
```

### Live-stack verification (debug profile)

```
docker compose ... run --rm db-migrate
  45/u item_ingredient_link (111ms)
  46/u ingredient_dedupe_seed (127ms)

psql: ingredient count = 413; recipe_item.item_id is_nullable = YES

ingredientcurate propose (live, qwen2.5:7b-instruct):
  items=54 known_ingredients=413 → 54 mappings, 0 unresolved
ingredientcurate apply -dry-run:
  ingredients_resolved=43 item_links=54 recipe_items_backfilled=67
  event_items_backfilled=0 — rolled back, 0 writes (verified)

Web E2E (Playwright MCP, http://localhost/recipes/pending/5):
  click "butter (ingredient) 95%" chip → Catalog Item = "butter (ingredient)"
  Save Review → review_json items[0]: itemId=275 itemKind=ingredient itemName=butter
  buildRecipe path: ItemKind=="ingredient" → ingredient_id=275, item_id=NULL
```

## Screenshot

`docs/proof/phase1-ingredient-chip.png` — review page with the generic-ingredient
chip bound ("butter (ingredient)") and suggestion chips rendered.

## For your review before merge

1. `docs/ingredient-curation.json` — 54 LLM-proposed links; debatable rows flagged
   in the PR ("pepper" vs black pepper, "spring water" vs "water", "mushroom" for ramen).
   Apply is **not** run — your call after review.
2. Known limitations (in PR body): catalog links grow incrementally via curation;
   multi-ingredient products don't fit single-FK; ingredient-level nutrition deferred;
   merge/admin UI in P2, grocery UX in P3, mobile in P4.
3. Dev DB note: import #5 was flipped to `reviewing` and given demo suggestions to
   capture the screenshot — its data is cosmetic.
