# Web UI Refresh — Sage/Cream/Olive Theme, Dashboard Warmth, Sidebar Zones

Restyle the web UI with a sage-green/warm-cream/olive palette, friendlier dashboard (meal placeholders, recipe suggestion cards, conversational copy app-wide), and a two-zone sidebar — web only.

## Status

**Planning complete — ready to implement.** The plan file is also tracked by the session at `C:\Users\aipal\.devin\plans\plan-05cb8ab2b81d1559.md`. To resume: start on `web-refresh-p1` from `main` (all prior branches merged; nothing open).

## Decisions (confirmed with user)

- **Thumbnails**: derived visual tiles — category/dish-type icon + deterministic accent color, no backend/schema changes
- **Status colors**: success + info → olive; warning stays amber, error stays red (legibility)
- **Nav zones**: Core Kitchen (Dashboard, Assistant, Meal Planning, Recipes, Inventory, Wine) on top; bottom split into Administration (Users, Pending Items) + Account (Household, Profile). Recipes' admin children (Categories, Pending Reviews) stay nested under Recipes
- **Copy**: app-wide conversational pass — headings, empty states, helper text; nav labels and form fields stay functional

## Phase 1 — `web-refresh-p1`: theme + sidebar

### `clients/web/app/providers.tsx` — palette + component overrides

```ts
palette: {
  primary:   { main: "#7C9473", light: "#A4B79C", dark: "#5F7A57" },  // soft sage
  background: { default: "#FAF6EF", paper: "#FFFDF8" },               // warm cream
  success:   { main: "#8A9A5B", dark: "#6E7B45", light: "#A9B87F" },  // soft olive
  info:      { main: "#8A9A5B", dark: "#6E7B45", light: "#A9B87F" },
  // warning amber + error red unchanged
  divider: "#E5DFD3",
  text: { primary: "#3E3E34", secondary: "#6B6B5E" },
}
```

- `MuiPaper`: keep soft shadow but warm it (`rgba(62,62,52,0.06)`); paper bg inherits `background.paper`
- `MuiAppBar`: `defaultProps: { color: "default" }` + `styleOverrides` — cream-tinted bar with sage text instead of solid blue (or keep primary bar if it reads better — decide during implementation by screenshotting both)
- `MuiListItemButton`/`MuiListItemIcon`: swap hardcoded `#4b5563` → `text.secondary`; selected-state alpha uses `primary.main` automatically
- `MuiChip`/`MuiBadge`: nothing needed for `color="success"`/`color="info"` — palette does it; check `color="default"` chips get warm-neutral styling

### `app/components/LenaLogo.tsx` — recolor defaults

`iconColor #059669` → sage `#5F7A57`, `textColor #1e293b` → `#3E3E34`. AppBar usage passes `inherit` — unaffected.

### `app/components/AdminLayout.tsx` — sidebar zones

- Restructure `NAVIGATION` into `CORE_NAV` and `BOTTOM_NAV` (admin + account):
  - Core: Dashboard, Assistant, Meal Planning (Weekly Plan, Events, Grocery Lists), Recipes (+admin children), Inventory, Wine
  - Administration (adminOnly, collapsible): Users, Pending Items
  - Account: Household, Profile
- Render core list, then a spacer (`flexGrow`/`mt:auto` in a flex-column drawer), divider + small uppercase captions ("Administration", "Account" styled `overline`/caption in `text.secondary`) for the bottom zone
- Bottom zone pinned via `Box sx={{ display:"flex", flexDirection:"column", height:"100%" }}` on `drawerContent`
- Update `AdminLayout.test.tsx` assertions for new structure; add a test asserting Household/Profile render after the divider zone

### Hardcoded color sweep (pages touching slate/blue hexes)

