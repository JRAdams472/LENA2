---
agent: devin-local
session: enshrined-taker
created: 2026-09-04T03:27:04Z
---
# LENA Go + PostgreSQL Ground-Up Rewrite — Modular Monolith with GraphQL BFF

A complete re-implementation of LENA in Go and PostgreSQL as a single modular monolith with an internal GraphQL BFF, strict per-user/catalog data split, no cross-domain SQL, and Flutter + Next.js clients.

# LENA Go + PostgreSQL Ground-Up Rewrite — Specification & Megaplan

## 1. Decisions Captured from Clarification

| Topic | Decision |
|---|---|
| Physical backend | **Modular monolith that can split later** — one Go binary with internal domain packages and clean package boundaries. |
| BFF placement | **BFF package inside the same Go binary** (`internal/bff`) rather than a separate deployable. It owns the GraphQL endpoint. |
| Frontends | **Flutter mobile** (primary) **plus upgrade the Next.js web app to also consume GraphQL**. |
| Cross-domain rule | **No cross-domain database queries / SQL joins.** Foreign keys are stored as IDs in one module; the BFF (or a higher layer) calls the owning module to hydrate names, etc. |
| Authentication | **Google now, multi-OIDC ready** — schema stores `(provider, external_subject)` and the JWT layer accepts a configurable issuer/audience list. |
| Data migration | **Fresh start** — keep all existing domains, but no migration from the current SQL Server instance. Seed reference data if desired. |
| Go / Postgres stack | **Implemented**: `graph-gophers/graphql-go`, Echo, `pgx` + `sqlc`, `golang-migrate`, `go-playground/validator`, structured logging (e.g. `uber-go/zap` or `rs/zerolog`). |
| Business logic placement | **All business logic lives in Go code** — no stored procedures, triggers, views with logic, or complex SQL-side transformations. SQL is restricted to CRUD, filtering, joins, and simple aggregations. |
| Output now | **Specification and planning documents only** under `docs/`. No Go/Flutter code is to be written until the specification is approved. |

---

## 2. High-Level Architecture

```
┌──────────────────────────────────────────────────────────────┐
│  Flutter mobile  │  Next.js web (upgraded to GraphQL)        │
└────────┬───────────────────────────────┬─────────────────────┘
         │                              │
         └──────────────────┬───────────┘
                            │ HTTPS / HTTP2
              ┌─────────────▼─────────────┐
              │  Caddy reverse proxy      │
              │  /graphql → api:8080      │
              │  / → web:3000             │
              └─────────────┬─────────────┘
                            │
              ┌─────────────▼─────────────┐
              │  Go monolith (cmd/lena)   │
              │  • HTTP router (Echo)     │
              │  • GraphQL BFF (graph-gophers)   │
              │  • Auth middleware        │
              └─────────────┬─────────────┘
                            │ in-process Go calls
        ┌───────────────────┼───────────────────┐
        │                   │                   │
  ┌─────▼─────┐      ┌──────▼──────┐     ┌─────▼─────┐
  │ identity  │      │   catalog   │     │ userprefs │
  │ module    │      │  modules    │     │ module    │
  └─────┬─────┘      └──────┬──────┘     └─────┬─────┘
        │                   │                  │
        │              ┌────▼────┐             │
        │              │ platform│             │
        │              │  (db,   │             │
        │              │ config, │             │
        │              │ logger) │             │
        │              └────┬────┘             │
        └───────────────────┼───────────────────┘
                            │
                 ┌──────────▼──────────┐
                 │  PostgreSQL 16      │
                 │  schema per domain  │
                 └─────────────────────┘
```

- **One binary** keeps the self-hosting and ops story simple.
- **Domain modules** are isolated packages with their own SQL queries, service interfaces, and models.
- **BFF (`internal/bff`)** is the only package that is allowed to call more than one domain module to assemble a GraphQL response.
- **GraphQL is the single public API** for both Flutter and Next.js.

---

## 3. Technology Stack

