# LENA2 Household Sharing Implementation Plan

Add multi-user household support to LENA2 so members share pantry stock, wine cellar holdings, meal plans, and grocery lists while keeping recipe favorites and item/wine likes personal, including database migrations, a new `internal/household` service, BFF GraphQL changes, a Next.js web UI, and unit/integration tests.

## Revision notes (post-audit-remediation update)

This plan predates the audit remediation program (phases 1–8). The following constraints from that work are now load-bearing and this revision encodes them:

- **Schema reality (A1-05):** `inventory.user_item`, `wine.user_bottle`, and `recipe.user_recipe_preference` moved to the `userprefs` schema in migration `0026`. All references below use current names.
- **Migration numbering:** `0024`–`0027` are taken; the household migration is `0028`.
- **Typed error contract (A1-02/A3-04):** every new service method wraps storage errors with `domainerr.FromStorage`; validation uses `domainerr.ErrValidation`/`ValidationError`; BFF never inspects `pgx`/`pgconn` directly.
- **No silent writes (A3-01):** new UPDATE/DELETE queries are `:one ... RETURNING` or `:execrows`; zero-row outcomes surface as `ErrNotFound`/`ErrConflict`, never `nil`.
- **Atomic multi-domain writes (A1-03):** cross-domain transactions are orchestrated in the BFF via `r.unitOfWork().InTx(ctx, fn)`; services join the ctx-carried transaction automatically through `dbtx.ContextExecer`. The BFF does not open `pgx` transactions directly.
- **Check-then-act (A3-05/A3-06 pattern):** state changes are single guarded statements (`WHERE status = 'pending'`, `WHERE household_id = $expected`), not read-then-write.
- **Auth cache (A2-05):** `Authenticator` caches `currentuser.User` per `issuer|subject` with a TTL. Any mutation that changes `household_id` or `is_searchable` must invalidate the cache or the caller serves stale context.
- **GraphQL contract (A1-11):** new stringly-typed fields are not allowed — invite status is a real `enum`.
- **PII surface:** `userResolver.Email()`, `Role()`, `IsProtected()`, `LastLoginAt()` return unconditionally. `searchHouseholdUsers` must therefore return a restricted type, not `User`.
- **Test infrastructure (A1-12/A4-05):** `platform/testenv` no longer exists — integration tests use `internal/testutil.NewTestDB`. `doGraphQL` is strict; use `doGraphQLExpectErrors` for negative cases.
- **Resolver size (A2-12):** production resolvers stay under ~60 lines; invite-accept orchestration is extracted to a helper.

## Implementation status

