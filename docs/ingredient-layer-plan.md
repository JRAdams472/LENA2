# Generic Ingredient Layer — Recipes & Lists on Unbranded Items, Inventory Stays Branded

> **Status: SHIPPED** (2026-10-04). All five phases merged to `main`:
>
> | Phase | Ticket | PR | Merge commit |
> |---|---|---|---|
> | P1 Schema + data | LEN-7 | #244 | `c5f28a3` |
> | P2 Backend adoption | LEN-8 | #245 | `9069480` |
> | P3 Web | LEN-9 | #246 | `82cb32e` |
> | P4 Mobile | LEN-10 | #247 | `427451e` |
> | P5 Close-out | LEN-11 | — | (this PR) |
>
> Deviations from the plan as written:
> - Admin ingredients screen shipped at `/inventory/ingredients` (catalog-admin convention), not `/admin/ingredients`.
> - Curation artifact (`docs/ingredient-curation.json`) was generated and dry-run-reviewed in P1 but **not applied** — the 106k-item catalog remains unlinked except links made via scan/edit/check-off flows. Apply is a deliberate user decision.
> - Branches shipped as `ingredient-layer-p<N>`; from P5 the repo convention switched to ticket-prefixed names (`LEN-11-close-out`).

Adopt the scaffolded-but-empty `inventory.ingredient` abstraction as the primary reference for recipes, meal plans, grocery lists, and imports; link branded items to ingredients via a global FK plus household overrides; seed the ingredient catalog via an LLM-assisted curation pass; rework backend, web, and mobile UX so recipes stop requiring brand matches while pantry stock/check-off flows resolve ingredient→brand.

## Objective

Stop forcing every recipe line to match a branded catalog item. Recipes, meal slots, grocery list lines, and recipe-import reviews reference **generic ingredients** ("corn"). Inventory (`userprefs.household_item`) and the UPC catalog (`inventory.item`) stay **branded** ("Green Giant Corn"). A two-level link — global `item.ingredient_id` plus per-household override — resolves "which brand satisfies this ingredient" for stock rollups, check-off crediting, and availability checks.

## Decisions locked in (user answers)

- **Link model**: global `inventory.item.ingredient_id` FK **plus** a household-level override table (household can remap an item to a different ingredient).
- **recipe_item**: `ingredient_id` becomes the primary ref; `item_id` stays as nullable **"preferred brand"** hint.
- **Seeding**: **LLM curation pass** — Ollama proposes ingredient names/mappings for the existing catalog-linked recipe items and a starter ingredient list; admin reviews before applying. No AI inside migrations — the curation emits a deterministic artifact that is reviewed, then applied.
- **Ingredient creation**: free-create for all users (like manual grocery items) **with a dupe check** — normalized unique index + fuzzy-match suggestions + idempotent create-or-get + admin merge tool so we don't get "47 entries for carrots".
- **Grocery check-off**: usual-brand auto-credit; if no usual brand recorded yet, fall back to prompting for a brand (first-time purchase path).
- **Item linking UX**: ingredient picker on item create/edit + review prompts; unlinked items still work as inventory, they just can't satisfy ingredient rollups.
- **Scope**: everything — schema, Go services, web, and mobile.
- **Branches**: dedicated series `ingredient-layer-p1`…`p5`, one branch + PR per phase off `main` (not `uat-fixes-3`).

## Current-state findings (verified)