| Layer | Choice | Rationale |
|---|---|---|
| Language | Go 1.23+ | Static, fast, single binary, excellent PostgreSQL tooling. |
| HTTP router | `github.com/labstack/echo/v4` | Mature, middleware-friendly, easy CORS/JWT wiring. `net/http` + `chi` is acceptable if preferred. |
| GraphQL | `github.com/graph-gophers/graphql-go` | Schema-first, reflection-based resolvers, no code generation. |
| DB driver | `github.com/jackc/pgx/v5` | Best PostgreSQL driver, pool support, `pgxpool`. |
| DB code gen | `sqlc` (`github.com/sqlc-dev/sqlc`) | Type-safe queries from `.sql` files; avoids ORM magic. |
| Migrations | `golang-migrate/migrate` | Industry standard, supports plain `.sql` up/down files. |
| Validation | `github.com/go-playground/validator/v10` | Struct tag validation, easy to integrate with GraphQL resolvers. |
| JWT / OIDC | `github.com/golang-jwt/jwt/v5` or `github.com/lestrrat-go/jwx/v2` | Validate ID tokens, support multiple issuers. |
| Logging | `github.com/rs/zerolog` or `github.com/uber-go/zap` | Structured JSON, no PII at `info` level. |
| Config | `github.com/kelseyhightower/envconfig` + `.env` | Docker-friendly, no secrets in repo. |
| Testing | `testcontainers-go` modules for Postgres | Spin up real DB in tests. |
| Flutter | `graphql_flutter` package | Apollo-compatible GraphQL client. |
| Web | Apollo Client or urql | Upgrade Next.js to query the same `/graphql` endpoint. |

---

## 4. Project Layout

```
lena/
├── cmd/lena/                 # main.go: wire modules, start HTTP+GraphQL server
├── internal/
│   ├── bff/                  # GraphQL schema, resolvers, orchestration
│   │   ├── graph/
│   │   │   ├── schema.graphqls
│   │   │   ├── resolver.go
│   │   │   └── model/        # custom scalar helpers (e.g. Time)
│   │   ├── auth.go           # JWT validation + current user
│   │   └── loaders.go        # optional DataLoader implementations
│   ├── identity/             # users, providers, auth
│   │   ├── service.go
│   │   ├── sqlc/
│   │   └── migrations/
│   ├── inventory/            # items, brands, categories, flavors, nutrients
│   ├── wine/                 # wine definitions, countries, regions, types
│   ├── recipe/               # recipes, ingredients, steps
│   ├── mealplan/             # meal plans, slots, slot items
│   ├── grocery/              # grocery lists + generation
│   ├── userprefs/            # user_item, user_bottle, user_recipe_preference
│   └── platform/
│       ├── postgres/         # pgxpool, tx management
│       ├── config/           # env config
│       ├── logger/           # zerolog setup
│       ├── pagination/       # Page, PageSize, total, clamp
│       └── validator/        # shared validator instance
├── migrations/               # golang-migrate .sql files, grouped by domain
├── docker-compose.yml
├── .env.example
└── docs/
    ├── go-rewrite-spec.md
    ├── postgres-data-model.md
    ├── graphql-schema.md
    ├── graphql-bff-orchestration.md
    ├── flutter-nextjs-integration.md
    ├── migration-runbook.md
    └── deployment.md
```

---

## 5. Domain Module Responsibilities

### 5.1 `identity`
- Store `users` with `(provider, external_subject)` unique key.
- Upsert user on authenticated request (refresh `email`, `display_name`, `last_login_at`).
- Provide `CurrentUser` context to other modules.

### 5.2 `inventory` (catalog)
- Global reference data: `item`, `brand`, `category`, `flavor_profile`, `nutrient_type`, `food_flavor`, `food_nutrient`.
- Purely catalog; no per-user stock.
- CRUD and search endpoints (used by BFF resolvers).

### 5.3 `wine` (catalog)
- Global reference data: `bottle` (definition), `country`, `region`, `type`, `vintage`, `grape_variety`, `bottle_flavor_profile`, `bottle_grape_variety`.
- No per-user cellar holding.

### 5.4 `recipe` (catalog)
- Global reference data: `recipe`, `recipe_item`, `recipe_step`.
- `is_favorite` is **not** here; it lives in `userprefs`.

