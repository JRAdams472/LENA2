# Surprise-Ready Recommendation Plan (Interaction Tracking + 4 Requested Features)

Track user interactions (searches, dropdown selections, recipe creation, menu adds, ratings) in a new `analytics` domain, and use that data to ship four concrete recommendation features now with lightweight Go heuristics, structured so a future Python/Surprise collaborative-filtering job can plug into the same serving table later.

## Decisions locked in (from clarifying questions)
- **Surprise scope**: Phase heuristics first (Phases 1-4 below). The actual Python/Surprise microservice is deferred to a later, optional Phase 5 — but Phase 4's cache table is designed so Surprise can write into it later with zero BFF/API changes.
- **Frequency ranking**: Personal history first, global popularity as fallback (blended).
- **Rating scale**: 1-5 stars, one rating per user per recipe (mirrors the existing `recipe.user_recipe_preference` favorite-table pattern).
- **Similarity signal for "similar ingredients"**: Shared catalog items now (`recipe_item.item_id` overlap); a recipe tagging system is called out as a future enhancement for richer content-based matching.

Not following `docs/updates.md`'s "Future Phases" section as written — this plan replaces it, though it reuses the same eventual `user_id | recipe_id | reason/score` cache-table idea for the "Serve" step so a later Surprise job is a drop-in.

## Current state (relevant findings)
- No ratings table exists anywhere (`grep rating` across `internal/` is empty).
- No interaction/analytics logging exists (`internal/platform/telemetry` is OpenTelemetry tracing/metrics only, not user-behavior logging).
- No server-side search: `clients/web/lib/api.ts` fetches the *entire* item/recipe catalog client-side (`fetchItemsWithPrefs`, `api.getRecipes`) and filters/paginates in JS. Brand/item dropdowns (`app/inventory/items/page.tsx`) are plain MUI `Autocomplete` over `api.getBrands()` / full item list — no frequency ordering.
- Domain packages (`internal/recipe`, `internal/inventory`, `internal/grocery`, `internal/userprefs`, `internal/wine`) all follow the same shape: a `Service` wrapping a generated `sqlc.Querier`, `WithTx`/`InTx` helpers, plain Go structs, migrations per schema (`migrations/000N_*.up/down.sql`), and mocked interfaces in `internal/bff/services.go` / `internal/bff/mock`.
- `recipe.recipe_item(recipe_id, item_id, quantity, unit, ...)` already gives us ingredient-overlap data for free.
- `mealplan.meal_plan` + `mealplan.meal_slot(recipe_id)` is the existing "menu history" — no new table needed to know what a user has planned/cooked over time.
- BFF (`internal/bff/resolver.go`, `resolver_*.go`) is the only layer allowed to orchestrate across domains; batch-loading via `*Children` helper structs (e.g. `recipeChildren`, `itemChildren`) is the established pattern for avoiding N+1 resolver calls — new analytics fields should follow this exact pattern.
- Git workflow (`AGENTS.md`): one branch per phase, named `phase-<N>`, PR into `main` after verification. Latest existing branch is `phase-19`, so this work starts at **phase-20**.

## Phase 20 — Interaction Tracking Foundation
**Branch:** `phase-20`

Goal: a generic, low-overhead way to record implicit/explicit signals and get fast frequency counts, without yet changing any user-facing behavior.

