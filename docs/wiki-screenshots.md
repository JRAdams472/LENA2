# Wiki Screenshot Runbook

How to capture real UI screenshots for the GitHub wiki — needed whenever a
feature adds or changes user-facing screens (the `AGENTS.md` close-out step
requires a wiki update, and pages are far more useful with shots).

First done: wiki commit `ff8115c`, 13 shots across 10 pages, 2026-09-27.

## Prerequisites

- Docker Desktop running.
- `clients/web`: `npm ci` done and `npx playwright install chromium` run once.
- Python 3 (stdlib only — the seed script uses `urllib`).
- The wiki repo cloned separately: `git clone https://github.com/JRAdams472/LENA2.wiki.git`
  (default branch `master`; pushes go straight to master — no PR on the wiki).

## 1. Bring up an isolated stack

**Never shoot against your dev database.** The dev compose volume may also be
pinned to an older Postgres major version and fail to boot. Run the whole
thing under a separate compose project so its containers, network, and
volumes are isolated:

```bash
LENA_DB_PASSWORD=e2e-change-me \
  docker compose -p lena2shots -f docker-compose.yml -f docker-compose.e2e.yml \
  --profile seed up -d --build
```

Notes:

- `--profile seed` is **required** — `api` depends on `db-seed` and compose
  fails with "service api depends on undefined service db-seed" without it.
- `LENA_DB_PASSWORD` must be set for interpolation (the e2e override also
  fixes it to `e2e-change-me`).
- If port 8085 is already allocated, a stale `lena2` dev container is still
  registered — `docker compose down` the dev project first.
- To start fully clean (fresh IDs, no leftover demo data): same command with
  `down -v` first. This only touches `lena2shots_*` volumes.

Endpoints: Caddy on `http://localhost` (web + `/graphql`), test issuer on
`http://localhost:8085` (mint tokens: `GET /token?sub=…&email=…&name=…`).

## 2. Seed demo content

Empty-state screenshots are worthless — seed first:

```bash
python tools/wiki-shots/seed_demo.py
```

Creates: a second household member, three recipes **with timed steps**
(required for a meaningful timeline), a meal plan starting today (so the
dashboard shows a meal), a generated grocery list, a food event with three
dish slots including a free-form one, two wine bottles, pantry stock, and
allergen demo data (member records + curated item flags so warning
surfaces and the profile editor render).

- Idempotent — re-running reuses entities by name.
- Env overrides: `DEMO_API`, `DEMO_ISSUER`, `DEMO_DATE` (defaults: localhost,
  today).
- Catalog items are matched by keyword over all ~106k seeded products,
  preferring the *shortest* name ("Ahold Gold Potatoes" over
  `"dirty" Potato Chips`) so shots look like real groceries. Extend the
  keyword list or the entities for new features — match the schema in
  `internal/bff/schema.graphqls` (`gql()` raises on unknown fields).
- `eventDate` is `YYYY-MM-DD`; `targetTime` is a UTC timestamp **whose date
  must equal the event date** or the resolver rejects it.
- Also seeds: a 21+ `birthdate` on the demo admin (unlocks sommelier /
  cocktail AI surfaces) and UPCs on two catalog items for the mobile scan
  walk — `012345678905` linked to an ingredient (found/adjust card),
  `036000291452` left unlinked (link-ingredient prompt); any other UPC
  exercises the submit-for-approval form.

## 3. Capture

```bash
node tools/wiki-shots/capture.mjs        # writes ./wiki-shots/*.png
SHOTS_OUT=/tmp/shots node tools/wiki-shots/capture.mjs
```

The script mints a token, seeds `localStorage.lena_id_token` (same mechanism
as `e2e/auth.setup.ts`), and shoots the canonical page list at
1440×900 @ 2x. Edit the shot list for new pages. It also `docker exec`s a
small SQL insert into `lena2shots-db-1` to seed a pending allergen
suggestion — that table has no non-AI write path and the e2e provider
returns canned text.

Hard-won details:

- **`waitUntil: "networkidle"` never resolves** — the app polls
  notifications every ~30s. Use `domcontentloaded` + a `waitFor` text that
  only exists once real data renders (a recipe name, an item name).
- **Always eyeball every PNG.** Spinners screenshot fine and look plausible
  at a glance; check for the tiny progress arc before publishing.
- `/inventory/items` streams the whole catalog client-side — its wait needs
  ~2 minutes, not 20s.
- Interactive content (the event timeline) needs real clicks:
  `scrollIntoViewIfNeeded` → click → wait for rendered text → `fullPage`.
- Field names drift — if a `waitFor` times out, check the page source for
  the actual label rather than guessing.