### 5.5 `userprefs`
- Per-user state: `user_item` (stock & fav), `user_bottle` (cellar holding), `user_recipe_preference`.
- Owned entirely by a `user_id`.
- Other modules store only the foreign key; they never join to `user_item`.

### 5.6 `mealplan`
- `meal_plan`, `meal_slot`, `meal_slot_item` — all per-user.
- `meal_slot` stores `recipe_id`; `meal_slot_item` stores `item_id`.
- No SQL joins to `recipe` or `inventory`. BFF resolves IDs.

### 5.7 `grocery`
- `grocery_list`, `grocery_list_item`.
- `grocery_list` may reference `meal_plan_id`.
- Generation logic lives in `grocery` module and reads `meal_plan` (allowed within its own domain) plus per-user stock from `userprefs` via explicit call, **not** a SQL join.

---

## 6. PostgreSQL Data Model

### 6.1 Schema layout

Use one PostgreSQL schema per domain to keep boundaries explicit:

```sql
CREATE SCHEMA IF NOT EXISTS identity;
CREATE SCHEMA IF NOT EXISTS inventory;
CREATE SCHEMA IF NOT EXISTS wine;
CREATE SCHEMA IF NOT EXISTS recipe;
CREATE SCHEMA IF NOT EXISTS mealplan;
CREATE SCHEMA IF NOT EXISTS grocery;
```

### 6.2 Type mapping (SQL Server → PostgreSQL)

| SQL Server | PostgreSQL |
|---|---|
| `INT IDENTITY(1,1)` | `BIGSERIAL` or `SERIAL` (use `BIGSERIAL` for new keys, align to `BIGINT`) |
| `DATETIME2` | `TIMESTAMPTZ` |
| `DECIMAL(10,2)` | `NUMERIC(10,2)` |
| `DECIMAL(5,2)` | `NUMERIC(5,2)` |
| `TINYINT` | `SMALLINT` (with CHECK 0..255 where needed) |
| `BIT` | `BOOLEAN` |
| `NVARCHAR(n)` | `VARCHAR(n)` or `TEXT` (Unicode already in Postgres) |

### 6.3 Core table definitions

#### `identity.users`

```sql
CREATE TABLE identity.users (
    user_id             BIGSERIAL PRIMARY KEY,
    provider            VARCHAR(50) NOT NULL DEFAULT 'google',
    external_subject    VARCHAR(255) NOT NULL,
    email               VARCHAR(320) NOT NULL,
    display_name        VARCHAR(200),
    is_active           BOOLEAN NOT NULL DEFAULT TRUE,
    last_login_at       TIMESTAMPTZ,
    created_by          VARCHAR(100) NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by          VARCHAR(100),
    updated_at          TIMESTAMPTZ,
    UNIQUE (provider, external_subject)
);
CREATE INDEX idx_users_email ON identity.users (email);
```

#### `inventory.item` (catalog only)

```sql
CREATE TABLE inventory.item (
    item_id     BIGSERIAL PRIMARY KEY,
    name        VARCHAR(200) NOT NULL,
    brand_id    BIGINT REFERENCES inventory.brand(brand_id),
    upc12       VARCHAR(12),
    upc14       VARCHAR(14),
    category_id BIGINT NOT NULL REFERENCES inventory.category(category_id),
    unit        VARCHAR(20) NOT NULL,
    created_by  VARCHAR(100) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by  VARCHAR(100),
    updated_at  TIMESTAMPTZ,
    UNIQUE (name, brand_id),
    UNIQUE (upc12),
    UNIQUE (upc14)
);
```

#### `inventory.user_item` (per-user)

```sql
CREATE TABLE inventory.user_item (
    user_item_id    BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES identity.users(user_id) ON DELETE CASCADE,
    item_id         BIGINT NOT NULL REFERENCES inventory.item(item_id) ON DELETE CASCADE,
    current_qty     NUMERIC(10,2) NOT NULL DEFAULT 0,
    min_qty         NUMERIC(10,2),
    purchase_at     TIMESTAMPTZ,
    expires_at      TIMESTAMPTZ,
    notes           VARCHAR(500),
    is_favorite     BOOLEAN NOT NULL DEFAULT FALSE,
    created_by      VARCHAR(100) NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by      VARCHAR(100),
    updated_at      TIMESTAMPTZ,
    UNIQUE (user_id, item_id)
);
CREATE INDEX idx_user_item_item_id ON inventory.user_item (item_id);
```

