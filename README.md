# LENA2

A self-hosted kitchen assistant for tracking pantry stock, planning meals, managing recipes, building grocery lists, and keeping a wine cellar — accessible from a web dashboard and a Flutter mobile app.

---

## What is LENA2?

LENA2 is a personal, privacy-first household management system. It replaces scattered notes, spreadsheets, and shopping-list apps with one place to:

- **Track food and supplies** — keep a catalog of items and your own pantry quantities, minimum-stock thresholds, and favorites.
- **Plan meals** — build weekly meal plans with servings and generate grocery lists automatically.
- **Manage recipes** — store ingredients, steps, and portions, then pull them into meal plans.
- **Shop smarter** — check off grocery items while you shop; checked-off food items can update pantry stock automatically.
- **Track wine** — maintain a wine cellar with bottles, types, countries, regions, vintages, and grape varieties.
- **Add items on the go** — use the mobile app to scan a UPC barcode, look up catalog items, and submit missing products for approval.

Every user owns their own data. Authentication is handled by Google sign-in via OpenID Connect, and the backend stores a `user_id` so all per-user records (meal plans, grocery lists, pantry entries, wine cellar) are isolated.

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

- `/` — today’s meal-plan slots and recommended recipes.

### Food inventory

- `/inventory/items` — catalog of food items with search, favorite, stock tracking, and CRUD.
- `/inventory/brands` — reference data for product brands.
- `/inventory/categories` — item categories.
- `/inventory/flavor-profiles` — flavor tags.
- `/inventory/food-flavors` — item-to-flavor associations.
- `/inventory/food-nutrients` — item-to-nutrient associations.
- `/inventory/nutrient-types` — nutrient reference data.

### Recipes & planning

- `/recipes` — recipe list, detail, and edit with ingredients and steps.
- `/meal-plans` — weekly meal plans with daily slots.
- `/grocery-lists` — shopping lists generated from meal plans, with check-off.

### Wine cellar

- `/wine/bottles` — bottles in the cellar.
- `/wine/countries`, `/wine/regions`, `/wine/types`, `/wine/vintages` — reference data.

### Administration

- `/users` — user management (admin only).
- `/items/pending` — approve or reject user-submitted items (admin only).
- `/profile` — current-user profile.

Most catalog pages require an **admin** role; day-to-day pantry and planning features are available to all authenticated users.

---

## Mobile functionality

The Flutter app (`clients/mobile`) is intended for quick, on-the-go actions:

- **Google sign-in** — securely persists the ID token to device storage; signed-out or expired tokens return to the login screen.
- **Dashboard** — today’s meal-plan slots and recommended recipes.
- **Grocery lists** — browse lists, check items off while shopping, and view list details.
- **Pantry** — view pantry quantities for tracked items.
- **Scan** — use the camera to scan a barcode, look up the item by UPC, add or remove stock, or submit a missing item for admin approval.
- **Bottom navigation** — Dashboard, Grocery, Scan (center action), Pantry.

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
docker compose -f docker-compose.yml -f docker-compose.e2e.yml up -d --build
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

- **Data isolation** — every per-user table is scoped by `user_id` from the request context; the backend never trusts a `userId` parameter from the client.
- **Admin vs member** — catalog reference data and user management require the `admin` role; meal plans, grocery lists, pantry, and cellar are per-user.
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
- `docs/mobile-redesign.md` — mobile redesign plan (p0–p5).
- `clients/web/README.md` — web client setup.
- `clients/mobile/README.md` — Flutter client setup.
