# LENA2 Household Sharing Implementation Plan

Add multi-user household support to LENA2 so members share pantry stock, wine cellar holdings, meal plans, and grocery lists while keeping recipe favorites and item/wine likes personal, including database migrations, a new `internal/household` service, BFF GraphQL changes, a Next.js web UI, and unit/integration tests.

## 1. Objective

Allow authenticated LENA2 users to form a household with one other user. The household shares the operational data (pantry, wine, meal plans, grocery lists) but preserves per-user preferences (recipe favorites, item/bottle likes). Users can search for other users by name or email, send a household invitation, and have the recipient approve or decline it from the dashboard. Either member may leave the household and return to a single-person household.

## 2. Acceptance Criteria

- Every user has a default single-person household created automatically on first login.
- `me` GraphQL query returns the caller's `household` and `isSearchable` fields.
- `searchHouseholdUsers` returns users matching a name/email term, excluding the caller, users already in the caller's household, and users who have opted out of search.
- `inviteHouseholdMember` creates a pending invitation; the target sees it on the dashboard.
- `acceptHouseholdInvite` moves the target into the inviter's household and merges the target's default household data into the shared household.
- `declineHouseholdInvite` and `cancelHouseholdInvite` close the invitation without merging data.
- `leaveHousehold` moves the caller to a new default single-person household.
- Pantry (`inventory.household_item`), wine (`wine.household_bottle`), `mealplan.meal_plan`, and `grocery.grocery_list` are filtered by household.
- `is_favorite` for items and bottles becomes a per-user preference; recipe favorites remain per-user.
- The web `/profile` page exposes a "searchable" toggle and a household section.
- A new web `/household` page shows household members, pending invitations, and a search/invite form.
- All new Go code has unit tests; all new web pages have Jest component tests.

## 3. Scope

### In Scope
- Database schema for households, invitations, and household-scoped tables.
- Migration of existing per-user data into default households.
- New `internal/household` Go package and `HouseholdService`.
- Updates to `internal/identity`, `internal/userprefs`, `internal/inventory` (favorites), `internal/wine` (favorites), `internal/mealplan`, `internal/grocery`.
- BFF GraphQL schema additions and resolvers.
- Web UI: profile settings (`isSearchable`), `/household` page, dashboard invite card.
- Unit and integration tests for new and changed code.

### Out of Scope (for this phase)
- Mobile UI changes; the Flutter app will use shared data automatically through the updated GraphQL resolvers but gets no new household screens in this phase.
- More than two members per household.
- Household names, roles, or permissions.
- Real-time/push notifications; pending invites are fetched by dashboard poll.

## 4. High-Level Architecture

```text
identity.users
  - household_id (FK -> household.households)
  - is_searchable (boolean)

household.households
  - household_id
  - created_by
  - created_at, updated_at

household.invites
  - invite_id
  - from_user_id, to_user_id
  - household_id (inviter's household)
  - status (pending, accepted, declined, cancelled)
  - created_by, created_at, updated_at

Shared tables (filter by household_id):
  - inventory.household_item
  - wine.household_bottle
  - mealplan.meal_plan
  - grocery.grocery_list

Per-user preference tables:
  - inventory.user_item_favorite (user_id, item_id, is_favorite)
  - wine.user_bottle_favorite (user_id, bottle_id, is_favorite)
  - recipe.user_recipe_preference (unchanged)
```

The BFF is the only package that orchestrates cross-domain work (identity, household, userprefs, inventory, etc.). Each domain service remains schema-bound. `currentuser.User` gains `HouseholdID` and `IsSearchable` so resolvers can pass the household context to the service layer without trusting client input.

## 5. Database Migrations

New files:

- `migrations/0024_household.up.sql`
- `migrations/0024_household.down.sql`

### 5.1 Household schema and tables

```sql
CREATE SCHEMA IF NOT EXISTS household;

CREATE TABLE household.households (
    household_id BIGSERIAL PRIMARY KEY,
    created_by   VARCHAR(100) NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ
);

CREATE TABLE household.invites (
    invite_id    BIGSERIAL PRIMARY KEY,
    from_user_id BIGINT NOT NULL REFERENCES identity.users(user_id) ON DELETE CASCADE,
    to_user_id   BIGINT NOT NULL REFERENCES identity.users(user_id) ON DELETE CASCADE,
    household_id BIGINT NOT NULL REFERENCES household.households(household_id) ON DELETE CASCADE,
    status       VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','accepted','declined','cancelled')),
    created_by   VARCHAR(100) NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by   VARCHAR(100),
    updated_at   TIMESTAMPTZ,
    UNIQUE (from_user_id, to_user_id, status)
);

CREATE INDEX idx_invites_to_status ON household.invites (to_user_id, status);
CREATE INDEX idx_invites_from_status ON household.invites (from_user_id, status);
```

