# Phase 4 — Mobile: generic ingredients (LEN-10)

Branch: `ingredient-layer-p4` · PR: against `main`

## What shipped

### `edit_recipe_screen.dart` — ingredient-first recipe lines
- Recipe query now selects `ingredient { id name }` per line; `items` query is
  joined by an `ingredients` search query.
- `_itemId` selection replaced by an **Ingredient** picker backed by
  `ingredients(search:)` — generic names only (`ground cumin`, not `McCormick
  cumin`), with `Create "<name>"` calling `getOrCreateIngredient`.
- A second, optional **Preferred brand** picker keeps branded-item selection
  (`itemId`) alongside `ingredientId`; save sends both, so lines can be
  ingredient-only, item-only, or both.
- Existing recipe rows hydrate ingredient-first with `item` fallback.
- Ingredient picks/searches record analytics with `EntityType.ingredient`.

### `grocery_list_screen.dart` — usual brands + first-time brand pick
- `groceryRoutingQuery` selects `usualBrand { id name }`.
- Row caption shows `usual: <brand>` when the household has a remembered
  usual item for the line's ingredient.
- Checking off an unchecked line that has `ingredient` but no `item` and no
  `usualBrand` opens **"Which <ingredient> did you buy?"** — an item search
  dialog. The pick calls `checkGroceryItemWithBrand`, which binds the item,
  marks the line checked, and records it as the household's usual.
- Bound items, usual-branded lines, manual lines, and unchecks still toggle
  normally. `needsBrandPick()` extracted for testing.

### `scan_screen.dart` — resolved ingredient + link prompt
- `itemByUpc` selects `ingredient` and `householdIngredient`.
- A UPC hit shows the resolved ingredient (household override wins, tagged
  "(household)"), or a "Link ingredient" prompt that searches ingredients,
  offers `Create "<name>"`, and calls `setHouseholdItemIngredient`.
- `resolvedIngredientOf()` extracted for testing.

### `edit_meal_plan_screen.dart`
- Slot items select `ingredient { id name }` and fall back to
  `ingredient.name` when no item is bound (ingredient-only overrides).

## Verification

- `flutter analyze` — **no issues**
- `flutter test` — **96/96 pass** (was 87; +9: `needsBrandPick` truth table ×5,
  `resolvedIngredientOf` precedence ×3, scan render smoke ×1)

## Live UAT — Pixel 10 Pro emulator, `LENA_DEBUG_ID_TOKEN`, debug stack

Full grocery check-off → usual-brand loop verified on a real device:

1. Opened List 1 (generated during P3 UAT) — checked `ground cumin` row shows
   `usual: Mccormick Mccormick Ground Cumin`.
2. Tapped the checkbox on the ingredient-only `paprika` row (item_id NULL,
   ingredient_id 181) → **brand picker opened** ("Which paprika did you buy?",
   live item search).
3. Picked `Frontier Paprika Ground` → dialog closed, row bound to item 15577
   and checked.
4. DB: `household_ingredient_item` 181→15577; `grocery_list_item` 18 bound +
   `is_checked=t`. Row now renders `usual: Frontier Natural Products Co-op
   Frontier Paprika Ground`.

### Screenshots
- `phase4-mobile-usual-brand.png` — both checked rows with `usual:` captions
- `phase4-mobile-brand-picker.png` — first-time brand-picker dialog
- `phase4-mobile-recipe-editor.png` — Ingredient + Preferred brand sections
- `phase4-mobile-ingredient-picker.png` — ingredient dropdown (generic names)

## Notes

- First emulator boot had a stuck adbd (device showed `offline`); a cold
  relaunch fixed it.
- Accidental `Cilantro` check during navigation was unchecked — final state
  matches pre-UAT (only cumin + paprika checked).
- `mobile_scanner` hardware scan was not exercised (emulator camera); scan
  flows are covered by unit tests and the ingredient-resolution logic is
  shared with the verified grocery path.
