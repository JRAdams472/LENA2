---
agent: devin-local
session: mini-court
created: 2026-09-05T04:47:12Z
---
# LENA2 Testing Megaplan

A phased, end-to-end plan to bring LENA2's unit and integration test coverage to (and beyond) the level of the original LENA project, split by backend, frontend, and Playwright e2e workstreams.

## 1. Objective

Add comprehensive, maintainable unit and integration tests to the LENA2 project that meet or exceed the coverage and depth of the original LENA project:

- **Original LENA** has dedicated unit-test projects for the API, Application, and Features layers, plus integration tests with a real database and a custom web test factory.
- **LENA2** currently has only `cmd/lena/main_test.go`, `internal/bff/auth_test.go`, `internal/platform/logger/logger_test.go`, and five Jest suites under `clients/web/__tests__`.

The plan below splits the work into six implementation phases plus a documentation phase, and covers the Go backend, the Next.js frontend, and Playwright end-to-end tests.

## 2. Acceptance Criteria

- `go test ./...` passes with no skipped tests.
- Every public method on every `internal/<domain>/Service` has at least one unit test.
- Every GraphQL query and mutation exposed by `internal/bff/resolver.go` is exercised by unit and/or integration tests.
- `npm test` (Jest) passes with expanded frontend coverage.
- Playwright smoke tests run successfully against the full stack in CI.
- Code-coverage reports are produced for Go, Jest, and Playwright and are stored as CI artifacts.
- CI runs the full test matrix on every pull request.

## 3. Constraints and Dependencies

- **Backend language**: Go 1.26.
- **Database**: PostgreSQL 16, accessed via `pgx/v5` and SQLC.
- **Existing mock surface**: SQLC generates a `Querier` interface per package (`internal/inventory/sqlc/querier.go`, `internal/wine/sqlc/querier.go`, etc.). This is the primary seam for mocking service unit tests.
- **Auth model**: Google ID-token validation with `lestrrat-go/jwx/v3`. Tests need a deterministic way to mint or mock valid tokens.
- **No mocking library in `go.mod` today**: add `testify` (`testify/assert`, `testify/mock`) and `testcontainers-go` modules.
- **Frontend**: Next.js 16 App Router, React 19, Jest 30, React Testing Library, no `msw` yet.
- **E2E**: Playwright is not installed yet.
- **CI**: `.github/workflows/docker.yml` only builds images; a new `.github/workflows/test.yml` is needed.

## 4. Phased Plan

### Phase 0 — Tooling and Test Harness

**Goal**: Make it possible to write and run tests in every layer.

1. **Go dependencies**:
   - `go get github.com/stretchr/testify` and `github.com/testcontainers/testcontainers-go/modules/postgres`.
   - Add `tools.go` with `go.uber.org/mock/mockgen` (or use `testify/mock` only if mocks are written by hand).
   - Add a `//go:generate mockgen ...` directive for each SQLC `Querier` interface and generate mocks into `<package>/mocks`.

2. **Shared test helpers** (`internal/platform/testenv` or per-package `*_test.go` helpers):
   - `NewTestDB(ctx) (*pgx.Conn, *postgres.PostgresContainer, error)` using `testcontainers-go`.
   - `RunMigrations(t, conn)` that applies all `migrations/*.up.sql` and `migrations/seed/*.sql` to the test container.
   - `TestToken(t, email string) string` that mints a JWT signed with a hard-coded test key and sets `kid`, `iss`, `aud` values configured in the BFF.
   - `MustUser(ctx, t)` to insert a user row for tests that need `created_by`/`updated_by`.

3. **Frontend tooling**:
   - Add `msw` and `@testing-library/user-event` to `clients/web`.
   - Add `jest.config.mjs` coverage settings and `test:coverage` script.
   - Add `clients/web/test-utils.tsx` that wraps components with `AuthProvider`, `QueryClientProvider`, and an `msw` server.