Use a partial unique index to allow only one pending invite between the same two users:

```sql
CREATE UNIQUE INDEX idx_invites_pending_unique
ON household.invites (from_user_id, to_user_id)
WHERE status = 'pending';
```

### 5.2 Identity additions

```sql
ALTER TABLE identity.users
    ADD COLUMN household_id  BIGINT REFERENCES household.households(household_id),
    ADD COLUMN is_searchable BOOLEAN NOT NULL DEFAULT TRUE;

CREATE INDEX idx_users_household ON identity.users (household_id);
```

Backfill: create one `household.households` row for every existing user and set `identity.users.household_id` to it.

### 5.3 Pantry migration

```sql
ALTER TABLE inventory.user_item
    ADD COLUMN household_id BIGINT;

-- backfill using the user's default household
UPDATE inventory.user_item ui
SET household_id = u.household_id
FROM identity.users u
WHERE ui.user_id = u.user_id;

-- extract personal favorites before dropping the column
CREATE TABLE inventory.user_item_favorite (
    user_id     BIGINT NOT NULL REFERENCES identity.users(user_id) ON DELETE CASCADE,
    item_id     BIGINT NOT NULL REFERENCES inventory.item(item_id) ON DELETE CASCADE,
    is_favorite BOOLEAN NOT NULL DEFAULT FALSE,
    created_by  VARCHAR(100) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by  VARCHAR(100),
    updated_at  TIMESTAMPTZ,
    PRIMARY KEY (user_id, item_id)
);

INSERT INTO inventory.user_item_favorite (user_id, item_id, is_favorite, created_by, created_at)
SELECT user_id, item_id, is_favorite, created_by, created_at
FROM inventory.user_item
WHERE is_favorite;

ALTER TABLE inventory.user_item
    DROP COLUMN is_favorite,
    DROP COLUMN user_id,
    ALTER COLUMN household_id SET NOT NULL,
    ADD CONSTRAINT user_item_household_item UNIQUE (household_id, item_id);

ALTER TABLE inventory.user_item RENAME TO household_item;
```

### 5.4 Wine migration

Same pattern as pantry:

```sql
ALTER TABLE wine.user_bottle ADD COLUMN household_id BIGINT;
UPDATE wine.user_bottle ub SET household_id = u.household_id FROM identity.users u WHERE ub.user_id = u.user_id;

CREATE TABLE wine.user_bottle_favorite (
    user_id     BIGINT NOT NULL REFERENCES identity.users(user_id) ON DELETE CASCADE,
    bottle_id   BIGINT NOT NULL REFERENCES wine.bottle(bottle_id) ON DELETE CASCADE,
    is_favorite BOOLEAN NOT NULL DEFAULT FALSE,
    created_by  VARCHAR(100) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by  VARCHAR(100),
    updated_at  TIMESTAMPTZ,
    PRIMARY KEY (user_id, bottle_id)
);

INSERT INTO wine.user_bottle_favorite (user_id, bottle_id, is_favorite, created_by, created_at)
SELECT user_id, bottle_id, is_favorite, created_by, created_at
FROM wine.user_bottle
WHERE is_favorite;

ALTER TABLE wine.user_bottle
    DROP COLUMN is_favorite,
    DROP COLUMN user_id,
    ALTER COLUMN household_id SET NOT NULL,
    ADD CONSTRAINT user_bottle_household_bottle UNIQUE (household_id, bottle_id);

ALTER TABLE wine.user_bottle RENAME TO household_bottle;
```

### 5.5 Meal plan and grocery list migration

```sql
ALTER TABLE mealplan.meal_plan ADD COLUMN household_id BIGINT;
UPDATE mealplan.meal_plan mp SET household_id = u.household_id FROM identity.users u WHERE mp.user_id = u.user_id;
ALTER TABLE mealplan.meal_plan
    DROP COLUMN user_id,
    ALTER COLUMN household_id SET NOT NULL;
ALTER INDEX idx_meal_plan_user_week RENAME TO idx_meal_plan_household_week;

ALTER TABLE grocery.grocery_list ADD COLUMN household_id BIGINT;
UPDATE grocery.grocery_list gl SET household_id = u.household_id FROM identity.users u WHERE gl.user_id = u.user_id;
ALTER TABLE grocery.grocery_list
    DROP COLUMN user_id,
    ALTER COLUMN household_id SET NOT NULL;
```

