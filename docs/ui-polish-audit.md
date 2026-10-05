# LEN-48 — Cross-Client UI Audit

Audit-driven polish pass for LEN-47. Scope: keep the existing sage/cream/olive
system in `docs/design.md`; fix every place the implementation drifts from it
or from accessibility/platform norms. No new visual identity.

## Evidence

- **Web**: 47 desktop (1440px) + 8 responsive (390px) captures on the
  `lena2shots` seeded stack — `audit-shots/`. Every PNG was inspected; three
  pages (`brands`, `food-nutrients`, `food-flavors`) were caught still loading
  because they fetch their entire catalog before rendering (see W-01).
- **Mobile**: 27 captures on `emulator-5554` (API 37, 420dp) via the new
  `integration_test/screenshot_test.dart` + `test_driver/` harness, signed in
  with `LENA_DEBUG_ID_TOKEN` — `clients/mobile/mobile-shots/`.

Severity: **H** = broken/unusable or misleading · **M** = drift or friction ·
**L** = polish.

## Web findings

| # | Sev | Route / scope | Finding | Suggested fix |
|---|-----|---------------|---------|---------------|
| W-01 | H | `/inventory/brands` (+ any large catalog) | `getBrandList` paginates the **entire** brand table before first render — 8,787 brands = ~88 sequential GraphQL round-trips; the page shows a spinner indefinitely. Only `getBrandList` does unbounded paging (api.ts:2766) but the pattern can bite any `getXList` as catalogs grow. | Server-side `search` + page the table; render page 1 immediately. |
| W-02 | H | `/events`, `/meal-plans`, `/grocery-lists`, `/wine/bottles` | `DataTable` derives columns from `Object.keys(rows[0])` with `label: key` — headers render raw camelCase (`eventDate`, `slotGranularityMinutes`, `weekStartDayOfWeek`, `generatedDate`, `lastUpdatedBy`). Wine Bottles additionally leaks audit fields (`lastUpdatedBy`, `lastUpdatedDate`, `bottleNumber`) and ~15 columns. | Pass explicit `columns` (reuse CrudPage field labels); hide audit/internal fields; curate bottle columns. |
| W-03 | H | All `CrudPage` tables | Boolean cells render literal `true`/`false` (Recipes Active, Ingredients, Categories Active/Protein, Allergens, Wine Flavor Profiles…). | Type-aware cell renderer: `boolean → Yes/No` (or icon). |
| W-04 | H | `/inventory/items`, `/grocery-lists` | ISO timestamps rendered raw: `2026-10-07T01:11:18.375Z`. | Detect ISO strings → `toLocaleDateString`. |
| W-05 | H | `/recipes` | Filter row overflows at 1440px — UPLOAD RECIPE + SCAN clipped (see `recipes-category-filter.png`). | Wrap filters/actions to a second row below ~1600px. |
| W-06 | H | `/meal-plans/[id]` @390px | Weekly grid shows only the Breakfast column; Lunch/Dinner cut off with no scroll affordance. | Day-stacked layout under `sm`, or scroll indicator. |
| W-07 | H | `DataTable` pages @390px | Recipes + Items tables horizontally scroll with no mobile fallback. | Card/list layout under `sm`. |
| W-08 | M | `/inventory/brands`, `/food-flavors`, `/food-nutrients`, `/recipes/pending`, `/recipe-categories` | Missing empty states — pages render a lone spinner-then-blank or `0–0 of 0` with no "No X yet" copy. `/inventory/items/pending` has one ("Nothing pending right now.") — the pattern exists. | Reuse the pending-items empty-state pattern across CrudPage. |
| W-09 | M | web-wide | 49 `CircularProgress` sites, **0** `Skeleton` — every load is a bare spinner; on CrudPage it's a floating dot under the title. | Add skeleton rows to DataTable/loading states. |
| W-10 | M | `/grocery-lists/[id]`, `/recipes/[id]` ingredients | `usual: Ahold Ahold Garlic Salt`, `Ahold — Ahold Angel Hair Pasta` — brand name duplicated when it's a prefix of the item name. | Dedupe brand prefix in `usualBrandLabel`/`brandItemLabel`. |
| W-11 | M | `/inventory/items` | Column weights wrong: Name wraps to 8 lines while Notes/Favorite get wide space; headers wrap to two lines. | Rebalance column widths; name column min-width. |
| W-12 | M | `/inventory/items` | `Current Quantity` and `Favorite` render as underlined text — reads as hyperlinks; affordance unclear. | Button/chip styling for inline-edit cells. |
| W-13 | M | `/grocery-lists/[id]` | Checked items look identical to unchecked except the tick (`garlie` row). | Strikethrough/dim checked rows. |
| W-14 | M | `/meal-plans/[id]` | Empty slots say **"Blank"** — reads like a bug; dashboard says "Nothing planned for lunch yet". Voice mismatch. | Use a soft empty label ("—" or "nothing planned"). |
| W-15 | M | header @390px | "Ask Dot" button truncates to "Ask I". | Icon-only header action under `sm`. |
| W-16 | M | `/recipes`, `/inventory/items` actions col | Icon buttons + text buttons stack awkwardly (MANAGE alone on line 2; ADD TO INVENTORY + CATEGORY stacked). | Consistent actions pattern: icons row or menu. |
| W-17 | M | `/grocery-lists` | Rows indistinguishable — two identical `2026-10-04` rows, no name/label column. | Show list name or `List #n` + item count. |
| W-18 | M | `/notification-settings` | MUTE button + delivery toggle side-by-side; relationship unclear. | Label/group the two controls or fold mute into a menu. |
| W-19 | M | `/meal-plans`, `/events/[id]` | Raw model values: `weekStartDayOfWeek: 0`; servings cell `8 / ×1.33 of 6` (two lines, cryptic). | Humanize: `Sun`, `8 (scaled ×1.33 from 6)`. |
| W-20 | L | `/meal-plans/[id]` @390px | SUGGEST MEALS + GENERATE GROCERY LIST wrap to two lines each. | Full-width stacked buttons. |
| W-21 | L | `app/page.tsx`, `LenaLogo.tsx` | Hex literals outside tokens: `ACCENT_COLORS` (`#7C9473`…), logo defaults `#5F7A57`/`#3E3E34`, login brand colors (acceptable for provider marks). | Move accents into palette or reference theme. |
| W-22 | L | `/login` | Discord filled brand-blue vs Google outlined — inconsistent weights. | Normalize to same button style. |
| W-23 | L | `/profile`, `/household` | `maxWidth: 520` forms leave ~⅔ of a 1440px viewport empty. | Center the column or keep-left by convention (pick one). |
| W-24 | L | long names | No truncation anywhere — `Ultra-volume Conditioner For Fine, Limp Hair…` wraps rows to extreme heights (Items, restock card). | Ellipsize titles at 2 lines. |
| W-25 | L | `/events/[id]` timeline | "no recipe steps to schedule" banner + "This dish cannot be scheduled" line say the same thing twice. | Keep the warning line, drop the banner (or vice-versa). |