1. **Migration `0014_create_analytics.up/down.sql`** — new `analytics` schema (added to `0001`-style `CREATE SCHEMA` list via a new migration, not editing `0001`):
   - `analytics.interaction_event`: `event_id BIGSERIAL PK, user_id BIGINT NULL REFERENCES identity.users ON DELETE SET NULL, event_type VARCHAR(40) NOT NULL, entity_type VARCHAR(20) NULL, entity_id BIGINT NULL, search_term VARCHAR(500) NULL, weight SMALLINT NOT NULL DEFAULT 1, metadata JSONB NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now()`. Indexes on `(event_type, created_at)`, `(entity_type, entity_id)`, `(user_id, created_at)`.
     - `weight` pre-encodes the signal-value table from `docs/updates.md` (rating=5, menu_add=4, recipe_create=4, search=2, browse/select=1) so any future aggregation (including Surprise) doesn't need a lookup table.
   - `analytics.user_selection_count`: `entity_type VARCHAR(20), entity_id BIGINT, user_id BIGINT REFERENCES identity.users ON DELETE CASCADE, select_count BIGINT NOT NULL DEFAULT 0, last_selected_at TIMESTAMPTZ, PRIMARY KEY (entity_type, entity_id, user_id)`.
   - `analytics.global_selection_count`: same minus `user_id`, `PRIMARY KEY (entity_type, entity_id)`.
   - (Two rollup tables instead of one nullable-`user_id` table, because Postgres unique constraints treat `NULL` as distinct — a single table can't safely hold one canonical "global" row per entity.)
2. **New Go package `internal/analytics`** (mirrors `internal/userprefs` shape):
   - `sqlc/` queries: `InsertInteractionEvent`, `UpsertUserSelectionCount`, `UpsertGlobalSelectionCount`, `GetUserSelectionCounts(entityType, userID, entityIDs)`, `GetGlobalSelectionCounts(entityType, entityIDs)`, `TopUserSelections(entityType, userID, limit)`, `TopGlobalSelections(entityType, limit)`.
   - `service.go`: `Service.RecordEvent(ctx, Event{UserID, EventType, EntityType, EntityID, SearchTerm, Weight}, by string) error` — inserts the raw event, then (when `EntityType`/`EntityID` set) upserts both rollup tables in the same transaction via `InTx`.
   - Event-type constants: `EventItemSelected`, `EventBrandSelected`, `EventRecipeSelected`, `EventItemSearched`, `EventRecipeSearched`, `EventRecipeCreated`, `EventMenuAdd`, `EventRatingGiven` (weights per the doc's signal table).
3. **BFF wiring**:
   - `internal/bff/services.go`: add `AnalyticsService` interface (`RecordEvent`, count getters) + register on `Resolver` / `NewResolver`.
   - New mutations in `schema.graphqls` + `resolver_analytics.go`: `recordSelection(entityType: String!, entityID: ID!): Boolean!` and `recordSearch(entityType: String!, term: String!): Boolean!` for client-driven events (fire-and-forget from the frontend; swallow/log errors so a tracking failure never breaks the UI).
   - Automatic server-side hooks (no client call needed, higher signal fidelity): call `AnalyticsService.RecordEvent` from inside `CreateRecipe` (resolver_recipe.go) with `EventRecipeCreated`, and from the meal-slot recipe-assignment mutation in `resolver_mealplan.go` (whatever it's currently named — confirm exact mutation name during implementation) with `EventMenuAdd`.
4. **Tests**: `internal/analytics/service_test.go` (gomock over generated `sqlc.Querier`, mirrors `internal/inventory/service_test.go`), `internal/analytics/integration_test.go` (real Postgres via existing test harness, mirrors `internal/grocery/integration_test.go`), `internal/bff/resolver_analytics_test.go` (mock `AnalyticsService`).
5. **Verification**: `go build ./...`, `go test ./...`, run migrations up/down against `docker-compose.yml`'s `db` service.

No visible frontend change yet — this phase is pure plumbing.

## Phase 21 — Frequency-Based Ranking (dropdown top-10 + search ordering)
**Branch:** `phase-21`

1. **GraphQL**: add `frequentBrands(limit: Int = 10): [Brand!]!` and `frequentItems(limit: Int = 10): [Item!]!` queries — each blends the current user's `TopUserSelections` with `TopGlobalSelections` as a fallback/fill (personal hits first, then global hits not already included, capped at `limit`).
   - Add `selectionCount`/`personalSelectionCount` fields to `Item`, `Brand`, and `Recipe` GraphQL types, batch-loaded the same way `itemChildren`/`recipeChildren` batch-load brands/categories today (new `selectionCounts` map passed into the resolver structs — no N+1 queries).
2. **Backend**: extend `InventoryService`/`RecipeService` BFF interfaces only if needed — most of this lives in the new `AnalyticsService` batch-count getters from Phase 20; resolvers just call them once per page/list and attach to existing `*Children` structs.
3. **Frontend** (`clients/web/lib/api.ts`, `app/inventory/items/page.tsx`, recipe/meal-plan item pickers):
   - Item/brand `Autocomplete` components: on focus with empty input, call `frequentItems`/`frequentBrands` instead of (or in addition to) the full list, so the dropdown opens showing the top 10 by frequency.
   - On selection (`onChange`), fire `recordSelection` mutation (best-effort, non-blocking).
   - On debounced search input, fire `recordSearch`; sort the (still client-side-filtered) results by returned `selectionCount`/`personalSelectionCount` before slicing to a page, in `getItemsPaged`, `searchItems`, `getRecipesPaged`.
4. **Tests**: BFF resolver tests for the two new queries and new fields (gomock); Jest tests for the updated `Autocomplete` wiring and API sort behavior in `clients/web/__tests__/lib/api-*.test.ts` and the inventory items page test.
5. **Verification**: `go test ./...`, `npm test` in `clients/web`, manual check via `browser_preview` that the items dropdown shows top-10 first.

## Phase 22 — Recipe Rating System
**Branch:** `phase-22`

1. **Migration `0015_create_recipe_rating.up/down.sql`**: `recipe.recipe_rating(user_id BIGINT NOT NULL REFERENCES identity.users ON DELETE CASCADE, recipe_id BIGINT NOT NULL REFERENCES recipe.recipe ON DELETE CASCADE, rating SMALLINT NOT NULL CHECK (rating BETWEEN 1 AND 5), created_by VARCHAR(100) NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_by VARCHAR(100), updated_at TIMESTAMPTZ, PRIMARY KEY (user_id, recipe_id))` — same shape as `recipe.user_recipe_preference`.
2. **`internal/recipe`**: add sqlc queries (`UpsertRecipeRating`, `GetRecipeRating`, `ListRecipeRatingsByRecipes`, `GetRecipeRatingSummary` returning avg+count) and `Service` methods `SetRating`, `GetUserRating`, `ListRatingSummariesByRecipes(ctx, recipeIDs)`.
3. **BFF**: mutation `RateRecipe(recipeID: ID!, rating: Int!): Recipe!` (validates 1-5, upserts, then calls `AnalyticsService.RecordEvent(EventRatingGiven, weight=5)`); `Recipe` fields `myRating: Int`, `averageRating: Float`, `ratingCount: Int`, batch-loaded into `recipeChildren` alongside items/steps/favorites.
4. **Frontend**: star-rating control on `app/recipes/[id]/page.tsx`, wired to `RateRecipe`, displaying `averageRating`/`ratingCount`.
5. **Tests**: `internal/recipe/service_test.go` + integration test additions; BFF resolver tests; Jest component test for the star control; update `RECIPE_FIELDS` fragment in `clients/web/lib/api.ts`.
6. **Verification**: full test suite + manual rating flow in `browser_preview`.

## Phase 23 — Content-Based Recipe Suggestions
**Branch:** `phase-23`

Builds on Phases 20-22's data (`analytics.interaction_event`, `recipe.recipe_rating`, existing `mealplan.meal_slot` history).

1. **Migration `0016_create_recipe_recommendation.up/down.sql`**: `analytics.recipe_recommendation(recommendation_id BIGSERIAL PK, user_id BIGINT NOT NULL REFERENCES identity.users ON DELETE CASCADE, recipe_id BIGINT NOT NULL REFERENCES recipe.recipe ON DELETE CASCADE, reason VARCHAR(30) NOT NULL, score NUMERIC(10,4) NOT NULL, generated_at TIMESTAMPTZ NOT NULL DEFAULT now(), UNIQUE (user_id, recipe_id, reason))`.
   - `reason` values used now: `ingredient_overlap`, `rating_recency`. A later Surprise job would insert `reason = 'collaborative_filtering'` rows into this *same* table — no schema or API change needed for Phase 25 to slot in.
2. **Ingredient-overlap suggestions ("new recipe → users with similar menu history")**:
   - `internal/recipe` (or new `internal/analytics` helper): `ComputeIngredientOverlapSuggestions(ctx, newRecipeID)` — for every user with ≥1 historical `meal_slot.recipe_id`, compute `|shared item_id set| / |union|` (Jaccard) between the new recipe's `recipe_item` set and each of the user's historical recipes' `recipe_item` sets, keep the max score per user, and upsert rows above a threshold (e.g. `score >= 0.3`) into `analytics.recipe_recommendation` with `reason='ingredient_overlap'`.
   - Trigger point: after `CreateRecipe` mutation commits in `resolver_recipe.go`, kick this off as an async goroutine (in-process; no new infra) so recipe creation stays fast. Guard with a small worker helper (e.g. `go func() { ... }()` wrapped with panic recovery and its own bounded-timeout context) — flag in the plan that if this needs to scale later, it should move to a proper job queue, but that's out of scope now.
3. **Rating-recency suggestions ("suggest recipes based on time since last used a highly-rated recipe")**:
   - `internal/recipe`: `ListRatingRecencySuggestions(ctx, userID, minRating=4, limit)` — SQL joining `recipe_rating` (this user, rating ≥ minRating) with the most recent `mealplan.meal_slot`/`meal_plan` date for that `(user, recipe)` pair (via `LEFT JOIN ... GROUP BY ... MAX(week_start_date)`), ordering by days-since-last-used descending (never-used-since-rating sorts first). Computed live at read time (cheap indexed join, no caching needed) rather than cached in the recommendation table, since "now" changes every day and caching would go stale.
4. **GraphQL**: `Query.recommendedRecipes(limit: Int = 10): [RecipeRecommendation!]!` returning `{ recipe, reason, score }`, merging cached `ingredient_overlap` rows for the current user with live `rating_recency` results (interleaved/sorted by score).
5. **Frontend**: "Suggested for You" panel (new component) on a dashboard/home page and/or the meal-plan edit page, consuming `recommendedRecipes`.
6. **Tests**: unit tests for the overlap/Jaccard math and the recency SQL (integration test against real Postgres fixtures), BFF resolver test for the merge/sort logic, Jest test for the new panel.
7. **Verification**: full test suite; manually create a recipe as one user, confirm it surfaces for another user with overlapping meal-slot history; manually verify recency ordering.

## Phase 24 (future, optional) — Python/Surprise Microservice
**Not started now — deferred per the "phase it" decision.** Documented here so the earlier phases are provably compatible with it.

- New `services/recommender` Python project; reads `analytics.interaction_event` (weighted), `recipe.recipe_rating`, and `mealplan.meal_slot` as implicit/explicit signals; trains an SVD/KNN model via the `surprise` library.
- New `docker-compose.yml` service(s): a scheduled job (cron container or a simple sleep-loop) that runs the training script nightly and writes `analytics.recipe_recommendation` rows with `reason = 'collaborative_filtering'`.
- **Zero changes needed** to the Phase 23 `recommendedRecipes` GraphQL query or frontend — it already reads from `analytics.recipe_recommendation` and merges by score, so Surprise output shows up automatically once it starts writing rows.
- Suggested trigger for actually starting this phase: once there's a meaningful volume of interaction data (e.g., enough users/ratings/menu-adds that collaborative filtering would outperform the Phase 23 heuristics) — revisit sizing with real data before committing to the Go/Python split.

## Cross-cutting
- Each phase gets its own `phase-2N` branch off `main`, its own PR, and must pass `go build ./...`, `go test ./...`, and `npm test`/`npm run build` in `clients/web` (and any lint/typecheck scripts already configured) before merge, per `AGENTS.md`.
- All new tables follow existing conventions: `created_by`/`created_at`/`updated_by`/`updated_at` audit columns, `BIGSERIAL` PKs, `ON DELETE CASCADE` from `identity.users`/`recipe.recipe` where appropriate.
- All new BFF fields/queries require an authenticated user (`userFromContext`), matching every existing resolver.
