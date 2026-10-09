# LENA2

A self-hosted kitchen assistant for tracking pantry stock, planning meals, managing recipes, building grocery lists, and keeping a wine cellar — accessible from a web dashboard and a Flutter mobile app.

---

## What is LENA2?

LENA2 is a personal, privacy-first household management system. It replaces scattered notes, spreadsheets, and shopping-list apps with one place to:

- **Track food and supplies** — keep a catalog of items and your own pantry quantities, minimum-stock thresholds, and favorites.
- **Plan meals** — build weekly meal plans with servings and generate grocery lists automatically.
- **Plan food events** — combine recipes into a gathering with absolute serve times, then generate a backwards-scheduled cooking timeline that flags appliance conflicts.
- **Manage recipes** — store ingredients, steps, and portions, then pull them into meal plans and events. Recipe steps carry timing metadata (duration, type, passive/hands-off, dependencies, appliance) that powers event scheduling.
- **Categorize and rediscover recipes** — tag recipes with course, cuisine, difficulty, main ingredient, and more; filter by faceted categories (any-of within a group, all-of across groups). Recipe lists are ranked by engagement — favorites first, then recipes your household has cooked, ones you've viewed, and ones matching your past searches.
- **Shop smarter** — check off grocery items while you shop; checked-off food items can update pantry stock automatically.
- **Track wine** — maintain a wine cellar with bottles, types, countries, regions, vintages, and grape varieties.
- **Add items on the go** — use the mobile app to scan a UPC barcode, look up catalog items, and submit missing products for approval.
- **Share households** — invite family members; recipes, plans, lists, events, pantry, and cellar are household-scoped, and changes notify the other members. One account can belong to several households and switch between them.

Authentication is handled by Google sign-in via OpenID Connect or Discord/Microsoft/Facebook OAuth2 (authorization-code flows use PKCE S256); a single account can link multiple provider logins (never auto-merged by email). Data is scoped by **household**: every member sees the same recipes, plans, lists, events, pantry, and wine cellar. A user can hold memberships in multiple households with a server-stored **active household** selecting which one they see; accepting an invite adds a membership and offers a merge that folds a solely owned household's pantry, cellar, plans, lists, and events into the shared one before dissolving it.

---

## Who is it for?

LENA2 is designed for anyone who wants a centralized, self-hosted alternative to commercial meal-planning and pantry apps:

- Home cooks who plan weekly menus.
- Families sharing a kitchen and shopping list.
- Wine collectors who want a simple cellar log.
- Small households that want to track what is in the pantry and what needs to be bought.

It is a single-tenant application: you run your own instance and your data lives in your database.

---

## Technology overview

| Layer | Technology |
|---|---|
| **Backend** | Go 1.26, Echo, `graphql-go`, sqlc, PostgreSQL 18 + pgvector |
| **Web app** | Next.js 16 (App Router), TypeScript, Material UI, React Query |
| **Mobile app** | Flutter 3.47+, `google_sign_in`, `graphql_flutter`, `mobile_scanner` |
| **Database** | PostgreSQL 18 with schema-per-domain, pgvector for recipe embeddings |
| **Reverse proxy** | Caddy 2 |
| **Dev / deploy** | Docker Compose |
| **Authentication** | Google OIDC + Discord/Microsoft/Facebook OAuth2 (PKCE), exchanged for LENA session tokens |

The web and mobile clients talk to a single **GraphQL Backend-for-Frontend (BFF)** exposed at `/graphql`. The Go backend uses `sqlc` for type-safe SQL queries and `testcontainers` for integration tests.

---

## Web functionality

The web dashboard (`clients/web`) is an admin-style application with a navigation drawer. It exposes the following areas:

### Dashboard

- `/` — today’s meal-plan slots with friendly empty-slot prompts, analytics-driven recipe suggestion cards (ingredient overlap, rating recency, category affinity, and household trending), pending household invites, and a **Time to restock** card listing depleted pantry items ranked by household engagement.

### Food inventory