## Mobile findings

| # | Sev | Screen | Finding | Suggested fix |
|---|-----|--------|---------|---------------|
| M-01 | H | all (bottom nav) | 8 fixed tabs truncate labels: "Dashbo…", "Househ…". | Shorten labels ("Home") or `labelBehavior: onlyShowSelected` / smaller type. |
| M-02 | H | `23-edit-bottle` | Filled dropdowns: floating label strikes through the value ("D̶e̶s̶s̶e̶r̶t̶" under "Type", "France"/"Country", "Bordeaux"/"Region", "2021"/"Vintage year", "Domaine Serene"/"Vineyard"). | Give `DropdownButtonFormField` a proper `decoration.labelText` or drop the overlaid hint. |
| M-03 | H | `02-grocery-lists` | `Generated: 2026-10-04T18:48:42.199322Z` — raw ISO in user-facing text. | Format `generatedAt` (`Oct 4, 2026`). |
| M-04 | M | `04-grocery-list` | `usual: Ahold Ahold Garlic Salt` (same dup-brand bug as W-10); `Source: mealplan` internal value shown. | Dedupe prefix; hide/humanize source. |
| M-05 | M | all | 35 `CircularProgressIndicator`, 0 skeletons — dashboard boot captured as blank cream + dot (00-boot). | Skeleton rows for list loads; branded splash for boot. |
| M-06 | M | boot flow | `IndexedStack` eagerly builds `ScanScreen` → CAMERA permission prompt on **first launch** before the user touches Scan (blocked the capture harness). | Lazy-build tab children or request permission on first Scan entry. |
| M-07 | M | theme drift | `Colors.red`/`black54`/`amber`/`orange`/`green` literals in 8+ files; raw `fontSize: 11–18` literals bypass `textTheme`. | Map to `colorScheme`/`textTheme` in `theme.dart`. |
| M-08 | M | `19-edit-meal-plan` | Raw model fields: "Week start day (0-6)", "Day (0-6)", slot rows "Day 0 - breakfast". | Weekday picker; "Sun — breakfast". |
| M-09 | M | `12-assistant` | Empty chat has no prompt suggestions (web offers chips). | Reuse the web suggestion chips. |
| M-10 | M | `04-grocery-list` | Allergen warning = lone tiny red triangle icon, no text. | Warning chip with label, or icon + count. |
| M-11 | M | `20-wine` | Two stacked FABs (+ = adjust, 🍷 = bottles) — add icon ambiguous. | Label the adjust FAB ("Adjust") or merge into an actions menu. |
| M-12 | M | `18-meal-plans` | Rows repeat the date twice; "Active" chip on **both** plans — can two plans be active? | Single date line; clarify active semantics. |
| M-13 | M | `10-household` | Notification rows: title repeats inside body ("…expires soon" ×2); household-name save icon floats far right. | Dedupe title/body; inline save affordance on the field. |
| M-14 | M | `14-recipes`, `24-items`, `20-wine`, `22-bottles` | Bare `ListTile` rows — other surfaces use `Card` rows (events, grocery lists, more). | Card-wrap rows for consistency. |
| M-15 | L | `24-items` | `each — Pantry / Ahold` subtitle mixes unit/category/brand with ambiguous separators. | `Pantry · Ahold` and put unit on qty. |
| M-16 | L | several | Very airy row spacing (recipes, items, bottles ~90px rows). | `dense` tiles or reduced vertical padding. |
| M-17 | L | `11-notification-settings` | Mute icon-button (bell-z) next to delivery switch — unclear relation (same as W-18). | Same fix as W-18. |