#### `recipe.recipe` (catalog)

```sql
CREATE TABLE recipe.recipe (
    recipe_id           BIGSERIAL PRIMARY KEY,
    recipe_name         VARCHAR(200) NOT NULL UNIQUE,
    description         VARCHAR(500),
    servings            INTEGER,
    prep_time_minutes   INTEGER,
    cook_time_minutes   INTEGER,
    is_active           BOOLEAN NOT NULL DEFAULT TRUE,
    created_by          VARCHAR(100) NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by          VARCHAR(100),
    updated_at          TIMESTAMPTZ
);
```

#### `recipe.user_recipe_preference`

```sql
CREATE TABLE recipe.user_recipe_preference (
    user_id         BIGINT NOT NULL REFERENCES identity.users(user_id) ON DELETE CASCADE,
    recipe_id       BIGINT NOT NULL REFERENCES recipe.recipe(recipe_id) ON DELETE CASCADE,
    is_favorite     BOOLEAN NOT NULL DEFAULT FALSE,
    created_by      VARCHAR(100) NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by      VARCHAR(100),
    updated_at      TIMESTAMPTZ,
    PRIMARY KEY (user_id, recipe_id)
);
```

#### `wine.bottle` (definition, catalog)

```sql
CREATE TABLE wine.bottle (
    bottle_id           BIGSERIAL PRIMARY KEY,
    type_id             BIGINT NOT NULL REFERENCES wine.type(type_id),
    country_id          BIGINT NOT NULL REFERENCES wine.country(country_id),
    region_id           BIGINT NOT NULL REFERENCES wine.region(region_id),
    vintage_year        INTEGER NOT NULL,
    vineyard            VARCHAR(200),
    abv                 NUMERIC(5,2),
    acidity             SMALLINT,
    tannin_level        SMALLINT,
    body                SMALLINT,
    sweetness           SMALLINT,
    oak_integration     BOOLEAN,
    bottle_size         VARCHAR(20) NOT NULL DEFAULT '750ml',
    created_by          VARCHAR(100) NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by          VARCHAR(100),
    updated_at          TIMESTAMPTZ
);
```

#### `wine.user_bottle` (per-user holding)

```sql
CREATE TABLE wine.user_bottle (
    user_bottle_id  BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES identity.users(user_id) ON DELETE CASCADE,
    bottle_id       BIGINT NOT NULL REFERENCES wine.bottle(bottle_id) ON DELETE CASCADE,
    bottle_number   INTEGER,
    quantity        INTEGER NOT NULL DEFAULT 1,
    purchase_at     TIMESTAMPTZ,
    purchase_price  NUMERIC(10,2),
    storage_temp    NUMERIC(5,1),
    location        VARCHAR(100),
    notes           VARCHAR(500),
    is_favorite     BOOLEAN NOT NULL DEFAULT FALSE,
    created_by      VARCHAR(100) NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by      VARCHAR(100),
    updated_at      TIMESTAMPTZ
);
```

#### `mealplan.meal_plan`

```sql
CREATE TABLE mealplan.meal_plan (
    meal_plan_id        BIGSERIAL PRIMARY KEY,
    user_id             BIGINT NOT NULL REFERENCES identity.users(user_id) ON DELETE CASCADE,
    plan_name           VARCHAR(200) NOT NULL,
    week_start_date     DATE NOT NULL,
    week_start_day_of_week SMALLINT NOT NULL DEFAULT 0,
    is_active           BOOLEAN NOT NULL DEFAULT TRUE,
    created_by          VARCHAR(100) NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by          VARCHAR(100),
    updated_at          TIMESTAMPTZ
);
CREATE INDEX idx_meal_plan_user_week ON mealplan.meal_plan (user_id, week_start_date);
```

#### `grocery.grocery_list`