### 5.6 Down migration

The `0024_household.down.sql` must reverse the above in dependency order: re-add `user_id` columns, restore per-user `user_item` and `user_bottle` rows from `household_id`, reconstruct `is_favorite` from the favorite tables, drop the favorite tables, drop the household columns, drop `household.invites` and `household.households`, and drop the `household` schema. Because the up migration aggregates data, the down migration is a best-effort rollback and should be clearly marked as such.

## 6. Backend (Go) Changes

### 6.1 `internal/platform/currentuser`

Update `currentuser.User`:

```go
type User struct {
    UserID          int64
    Provider        string
    ExternalSubject string
    Email           string
    DisplayName     string
    IsAdmin         bool
    HouseholdID     int64
    IsSearchable    bool
}
```

### 6.2 `internal/identity`

Add to `identity.User` and `toUser`:
- `HouseholdID`
- `IsSearchable`

Add to `queries.sql`:
- `GetUserByID` returns `household_id` and `is_searchable`.
- `SetUserHousehold` updates `household_id`.
- `SetUserSearchable` updates `is_searchable`.
- `ListUsersByHousehold` returns users in a household.
- `SearchUsers` partial/fuzzy name + exact-ish email search with `is_searchable = true`, excluding caller and same-household users.
- `UpsertUser` inserts with a null `household_id` for new rows; the BFF will create a default household immediately after.

Add service methods:
- `GetByID` (updated)
- `SetUserHousehold(ctx, userID, householdID int64) error`
- `SetUserSearchable(ctx, userID int64, isSearchable bool, by string) error`
- `ListUsersByHousehold(ctx, householdID int64) ([]User, error)`
- `SearchUsers(ctx, term string, excludeUserID, excludeHouseholdID int64, limit int32) ([]User, error)`

### 6.3 New package `internal/household`

Files to create:

- `internal/household/queries.sql`
- `internal/household/sqlc/` (db.go, generate.go, mock, querier.go, queries.sql.go, models.go)
- `internal/household/service.go`
- `internal/household/service_test.go`
- `internal/household/integration_test.go`

Tables owned: `household.households`, `household.invites`.

Service methods:

```go
func (s *Service) CreateHousehold(ctx context.Context, by string) (Household, error)
func (s *Service) GetHouseholdByID(ctx context.Context, householdID int64) (Household, error)
func (s *Service) CreateInvite(ctx context.Context, fromUserID, toUserID, householdID int64, by string) (Invite, error)
func (s *Service) GetInviteByID(ctx context.Context, inviteID int64) (Invite, error)
func (s *Service) ListPendingInvitesForUser(ctx context.Context, userID int64) ([]Invite, error)
func (s *Service) ListSentInvitesForUser(ctx context.Context, userID int64) ([]Invite, error)
func (s *Service) UpdateInviteStatus(ctx context.Context, inviteID int64, status string, by string) (Invite, error)
```

### 6.4 `internal/userprefs`

- Update `queries.sql` to read from `inventory.household_item` and `wine.household_bottle` filtered by `household_id`.
- Move `is_favorite` reads/writes to the new `inventory.user_item_favorite` and `wine.user_bottle_favorite` tables; the `UserItem` and `UserBottle` service models no longer contain `IsFavorite`.
- Add `GetUserItemFavorite(ctx, userID, itemID)` and `SetUserItemFavorite` / similar for bottles.
- Add `MigrateHouseholdData(ctx, fromHouseholdID, toHouseholdID int64) error` that merges `household_item` and `household_bottle` rows from the target's old default household into the new shared household. On `(household_id, item_id)` or `(household_id, bottle_id)` collisions, sum `current_qty`/`quantity` and keep the most recent non-null `purchase_at`, `expires_at`, `location`, `notes`.

### 6.5 `internal/inventory`, `internal/wine`, `internal/mealplan`, `internal/grocery`

For each package:
- Update `queries.sql` to use `household_id` instead of `user_id` on the shared tables.
- Update service method signatures from `userID int64` to `householdID int64` where appropriate.
- Update service models (`MealPlan`, `GroceryList`) to use `HouseholdID`.
- Regenerate sqlc code.
- Update unit tests and mock expectations.