- `inventory.ingredient` exists (name, category_id, default_unit_id, is_active) but has **0 rows**; `inventory.item` has 106,065 (seeded from `migrations/seed/Grocery_UPC_Database.csv`).
- `recipe.recipe_item`: surrogate PK `recipe_item_id` already exists; `item_id NOT NULL`, `ingredient_id` nullable FK — currently 67 rows, all branded, 0 ingredient refs.
- GraphQL already exposes: `Ingredient`, `IngredientPage`, `ingredients(page,pageSize,search)` (public), `createIngredient/updateIngredient/deleteIngredient` (`@admin`-gated), `ingredient` fields on `RecipeItem`/`GroceryListItem`/`MealSlotItem`, `ingredientId` on grocery/aisle/route inputs.
- Import matcher (`internal/ocrimport/catalog.go`) already produces `kind:"ingredient"` suggestions with a +0.05 ingredient boost — but `RecipeImportReviewItem` has no `ingredientId` field, so they can never be saved. `buildRecipe` writes only `ItemID`.
- `AddMealSlotItemInput.itemId` and `EventRecipeItemInput.itemId` are `ID!` required; DB columns already have `ingredient_id`.
- Grocery check-off (`resolver_grocery.go` ~L405) credits pantry via `AdjustHouseholdItemQuantity` — **only when `ItemID != nil`**; ingredient lines get no stock credit today.
- `GenerateGroceryList` aggregates `groceryNeed` keyed by `itemID` and subtracts `householdItemStock` (item-level).
- `NormalizeName`/`singularize` helpers live in `internal/ocrimport/similarity.go` (reusable for dupe checks).
- `analytics.EntityIngredient` already exists. `recipeembed` service rebuilds recipe embedding text from ingredient names on save/sweep.
- `grocery.aisle_assignment`/`item_route` already support `ingredient_id` — store routing needs no schema change.
- Nutrition (`food_nutrient`, `setItemNutrients`) is item-level — stays branded; recipe/plan nutrition rollups must resolve ingredient→a representative item.
- Mobile: `grocery_list_screen.dart` already renders `ingredient{name}`; `edit_recipe_screen.dart` uses `itemId`; `scan_screen.dart` does UPC→item only.

## Data model — migrations (next numbers: 0045+)

### 0045_item_ingredient_link

- `ALTER TABLE inventory.item ADD COLUMN ingredient_id BIGINT REFERENCES inventory.ingredient(ingredient_id)` (nullable; index it).
- `CREATE TABLE userprefs.household_item_ingredient (household_id, item_id, ingredient_id, created_by/at, updated_by/at, PRIMARY KEY (household_id, item_id))` — household override for "this item is actually X for us". Resolution order everywhere: **override → item.ingredient_id**.
- `CREATE TABLE userprefs.household_ingredient_item (household_id, ingredient_id, item_id, last_used_at, PRIMARY KEY (household_id, ingredient_id))` — "usual brand" per ingredient, written on brand-picked check-offs.

### 0046_ingredient_uniqueness

- `CREATE UNIQUE INDEX idx_ingredient_name_norm ON inventory.ingredient (lower(regexp_replace(name, '\s+', ' ', 'g')))` — hard dupe guarantee (server additionally singularizes for display suggestions).
- No approval/status column — free-create per decision; moderation is the merge tool.

### 0047_recipe_item_ingredient (two-stage)

- `ALTER TABLE recipe.recipe_item ALTER COLUMN item_id DROP NOT NULL`.
- `ALTER TABLE recipe.recipe_item ALTER COLUMN ingredient_id SET NOT NULL` — **only after the backfill is applied and verified** (see seeding); may need to be a second migration (0048) applied post-curation.
- `CREATE UNIQUE INDEX idx_recipe_item_recipe_ingredient ON recipe.recipe_item (recipe_id, ingredient_id)` — preserves the old "one line per thing per recipe" semantics (same ingredient in two sections was already impossible under the old `(recipe_id,item_id)` PK).
- Same treatment for `mealplan.meal_slot_item` (`item_id` nullable→ingredient primary) and `event.event_recipe_item` (already has `ingredient_id`; make `item_id` optional in code).
- Down migrations reverse everything (restore NOT NULL only works pre-backfill — document that 0047.down must run before the curation is applied, or carry a reverse mapping).

## Curation / seeding (LLM pass — decision)