```sql
CREATE TABLE grocery.grocery_list (
    grocery_list_id     BIGSERIAL PRIMARY KEY,
    user_id             BIGINT NOT NULL REFERENCES identity.users(user_id) ON DELETE CASCADE,
    meal_plan_id        BIGINT REFERENCES mealplan.meal_plan(meal_plan_id),
    generated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by          VARCHAR(100) NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by          VARCHAR(100),
    updated_at          TIMESTAMPTZ
);
```

> Remaining tables (`category`, `brand`, `flavor_profile`, `food_flavor`, `food_nutrient`, `nutrient_type`, `country`, `region`, `type`, `vintage`, `grape_variety`, `bottle_flavor_profile`, `bottle_grape_variety`, `recipe_item`, `recipe_step`, `meal_slot`, `meal_slot_item`, `grocery_list_item`) follow the same pattern and are detailed in `docs/postgres-data-model.md`.

---

## 7. The "No Cross-Domain SQL" Rule

- Every module's `sqlc` queries may only touch tables in its own schema (and `identity.users` if explicitly needed for scoping).
- A `mealplan.meal_slot` stores `recipe_id`. The `mealplan` module **does not** know recipe names.
- A `grocery.grocery_list_item` stores `item_id` and `manual_item_name` (fallback). The `grocery` module **does not** join to `inventory.item`.
- The `bff` resolvers are the only code that calls `recipe.FindByID()` from a `MealSlot` resolver, `inventory.FindByID()` from a `GroceryListItem` resolver, etc.
- For performance, the BFF may use a per-request or DataLoader cache so repeated IDs are batched. This is an implementation detail, not a relaxation of the rule.

---

## 7.5 The "No Business Logic in SQL" Rule

Business logic from the original LENA stored procedures must **not** be ported into PostgreSQL. SQL in this rewrite is only for data access.

### 7.5.1 Allowed in SQL

- Simple `INSERT`, `SELECT`, `UPDATE`, `DELETE`.
- Filtering, sorting, and pagination (`WHERE`, `ORDER BY`, `LIMIT`, `OFFSET`).
- Joins **within a single domain schema** to fetch related rows.
- Simple aggregations (`COUNT`, `SUM`, `MIN`, `MAX`) when the result set is the data being returned.
- Constraints, indexes, and foreign keys.

### 7.5.2 Not allowed in SQL

- Stored procedures, functions, or triggers that encode business rules.
- Views that transform data or embed business decisions.
- Complex SQL-side calculations, conditional logic (`CASE` chains that implement rules), or string manipulation to derive values.
- Multi-step workflows, loop constructs, or cursor-based processing.

### 7.5.3 Where the logic goes instead

- **Domain services** (`internal/<domain>/service.go`) own business rules and orchestration.
- The **BFF** (`internal/bff`) owns cross-domain orchestration and client-facing transformations.
- Validation, computation, and side effects happen in Go before or after the sqlc-generated queries are called.

---

## 8. Authentication & Authorization

1. Client sends `Authorization: Bearer <id_token>` (Google ID token, or any configured OIDC ID token).
2. Auth middleware validates the JWT:
   - `iss` in allowed list (`https://accounts.google.com` by default).
   - `aud` matches configured client ID(s).
   - `exp` not expired.
   - Signature verified using provider JWKS.
3. Middleware extracts `sub`, `email`, `name`, `provider`.
4. Calls `identity.UpsertUser` (cached by `provider:subject`) to get `user_id`.
5. Stores `CurrentUser` (user_id, email, subject) in request context.
6. All GraphQL resolvers access `CurrentUser` from context; unauthenticated requests are rejected before reaching resolvers.
7. Multi-OIDC ready: `provider` column and `iss` allowlist mean a future provider can be added with only config and JWKS changes.

---

## 9. GraphQL BFF Contract (Outline)

The BFF exposes one endpoint: `POST /graphql` (or `/api/graphql`).

### 9.1 Core types

