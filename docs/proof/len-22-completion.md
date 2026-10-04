# LEN-22 — Allergy Tracking P3: Clients — Proof of Completion

Branch: `LEN-22-allergy-p3`
Scope: member allergy/dietary editors, warning surfaces, admin allergen
curation UI, plus the resolver-source propagation fixes UAT exposed.

## What shipped

### Web (`clients/web`)

- `lib/api.ts` — Gql types + fragments (`ALLERGY_FIELDS` = `allergens` +
  `allergyWarnings`) wired into ITEM_FIELDS, INGREDIENT_FIELDS,
  RECIPE_FIELDS, MEAL_PLAN_FIELDS slot fragment, EVENT_RECIPE_FIELDS,
  GROCERY_LIST_FIELDS, and `groceryRouteGroups`; mappers
  `toAllergen`/`toAllergenFlag`/`toAllergyWarning`/`toMemberAllergen`;
  API functions `getAllergens`, `getMyAllergies`, `setMyAllergy`,
  `createAllergen`, `updateAllergen`, `setIngredientAllergen`,
  `setItemAllergen`.
- `lib/types.ts` — `Allergen`, `AllergenFlag`, `MemberAllergen`,
  `AllergyWarning` + optional `allergens`/`allergyWarnings` on Item,
  Ingredient, Recipe, MealSlot, EventRecipe, GroceryListItem.
- `app/components/AllergyWarning.tsx` — `AllergyWarningChip` (compact,
  severe vs advisory styling), `AllergyWarningsAlert` (detail-page
  alert listing member + allergen + both kinds), `AllergenFlagsLine`
  (flag chips; empty set renders **"No allergen information"** — never
  "no allergens"/"safe").
- `app/components/AllergenFlagsEditor.tsx` — contains / may_contain /
  remove editor shared by the item and ingredient admin dialogs; helper
  copy states "No flags means no allergen information — not that it's
  safe."
- `app/profile/page.tsx` — "Allergies & dietary restrictions" card:
  tri-state (None / Dietary / Allergy) per allergen, own records only,
  `setMyAllergy(on)` per change.
- `app/inventory/allergens/page.tsx` — admin registry CRUD (create,
  rename/description edit, deactivate as soft-delete).
- Warning surfaces wired: recipe detail alert, recipe browse Warnings
  column, meal-plan slot chip, event recipe-row chip, grocery row chip.
- `AdminLayout` nav entry for `/inventory/allergens`.

### Mobile (`clients/mobile`)

- `lib/allergy.dart` — raw-map extraction (`allergyFlags`,
  `allergyWarnings`), `warningLabel` ("member — allergen (kind;
  flagKind)"), `hasSevereWarnings`, `AllergyBadge` icon + tap-to-open
  detail dialog.
- `lib/screens/household_screen.dart` — "Allergies & dietary
  restrictions" section: per-allergen segmented (None/Dietary/Allergy)
  bound to `myAllergies`, writes via `setMyAllergy`.
- Warning badges on `grocery_list_screen` rows, `edit_recipe_screen`
  banner + allergens chip, `edit_meal_plan_screen` slot rows,
  `event_detail_screen` dish rows.

### Backend fixes (found during P3 UAT)

`allergySource` is lazy-load context; resolvers built outside the
preload paths must carry it or `allergens`/`allergyWarnings` error
("allergy context unavailable"). Three passthrough resolver types
dropped the source when handing children their `itemResolver`/
`ingredientResolver`:

- `mealPlanResolver`/`mealPlanPageResolver`/`mealSlotItemResolver`
  (`resolver_mealplan.go`)
- `recipeItemResolver` (`resolver_recipe.go`)
- `userItemResolver` + mutation returns (`resolver_userprefs.go`)

All now propagate `firstSource(preloaded, lazy)`. Symptom seen in UAT:
`/meal-plans/1` returned 34 field errors; after the fix the page and
all nested fragments resolve clean.

## UAT evidence (debug stack, rebuilt `lena2-api:debug` + `lena2-web:debug`)

Seeded via GraphQL: `setMyAllergy(milk, allergy)` for Jason Adams;
`setItemAllergen` milk/`contains` on a recipe's inventory item;
advisory `dietary`/`may_contain` pair (pork) set then cleared;
record + flag clears verified.

| Surface | Result |
|---|---|
| `myAllergies`, `setMyAllergy` on/off | correct round-trip |
| Recipe detail (`/recipes/1`) | alert "Jason Adams — milk (allergy; contains)" + "Allergens: milk" |
| Unflagged recipe (`/recipes/4`) | "Allergens: **No allergen information**" — not "no allergens" |
| Recipe browse (`/recipes`) | Warnings column chip "1 allergy warning" |
| Meal-plan page (`/meal-plans/1`) | slot chip; **34-error regression found & fixed** |
| Event page (`/events/1`) | recipe-row chip on mutation + query paths |
| Grocery list (`/grocery-lists/1`) | row chip via `groceryRouteGroups` (missing fragment found & fixed) |
| Profile (`/profile`) | tri-state editor, milk=Allergy persisted |
| `/inventory/allergens` | registry table + create/edit |
| Item edit dialog | flag editor, existing milk flag, "no info" helper copy |

### Emulator (Android, debug build, `LENA_DEBUG_ID_TOKEN` session)

- Household screen: editor renders; tap set `crustacean shellfish =
  dietary` → `myAllergies` confirmed remotely; cleared back to none.
- Grocery list: ⚠ badge on the flagged row; tap opens "Allergy
  warnings — Jason Adams — milk (allergy; contains)" dialog.
- Recipe edit: red warning banner + "Allergens: milk" chip.
- Event detail: ⚠ badge on the flagged dish.
- Meal plan edit: ⚠ badge on the flagged slot.

Screenshots captured during UAT are retained locally (`uat-shots/`).

## Test coverage

- **Web**: `api-allergens.test.ts` (mutations/queries/mappers),
  `AllergyWarning.test.tsx` (chip, alert, flags line incl. "No
  allergen information"), grocery ItemRow warning case, profile
  allergy-card case. 551/551 jest green; `tsc --noEmit` clean;
  eslint 0 errors (11 pre-existing warnings elsewhere).
- **Mobile**: `allergy_test.dart` — 8 tests (extraction, labels,
  severe classification, badge render + dialog). Full suite 104/104;
  new/edited files pass `dart format --set-exit-if-changed`.
- **Go**: `go test ./...` green; `golangci-lint run ./cmd/... ./internal/...` 0 issues.

## Notes

- Pre-existing debug-mode layout overflows observed on
  `edit_meal_plan_screen` ("Recipe (optional)" dropdown 78px, add-item
  row 374px) — unrelated to this change; candidate for a future UI
  pass.
- Left in dev DB for inspection: `milk` allergy record on Jason Adams,
  milk/`contains` flag on the cheese item, "Allergy UAT Dinner" event.