`app/page.tsx` (`PlanMealLink` #475569/#cbd5e1/#f1f5f9/#f8fafc, tile bg `#f8fafc`), `LoginScreen.tsx` Google button (keep brand per-button colors — Microsoft/Discord/Facebook brand colors stay), plus any `#f8fafc`/`#475569`-family hexes in `household`, `notifications`, `events`, `meal-plans`, `items/pending`, `recipes/*` pages — replace with theme tokens (`background.default`→ warm cream tints, `text.secondary`, `divider`) so all pages track the palette.

## Phase 2 — `web-refresh-p2`: dashboard + copy pass

### `app/page.tsx` — dashboard

1. **Empty meal slots**: replace bare `+ Plan a meal` with a soft placeholder — large faded meal icon (existing `FreeBreakfastIcon`/`LunchDiningIcon`/`DinnerDiningIcon` at ~40px, `color: alpha(primary.main, 0.35)`) + conversational line ("Nothing planned for breakfast yet" + "Plan it →" link). Tile bg → `alpha(primary.main, 0.06)` instead of `#f8fafc`.
2. **Suggestion cards**: replace link+chip rows with small cards — `Paper variant="outlined"`, leading ~48px rounded tile: deterministic accent color from `recipeID` (hash → small sage/olive/terracotta/wheat ramp defined in theme or a local const), icon mapped from recipe categories (dish-type/cuisine keywords → `DinnerDining`/`LocalCafe`/`Cake`/`SetMeal` etc., fallback `Restaurant`), then name, reason chip (olive), and meta line (prep+cook min, ★ rating). Cap at 5 shown ("and N more" link to `/recipes`).
3. **Conversational copy (dashboard)**: keep "Dashboard" H4 + warm subtitle; "Suggested for You" → "Delicious ideas for tonight"; "Running low" → "Time to restock"; "No suggestions yet" → "Nothing to suggest yet — rate a few recipes and we'll get ideas flowing." Keep `REASON_LABELS` (tests assert them) unless e2e/test updated.
4. **"Running low" names**: `stripSize`/`sizeBadge` already strip sizes into chips; extend `stripSize` to also collapse doubled whitespace and strip leading brand prefix if it duplicates `it.brand`. Warning dot stays amber per decision.

### App-wide copy pass (the voice sweep)

Audit `Typography variant="h[45]"` titles + `CrudPage`/`DataTable` `title` props + empty-state strings:

- `DataTable` "No data" → "Nothing here yet"
- `grocery-lists` pages: "Grocery Lists" / "Grocery List" headings, "Add Manual Item" → "Jot something down" (keep field labels functional), "Suggested Restock" heading → "Time to restock" — **update `e2e/restock.spec.ts:122` assertion** and the mealplans spec comment accordingly
- `notifications` "Notification settings", `household` headings, `items/pending` "Pending Items", `profile` "Profile", `users` "Users" — keep page H4s functional (nav labels match), but soften empty states and helper text to second-person ("Nothing pending right now", "Your household")
- Keep nav item labels, button labels, and form field labels functional — voice change targets headings, empty states, helper text only

### `docs/newfeatures.md`

New `## Web UI refresh` section marked ✅ Done describing palette, suggestion cards, meal placeholders, sidebar zones (reference PRs).

## Tests to update

- `__tests__/app/page.test.tsx` — heading/empty-state assertions ("Dashboard", "No suggestions yet", reason labels if renamed), meal-slot empty state, suggestion card render
- `__tests__/components/AdminLayout.test.tsx` — nav structure assertions + new bottom-zone test
- `e2e/restock.spec.ts` — "Suggested Restock" → new heading text
- `e2e/mealplans.spec.ts` — comment references; verify no heading assertions break
- `e2e/smoke.spec.ts` — check for nav/heading assertions

## Verification

- `npx tsc --noEmit`, `npx jest` (full suite), `npx eslint`
- `npx playwright test` against lena2shots stack for the changed specs
- Re-capture `dashboard.png`, `grocery-list.png` (+ any others visibly changed) via the wiki-shots stack; inspect for contrast/spinner issues; update wiki Home/Grocery-Lists pages
- Contrast sanity: sage `#7C9473` on cream `#FAF6EF` ≈ 3.6:1 — fine for large text/UI; body text stays `#3E3E34` (~9:1)

## Risks

- Sage primary on AppBar may look washed out — screenshot-review during implementation, fall back to cream bar + sage text if so
- App-wide copy sweep churns test assertions — grep `__tests__`/`e2e` for every renamed string before finalizing each name
- Derived tiles without real photos can look samey — vary accent hue by hash so cards differentiate

## Out of scope

- Mobile app (unchanged — web only)
- Recipe image upload/storage (separate future feature)
- Dark mode

## Resume checklist

1. `git checkout main && git pull --ff-only && git checkout -b web-refresh-p1`
2. Implement Phase 1 → verify → PR
3. Merge, then `web-refresh-p2` → implement → verify → PR
4. Closeout per `AGENTS.md`: delete merged branches, coverage check, `audit/summary.md` re-walk, `newfeatures.md`/`README.md`/wiki updates + screenshots