```graphql
type User {
  id: ID!
  email: String!
  displayName: String
  provider: String!
  externalSubject: String!
  lastLoginAt: Time
}

type Item {
  id: ID!
  name: String!
  brand: Brand
  upc12: String
  upc14: String
  category: Category!
  unit: String!
  # no per-user state here
}

type UserItem {
  id: ID!
  item: Item!
  currentQty: Float!
  minQty: Float
  purchaseAt: Time
  expiresAt: Time
  notes: String
  isFavorite: Boolean!
}

type Recipe {
  id: ID!
  name: String!
  description: String
  servings: Int
  prepTimeMinutes: Int
  cookTimeMinutes: Int
  isActive: Boolean!
  items: [RecipeItem!]!
  steps: [RecipeStep!]!
  isFavorite: Boolean! # resolved from userprefs by BFF
}

type Bottle {
  id: ID!
  type: WineType!
  country: Country!
  region: Region!
  vintageYear: Int!
  vineyard: String
  # ... definition fields
}

type UserBottle {
  id: ID!
  bottle: Bottle!
  quantity: Int!
  # ... holding fields
}

type MealPlan {
  id: ID!
  name: String!
  weekStartDate: Date!
  slots: [MealSlot!]!
}

type MealSlot {
  id: ID!
  dayOfWeek: Int!
  mealType: String!
  recipe: Recipe
  servings: Int
  items: [MealSlotItem!]!
}

type GroceryList {
  id: ID!
  generatedAt: Time!
  items: [GroceryListItem!]!
}

type GroceryListItem {
  id: ID!
  item: Item
  manualItemName: String
  quantityNeeded: Float!
  unitOfMeasure: String
  source: String!
  isChecked: Boolean!
}
```

### 9.2 Example mutations

```graphql
type Mutation {
  # inventory catalog
  createItem(input: CreateItemInput!): Item!
  updateItem(id: ID!, input: UpdateItemInput!): Item!
  deleteItem(id: ID!): Boolean!

  # user preferences
  adjustUserItem(itemId: ID!, quantity: Float!, purchaseAt: Time): UserItem!
  setItemFavorite(itemId: ID!, isFavorite: Boolean!): UserItem!

  # recipes
  createRecipe(input: CreateRecipeInput!): Recipe!
  setRecipeFavorite(recipeId: ID!, isFavorite: Boolean!): Boolean!

  # meal planning
  createMealPlan(input: CreateMealPlanInput!): MealPlan!
  addMealSlot(input: AddMealSlotInput!): MealSlot!

  # grocery
  generateGroceryList(mealPlanId: ID!): GroceryList!
  toggleGroceryItemChecked(groceryListItemId: ID!): GroceryListItem!
}
```

### 9.3 Resolver orchestration pattern

- `Query.mealPlan(id)` calls `mealplan.Service.GetPlan(ctx, id, userID)` → returns `MealPlan` with `SlotIDs`.
- `MealPlan.slots` calls `mealplan.Service.GetSlotsForPlan(ctx, planID)`.
- `MealSlot.recipe` checks `recipe_id` then calls `recipe.Service.GetByID(ctx, recipeID)`.
- `MealSlot.items` calls `mealplan.Service.GetSlotItems(ctx, slotID)`; each `MealSlotItem.item` calls `inventory.Service.GetByID(ctx, itemID)` if not null.
- `GroceryListItem.item` works the same way.
- `Recipe.isFavorite` calls `userprefs.Service.RecipeIsFavorite(ctx, userID, recipeID)`.
- `Item` fields are pure catalog; `currentStock` and `isFavorite` are exposed via `UserItem` type only.

---

## 10. Frontend Integration

### 10.1 Flutter

- Add `graphql_flutter` to `mobile/pubspec.yaml`.
- Configure `AuthLink` to attach `Authorization: Bearer <token>`.
- Replace `api_service.dart` HTTP calls with GraphQL operations aligned to the BFF schema.
- Keep `flutter_secure_storage` for token persistence.

### 10.2 Next.js

- Replace the typed `lib/api.ts` REST client with Apollo Client or urql configured to call `/graphql`.
- Co-locate GraphQL queries/mutations with pages.
- Reuse existing Material UI components; only the data layer changes.

---

## 11. Migrations, Seeding & Fresh Start

- Use `golang-migrate` with files in `migrations/`.
- Numbering: `0001_identity.up.sql`, `0001_identity.down.sql`, `0002_inventory.up.sql`, etc.
- Domain order: `identity` → `inventory`, `wine`, `recipe` (catalogs) → `userprefs` → `mealplan` → `grocery`.
- **No data migration** from SQL Server. Seed scripts may insert default reference data (categories, nutrient types, wine countries) using `migrations/seed/*.sql` or a `cmd/seed` tool.
- Add `sqlc` configuration per module to generate `Querier` interfaces from `internal/<module>/*.sql`.