### 6.6 `internal/bff`

- `schema.graphqls` additions:

```graphql
type Household {
  id: ID!
  members: [User!]!
  createdAt: Time!
}

type HouseholdInvite {
  id: ID!
  fromUser: User!
  toUser: User!
  household: Household!
  status: String!
  createdAt: Time!
}

input UpdateProfileInput {
  firstName: String
  lastName: String
  backupEmail: String
  isSearchable: Boolean
}

extend type Query {
  myHousehold: Household
  householdInvites: [HouseholdInvite!]!
  searchHouseholdUsers(term: String!, limit: Int = 20): [User!]!
}

extend type Mutation {
  inviteHouseholdMember(userId: ID!): HouseholdInvite!
  acceptHouseholdInvite(inviteId: ID!): Household!
  declineHouseholdInvite(inviteId: ID!): HouseholdInvite!
  cancelHouseholdInvite(inviteId: ID!): HouseholdInvite!
  leaveHousehold: Boolean!
}
```

- Update `User` type in `schema.graphqls`:

```graphql
type User {
  ...existing fields...
  isSearchable: Boolean!
  household: Household
}
```

- Update `services.go` to add `HouseholdService` interface and `IdentityService` to expose new methods.
- Create `internal/bff/resolver_household.go` with resolvers for the new queries and mutations.
- Update `internal/bff/resolver_identity.go` `me` resolver to return `household` and `isSearchable`.
- Update `internal/bff/resolver_inventory.go`, `resolver_wine.go`, `resolver_mealplan.go`, `resolver_grocery.go`, `resolver_userprefs.go` to pass `currentuser.HouseholdID` to the services and to load per-user favorites for `isFavorite` fields.
- Update `internal/bff/auth.go` `Authenticator` to:
  1. Load or create a default household for the user.
  2. Update `identity.users.household_id` if needed.
  3. Populate `currentuser.User.HouseholdID` and `IsSearchable`.
- Regenerate `internal/bff/mock/services.go`.

### 6.7 `cmd/lena/main.go`

- Wire `householdSvc := household.NewService(pool)`.
- Pass `householdSvc` to `bff.NewResolver` and `bff.NewAuthenticator` (update their constructors/signatures).

## 7. Web UI Changes

Files:

- `clients/web/lib/types.ts` — add `Household`, `HouseholdInvite` types; extend `User` with `household` and `isSearchable`.
- `clients/web/lib/api.ts` — add API functions for `myHousehold`, `householdInvites`, `searchHouseholdUsers`, `inviteHouseholdMember`, `acceptHouseholdInvite`, `declineHouseholdInvite`, `cancelHouseholdInvite`, `leaveHousehold`, and add `isSearchable` to `updateMyProfile`.
- `clients/web/app/profile/page.tsx` — add a FormControlLabel/Switch for `isSearchable` and send it with `updateMyProfile`.
- `clients/web/app/household/page.tsx` (new) — page with:
  - Current household members list.
  - Incoming/outgoing pending invitations.
  - Search field with `searchHouseholdUsers` and "Invite" buttons.
  - "Leave household" button with confirmation.
- `clients/web/app/page.tsx` — add a card to the dashboard that shows pending household invites and buttons to accept/decline.
- Update navigation (e.g. `clients/web/app/components/AdminLayout.tsx`) to add a "Household" link if appropriate.

Follow `clients/web/CLAUDE.md` and verify the current Next.js API in `node_modules/next/dist/docs` before adding pages or data fetching, as the project uses a non-standard Next.js build.

## 8. Testing Strategy

### 8.1 Go

For each new/changed package, add or update `*_test.go`:

- `internal/household/service_test.go` — unit tests with gomock sqlc querier for create household, create/accept/decline/cancel/list invites.
- `internal/household/integration_test.go` — `testenv.NewTestDB` to exercise the full flow against Postgres.
- `internal/identity/service_test.go` — `SetUserHousehold`, `SetUserSearchable`, `SearchUsers`, `ListUsersByHousehold`.
- `internal/bff/resolver_household_test.go` — gomock service mocks for the new resolvers, covering authz, same-household rejection, and invite acceptance.
- `internal/userprefs/service_test.go` — updated mocks and tests for `HouseholdID` filtering and favorite tables.
- `internal/mealplan/service_test.go`, `internal/grocery/service_test.go`, `internal/inventory/*_test.go`, `internal/wine/*_test.go` — update expectations from `userID` to `householdID`.
- Regenerate mocks after sqlc changes:
    - `go generate ./internal/household/...` (new)
    - `go generate ./internal/identity/...`
    - `go generate ./internal/userprefs/...`
    - `go generate ./internal/inventory/...`
    - `go generate ./internal/wine/...`
    - `go generate ./internal/mealplan/...`
    - `go generate ./internal/grocery/...`
    - `go generate ./internal/bff/...`