- **Dark pass:** after the light and 390px sets, the script re-shoots the
  priority surfaces (dashboard, assistant + answer, events + timeline,
  grocery list, recipes, meal plan, notifications, household, items) in a
  context with `colorScheme: 'dark'` **and** `localStorage['mui-mode'] =
  'dark'` — the app resolves its MUI cssVariables theme from that key, so
  emulation alone is not enough. `*-dark.png` names are the wiki gallery
  set; `login` shoots both schemes.
- Store routing shots drive real dialogs: the Store select, the "Edit
  aisles" layout dialog, and a row's "item actions" → "Move to …" menu.

## 4. Mobile walk

```bash
cd clients/mobile
flutter drive \
  --driver=test_driver/integration_test.dart \
  --target=integration_test/screenshot_test.dart \
  -d <emulator> \
  --dart-define=LENA_API_URL=http://10.0.2.2/graphql \
  --dart-define=LENA_DEBUG_ID_TOKEN=<test-issuer token> \
  --dart-define=LENA_DEBUG_SCAN_UPC=012345678905,036000291452,999999999999
```

- `LENA_DEBUG_SCAN_UPC` (debug builds only, `kDebugMode`-gated) adds a
  "Simulate scan" app-bar action on the Scan tab — it feeds each comma-
  separated code through the real detect→lookup pipeline, so the
  found/adjust, link-ingredient, and submit-for-approval shots are real
  UI on an emulator with no camera barcode to read.
- Shots land in `mobile-shots/` on the host; the walk covers every tab,
  the More destinations, scan states, and a dark pass over the priority
  tabs.

## 5. Publish to the wiki

```bash
cp wiki-shots/*.png /path/to/LENA2.wiki/images/
# add  ![alt text](images/name.png)  to the relevant pages
cd /path/to/LENA2.wiki && git add -A && git commit -m "…" && git push origin master
```

- Images live in `images/` in the wiki repo; reference them with standard
  markdown `![alt](images/foo.png)` — relative paths resolve against the
  wiki repo.
- Keep alt text descriptive — it's the caption.
- Re-verify any wiki claims while you're in there: screenshots have already
  caught docs describing features that don't exist yet.

## 6. Teardown

```bash
LENA_DB_PASSWORD=e2e-change-me \
  docker compose -p lena2shots -f docker-compose.yml -f docker-compose.e2e.yml \
  --profile seed down          # frees ports 80/8085, keeps data
# add -v to also delete the isolated volumes
```

## Gotchas observed

| Symptom | Cause / fix |
|---|---|
| `service "api" depends on undefined service "db-seed"` | Missing `--profile seed` |
| `database files are incompatible` | Dev volume is PG16, image is PG18 — separate `-p` project avoids touching it |
| `port 8085 already allocated` | Stale `lena2` dev containers — `down` them |
| Item keywords return nothing | First 2000 items ≠ whole catalog — page through it (`find_items`) |
| `createRecipe` 403 | Mutations are `@admin` — use the `e2e-user-1` token, which is seeded admin |
| Empty region list | `regions(countryId:…)` — iterate countries until one has regions |
| Screenshot is a spinner | No `waitFor` on rendered content — always assert text first |
| Grocery list shows no items | Was a real UI bug (`mealplan` source unmapped) — treat odd shots as findings, not noise |

## Shot → page map (canonical set)

Light: dashboard, recipes, recipe-detail(+ingredients+tweaks+allergy-warning),
recipe-categories-admin, recipe-category-filter, recipes-semantic,
meal-plans, meal-plan-week, grocery-lists, grocery-list(+brand-picker,
store-picker, edit-aisles, move-aisle), notification-settings,
notification-bell, events, event-detail, event-timeline, assistant,
assistant-answer, wine-bottles, household, inventory-items,
ingredients-admin, allergen-registry, allergen-suggestions,
profile(+allergies), all admin/catalog pages (brands, flavor-profiles,
food-flavors, food-nutrients, nutrient-types, wine catalogs, users,
pending items/recipes), recipe-import-review when the queue is live.

Dark (`*-dark.png`): dashboard, assistant(+answer), events, event-detail,
event-timeline, grocery-list, recipes, recipe-detail, meal-plan-week,
notification-settings, household, inventory-items, login.

390px mobile-web pass: dashboard, recipes, recipe-detail, meal-plan-week,
grocery-list, assistant, household, inventory-items (`*-mobile.png`).

Mobile emulator walk (`mobile-shots/`): every tab, household switcher /
create / invitations / accept-merge, notification settings, generate
grocery dialog, grocery store picker + move-aisle menu, event detail +
timeline + edit, recipe filter/edit/tweaks set, meal plan + recipe picker,
wine + bottle picker + catalog + edit, items + edit item, scan idle +
simulated found/added/unlinked/link-ingredient/submit states, and a dark
pass over the More/dashboard/grocery(+list)/events/scan/pantry/assistant/
household tabs.