4. **Playwright scaffolding**:
   - `npm init playwright@latest` in `clients/web` or a top-level `e2e/` package.
   - `playwright.config.ts` with a `webServer` option or a `docker-compose.test.yml` override.
   - `e2e/auth.setup.ts` that seeds `localStorage` with a fake, BFF-valid ID token.

5. **CI workflow** (`.github/workflows/test.yml`):
   - Job 1: `go vet ./...` + `go test ./...` with coverage.
   - Job 2: `npx tsc --noEmit`, `npm test`, `npm run build` in `clients/web`.
   - Job 3 (optional in later phase): Playwright against a dockerized stack.

### Phase 1 — Backend Unit Tests: Services

Strategy: test each `Service` method by mocking the SQLC `Querier` interface. Each test file lives next to the code it tests (`<package>/service_test.go`, `<package>/sqlc_test.go` for mappers).

#### 1.1 `internal/inventory`

Create `internal/inventory/service_test.go` covering:

- Brand CRUD: `CreateBrand`, `GetBrandByID`, `ListBrands`, `UpdateBrand`, `DeleteBrand`.
- Category CRUD: `CreateCategory`, `GetCategoryByID`, `ListCategories`, `UpdateCategory`, `DeleteCategory`.
- Item CRUD: `CreateItem`, `GetItemByID`, `ListItems`, `UpdateItem`, `DeleteItem`.
- FlavorProfile CRUD: `CreateFlavorProfile`, `GetFlavorProfileByID`, `ListFlavorProfiles`, `UpdateFlavorProfile`, `DeleteFlavorProfile`.
- NutrientType CRUD: `CreateNutrientType`, `GetNutrientTypeByID`, `ListNutrientTypes`, `UpdateNutrientType`, `DeleteNutrientType`.
- Junction operations: `CreateFoodNutrient`, `DeleteFoodNutrient`, `ListFoodNutrientsByItem`, `CreateFoodFlavor`, `DeleteFoodFlavor`, `ListFoodFlavorsByItem`.

Edge cases to assert:
- Missing rows return `sql.ErrNoRows` or a wrapped `not found` error.
- Audit fields (`created_by`, `updated_by`, `created_at`, `updated_at`) are populated.
- `pgtype.Text` conversions for nullable `description` columns.
- Active/inactive filtering where applicable.

#### 1.2 `internal/wine`

Create `internal/wine/service_test.go` covering:

- Country, Region, Type, Vintage, GrapeVariety, WineFlavorProfile CRUD.
- Bottle CRUD: `CreateBottle`, `GetBottleByID`, `ListBottles`, `UpdateBottle`, `DeleteBottle`.
- Junction operations: `AddBottleGrapeVariety`, `ListBottleGrapeVarieties`, `RemoveBottleGrapeVariety`, `AddBottleFlavorProfile`, `ListBottleFlavorProfiles`, `RemoveBottleFlavorProfile`.

Edge cases:
- Region `country_id` FK handling.
- Bottle update with empty `name` or `vintage_year`.
- Duplicate flavor/grape additions return the appropriate error.

#### 1.3 `internal/recipe`

Create `internal/recipe/service_test.go` covering:

- `CreateRecipe`, `GetRecipeByID`, `ListRecipes`, `UpdateRecipe`, `DeleteRecipe`.
- `AddRecipeItem`, `ListRecipeItems`, `RemoveRecipeItem`.
- `AddRecipeStep`, `ListRecipeSteps`, `UpdateRecipeStep`, `DeleteRecipeStep`.

Edge cases:
- Recipe active/inactive filtering.
- Step ordering and renumbering.
- Removing a recipe cascades or fails as expected.

#### 1.4 `internal/mealplan`

Create `internal/mealplan/service_test.go` covering:

- `CreateMealPlan`, `GetMealPlanByID`, `ListMealPlans`, `UpdateMealPlan`, `DeleteMealPlan`.
- `AddMealSlot`, `GetMealSlotByID`, `ListMealSlotsForPlan`, `UpdateMealSlot`, `DeleteMealSlot`.
- `AddMealSlotItem`, `ListMealSlotItems`, `DeleteMealSlotItem`.