---

## 12. Deployment & Operations

- `docker-compose.yml` with:
  - `db`: `postgres:16-alpine` with healthcheck.
  - `db-migrate`: one-shot `golang-migrate` container.
  - `api`: Go monolith built from `Dockerfile`.
  - `proxy`: Caddy on 80/443.
- Environment variables in `.env`:
  - `GOOGLE_CLIENT_ID`
  - `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB`
  - `LENA_DB_PASSWORD` (app role)
  - `CORS_ALLOWED_ORIGINS`
- Backup: standard `pg_dump` on a schedule; no custom logic.

---

## 13. Testing Strategy

| Test type | Tooling |
|---|---|
| Unit | `go test` + mocks for service interfaces |
| DB integration | `testcontainers-go` for PostgreSQL |
| GraphQL | `graph-gophers/graphql-go` resolver tests + `httptest` |
| Lint | `golangci-lint` |
| CI | GitHub Actions: `go build`, `go test`, `golangci-lint`, `docker compose up --build` |

---

## 14. Phased Implementation Plan (for after this spec)

The actual code, when approved, should be built in this order:

1. **Phase 0: Bootstrap** — `go.mod`, `docker-compose.yml`, Postgres, `golang-migrate`, `sqlc`.
2. **Phase 1: Identity + Auth** — users table, JWT middleware, current user context.
3. **Phase 2: Catalog modules** — inventory, wine, recipe (full CRUD).
4. **Phase 3: User preferences** — `user_item`, `user_bottle`, `user_recipe_preference`.
5. **Phase 4: Meal planning & grocery** — meal plans, slots, generation.
6. **Phase 5: BFF + GraphQL** — schema, resolvers, orchestration.
7. **Phase 6: Flutter & Next.js** — point clients to GraphQL.
8. **Phase 7: Hardening** — tests, lint, PII logging, CORS, Seq/OTel.

---

## 15. Risks & Considerations

- **N+1 queries** from resolver orchestration: mitigate with DataLoader or per-request batching.
- **Authorization on catalog deletes** — a user deleting a catalog item deletes it for everyone (ON DELETE CASCADE to `user_item`). Either restrict catalog deletes or make UI "remove my UserItem row".
- **Audit fields** — `created_by`/`updated_by` are strings (email); `user_id` is the scoping dimension. Keep both.
- **GraphQL as the only API** means the web and mobile share a single contract; versioning and deprecation discipline are required.
- **No SQL migration from MSSQL** — seed reference data manually or with scripts.

---

## 16. Documentation Output Map (to be written under `docs/`)

| File | Contents |
|---|---|
| `docs/go-rewrite-spec.md` | Executive summary, decisions, architecture diagram, stack. |
| `docs/postgres-data-model.md` | Full DDL per schema, indexes, constraints, type mapping. |
| `docs/graphql-schema.md` | Full `schema.graphqls` and operation examples. |
| `docs/graphql-bff-orchestration.md` | Resolver patterns, no-cross-domain rule, DataLoader strategy. |
| `docs/auth-oidc.md` | JWT validation, multi-provider design, user upsert flow. |
| `docs/flutter-nextjs-integration.md` | Client packages, auth link, sample queries. |
| `docs/migration-runbook.md` | `golang-migrate` usage, seeding, fresh-start process. |
| `docs/deployment.md` | `docker-compose`, Caddy, environment variables, backups. |

---

## 17. Acceptance Criteria

- [ ] All `docs/` files above are produced and peer-review ready.
- [ ] The specification is internally consistent with the "no cross-domain SQL" rule.
- [ ] The specification explicitly states that all business logic lives in Go code and that no stored procedures, triggers, or logic-embedding views are used.
- [ ] The data model explicitly separates per-user tables from catalog tables.
- [ ] The GraphQL contract is sufficient for the existing Flutter grocery list flow and the existing Next.js pages.
- [ ] The plan can be handed to another engineer who can implement it without further clarification on the architecture.