- `/inventory/items` — catalog of food items with ranked search, favorite, stock tracking, minimum-quantity thresholds, and CRUD.
- `/inventory/brands` — reference data for product brands.
- `/inventory/categories` — item categories.
- `/inventory/flavor-profiles` — flavor tags.
- `/inventory/food-flavors` — item-to-flavor associations.
- `/inventory/food-nutrients` — item-to-nutrient associations.
- `/inventory/nutrient-types` — nutrient reference data.
- `/inventory/ingredients` — generic-ingredient catalog (admin): search, create, edit, deactivate, and merge (the dedupe safety valve). Each ingredient and item carries `contains` / `may_contain` allergen flags, edited in the row dialogs.
- `/inventory/allergens` — the allergen registry (admin). Member records and entity flags both key on these rows; removal is deactivation so references stay resolvable.
- `/inventory/allergen-suggestions` — the AI flag review queue (admin): run the assistant over a recipe, then accept or dismiss each proposed flag. Nothing applies without an explicit accept.

### Recipes & planning

- `/recipes` — recipe list, detail, and edit with ingredients and steps (including per-step timing metadata). Category filters narrow the list server-side; assignments happen on the detail page. `/recipes/categories` is the admin's taxonomy manager (groups, exclusivity, display order, categories). Recipes surface **allergy warnings** — a chip on list rows and an alert on the detail page naming which household member conflicts and how. Any household member can also apply **household tweaks** — swap, adjust, remove, or add ingredient lines and steps as a per-household delta over the shared recipe; the tweaks panel lives on the detail page (and in the mobile recipe screen), with an Original recipe toggle, a stale banner when the canonical recipe changes, and deltas applied to planning, grocery lists, scaling, event snapshots, and AI suggestions.
- `/meal-plans` — weekly meal plans with daily slots and per-slot servings; slot rows carry the same allergy-warning chips.
- `/events` — food events: dish slots with granularity-snapped serve times, per-slot recipe snapshots (steps + ingredients copied per event so edits never touch the shared recipe), servings scaling, and a generated cooking timeline with conflict warnings and allergy warnings.
- `/grocery-lists` — shopping lists generated from meal plans, with check-off, plus a **Time to restock** section listing pantry items at or below their minimum, ranked by household engagement. Flagged rows show an allergy-warning chip. Each list can be routed through a household-defined **store**: the server groups items into the store's ordered aisles, learns per-item positions from check-off order, and honors your manual arrangement (drag handles or the move-to menu) across regenerated lists. Web and mobile render the same server-computed order. When `LENA_INSTACART_API_KEY` is configured, a **Shop with Instacart** action pushes the unchecked lines to Instacart's Developer Platform and hands back a shareable shopping-list link (copy or open — checkout happens on Instacart's site).
- **OCR recipe import** — bulk-import scanned cookbook pages, recipe cards, and photos using local OCR and a local LLM. Admin-only; see `docs/recipe-ocr-usage.md`.

### Wine cellar

- `/wine/bottles` — bottles in the cellar.
- `/wine/countries`, `/wine/regions`, `/wine/types`, `/wine/vintages` — reference data.

### Administration

- `/users` — user management (admin only).
- `/items/pending` — approve or reject user-submitted items (admin only).
- `/household` — household members, roles, and invites; each member manages their own allergy/dietary records here. A **Your households** section lists every membership with the active one marked — switch between them, create a new household, or leave any of them; accepting an invite while you solely own a household offers a merge.
- `/profile` — current-user profile (name, backup email, discoverability, and your own allergy/dietary records).

The header bell shows unread household notifications — meal-plan, grocery-list, event, and invite changes made by other members — with deep links to the changed item. An hourly sweep also produces reminders: protein defrosting (scaled by weight — 48 hours up to 8 lbs, then 24 hours per additional 4 lbs), multi-day recipe prep (steps of 24h+), and pantry items nearing their expiry date. Expiry reminders include an **Add to list** action that drops a replacement onto the current grocery list.

`/notifications` manages delivery per user: each category (household, events, meal reminders, expiry) can be toggled off entirely, opted into **mobile push** independently (push is off by default — the `_all` row is the master switch), or muted for a preset window, and a global mute pauses everything.

**Mobile push** (Android) delivers notifications through Firebase Cloud Messaging: the server writes a `push_delivery` outbox row in the same transaction as the event, and a `DeliveryWorker` drains it — `LENA_PUSH_PROVIDER=log` prints redacted sends for dev/e2e, `fcm` needs `LENA_FCM_CREDENTIALS_FILE` (see `docs/deployment.md`). Tapping a push deep-links to the subject (recipe, pantry item, event) via the shared notification-routing map.

Most catalog pages require an **admin** role; day-to-day pantry and planning features are available to all authenticated users.

---

## Mobile functionality

The Flutter app (`clients/mobile`) is intended for quick, on-the-go actions:

- **Google sign-in** — securely persists the ID token to device storage; signed-out or expired tokens return to the login screen.
- **Dashboard** — today’s meal-plan slots, recommended recipes, and pending household invites.
- **Grocery lists** — browse lists, check items off while shopping, and view list details grouped by the selected store's aisles; reorder within an aisle by dragging, or move items between aisles from the row menu. A **Shop with Instacart** action (when the server has a provider configured) generates a shoppable link for the unchecked items and opens it externally via `url_launcher`.
- **Events** — browse food events, manage dish slots (recipe or free-form, meal type, servings, serve time), and view the cooking timeline as a step-by-step checklist.
- **Pantry** — view pantry quantities and minimums for tracked items.
- **Scan** — use the camera to scan a barcode, look up the item by UPC, add or remove stock, or submit a missing item for admin approval.
- **Household** — members, roles, and invites; the tab badge shows unread household notifications. An active-household header opens a switcher sheet covering every membership (switch, leave, or create a new one), and the same merge prompt as the web appears on invite accept. Your allergy/dietary records live here too — flag rows carry warning badges that open a detail dialog naming the conflict.
- **Meal plans** — a week view presents the plan as a 7-day × Breakfast/Lunch/Dinner grid on wide panes and scrolling day sections on phones; slots add/edit/remove in place, **Suggest meals** proposes recipes from the server's AI (reasons + expiring items), and a **Nutrition** sheet renders aggregate nutrients and warnings.
- **Cook mode** — a full-screen step pager on recipes: big `N / total` counter, headline instructions, step-type and hands-off badges, and tap-to-start countdown chips for timed steps. The screen stays awake while cooking (`wakelock_plus`); ingredients sit in a side rail on wide screens or a bottom sheet on phones.
- **Adaptive layouts** — Material 3 breakpoints (`lib/responsive.dart`): bottom navigation under 600dp, a `NavigationRail` at 600dp+, and list screens (recipes, meal plans, grocery, events, pantry items, bottles) split into a two-pane master/detail (`AdaptiveDetail`) at 840dp+. Form content caps at 840dp centered.
- **Navigation** — Dashboard, Grocery, Events, Scan, Pantry, Household, Ask Dot, and More — bottom bar on phones, rail on tablets.

UPC normalization follows this rule: 12 digits go to `upc12`, 13 digits are left-padded with `0` and treated as `upc14`, and 14 digits go straight to `upc14`. Anything else is considered not found.

---

## AI assistant

LENA can run a local LLM (Ollama) as an optional assistant. Set `LENA_AI_PROVIDER=ollama` plus `LENA_OLLAMA_URL`/`LENA_AI_MODEL`, or run `docker compose --profile ai up` to start the bundled Ollama service. With no provider configured every AI surface stays hidden and queries return `UNAVAILABLE`.

- **Ask Dot** (`/assistant` on web via the header's second row, the Assistant tab on mobile) — free-form questions answered with read-only, household-scoped tools (pantry, expiring items, meal plan, recipes, household tastes, cellar, event timeline). Each reply lists the lookups it made.
- **Suggest Meals** (`/meal-plans`) — proposes recipes for open slots, informed by stock, near-expiry items, and household taste analytics; applied via the normal meal-slot mutations after review.
- **Suggest Fixes** (`/events` timeline) — proposes schedule fixes (shift serve time, reassign appliance, adjust duration/dependency) for timeline conflicts; applied through the existing event mutations.
- **Sommelier & bartender** (`/recipes`) — wine pairings for a recipe and cocktail picks with an in-stock toggle. Both are gated server-side on a stored `birthdate` showing 21+.
- **Semantic recipe search** (`/recipes`, plus Dot's `search_recipes_semantic` tool) — recipes ranked by pgvector cosine distance blended with engagement (favorites, household use, views), so "something cozy for a rainy night" works. Embeddings come from a small Ollama model (`LENA_AI_EMBED_MODEL`, default `nomic-embed-text`, 768 dims); a sweep keeps them fresh whenever recipes are saved or the model changes. No embedder configured → the web toggle hides and semantic mode returns `UNAVAILABLE`; keyword search is untouched.
- **Allergen flag suggestions** (`/inventory/allergen-suggestions`, admin) — the assistant reads a recipe's flaggable lines and proposes `contains`/`may_contain` flags with rationale. Proposals land in a review queue (`inventory.allergen_suggestion`) and apply only when an admin accepts — the flag writes under the reviewer's attribution in the same transaction.

The model never writes — every suggestion is advisory and applies only through the existing mutations. `internal/platform/llm` defines a provider-agnostic interface, so swapping Ollama for a commercial API is a config change plus a small adapter. `LENA_AI_PROVIDER=mock` gives a deterministic canned assistant for e2e.

**On-device inference.** Ask Dot and all four suggestion surfaces can run the model on the user's own hardware instead of the server — the "cool AI" modern phones and browsers advertise. The client drives the same read-only, household-scoped tool catalog and prompts the server advertises (`assistantTools`, `callAssistantTool`, `assistantPrompt`, `prepareAssistantRequest` in the schema), so household authorization never moves to the client. Web tries Chrome's built-in model first, then a lazy-loaded WebLLM download (explicit opt-in, with progress), then falls back to the server. Mobile runs Gemma via `flutter_gemma` with a bundled model manager (download/delete) — configure the source with `LENA_LOCAL_MODEL_URL`/`_TOKEN`/`_ID` dart-defines. Structured suggestions validate local output against the server-prepared context before rendering, and every local path falls back transparently to the server provider.

---

## Quick start

### Run the full stack with Docker Compose

1. Copy the example environment file and fill in your Google OAuth client ID:

   ```bash
   cp .env.example .env
   # edit .env with your GOOGLE_CLIENT_ID and database passwords
   ```

2. Build and start everything:

   ```bash
   docker compose up --build
   ```

3. Open the web app at `http://localhost`.

The GraphQL endpoint is available at `http://localhost/graphql` and the `/health` endpoint at `http://localhost/health`.

### Run the backend locally (no Docker)

1. Start PostgreSQL 18 with the pgvector extension (e.g. the `pgvector/pgvector:pg18` image) and create a `lena` database plus a `lena_app` user.
2. Apply migrations:

   ```bash
   migrate -path ./migrations -database "postgres://lena_app:password@localhost:5432/lena?sslmode=disable" up
   ```

3. Run the Go server:

   ```bash
   go run ./cmd/lena
   ```

### Run the web app locally

```bash
cd clients/web
npm install
npm run dev
```

The dev server starts on `http://localhost:3000` by default.

### Run the mobile app locally

```bash
cd clients/mobile
flutter pub get
flutter run \
  --dart-define=LENA_API_URL=http://10.0.2.2:8080/graphql \
  --dart-define=LENA_GOOGLE_SERVER_CLIENT_ID=<your-web-client-id>
```

For a physical device or iOS simulator, replace the API URL with the host machine’s IP or `localhost` accordingly. See `clients/mobile/README.md` for full iOS and Android Google sign-in configuration.

---

## Authentication

LENA2 uses Google OIDC ID tokens:

1. The user signs in with Google in the web or mobile client.
2. The client sends the ID token as `Authorization: Bearer <token>` on every request.
3. The backend validates the token against the configured issuer and audience.
4. The user record is upserted in the `identity.users` table and a `user_id` is placed in the request context.

**Sessions.** Set `LENA_SESSION_SECRET` (≥32 bytes; the API refuses to boot with a weaker one) and sign-in exchanges the provider credential for a LENA session: a short-lived signed access token (`iss=lena`, ~15 min) plus a rotating refresh token (~30 days, stored only as a hash). Clients refresh instead of re-signing in with a provider; replaying a rotated token revokes the whole session family, and admin deactivation revokes a user's sessions immediately. Unset keeps OIDC-only mode. See `docs/auth-oidc.md` §8 and `docs/refresh-tokens-plan.md`.

Initial admins are promoted by adding their email to `LENA_ADMIN_EMAILS`. Protected admins can be listed in `LENA_PROTECTED_EMAILS` so they cannot be banned or demoted.

## API idempotency

Every GraphQL mutation is safe to retry — client retries, mobile offline replays, and double-submits apply exactly once:

- Both clients send an `Idempotency-Key: <uuid>` header on mutations. The server stores `(user_id, key) → response`; a replayed request returns the stored response verbatim with `Idempotency-Replayed: true` and never re-executes the resolver.
- The same key with a different payload is rejected (`IDEMPOTENCY_KEY_REUSED`); a duplicate arriving while the first is still running waits briefly, then replays (`IDEMPOTENCY_IN_FLIGHT` if the twin stalls).
- Requests without a key are deduplicated by a payload hash inside a short window (default 30 s), which covers double-clicks and browser retries.
- Queries bypass deduplication entirely. Keys are scoped to the authenticated user; the table self-cleans via TTL.
- Knobs: `LENA_IDEMPOTENCY_{ENABLED,KEY_TTL,AUTO_TTL,IN_FLIGHT_TTL,WAIT_TIMEOUT}`. Details in `docs/idempotency-plan.md`.

Semantic note: `generateGroceryList` regenerates in place — a second call for the same plan replaces the generated lines in the existing list rather than creating a duplicate, and manual lines are preserved.

---

## Testing

### Go tests

```bash
# Unit + integration (Docker Desktop required on Windows for testcontainers)
go test ./cmd/... ./internal/...

# Unit only, skipping integration suites
go test -short ./cmd/... ./internal/...
```

### Web tests

```bash
cd clients/web
npm test                 # Jest unit/component tests
npm run test:e2e         # Playwright end-to-end tests
```

The Playwright suite runs against the Docker Compose stack using a local OIDC test issuer. Bring the stack up with:

```bash
docker compose -f docker-compose.yml -f docker-compose.e2e.yml --profile seed up -d --build
```

### Mobile tests

```bash
cd clients/mobile
flutter analyze
flutter test --coverage
```

All three surfaces feed `tools/coveragegate` — see `docs/testing.md` §
Coverage gate for the enforced floors and exclusion policy.

A screenshot walk (`integration_test/screenshot_test.dart`) captures all 27
reachable screens on an emulator against the seeded `lena2shots` stack —
mint a token from the test issuer and pass it as `LENA_DEBUG_ID_TOKEN`:

```bash
flutter drive --driver=test_driver/integration_test.dart \
  --target=integration_test/screenshot_test.dart -d emulator-5554 \
  --dart-define=LENA_API_URL=http://10.0.2.2/graphql \
  --dart-define=LENA_DEBUG_ID_TOKEN=<token>
```

PNGs land in `clients/mobile/mobile-shots/` when the walk completes.


---

## CI

`.github/workflows/test.yml` runs on pushes to `main`, `phase-*`, `mobile-redesign-*`, and `len-*`/`LEN-*` (ticket) branches, and on PRs targeting `main`:

- **Go** — build, vet, `gofmt`, tests with coverage.
- **Lint** — `golangci-lint` (incl. `gosec`), plus a `pip-audit` gate on the
  pinned OCR dependencies (`ocr-import` job).
- **Web** — TypeScript, ESLint, Jest, Next.js build.
- **E2E** — Playwright suite against the Docker stack.
- **Mobile** — `flutter pub get`, `flutter analyze`, `flutter test --coverage`.
- **Coverage** — `tools/coveragegate` enforces the floors in
  `tools/coveragegate/floors.json` across all three surfaces: every Go
  package and every web/mobile source file must clear 70% (tiny files
  exempt), with surface totals at go ≥70 / web ≥75 / mobile ≥70. Generated
  code (sqlc, gomock, platform-plugin wrappers) is excluded via the config.
- **Docker** — builds `lena2-api` and `lena2-web` images.

`.github/workflows/cleanup.yml` purges old workflow artifacts on a schedule.

---

## Project layout

```text
.
├── cmd/lena              # Go API entry point
├── cmd/testissuer        # Local OIDC test issuer for e2e tests
├── internal/             # Go packages (bff, inventory, recipe, mealplan, grocery, etc.)
│   └── platform/         # Shared utilities (config, auth, dbtx, ocrclient, testenv)
├── migrations/           # PostgreSQL migrations and seed data
├── clients/
│   ├── web/              # Next.js admin dashboard
│   └── mobile/           # Flutter mobile client
├── docs/                 # Architecture, data model, deployment, and testing guides
├── docker-compose.yml    # Production-like local stack
├── docker-compose.e2e.yml # E2E test stack override
├── Dockerfile            # Go API image
├── Caddyfile             # Reverse proxy config
└── .env.example          # Required environment variables
```

---

## Useful notes

- **Data isolation** — shared data is scoped by `household_id` resolved from the authenticated user in the request context; the backend never trusts a client-supplied household or user parameter.
- **Household sharing** — invites move a user's meal plans, grocery lists, and food events into the target household inside the accept transaction; mutations notify the other members.
- **Admin vs member** — catalog reference data and user management require the `admin` role; meal plans, grocery lists, pantry, events, and cellar are per-household.
- **Event snapshots** — linking a recipe to an event copies its steps and ingredients into the slot; edits inside the event never modify the shared recipe, and `syncEventRecipe` re-copies on demand.
- **UPC lookup** — mobile and web can query `itemByUpc` to find an item by UPC before adding it to pantry.
- **Item submission** — non-admin users can `submitItem` for items not yet in the catalog. Submitted items are visible only to their creator until an admin approves them.
- **Grocery sync** — checking a grocery item off can increase `inventory.user_item` stock by the quantity needed; unchecking decreases it.
- **Ingredients vs items** — recipes, meal slots, grocery lines, and import reviews reference generic `inventory.ingredient` rows ("corn"); `item_id` is an optional preferred brand. Stock rollups and check-offs resolve ingredient→brand via the catalog link (`item.ingredient_id`), a per-household override, and a remembered "usual brand" (`household_ingredient_item`) — the first check-off of an ingredient line asks which brand you bought and remembers it. Unlinked items still work as pantry inventory, they just don't satisfy ingredient-level rollups. See `docs/ingredient-layer-plan.md`.
- **Rotating sessions** — provider credentials exchange for a LENA session (short-lived `iss=lena` access token + ~30-day rotating refresh token); replaying a rotated token revokes the whole family.
- **Nonce CSP** — the web tier issues a per-request Content-Security-Policy nonce (`clients/web/proxy.ts`): `script-src 'self' 'nonce-…' 'strict-dynamic'` with no `unsafe-inline`; `style-src` keeps `unsafe-inline` for Emotion/MUI. Caddy carries every other security header (XFO, XCTO, Referrer-Policy, Permissions-Policy, HSTS).

---

## Documentation

- `docs/deployment.md` — full Docker Compose and environment setup.
- `docs/testing.md` — test suite details.
- `docs/auth-oidc.md` — authentication flow.
- `docs/graphql-schema.md` — GraphQL API reference.
- `docs/postgres-data-model.md` — database schema overview.
- `docs/design.md` — shared design language: sage/cream palette, light + dark token tables, type ramp, spacing/motion spec.
- `clients/web/README.md` — web client setup.
- `clients/mobile/README.md` — Flutter client setup.