1. **Starter ingredient list**: LLM-generate a curated list of ~150–300 common generic ingredients (deterministic output committed as `migrations/seed/0003_ingredients.sql` or an INSERT migration). Reviewed in the PR diff — the PR *is* the review.
2. **Curation tool**: new `cmd/ingredientcurate` (reuses `internal/platform/ollamaclient`) — reads the 67 recipe_items' items, proposes `{item_id, item_name → ingredient_name, new_or_existing}`, writes a review artifact (JSON or SQL file). Admin edits/approves the artifact, then the tool applies it: insert missing ingredients, set `item.ingredient_id`, set `recipe_item.ingredient_id`, keep `item_id` as preferred brand.
3. Alternative cheaper path for the 67 rows since volume is tiny: deterministic `NormalizeName`+singularize auto-map into the seeded list first, then LLM only for unmatched names. Tool prints unmatched rows for manual assignment.
4. After remap: run `recipeembed` sweep to rebuild embeddings from ingredient names.

## Backend changes

### Inventory service (`internal/inventory`)

- `Item`/`CreateItemInput`/`UpdateItemInput`: add `ingredientId` (global link).
- New: `SetHouseholdItemIngredient` / `ClearHouseholdItemIngredient` (override), `ResolveItemIngredient(ctx, householdID, itemID)` (override→global), `ResolveIngredientItems(ctx, householdID, ingredientID)` (all items whose resolved ingredient matches — for stock rollup), `GetOrCreateIngredient(ctx, name, ...)` (normalized dedupe — returns existing on conflict), `MergeIngredients(ctx, fromID, intoID)` admin tool (repoint all FKs: recipe_item, meal_slot_item, grocery_list_item, item.ingredient_id, overrides, usuals, aisle/route, event items; then deactivate source).

### GraphQL schema (`internal/bff/schema.graphqls`)

