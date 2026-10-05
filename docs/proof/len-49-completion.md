# LEN-49 — UI polish P2 (web fixes) — completion proof

Phase 2 of LEN-47. All 25 web findings in `docs/ui-polish-audit.md` addressed on
`len-49-p2-web-fixes`. Sage/cream/olive identity unchanged.

## Finding → fix map

| ID | Fix |
|----|-----|
| W-01 | `getBrandsPaged` + brands page → `pagedListFn` (was ~88 unbounded GraphQL round-trips; verified rendering in `inventory-brands.png`) |
| W-02 | Explicit `fields`/`tableFields` on events, meal-plans, grocery-lists, wine-bottles; `DataTable` humanizes inferred headers + hides ID/audit keys in inferred mode only |
| W-03 | `displayText` renders booleans as Yes/No (verified: Recipes Active, Events Active) |
| W-04 | `displayText` + `fmtDate()` localize ISO strings; date-only values parse as local (no UTC day-shift) — verified `10/7/2026` cells |
| W-05 | Recipe filter row wraps (`flexWrap`) — verified `recipes.png` (two-row filter bar at 1440px) |
| W-06 | Meal-plan weekly grid → stacked day cards under `sm` — verified `meal-plan-week-mobile.png` |
| W-07 | `DataTable` card layout under `sm` — verified `recipes-mobile.png` |
| W-08 | Empty states everywhere + new "No results on this page" for sparse server-paged tables (food-flavors/nutrients page over items) — verified |
| W-09 | `DataTable` skeleton rows (`role="status"`) replace lone spinner |
| W-10 | `brandedName`/`brandSuffix` in `lib/format.ts` dedupe brand prefixes — verified `usual: Ahold Garlic Salt` (was `Ahold Ahold`) |
| W-11 | `FieldDef.minWidth` + name-column min width — verified `inventory-items.png` |
| W-12 | Quantity/favorite cells → chip-style buttons — verified |
| W-13 | Checked grocery rows dimmed + italicized — verified `garlie` row |
| W-14 | "Blank" slots → "Nothing planned — tap to add" — verified |
| W-15 | Ask Dot label hidden under `sm` (icon-only) — verified mobile shots |
| W-16 | Row actions unified to icon row (edit/delete/manage) |
| W-17 | Grocery-list rows → `List #n`, Generated date, Store, item count (no false Meal Plan column — schema has none) |
| W-18 | Mute control keeps `Mute` label + `aria-haspopup` + tooltip for context |
| W-19 | `weekStartDayOfWeek` → weekday names ("Sunday"); servings → `8 · scaled ×1.33 — recipe serves 6` — verified `event-detail.png` |
| W-20 | Meal-plan action buttons wrap/stack at 390px — verified |
| W-21 | `LenaLogo` + `ACCENT_COLORS` read theme tokens (terracotta/wheat kept as named constants) |
| W-22 | Login provider buttons normalized to full width |
| W-23 | Profile/household forms `width:100%; maxWidth` responsive |
| W-24 | 2-line `line-clamp` on cell contents — verified items name clamp |
| W-25 | Duplicate unschedulable banner removed from events/[id] |

## Additional fixes found during verification

- `DataTable` audit-filter previously dropped **explicit** `foodId`/`nutrientId`
  columns on Food Nutrients — filter now applies to inferred columns only.
- Empty-page copy (`Nothing here yet`) shown while `totalCount > 0` — new
  "No results on this page" branch (verified `inventory-food-flavors.png`).
- Prep/cook time cells → `5 min`/`15 min`; meal-plan + grocery-list subtitles
  localized via `fmtDate`.
- `capture.mjs` `mshot` accepts locator functions (Ask Dot label is now hidden
  on mobile by design — wait anchor switched to the placeholder input).

## Verification

- `npx tsc --noEmit` clean; `npx eslint .` clean; `npx jest` **573/573**
  (2 new DataTable tests for friendly rendering + inferred-header humanization).
- 7 initially-failing suites updated where expectations encoded raw values
  (`true`, `2024-01-01`, `"10"`, bare `5`); all green.
- Rebuilt `lena2shots-web` image; re-ran `tools/wiki-shots/capture.mjs` —
  53 screenshots inspected: no blank pages, spinners, overflow, dead actions,
  camelCase headers, or misleading labels. `recipe-import-review` skipped
  (empty queue, same as P1).

## Evidence

`tools/wiki-shots/wiki-shots/*.png` (desktop + 390px), attached to LEN-49.
