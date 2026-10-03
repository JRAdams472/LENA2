# Phase 3 (LEN-9) — Web client — proof of completion

Branch: `ingredient-layer-p3`
Scope: ingredient-first UX across the web client — recipe editor, import review,
grocery list, item create/edit, and a new admin ingredients page.

## What shipped

### API layer (`clients/web/lib/api.ts`, `lib/types.ts`)

- `Item.ingredient` / `Item.householdIngredient`, `RecipeItem.ingredientId`,
  `MealSlotItem.ingredientId`, `GroceryListItem.ingredient` + `usualBrand`,
  `itemID` nullable across recipe/meal-slot/event/grocery inputs.
- New calls: `searchIngredients` (paged), `createIngredient`, `updateIngredient`,
  `deleteIngredient`, `getOrCreateIngredient`, `mergeIngredient`,
  `setItemIngredient`, `setHouseholdItemIngredient`,
  `checkGroceryItemWithBrand`.
- All write paths send `itemId` / `ingredientId` / both; `ingredientId: null`
  preserved explicitly for item-only rows.

### Recipe editor (`app/recipes/[id]/page.tsx`)

- Add-row redesigned ingredient-first: `Ingredient` autocomplete (debounced
  search against 413 seeded ingredients + `Create "<name>"` inline-create via
  `getOrCreateIngredient`) + optional `Preferred brand` item picker.
- Ingredient-only recipe lines supported; item+ingredient bindings preserved
  through add/remove/update mutations (`removeRecipeItem` takes
  `{ itemID?, ingredientID? }`).

### Import review (`app/recipes/pending/[id]/page.tsx`)

- Catalog combobox now searches ingredients alongside items; options are keyed
  `kind:id` so item 5 and ingredient 5 stay distinct, and labels render
  `name (ingredient)` for ingredient options.
- Ingredient suggestion chips (from P1's `itemKind` plumbing) remain the
  primary binding affordance — verified end-to-end in P1.

### Grocery list (`app/grocery-lists/[id]/page.tsx`)

- Rows render ingredient-primary (`ingredient.name` → `itemName` →
  `manualItemName` fallback) with a `usual: <brand item>` caption when the
  household has a usual brand.
- Add row is a free-solo `Ingredient or item` autocomplete — ingredients sort
  first, then branded items; bound picks carry `itemId`/`ingredientId`, free
  text stays a manual name.
- **First-time brand pick**: checking off an ingredient-only line opens
  "Which `<ingredient>` did you buy?", searches branded items, and calls
  `checkGroceryItemWithBrand` — the pick is credited to the line and recorded
  as the household's usual brand.

### Item create/edit (`app/inventory/items/page.tsx`, `CrudDialog.tsx`)

- `CrudDialog` gained an `extraFields` render slot.
- Item dialog shows `Catalog ingredient` (admin-only `setItemIngredient`
  link) and `Household ingredient` (`setHouseholdItemIngredient` override)
  pickers, built on the shared `IngredientAutocomplete`.

### Admin ingredients page (`app/inventory/ingredients/page.tsx`)

- Server-side search + paginated list (name, category, default unit, active).
- Create / edit / delete (soft-delete via `isActive`), and `Merge into…` —
  repoints all references of the source ingredient to a picked survivor.
- Wired into `AdminLayout` under Inventory with `adminOnly: true`; child-level
  `adminOnly` filtering was fixed so admin-only children (Ingredients, Pending
  Reviews, Users, Pending Items) no longer leak to members.

### Shared component (`app/components/IngredientAutocomplete.tsx`)

- Debounced ingredient search, optional `Create "<name>"` free-create through
  `getOrCreateIngredient`, `value`/`onChange` + `onSelect` hook (recipe editor
  uses it for `recordSelection`/`recordSearch` analytics).

## Verification

- **Jest**: 46 suites / **527 tests green**, including new coverage:
  - `api.ts` ingredient queries/mutations + nullable-ID payloads
  - `ItemRow` brand-picker flow (dialog → search → `checkGroceryItemWithBrand`)
    and ingredient-primary label
  - items page with `useMe`-gated pickers
  - ingredients admin page: list/search/create/edit/merge
- `npx tsc --noEmit` — clean
- `npx eslint` on all touched files — 0 errors

## Live smoke test (Playwright, rebuilt `lena2-web:debug`)

Recipe → meal plan → grocery → check-off loop, verified against the DB:

1. `/recipes/1` — typed `cumin`, picked seeded `ground cumin` (Create option
   also offered), added 1 tsp → `recipe_item`: `item_id NULL,
   ingredient_id=190`. `phase3-recipe-ingredient-editor.png`
2. `generateGroceryList(mealPlanId=1)` → `ground cumin 0.25 teaspoon` renders
   ingredient-primary among branded item lines.
3. Check-off → brand picker "Which ground cumin did you buy?" → searched
   `cumin` → picked `Mccormick Mccormick Ground Cumin` →
   `checkGroceryItemWithBrand` credited the line (`item_id=53513,
   ingredient_id=190, is_checked=t`) and recorded the usual brand
   (`userprefs.household_ingredient_item`: household 2, ingredient 190 →
   item 53513). Caption now reads `usual: Mccormick Mccormick Ground Cumin`.
   `phase3-grocery-usual-brand.png`
4. Add row — `paprika` sorted `paprika (ingredient)` / `smoked paprika
   (ingredient)` first; added ingredient-only manual row
   (`item_id NULL, ingredient_id=181, source=manual`).
5. `/inventory/ingredients` — search + paginated list + create/edit/delete/
   merge UI live. `phase3-ingredients-admin.png`
6. Item Create dialog — `Catalog ingredient` (admin) + `Household ingredient`
   (override) pickers render. `phase3-item-ingredient-pickers.png`

## Notes

- Route is `/inventory/ingredients` (project's catalog-admin convention:
  `/inventory/brands`, `/inventory/categories`, …) rather than the
  `/admin/ingredients` wording in the plan.
- `AddMealSlotItemInput.itemId` went optional in P2 — client payloads updated
  to match (nullable `itemID` everywhere downstream).