Edge cases:
- Multi-tenancy (`userID` isolation).
- Deleting a plan cascades to slots and items.

#### 1.5 `internal/grocery`

Create `internal/grocery/service_test.go` covering:

- `CreateGroceryList`, `GetGroceryListByID`, `ListGroceryLists`, `DeleteGroceryList`.
- `AddGroceryListItem`, `ListGroceryListItems`, `GetGroceryListItemByID`, `UpdateGroceryListItem`, `DeleteGroceryListItem`.
- `Generate` from a meal plan.

Edge cases:
- `Generate` with empty meal plan, duplicate items across slots, and checked/unchecked state.
- `ListGroceryLists` respects `userID`.

#### 1.6 `internal/userprefs`

Create `internal/userprefs/service_test.go` covering:

- `UpsertUserItem`, `GetUserItemByID`, `ListUserItems`, `DeleteUserItem`.
- `UpsertUserBottle`, `GetUserBottleByID`, `ListUserBottles`, `DeleteUserBottle`.
- `SetRecipeFavorite`, `GetRecipeFavorite`, `DeleteRecipeFavorite`.

Edge cases:
- Upsert semantics (insert vs update).
- User isolation.

#### 1.7 `internal/identity`

Create `internal/identity/service_test.go` covering:

- `UpsertUser` (first login vs. returning user).
- `GetByID`.

#### 1.8 `internal/platform`

Add or extend tests for:

- `internal/platform/logger` (already partially covered; add negative and PII cases).
- `internal/platform/config` parsing and defaults.
- `internal/platform/postgres` connection string and pool settings.

### Phase 2 — Backend Integration Tests

Strategy: use `testcontainers-go` to start PostgreSQL, run migrations, and exercise real services, the BFF GraphQL endpoint, and `cmd/lena`.

1. **Package-level integration tests** (`<package>/integration_test.go`):
   - For each domain, run the full CRUD lifecycle against a real database.
   - Assert SQL constraints (FKs, unique constraints) produce meaningful errors.