## Code-level token drift

- **Web** — `app/page.tsx` `ACCENT_COLORS` + `LenaLogo` default props carry
  hex literals that duplicate `providers.tsx` tokens; login page carries
  provider-brand hex (justified). 49 `CircularProgress` / 0 `Skeleton`.
- **Mobile** — `Colors.*` material-palette literals in `allergy.dart`,
  `assistant`, `edit_recipe`, `household`, `recipes`, `scan`; `fontSize`
  literals in `edit_meal_plan`, `edit_recipe`, `event_timeline`,
  `grocery_list`, `household`, `recipes` — all bypassing `theme.dart`.

## Token-change proposals (awaiting approval)

1. **None required for the palette** — every finding fixes by *using* existing
   tokens, not changing them. Sage/cream/olive stays untouched.
2. **Suggested additions (not new colors):**
   - a `Skeleton`-based loading-state convention for `DataTable` + mobile lists
     (component pattern, no token change);
   - optional `navigation` label-scale tweak under the existing type ramp if
     shortening "Dashboard"/"Household" is rejected.
3. **Dart side:** route `Colors.red` → `colorScheme.error`, `amber/orange` →
   existing warning semantics, `fontSize` literals → `textTheme` styles.
   All are theme *usage* fixes.

## Coverage checklist

- [x] 37 web routes (36 + login) — 47 shots incl. 390px pass on 8 high-traffic
      routes; `/recipes/pending/[id]` skipped (empty queue — itself finding W-08).
- [x] 27 mobile screens — boot, 8 tabs, 4 More destinations, 13 deep screens,
      2 dialogs/sheets.
- [x] Every PNG inspected for spinners/stale state (3 web + 1 mobile were
      caught mid-load → W-01/W-08/M-05).
- [x] Code-level drift sweep (both clients).