- **p1 (merged, PR #138):** additive migration `0028` (household schema, invites, `users.household_id`/`is_searchable`, deterministic `household_id = user_id` backfill), `internal/household` service, `internal/identity` household/search methods, `currentuser.User.HouseholdID`/`IsSearchable` fields.
- **p2 (this branch):** switchover migration `0029`, `userprefs.household_item`/`household_bottle`, `user_item_favorite`/`user_bottle_favorite` split, `MergeHouseholdStock`/`ReassignHousehold`, `mealplan`/`grocery` `household_id`, analytics household fan-out, all BFF resolver call sites on `HouseholdID`/`UserID` as appropriate. **Pulled forward from p3:** the authenticator's default-household ensure (`ensureDefaultHousehold` on the cache-miss path) and `InvalidateUser` — required so `u.HouseholdID` is always populated before household-scoped queries run.
- **p3 (this branch):** GraphQL schema (`HouseholdUser`, `InviteStatus` enum, household/invite resolvers), invite accept/leave orchestration with cache invalidation, `me.isSearchable`/`me.household`.
- **p4 (next):** web UI — `/household` page, invite dashboard card, profile `isSearchable` toggle.
- **p4:** web UI.

## 1. Objective

Allow authenticated LENA2 users to form a household with one other user. The household shares the operational data (pantry, wine, meal plans, grocery lists) but preserves per-user preferences (recipe favorites, item/bottle likes). Users can search for other users by name or email, send a household invitation, and have the recipient approve or decline it from the dashboard. Either member may leave the household and return to a single-person household.

## 2. Acceptance Criteria

- Every user has a default single-person household created automatically on first login (inside the authenticator's cache-miss path — see §6.6).
- `me` GraphQL query returns the caller's `isSearchable` and `household` fields; `household` resolves non-null only when the `User` being resolved is the caller (self-gated — see §6.6).
- `searchHouseholdUsers` returns `HouseholdUser` results (no email/role/activity fields) matching a name/email term, excluding the caller, users already in the caller's household, inactive users, and users who have opted out of search.
- `inviteHouseholdMember` creates a pending invitation; the target sees it on the dashboard. A duplicate pending invite between the same pair returns `CONFLICT`.
- `acceptHouseholdInvite` atomically moves the target into the inviter's household and merges the target's default household data (pantry, cellar, meal plans, grocery lists) in a single transaction.
- `declineHouseholdInvite` and `cancelHouseholdInvite` close the invitation without merging data; each enforces actor rules (`to_user` accepts/declines, `from_user` cancels).
- `leaveHousehold` atomically moves the caller to a new default single-person household.
- Pantry (`userprefs.household_item`), wine (`userprefs.household_bottle`), `mealplan.meal_plan`, and `grocery.grocery_list` are filtered by household.
- `is_favorite` for items and bottles becomes a per-user preference; recipe favorites remain per-user (`userprefs.user_recipe_preference` unchanged).
- The web `/profile` page exposes a "searchable" toggle and a household section.
- A new web `/household` page shows household members, pending invitations, and a search/invite form.
- All new Go code has unit tests; integration tests cover the merge/leave flows against real Postgres; all new web pages have Jest component tests.

## 3. Scope

### In Scope
- Database schema for households, invitations, and household-scoped tables.
- Migration of existing per-user data into default households.
- New `internal/household` Go package and `HouseholdService`.
- Updates to `internal/identity`, `internal/userprefs`, `internal/mealplan`, `internal/grocery`.
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
  - userprefs.household_item      (was userprefs.user_item)
  - userprefs.household_bottle    (was userprefs.user_bottle)
  - mealplan.meal_plan            (household_id column)
  - grocery.grocery_list          (household_id column)

Per-user preference tables (unchanged ownership: userprefs schema):
  - userprefs.user_item_favorite (user_id, item_id, is_favorite)
  - userprefs.user_bottle_favorite (user_id, bottle_id, is_favorite)
  - userprefs.user_recipe_preference (unchanged)
```

The BFF orchestrates cross-domain work (identity, household, userprefs, grocery, mealplan) inside `unitOfWork` transactions. Each domain service remains schema-bound. `currentuser.User` gains `HouseholdID` and `IsSearchable` so resolvers pass household context to the service layer without trusting client input.

## 5. Database Migrations

The migration is split across two phases so `main` never carries a schema the
deployed code can't serve:

- `migrations/0028_household.up.sql` / `.down.sql` — **additive** (phase 1):
  household schema, `household.invites`, `identity.users.household_id` +
  `is_searchable` with backfill. Nothing existing is renamed or dropped.
- `migrations/0029_household_switchover.up.sql` / `.down.sql` — **destructive**
  (lands with the phase that switches reads/writes to `household_id`):
  `household_id` columns + backfill on the four shared tables, favorites
  extraction, `user_id`/`is_favorite` drops, and the `user_item`/`user_bottle`
  renames. Splitting it this way avoids NOT NULL violations on rows written
  between the column add and the code switch, and keeps each migration paired
  with the code that serves it.

### 5.0 Phase-1 migration (0028) — additive

```sql
CREATE SCHEMA household;

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
    status       VARCHAR(20) NOT NULL DEFAULT 'pending'
                 CHECK (status IN ('pending','accepted','declined','cancelled')),
    created_by   VARCHAR(100) NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by   VARCHAR(100),
    updated_at   TIMESTAMPTZ
);

CREATE INDEX idx_invites_to_status   ON household.invites (to_user_id, status);
CREATE INDEX idx_invites_from_status ON household.invites (from_user_id, status);

CREATE UNIQUE INDEX idx_invites_pending_unique
    ON household.invites (from_user_id, to_user_id)
    WHERE status = 'pending';

ALTER TABLE identity.users
    ADD COLUMN household_id  BIGINT REFERENCES household.households(household_id),
    ADD COLUMN is_searchable BOOLEAN NOT NULL DEFAULT TRUE;

CREATE INDEX idx_users_household ON identity.users (household_id);

-- One single-person household per existing user; deterministic 1:1 ids.
INSERT INTO household.households (household_id, created_by)
SELECT user_id, left(email, 100) FROM identity.users ORDER BY user_id;
SELECT setval('household.households_household_id_seq',
              (SELECT COALESCE(MAX(household_id), 1) FROM household.households));
UPDATE identity.users SET household_id = user_id;
```

`0028_household.down.sql` drops the users columns, both household tables, and
the schema.

### 5.1 Switchover migration (0029) — reference sketch

The switchover lands in the phase that flips the services to `household_id`.
The household schema/tables and the `identity.users` columns already exist
from 0028 (§5.0); what remains is:

#### Pantry (userprefs.user_item → userprefs.household_item)

```sql
ALTER TABLE userprefs.user_item
    ADD COLUMN household_id BIGINT;

-- backfill using the user's default household
UPDATE userprefs.user_item ui
SET household_id = u.household_id
FROM identity.users u
WHERE ui.user_id = u.user_id;

-- extract personal favorites before dropping the column
CREATE TABLE userprefs.user_item_favorite (
    user_id     BIGINT NOT NULL REFERENCES identity.users(user_id) ON DELETE CASCADE,
    item_id     BIGINT NOT NULL REFERENCES inventory.item(item_id) ON DELETE CASCADE,
    is_favorite BOOLEAN NOT NULL DEFAULT FALSE,
    created_by  VARCHAR(100) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by  VARCHAR(100),
    updated_at  TIMESTAMPTZ,
    PRIMARY KEY (user_id, item_id)
);

INSERT INTO userprefs.user_item_favorite (user_id, item_id, is_favorite, created_by, created_at)
SELECT user_id, item_id, is_favorite, created_by, created_at
FROM userprefs.user_item
WHERE is_favorite;

ALTER TABLE userprefs.user_item
    DROP COLUMN is_favorite,
    DROP COLUMN user_id,
    ALTER COLUMN household_id SET NOT NULL,
    ADD CONSTRAINT household_item_household_item UNIQUE (household_id, item_id);

ALTER TABLE userprefs.user_item RENAME TO household_item;
```

#### Wine (userprefs.user_bottle → userprefs.household_bottle)

Same pattern as pantry:

```sql
ALTER TABLE userprefs.user_bottle ADD COLUMN household_id BIGINT;
UPDATE userprefs.user_bottle ub SET household_id = u.household_id FROM identity.users u WHERE ub.user_id = u.user_id;

CREATE TABLE userprefs.user_bottle_favorite (
    user_id     BIGINT NOT NULL REFERENCES identity.users(user_id) ON DELETE CASCADE,
    bottle_id   BIGINT NOT NULL REFERENCES wine.bottle(bottle_id) ON DELETE CASCADE,
    is_favorite BOOLEAN NOT NULL DEFAULT FALSE,
    created_by  VARCHAR(100) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by  VARCHAR(100),
    updated_at  TIMESTAMPTZ,
    PRIMARY KEY (user_id, bottle_id)
);

INSERT INTO userprefs.user_bottle_favorite (user_id, bottle_id, is_favorite, created_by, created_at)
SELECT user_id, bottle_id, is_favorite, created_by, created_at
FROM userprefs.user_bottle
WHERE is_favorite;

ALTER TABLE userprefs.user_bottle
    DROP COLUMN is_favorite,
    DROP COLUMN user_id,
    ALTER COLUMN household_id SET NOT NULL,
    ADD CONSTRAINT household_bottle_household_bottle UNIQUE (household_id, bottle_id);

ALTER TABLE userprefs.user_bottle RENAME TO household_bottle;
```

#### Meal plan and grocery list

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

### 5.2 Down migrations

`0028_household.down.sql` is a clean structural reversal (drop columns,
tables, schema — no data is at risk since nothing user-facing depends on it
yet). `0029_household_switchover.down.sql` reverses in dependency order:
re-add `user_id` columns, restore per-user rows from `household_id`,
reconstruct `is_favorite` from the favorite tables, drop the favorite tables
and household columns. Because the up migration aggregates data, the 0029
down migration is a best-effort rollback and should be clearly marked as such.

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

Add to `queries.sql` (all mutations `:one ... RETURNING` or `:execrows` — zero rows must surface, per A3-01):
- `GetUserByID` returns `household_id` and `is_searchable`.
- `SetUserHousehold` — **conditional**: `SET household_id = $2 WHERE user_id = $1 AND household_id = $3` (`:execrows`). The `expectedHouseholdID` guard turns the double-accept race into an `ErrConflict` instead of a lost update.
- `SetUserSearchable` updates `is_searchable`.
- `ListUsersByHousehold` returns users in a household.
- `ListUsersByIDs` batch fetch for invite/member hydration (avoids N+1 on `members`/`fromUser`/`toUser`).
- `SearchUsers` name/email match filtered by `is_searchable = true AND is_active = true`, excluding caller and same-household users, `LIMIT` bound.
- `UpsertUser` inserts with a null `household_id` for new rows; the authenticator creates a default household on the first cache-miss (§6.6).

Add service methods (all wrap storage errors with `domainerr.FromStorage`):
- `GetByID` (updated)
- `SetUserHousehold(ctx, userID, householdID, expectedHouseholdID int64) error`
- `SetUserSearchable(ctx, userID int64, isSearchable bool, by string) error`
- `ListUsersByHousehold(ctx, householdID int64) ([]User, error)`
- `ListUsersByIDs(ctx, ids []int64) ([]User, error)`
- `SearchUsers(ctx, term string, excludeUserID, excludeHouseholdID int64, limit int32) ([]User, error)`

### 6.3 New package `internal/household`

Files to create:

- `internal/household/queries.sql`
- `internal/household/sqlc/` (db.go, generate.go, mock, querier.go, queries.sql.go, models.go)
- `internal/household/service.go`
- `internal/household/service_test.go`
- `internal/household/integration_test.go`

Tables owned: `household.households`, `household.invites`. The service takes a `dbtx.Pool` and builds its querier over `dbtx.ContextExecer` (matching every other domain service) so calls inside a `unitOfWork().InTx` join the caller's transaction.

Invite writes are status-guarded, not check-then-act:

```sql
-- name: TransitionInvite :one
UPDATE household.invites
SET status = $3, updated_by = $4, updated_at = now()
WHERE invite_id = $1 AND status = 'pending'
RETURNING *;
```

Zero rows → `domainerr.ErrConflict` (invite already resolved). `CreateInvite` relies on `idx_invites_pending_unique` → `domainerr.ErrConflict` on `23505` via `FromStorage`.

Service methods:

```go
func (s *Service) CreateHousehold(ctx context.Context, by string) (Household, error)
func (s *Service) GetHouseholdByID(ctx context.Context, householdID int64) (Household, error)          // ErrNotFound
func (s *Service) CreateInvite(ctx context.Context, fromUserID, toUserID, householdID int64, by string) (Invite, error) // ErrConflict on duplicate pending
func (s *Service) GetInviteByID(ctx context.Context, inviteID int64) (Invite, error)                  // ErrNotFound
func (s *Service) ListPendingInvitesForUser(ctx context.Context, userID int64) ([]Invite, error)
func (s *Service) ListSentInvitesForUser(ctx context.Context, userID int64) ([]Invite, error)
func (s *Service) TransitionInvite(ctx context.Context, inviteID int64, to Status, by string) (Invite, error) // ErrConflict if not pending
```

### 6.4 `internal/userprefs`

- Update `queries.sql` to read from `userprefs.household_item` and `userprefs.household_bottle` filtered by `household_id`. All user-scoped `WHERE user_id` predicates become `WHERE household_id` on the stock tables.
- Move `is_favorite` reads/writes to `userprefs.user_item_favorite` and `userprefs.user_bottle_favorite`; the `UserItem` and `UserBottle` service models no longer contain `IsFavorite`.
- Add `GetUserItemFavorite`/`SetUserItemFavorite` (and bottle equivalents) keyed by `(user_id, item_id)` — favorites stay **per-user**, never household-scoped.
- Add `MergeHouseholdStock(ctx, fromHouseholdID, toHouseholdID int64) error`: merges `household_item` and `household_bottle` rows. On `(household_id, item_id)`/`(household_id, bottle_id)` collisions, sum `current_qty`/`quantity` and keep the most recent non-null `purchase_at`, `expires_at`, `location`, `notes`. Single `INSERT ... SELECT ... ON CONFLICT` statements — atomic inside the caller's `unitOfWork`.
- The atomic `GREATEST(0, qty + delta)` pantry adjustment (A2-01) is preserved unchanged — only its `WHERE` clause switches to `household_id`.

### 6.5 `internal/mealplan`, `internal/grocery`

For each package:
- Update `queries.sql` to filter `mealplan.meal_plan` / `grocery.grocery_list` by `household_id` instead of `user_id`.
- Update service method signatures from `userID int64` to `householdID int64`.
- Update service models (`MealPlan`, `GroceryList`) to use `HouseholdID`.
- Add `ReassignHousehold(ctx, fromHouseholdID, toHouseholdID int64) error` (`UPDATE ... SET household_id = $2 WHERE household_id = $1`, `:exec` is fine — repointing zero rows is not an error) for the invite-accept merge. These live in their own domains — `userprefs` cannot write them (schema-per-module).
- Regenerate sqlc code; update unit tests and mock expectations.

`internal/inventory` and `internal/wine` catalog tables (items, bottles, brands, nutrient types) stay global — no `household_id` changes. Only their favorites wiring shifts (§6.4).

### 6.6 `internal/bff`

- `schema.graphqls` additions — note the restricted user type and the enum (A1-11):

```graphql
enum InviteStatus {
  PENDING
  ACCEPTED
  DECLINED
  CANCELLED
}

# Restricted projection for household flows — intentionally omits email,
# role, isProtected, lastLoginAt. The full User type returns those fields
# unconditionally, so it must never be used for other-users data.
type HouseholdUser {
  id: ID!
  displayName: String
  firstName: String
  lastName: String
}

type Household {
  id: ID!
  members: [HouseholdUser!]!
  createdAt: Time!
}

type HouseholdInvite {
  id: ID!
  fromUser: HouseholdUser!
  toUser: HouseholdUser!
  status: InviteStatus!
  createdAt: Time!
}
```

- `UpdateProfileInput` gains `isSearchable: Boolean`.
- `User` type gains `isSearchable: Boolean!` and `household: Household`. The `household` field resolver returns non-null **only when the resolved user is the caller** — otherwise every `adminUsers` page would leak household membership and N+1 against `household.households`.
- Queries/mutations:

```graphql
extend type Query {
  myHousehold: Household
  householdInvites: [HouseholdInvite!]!
  searchHouseholdUsers(term: String!, limit: Int = 20): [HouseholdUser!]!
}

extend type Mutation {
  inviteHouseholdMember(userId: ID!): HouseholdInvite!
  acceptHouseholdInvite(inviteId: ID!): Household!
  declineHouseholdInvite(inviteId: ID!): HouseholdInvite!
  cancelHouseholdInvite(inviteId: ID!): HouseholdInvite!
  leaveHousehold: Boolean!
}
```

- `services.go`: add a role-scoped `HouseholdService` interface; extend `IdentityService` with the §6.2 methods; regenerate gomock.
- `resolver_household.go` (new):
  - `inviteHouseholdMember`: reject self-invite and already-household-mate with `ErrValidation`; create invite (`ErrConflict` → `CONFLICT` on duplicate pending).
  - `acceptHouseholdInvite` / `declineHouseholdInvite`: caller must be `invite.to_user`; `cancelHouseholdInvite`: caller must be `invite.from_user`. Non-party access returns `NOT_FOUND` (do not leak invite existence).
  - `acceptHouseholdInvite` runs in `r.unitOfWork().InTx` — orchestration extracted to a helper (`acceptInvite`, §A2-12 size rule):
    1. `TransitionInvite(id, accepted)` — conflict if already resolved.
    2. `userprefs.MergeHouseholdStock(targetHH → inviterHH)`.
    3. `mealplan.ReassignHousehold`, `grocery.ReassignHousehold`.
    4. `identity.SetUserHousehold(target, inviterHH, expectedHouseholdID = targetHH)` — conflict if the target's household changed mid-flight.
    5. Invalidate the auth cache for **both** users (below).
  - `leaveHousehold`: `CreateHousehold` + conditional `SetUserHousehold` in one `InTx`; invalidate caller's cache.
  - `searchHouseholdUsers`: `term` trimmed, minimum length 2; `limit` clamped to `[1,50]` (same `clamp` helper pattern as `pageArgs`); returns `HouseholdUser` projections.
- `resolver_identity.go`: `me` returns `isSearchable`; `updateMyProfile` accepts `isSearchable`; `userResolver.Household` self-gated.
- Existing resolvers (`resolver_userprefs.go`, `resolver_mealplan.go`, `resolver_grocery.go`, and the `isFavorite` field resolvers in `resolver_inventory.go`/`resolver_wine.go`) pass `u.HouseholdID` for stock/plan/list data and `u.UserID` for favorites.
- `auth.go` changes — the cache is the trap to get right:
  1. Default-household ensure happens **inside the `cachedUser` miss path only** — after `UpsertUser`, if `user.HouseholdID == 0`, create a household and `SetUserHousehold` (expected = null/0), then populate `currentuser.User.HouseholdID`/`IsSearchable`. This runs once per cache TTL, not per request (A2-05).
  2. Add `InvalidateUser(provider, subject string)` (key eviction) and `InvalidateUserID(ctx, userID)` (scan the cache map — it is small) on `Authenticator`. Household mutations call both parties' invalidations; `updateMyProfile` invalidates on `isSearchable` change.
- Nested fetch depth: `householdInvites { items }` hydrates `fromUser`/`toUser` via one `ListUsersByIDs` batch; `members` via one `ListUsersByHousehold` — no per-row queries (A2-07).

### 6.7 `cmd/lena/main.go`

- Wire `householdSvc := household.NewService(pool)`.
- Pass `householdSvc` to `bff.NewResolver` and `bff.NewAuthenticator` (update their constructors/signatures).

## 7. Web UI Changes

Files:

- `clients/web/lib/types.ts` — add `Household`, `HouseholdInvite`, `HouseholdUser`, `InviteStatus` types; extend `User` with `household` and `isSearchable`.
- `clients/web/lib/api.ts` — add API functions for `myHousehold`, `householdInvites`, `searchHouseholdUsers`, `inviteHouseholdMember`, `acceptHouseholdInvite`, `declineHouseholdInvite`, `cancelHouseholdInvite`, `leaveHousehold`, and add `isSearchable` to `updateMyProfile`.
- `clients/web/app/profile/page.tsx` — add a FormControlLabel/Switch for `isSearchable` and send it with `updateMyProfile`.
- `clients/web/app/household/page.tsx` (new) — page with:
  - Current household members list.
  - Incoming/outgoing pending invitations.
  - Search field with `searchHouseholdUsers` and "Invite" buttons.
  - "Leave household" button with confirmation.
- `clients/web/app/page.tsx` — add a card to the dashboard that shows pending household invites and buttons to accept/decline.
- Update navigation (e.g. `clients/web/app/components/AdminLayout.tsx`) to add a "Household" link if appropriate.

Follow `clients/web/AGENTS.md` and verify the current Next.js API in `node_modules/next/dist/docs` before adding pages or data fetching, as the project uses a non-standard Next.js build.

## 8. Testing Strategy

### 8.1 Go

- `internal/household/service_test.go` — gomock sqlc querier tests for create household, create/transition/list invites, `ErrConflict` on non-pending transition and duplicate pending.
- `internal/household/integration_test.go` — `testutil.NewTestDB`: full invite lifecycle, double-accept race guard (second `TransitionInvite` → `ErrConflict`), duplicate-pending `23505 → ErrConflict`. If a fake `Store` is kept for fast tests, pin it with a shared contract suite run against the SQL store (the `recipeimport` A4-09 pattern) — never let the double diverge silently.
- `internal/identity/service_test.go` / integration — `SetUserHousehold` (incl. expected-mismatch → `ErrConflict`), `SetUserSearchable`, `SearchUsers` (exclusions: caller, same-household, opted-out, inactive), `ListUsersByHousehold`/`ListUsersByIDs`.
- `internal/bff/resolver_household_test.go` — gomock services: authz (non-party → `NOT_FOUND`, wrong actor → `FORBIDDEN`), self-invite/`ErrValidation`, accept orchestration order, cache invalidation calls.
- `internal/bff/bff_integration_test.go` — extend the cross-user suite to cross-**household** denial (post-migration, `userID` scoping becomes `householdID` scoping — every existing cross-user test must be re-aimed); strict `doGraphQL`/`doGraphQLExpectErrors` as appropriate.
- `internal/userprefs`, `internal/mealplan`, `internal/grocery`, `internal/inventory`, `internal/wine` tests — update `userID` → `householdID` expectations; add `MergeHouseholdStock` collision tests (qty sum, latest non-null wins); favorite-table isolation tests (favorite survives household change).
- Regenerate mocks after sqlc changes (`go generate ./internal/...`).

### 8.2 Web

- `clients/web/__tests__/lib/api-household.test.ts` — mock GraphQL for the new API calls.
- `clients/web/__tests__/app/profile/household-settings.test.tsx` — profile `isSearchable` toggle.
- `clients/web/__tests__/app/household/page.test.tsx` — household page rendering, search, invite, accept, leave.
- `clients/web/__tests__/app/page.test.tsx` — dashboard pending invite card.

## 9. Implementation Steps

1. **Schema and data layer (phase 1 — this branch)**
   - `migrations/0028_household.up.sql`/`down.sql`: additive foundation only (§5.0).
   - `internal/household` package: queries, sqlc, service, tests.
   - `internal/identity` household/search additions; `currentuser.User` fields.
   - `testutil`-backed integration tests verify the migration and invite guards.

2. **sqlc generation**
   - Add household stanza to `sqlc.yaml`.
   - Update `queries.sql` files in `identity`, `userprefs`, `mealplan`, `grocery`.
   - Run `go generate ./...` / `sqlc generate` as needed.

3. **Domain services**
   - Implement `internal/household/service.go` over `dbtx.ContextExecer`.
   - Update `internal/identity/service.go` with household/search helpers.
   - Update `internal/userprefs/service.go` for household filtering, favorite tables, `MergeHouseholdStock`.
   - Update `internal/mealplan`, `internal/grocery` for `HouseholdID` + `ReassignHousehold`.

4. **BFF**
   - Update `schema.graphqls` (enum + restricted `HouseholdUser`).
   - Update `currentuser.User`.
   - Update `auth.go`: default-household ensure on cache miss; `InvalidateUser`/`InvalidateUserID`.
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

7. **Verification** — match CI (`test.yml`), not just local:
   - `go test -race -count=1 ./cmd/... ./internal/...` (coverage gate ≥ `GO_COVERAGE_MIN`)
   - `go vet ./...`
   - `gofmt -l` clean; `golangci-lint run` clean
   - `cd clients/web && npm test && npm run lint && npm run build`
   - `docker compose config` still valid

8. **Branch and PR**
   - Per `AGENTS.md`, work on a dedicated branch (`household` or `household-p1`; the `audit-remediation-phaseN` series is complete).
   - Commit this plan revision alongside the implementation.
   - Open a pull request to `main` summarizing the household feature, data model, and security invariants; merge only after CI is green.

## 10. Verification Commands

```sh
# Go
go test -race -count=1 ./cmd/... ./internal/...
go vet ./...
golangci-lint run ./...

# Web
cd clients/web
npm test
npm run lint
npm run build

# Compose sanity
docker compose config
```

## 11. Risks and Considerations

- **Data migration complexity.** The migration renames `userprefs.user_item` and `userprefs.user_bottle`, extracts `is_favorite` into new tables, and backfills household IDs. A mistake is destructive; validate with `testutil` integration tests and a full `up`/`down` round-trip on a copy of production data.
- **Merging on accept.** Duplicate `item_id`/`bottle_id` rows merge deterministically (sum quantities, latest non-null fields win). Document in the API; users may need to reconcile stock after joining.
- **Auth cache staleness (new).** `HouseholdID`/`IsSearchable` live in the cached `currentuser.User`. Any missed invalidation after accept/leave/profile-toggle serves stale household context until TTL — the invalidation calls in `acceptHouseholdInvite`, `leaveHousehold`, and `updateMyProfile` are mandatory, and the resolver tests must assert them.
- **Accept-time race.** Two concurrent accepts (or accept vs. leave) are serialized by the `TransitionInvite` pending-guard plus the conditional `SetUserHousehold`; the loser gets `ErrConflict`, never a silent double-move.
- **PII exposure (new).** Returning `User` from `searchHouseholdUsers` would leak email/role/activity of arbitrary users — hence `HouseholdUser`. The same applies to `HouseholdInvite.fromUser`/`toUser` and `Household.members`.
- **Mobile regressions.** Mobile uses the same `me`, `mealPlans`, `groceryLists`, `userItems`, and `userBottles` queries. Resolver shapes stay identical; only the backing `household_id`/`isFavorite` resolution changes. `me.household` is a new field — mobile ignores unknown fields.
- **Web build compatibility.** `clients/web/AGENTS.md` warns the Next.js build is non-standard; verify new pages against `node_modules/next/dist/docs` first.