2. **`internal/bff` HTTP/GraphQL tests** (`internal/bff/bff_integration_test.go`):
   - Build an `echo` server with a real `Resolver` backed by real services and a test DB.
   - Test the top 20 queries/mutations from `resolver.go` (e.g., `Items`, `Bottles`, `Recipes`, `CreateMealPlan`, `GenerateGroceryList`, `SetRecipeFavorite`).
   - Assert auth behavior: missing token, invalid token, valid token, and forbidden operation (e.g., reading another user's meal plan).

3. **`cmd/lena` integration test**:
   - Start the server with `testcontainers-go` DB and a temporary port.
   - Test `/health` and a few GraphQL queries end-to-end.

4. **Test fixtures**:
   - Seed helper functions: `SeedCountries`, `SeedItems`, `SeedUser`, `SeedRecipe`, etc., in `internal/bff/testdata` or `internal/testfixtures`.

### Phase 3 — BFF Resolver Unit Tests

Create `internal/bff/resolver_test.go` with mocked services:

- Query resolvers: `Me`, `Brand`, `Brands`, `Item`, `Items`, `Recipe`, `Recipes`, `Bottle`, `Bottles`, `MealPlan`, `MealPlans`, `GroceryList`, `GroceryLists`, `Nutrition`, etc.
- Mutation resolvers: `CreateItem`, `UpdateItem`, `DeleteItem`, `CreateRecipe`, `UpdateRecipe`, `DeleteRecipe`, `CreateMealPlan`, `AddMealSlot`, `GenerateGroceryList`, `SetRecipeFavorite`, `AdjustUserItem`, etc.
- Auth: each resolver should verify `userFromContext` is called and unauthorized requests fail.
- Pagination: verify `pageInfo` is populated correctly for paged resolvers.

### Phase 4 — Frontend Jest Unit and Component Tests

#### 4.1 API wrapper tests

Extend `clients/web/__tests__/lib/api.test.ts` to cover:

- All CRUD wrappers added for Brand, Category, GrapeVariety, WineFlavorProfile, Country, Region, Type, Vintage, NutrientType, FlavorProfile.
- `setItemFavorite`, `setBottleFavorite`, `adjustUserItem`, `adjustUserBottle`.
- `generateGroceryList`, `toggleGroceryItemChecked`, `addGroceryItem`, `deleteGroceryItem`.
- Error paths: `ApiError` messages, GraphQL `errors` array, non-JSON responses.

#### 4.2 Component tests

Create:

- `__tests__/components/CrudPage.test.tsx` — create, edit, delete, validation, active-only toggle.
- `__tests__/components/CrudDialog.test.tsx` — field rendering, form submission, cancel.
- `__tests__/components/DataTable.test.tsx` (already exists; expand sorting, pagination, selection).
- `__tests__/components/AdminLayout.test.tsx` — navigation groups and login gating.

#### 4.3 Page tests

For one representative page per catalog (e.g., `__tests__/app/inventory/categories/page.test.tsx`):

- Render the page inside `AuthProvider` + `QueryClientProvider`.
- Use `msw` to mock the GraphQL responses.
- Assert table rows, create dialog, update, and delete flows.

#### 4.4 Auth tests

Expand `__tests__/app/auth-gate.test.tsx`:

- Expired token behavior.
- Invalid signature handling.
- `googleLogout` on sign-out.

### Phase 5 — Playwright End-to-End Tests

1. **Setup**:
   - `playwright.config.ts` with projects for `chromium`, `firefox`, `webkit` (or just `chromium` initially).
   - `webServer` that runs `npm run dev` in `clients/web` and a dockerized API/DB, or use `docker compose -f docker-compose.yml -f docker-compose.test.yml up` in CI.
   - `auth.setup.ts` stores a signed test token in `playwright/.auth/user.json`.

2. **Smoke tests** (`e2e/smoke.spec.ts`):
   - Login redirects to app.
   - Navigation links work.
   - `/health` returns 200.

3. **CRUD catalog tests** (`e2e/catalogs.spec.ts`):
   - Brand, Category, Country, Region, Type, Vintage, GrapeVariety, WineFlavorProfile, NutrientType, FlavorProfile.
   - Create a row, see it in the table, edit it, toggle active, delete it.

4. **Domain tests**:
   - `e2e/items.spec.ts`: create an item, set brand, category, nutrients, flavors.
   - `e2e/recipes.spec.ts`: create recipe, add items/steps, delete.
   - `e2e/mealplans.spec.ts`: create meal plan, add slot, add item, generate grocery list.
   - `e2e/grocery.spec.ts`: toggle checked, add manual item, delete list.
   - `e2e/wine.spec.ts`: create bottle, assign country/region/type/vintage/grape/flavor.

5. **Auth tests**:
   - Unauthenticated user is redirected to `/login`.
   - Attempting to access another user's data is rejected.

### Phase 6 — CI, Coverage, and Quality Gates

1. **Go coverage**:
   - `go test -race -coverprofile=coverage.out ./...`
   - Fail CI if coverage drops below a baseline (e.g., 60%) or if any package is at 0%.
   - Upload `coverage.out` artifact.

2. **Jest coverage**:
   - `npm test -- --coverage` with `coverageThreshold` for statements, branches, functions, lines.
   - Store `clients/web/coverage` as artifact.

3. **Playwright in CI**:
   - Run in `.github/workflows/test.yml` on `ubuntu-latest` with the docker stack.
   - Publish HTML report as artifact.

4. **Pre-merge checks**:
   - `go build ./...`
   - `go vet ./...`
   - `npx tsc --noEmit`
   - `npm run lint`
   - `npm run build`
   - `go test ./...`
   - `npm test`
   - Playwright smoke

### Phase 7 — Documentation and Plan Persistence

1. **Developer docs** (`docs/testing.md` after plan approval):
   - How to run unit, integration, and e2e tests locally.
   - How to add a new service test with a mocked `Querier`.
   - How the test token and `testcontainers-go` fixtures work.

2. **Plan artifact**:
   - Copy the approved plan to `docs/testing-megaplan.md` as requested.

## 5. File-by-File Test Matrix (Go Backend)

| Package | Unit Test File | Integration Test File | Key Mocks |
|---|---|---|---|
| `internal/inventory` | `service_test.go` | `integration_test.go` | `sqlc.Querier` |
| `internal/wine` | `service_test.go` | `integration_test.go` | `sqlc.Querier` |
| `internal/recipe` | `service_test.go` | `integration_test.go` | `sqlc.Querier` |
| `internal/mealplan` | `service_test.go` | `integration_test.go` | `sqlc.Querier` |
| `internal/grocery` | `service_test.go` | `integration_test.go` | `sqlc.Querier` |
| `internal/userprefs` | `service_test.go` | `integration_test.go` | `sqlc.Querier` |
| `internal/identity` | `service_test.go` | `integration_test.go` | `sqlc.Querier` |
| `internal/bff` | `auth_test.go` (exists), `resolver_test.go` | `integration_test.go` | Real services + test DB |
| `internal/platform` | `*_test.go` per package | — | Environment / log sink |
| `cmd/lena` | `main_test.go` (exists) | `main_integration_test.go` | Test DB + temp ports |

## 6. Frontend Test Matrix

| Area | Test Files | Focus |
|---|---|---|
| API client | `__tests__/lib/api.test.ts` (expand) | All wrappers, error handling, auth headers |
| API meal-plan + grocery | `__tests__/lib/api-mealplan-grocery.test.ts` (expand) | Complex list/generate flows |
| Components | `__tests__/components/CrudPage.test.tsx` (new) | CRUD lifecycle, active filter |
|  | `__tests__/components/CrudDialog.test.tsx` (new) | Form behavior, validation |
|  | `__tests__/components/DataTable.test.tsx` (expand) | Sorting, pagination, selection |
|  | `__tests__/components/AdminLayout.test.tsx` (new) | Navigation gating |
| Pages | `__tests__/app/inventory/categories/page.test.tsx` (example) | Table + create + edit + delete |
| Auth | `__tests__/app/auth-gate.test.tsx` (expand) | Expired/invalid tokens |

## 7. E2E Test Matrix (Playwright)

| Spec | Flows |
|---|---|
| `e2e/smoke.spec.ts` | Login, navigation, `/health` |
| `e2e/catalogs.spec.ts` | CRUD for all reference-data catalogs |
| `e2e/items.spec.ts` | Item creation with brand/category/nutrients/flavors |
| `e2e/recipes.spec.ts` | Recipe CRUD + items/steps |
| `e2e/mealplans.spec.ts` | Meal plan → slot → item → generate grocery list |
| `e2e/grocery.spec.ts` | Grocery list CRUD, toggle, manual add |
| `e2e/wine.spec.ts` | Bottle CRUD with all relationships |
| `e2e/auth.spec.ts` | Unauthenticated redirect, cross-user isolation |

## 8. Risks and Mitigation

| Risk | Mitigation |
|---|---|
| Testcontainers can be slow/flaky in CI | Pin image versions, reuse containers across package tests, and provide `LENA_TEST_DATABASE_URL` fallback. |
| Google JWT validation makes e2e auth hard | Use a test-only signing key and expose `LENA_AUTH_ISSUERS`/`LENA_AUTH_AUDIENCES` environment variables. |
| Mocking SQLC by hand is tedious | Use `mockgen` or `testify/mock` and commit generated mocks in `sqlc/mocks`. |
| Frontend App Router components are hard to render in Jest | Focus component tests on Client Components; use Playwright for full-page server-component paths. |
| Test runtime becomes long | Split CI jobs by layer (unit, integration, e2e) and run them in parallel. |

## 9. Definition of Done

- All phases above are merged to `main` via `phase-<N>` pull requests.
- `go test ./...`, `npm test`, and the Playwright smoke suite pass in CI on every PR.
- Coverage baselines are documented and enforced.
- `docs/testing.md` and `docs/testing-megaplan.md` are in the repo.
