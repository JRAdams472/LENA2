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
- **Share a household** — invite family members; recipes, plans, lists, events, pantry, and cellar are household-scoped, and changes notify the other members.

Authentication is handled by Google sign-in via OpenID Connect. Data is scoped by **household**: every member sees the same recipes, plans, lists, events, pantry, and wine cellar, and accepting a household invite merges the new member's existing plans, lists, and events into the shared household.

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
| **Backend** | Go 1.27, Echo, `graphql-go`, sqlc, PostgreSQL 16 |
| **Web app** | Next.js 16 (App Router), TypeScript, Material UI, React Query |
| **Mobile app** | Flutter 3.47+, `google_sign_in`, `graphql_flutter`, `mobile_scanner` |
| **Database** | PostgreSQL 16 with schema-per-domain |
| **Reverse proxy** | Caddy 2 |
| **Dev / deploy** | Docker Compose |
| **Authentication** | Google OIDC ID tokens as JWT bearer tokens |

The web and mobile clients talk to a single **GraphQL Backend-for-Frontend (BFF)** exposed at `/graphql`. The Go backend uses `sqlc` for type-safe SQL queries and `testcontainers` for integration tests.

---

## Web functionality

The web dashboard (`clients/web`) is an admin-style application with a navigation drawer. It exposes the following areas:

### Dashboard

- `/` — today’s meal-plan slots, analytics-driven recipe suggestions (ingredient overlap, rating recency, category affinity, and household trending), pending household invites, and a **Running low** card listing depleted pantry items ranked by household engagement.

### Food inventory

- `/inventory/items` — catalog of food items with ranked search, favorite, stock tracking, minimum-quantity thresholds, and CRUD.
- `/inventory/brands` — reference data for product brands.
- `/inventory/categories` — item categories.
- `/inventory/flavor-profiles` — flavor tags.
- `/inventory/food-flavors` — item-to-flavor associations.
- `/inventory/food-nutrients` — item-to-nutrient associations.
- `/inventory/nutrient-types` — nutrient reference data.

### Recipes & planning

- `/recipes` — recipe list, detail, and edit with ingredients and steps (including per-step timing metadata). Category filters narrow the list server-side; assignments happen on the detail page. `/recipes/categories` is the admin's taxonomy manager (groups, exclusivity, display order, categories).
- `/meal-plans` — weekly meal plans with daily slots and per-slot servings.
- `/events` — food events: dish slots with granularity-snapped serve times, per-slot recipe snapshots (steps + ingredients copied per event so edits never touch the shared recipe), servings scaling, and a generated cooking timeline with conflict warnings.
- `/grocery-lists` — shopping lists generated from meal plans, with check-off, plus a **Suggested Restock** section listing pantry items at or below their minimum, ranked by household engagement.
- **OCR recipe import** — bulk-import scanned cookbook pages, recipe cards, and photos using local OCR and a local LLM. Admin-only; see `docs/recipe-ocr-usage.md`.

### Wine cellar

- `/wine/bottles` — bottles in the cellar.
- `/wine/countries`, `/wine/regions`, `/wine/types`, `/wine/vintages` — reference data.

### Administration

- `/users` — user management (admin only).
- `/items/pending` — approve or reject user-submitted items (admin only).
- `/household` — household members, roles, and invites.
- `/profile` — current-user profile (name, backup email, discoverability).

The header bell shows unread household notifications — meal-plan, grocery-list, event, and invite changes made by other members — with deep links to the changed item. An hourly sweep also produces reminders: protein defrosting (scaled by weight — 48 hours up to 8 lbs, then 24 hours per additional 4 lbs), multi-day recipe prep (steps of 24h+), and pantry items nearing their expiry date. Expiry reminders include an **Add to list** action that drops a replacement onto the current grocery list.

`/notifications` manages delivery per user: each category (household, events, meal reminders, expiry) can be toggled off entirely or muted for a preset window, and a global mute pauses everything.

Most catalog pages require an **admin** role; day-to-day pantry and planning features are available to all authenticated users.

---

## Mobile functionality

The Flutter app (`clients/mobile`) is intended for quick, on-the-go actions:

- **Google sign-in** — securely persists the ID token to device storage; signed-out or expired tokens return to the login screen.
- **Dashboard** — today’s meal-plan slots, recommended recipes, and pending household invites.
- **Grocery lists** — browse lists, check items off while shopping, and view list details.
- **Events** — browse food events, manage dish slots (recipe or free-form, meal type, servings, serve time), and view the cooking timeline as a step-by-step checklist.
- **Pantry** — view pantry quantities and minimums for tracked items.
- **Scan** — use the camera to scan a barcode, look up the item by UPC, add or remove stock, or submit a missing item for admin approval.
- **Household** — members, roles, and invites; the tab badge shows unread household notifications.
- **Bottom navigation** — Dashboard, Grocery, Events, Scan, Pantry, Household.

UPC normalization follows this rule: 12 digits go to `upc12`, 13 digits are left-padded with `0` and treated as `upc14`, and 14 digits go straight to `upc14`. Anything else is considered not found.

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

1. Start PostgreSQL 16 and create a `lena` database plus a `lena_app` user.
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
flutter test
```

---

## CI

`.github/workflows/test.yml` runs on `main`, `phase-*`, and `mobile-redesign-*` branches:

- **Go** — build, vet, `gofmt`, tests with coverage.
- **Web** — TypeScript, ESLint, Jest, Next.js build.
- **E2E** — Playwright suite against the Docker stack.
- **Mobile** — `flutter pub get`, `flutter analyze`, `flutter test`.
- **Docker** — builds and pushes `lena2-api` and `lena2-web` images.

`.github/workflows/ci.yml` and `.github/workflows/docker.yml` cover linting and image publishing.

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
- **No refresh tokens** — clients must re-sign in with Google when the ID token expires.

---

## Documentation

- `docs/deployment.md` — full Docker Compose and environment setup.
- `docs/testing.md` — test suite details.
- `docs/auth-oidc.md` — authentication flow.
- `docs/graphql-schema.md` — GraphQL API reference.
- `docs/postgres-data-model.md` — database schema overview.
- `clients/web/README.md` — web client setup.
- `clients/mobile/README.md` — Flutter client setup.