- `type RecipeItem { item: Item (was Item!), ingredient: Ingredient! (after 0047; nullable during transition) }`.
- `RecipeItemInput`, `AddMealSlotItemInput`, `EventRecipeItemInput`: `itemId: ID` (was `ID!`) + `ingredientId: ID`; resolver validates ≥1 present; if only `itemId` given, resolve ingredient via link (error if unlinked and no ingredientId — message tells user to link or pick ingredient).
- `RecipeImportReviewItem`/`RecipeImportReviewItemInput`: add `ingredientId`, `ingredientName` — suggestions of `kind:"ingredient"` finally have somewhere to land.
- `type Item`: add `ingredient: Ingredient`, `householdIngredient: Ingredient` (override-aware view).
- `type GroceryListItem`: add `usualBrand: Item` (from `household_ingredient_item`).
- Mutations: `createIngredient` — add a **non-@admin** sibling `getOrCreateIngredient(name, categoryId, defaultUnit)` for free-create with built-in dedupe; keep admin CRUD; add `mergeIngredient(fromId, intoId) @admin`; `setItemIngredient(itemId, ingredientId)`; `setHouseholdItemIngredient(itemId, ingredientId)`; `checkGroceryItemWithBrand(listItemId, itemId)` (brand-pick check-off: sets line's `item_id`, credits stock, records usual).
- `AddGroceryItemInput` already supports ingredientId — UI just starts using it.

### Recipe / import (`internal/recipe`, `internal/app/recipeimport`, `internal/ocrimport`)

- `recipe.RecipeItem`: IngredientID becomes the meaningful field; `AddRecipeItem` writes it; ListRecipeItems joins ingredient name (resolver exposes both).
- `buildRecipe`: parse `ingredientId` (and itemId when brand chosen); item-only rows resolve ingredient server-side via link.
- Review save (`resolver_recipe_import.go`): persist `ingredientId`/`ingredientName` into `review_json` items.
- Matcher already boosts ingredients — once the table is seeded, suggestions just work. Auto-accept rules: keep item auto-accept only for exact UPC-ish hits; ingredient suggestions become the common accepted path.

### Grocery (`internal/bff/resolver_grocery.go`, `internal/grocery`)

- `aggregateGroceryNeeds`: key needs by **ingredientID** (fall back to itemID when a recipe line is brand-only).
- `householdItemStock`: roll pantry stock up to ingredient level via `ResolveItemIngredient` (override-aware); subtract from ingredient needs.
- `groceryNeedLines`: emit `ingredient_id` lines (with `item_id` when a preferred brand exists on the recipe line).
- Check-off: keep existing item-credit; for ingredient lines — if a usual brand exists, credit it automatically; expose `usualBrand` so the client can prefill/confirm. New `checkGroceryItemWithBrand` handles the first-time pick (line gets `item_id` + ingredient, stock credited, `household_ingredient_item` upsert).
- `assignItemToAisle`/route learning already take `ingredientId` — generated lines should route by ingredient.
- `addItemToCurrentGroceryList(itemId)` — add `ingredientId` variant or a new mutation for "add corn to list".

### AI tools (`internal/ai/tools`)

- `recipes.go`/`cellar.go` `ItemNamer` usage: recipe ingredient names come from the **ingredient** (fall back to preferred-brand item name).
- Pantry/stock-aware tools (`pantry.go`, cocktail `inStockOnly`): resolve household items→ingredients for availability checks.

### Analytics / embeddings / nutrition

- Events: use `EntityIngredient` when the line is ingredient-keyed; keep `EntityItem` for branded lines (`groceryEntityEvent` already branches).
- `recipeembed`: embed text uses ingredient names — sweep after backfill.
- Nutrition rollups (plan/nutrition aggregation in `expandPlanLines` consumers): resolve ingredient→representative item (preferred-brand → household usual → globally most-linked item for that ingredient → skip with a note). Document that ingredient-level nutrition is a future table if needed.

## Web changes (`clients/web`)

- **Recipe editor** (`app/recipes/[id]/page.tsx`): picker switches `api.searchItems` → `api.searchIngredients` (new client fn over existing `ingredients(search)` query); shows preferred-brand chip when `item` set; "link a brand" secondary affordance.
- **Import review** (`app/recipes/pending/[id]/page.tsx`): catalog field binds `ingredientId` (+ ingredient suggestions become the primary chips); brand still selectable via item suggestions when it matters. Update `SaveReview` input mapping + tests.
- **Grocery list**: render ingredient name + usual-brand subtext; first-time check of an unlinked-usual line opens a small brand picker (search items or scan later); `checkGroceryItemWithBrand` mutation wired in; "Add item" flow searches ingredients first.
- **Item create/edit**: `ingredientId` picker with auto-suggestion (`getOrCreateIngredient` for inline create).
- **New admin screen**: `/admin/ingredients` — list/search/create/edit/**merge** (dedupe tool — this is the "47 carrots" safety valve).
- `api.ts` type updates (`RecipeItem.item` nullable, `ingredient`, `usualBrand`, input shapes) + jest updates.

## Mobile (`clients/mobile`)

- `edit_recipe_screen.dart`: `_itemId` → ingredient picker (same `ingredients(search)` query).
- `grocery_list_screen.dart`: already reads `ingredient{name}`; add usual-brand display + brand-pick flow on first check-off.
- `scan_screen.dart`: after item create, prompt to link ingredient (and for UPC hits, show resolved ingredient); feeds `household_ingredient_item` on purchase-adjacent actions where applicable.
- Any screens reading `RecipeItem.item.name` switch to `ingredient.name` (with item fallback).

## Phases — dedicated branch series

**Linear: [LEN-6](https://linear.app/jradams472-lenna/issue/LEN-6)** (parent) →
[LEN-7](https://linear.app/jradams472-lenna/issue/LEN-7) P1 ·
[LEN-8](https://linear.app/jradams472-lenna/issue/LEN-8) P2 ·
[LEN-9](https://linear.app/jradams472-lenna/issue/LEN-9) P3 ·
[LEN-10](https://linear.app/jradams472-lenna/issue/LEN-10) P4 ·
[LEN-11](https://linear.app/jradams472-lenna/issue/LEN-11) P5

Per `AGENTS.md` convention (`mobile-redesign-p0`–`p5` precedent), each phase gets its own branch off `main` (not `uat-fixes-3`), PR'd and merged in order before the next starts. Proof of completion (logs, Playwright screenshots, CI results) is posted to each phase's ticket before merge approval.

- **`ingredient-layer-p1` — Schema + data** [LEN-7]: migrations 0045/0046 (nullable transition), starter ingredient seed, curation tool run + reviewed artifact, backfill `item.ingredient_id` + `recipe_item.ingredient_id`, embedding sweep, then 0047/0048 NOT NULL + unique index. Service-layer methods (`Resolve*`, `GetOrCreate`, `Merge`, usual-brand) land here with tests.
- **`ingredient-layer-p2` — Backend adoption** (branches off p1 post-merge): schema changes, recipe/import write paths, grocery gen + check-off + stock rollup, meal slot/event inputs, AI tool naming, analytics branching. All GraphQL breaking changes land here.
- **`ingredient-layer-p3` — Web** (off p2): pickers, review binding, grocery brand UX, item linking, admin ingredients page, tests.
- **`ingredient-layer-p4` — Mobile** (off p3): recipe editor, grocery check-off brand flow, scan link prompt.
- **`ingredient-layer-p5` — Close-out** (off p4): docs (`deployment.md`/`newfeatures.md`/README), wiki update, UAT pass on the import→recipe→plan→list→check→pantry loop.

Each PR summarizes its phase; merge only after build/tests/lint pass per repo rules. `uat-fixes-3` stays free for unrelated UAT work.

## Verification

- [ ] `go test ./internal/...` — new service tests for resolve/merge/get-or-create; update bff integration tests to exercise ingredient-first recipes.
- [ ] Live check on this UAT DB: all 67 recipe_items get `ingredient_id`; no recipe loses a line; preferred brands preserved.
- [ ] UAT loop: import a recipe → rows resolve to ingredients → approve → add to meal plan → generate list (ingredient lines, pantry stock subtracted via brand rollup) → first check-off prompts brand → stock credited + usual remembered → second check-off auto-credits.
- [ ] Dupe check: create "Carrots" then "carrots"/"carrot" → single row; merge tool repoints FKs.
- [ ] Household override: remap an item for household B only; rollup reflects it for B not A.
- [ ] Web + mobile smoke: no console errors; scan→link→stock path on mobile.
- [ ] CI green (go/lint/web/e2e/mobile/ocr-import); `golangci-lint` clean.

## Risks / caveats to flag

- **Breaking GraphQL changes**: `RecipeItem.item` `Item!`→`Item`, three input `itemId` `ID!`→`ID` — all in-repo clients updated together; any external consumer breaks (none known — flag in PR).
- **106k items are unlinked**: linking is incremental via scan/edit/review prompts; stock rollups only see linked brands — expected bootstrap cost.
- **Unusual data**: items that are genuinely multi-ingredient (trail mix) don't fit single-FK — they stay unlinked or map to the dominant ingredient; document.
- **Nutrition**: ingredient lines have no nutrients — rollups use representative items and can be lossy; ingredient-level nutrition is a possible follow-up.
- **Down-migration ordering**: 0047.down must run before ingredient backfill exists, or needs its own mapping table — keep curation artifact around for reversal.
- **Migration 0045 must ship before/with backfill** — verify live DB has `inventory.ingredient` unpopulated assumptions in tests (integration tests use testcontainers with all migrations — they'll need the seed list or create fixtures).
- **Analytics continuity**: entity switches to ingredient for new events — historical item signals still count.
- **Store router**: generated lines switching to ingredient_id resets learned-order granularity (ingredient-level learning already exists — acceptable, arguably better).

## Out of scope

- Per-item nutrition for ingredients; multi-ingredient items (trail mix); household-level *ingredient* renames/aliases (names are global); OCR/AI pipeline changes beyond the curation tool; wine/cellar data model (uses items for pairings, updated only for naming).