### 8.2 Web

- `clients/web/__tests__/lib/api-household.test.ts` — mock GraphQL for the new API calls.
- `clients/web/__tests__/app/profile/household-settings.test.tsx` — profile `isSearchable` toggle.
- `clients/web/__tests__/app/household/page.test.tsx` — household page rendering, search, invite, accept, leave.
- `clients/web/__tests__/app/page.test.tsx` — dashboard pending invite card.

## 9. Implementation Steps

1. **Schema and migrations**
   - Write `migrations/0024_household.up.sql` and `down.sql` with the tables, columns, backfill, and favorite extraction.
   - Run `go test ./internal/platform/testenv` with a temporary integration test to verify the migration round-trips.

2. **sqlc generation**
   - Add household stanza to `sqlc.yaml`.
   - Update `queries.sql` files in `identity`, `userprefs`, `inventory`, `wine`, `mealplan`, `grocery`.
   - Run `go generate ./...` and `sqlc generate` as needed.

3. **Domain services**
   - Implement `internal/household/service.go`.
   - Update `internal/identity/service.go` with household/search helpers.
   - Update `internal/userprefs/service.go` for household filtering and favorite tables.
   - Update `internal/mealplan`, `internal/grocery`, `internal/inventory`, `internal/wine` to use `HouseholdID`.

4. **BFF**
   - Update `schema.graphqls`.
   - Update `currentuser.User`.
   - Update `auth.go` to ensure a default household.
   - Add `resolver_household.go` and update existing resolvers to use `currentuser.HouseholdID`.
   - Wire `HouseholdService` in `cmd/lena/main.go`.

5. **Web**
   - Update `lib/types.ts` and `lib/api.ts`.
   - Update `app/profile/page.tsx`.
   - Create `app/household/page.tsx`.
   - Update `app/page.tsx` dashboard for pending invites.

6. **Tests**
   - Write Go unit and integration tests.
   - Write Jest component tests.

7. **Verification**
   - `go test ./cmd/... ./internal/...`
   - `go vet ./...`
   - `gofmt -w` all changed Go files
   - `cd clients/web && npm test`
   - `cd clients/web && npm run build`

8. **Branch and PR**
   - Per `AGENTS.md`, create a `phase-<N>` branch (e.g. `phase-6` or `phase-household`) from `main`.
   - Copy the approved plan to `docs/household-plan.md` and commit both the plan and any supporting design notes.
   - Open a pull request to `main` summarizing the household feature, data model, and security invariants.

## 10. Verification Commands

```sh
# Go
 go test ./cmd/... ./internal/...
 go vet ./cmd/... ./internal/...

# Web
 cd clients/web
 npm test
 npm run build

# Optional full stack smoke
 docker compose up --build
```

## 11. Risks and Considerations

- **Data migration complexity.** The migration renames `inventory.user_item` and `wine.user_bottle`, extracts `is_favorite` into new tables, and backfills household IDs. A mistake here would be destructive; validate with `testenv` integration tests and a full round-trip `up`/`down` test on a copy of production data.
- **Merging on accept.** When two single-person households merge, duplicate `item_id`/`bottle_id` rows require a deterministic merge rule (sum quantities, keep latest dates). Document this in the API; users may need to reconcile stock after joining.
- **Race conditions on invites.** The partial unique index and transaction wrapping in `acceptHouseholdInvite` prevent double-accepts but require careful ordering.
- **Cross-domain transaction orchestration.** The BFF resolver for `acceptHouseholdInvite` must use `dbtx.InTx` with `household.WithTx`, `identity.WithTx`, and `userprefs.WithTx` to keep the household, user, and data moves atomic.
- **Web build compatibility.** `clients/web/CLAUDE.md` warns that the Next.js build is non-standard; verify any new pages against `node_modules/next/dist/docs` before relying on familiar patterns.
- **Mobile regressions.** Mobile uses the same `me`, `mealPlans`, `groceryLists`, `userItems`, and `userBottles` queries. The resolvers must continue returning the same GraphQL shapes; only the backing `household_id` and `isFavorite` resolution changes.
