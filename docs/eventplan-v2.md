# Food Events — Implementation Plan (v2, current architecture)

This supersedes `eventplan.md`, which predates household sharing, the audit
remediations, and the `lena_app` grants pattern. The feature is re-based as
household-scoped shared data with event notifications and step-timing
metadata designed so a future master-timeline engine can backwards-schedule
every recipe step to its target serve time — without a breaking migration.

## Goal

A "food event" (party/gathering) combines multiple meals (lunch, dinner, or
all meals of a day) into one household event and assigns recipes to be
delivered/served at specific absolute times with 15- or 30-minute
granularity. The timeline/scheduling engine is out of scope here but the
schema supports it.

## Architecture decisions (deltas from the original plan)

- **Household scope, not `user_id`** — `food_event.household_id`, matching
  mealplan/grocery/pantry post-0029. `household_id` always comes from
  `currentuser`, never client input.
- **All household members write** — same convention as meal-plan
  mutations; no role gate.
- **Event notifications** — `event_created`, `event_updated`,
  `event_deleted` kinds added to `household.notifications`; written inside
  the producing transaction; nullable `food_event_id` FK for deep-linking.
- **Web UI only for now** — the mobile events UI lands with the future
  master-timeline phase.
- **Step deps reference `step_number`, not `step_id`** —
  `UpdateRecipeWithChildren` deletes and re-inserts all steps, so `step_id`
  is ephemeral; `(recipe_id, step_number)` is `UNIQUE` and stable.
- **Step metadata**: `duration_minutes`, `step_type`
  (prep|cook|rest|wait|serve|other), `is_passive` (hands-free steps the
  scheduler can overlap), `depends_on_step_number` (default edge = previous
  step), `appliance` (oven|stovetop|mixer|… — seeds future AI
  resource-contention scheduling via the existing Ollama infrastructure).

## Phases

Each phase: own branch, PR to `main`, merge after green CI + approval.

### Phase 1 — `events-p1`: schema + domain modules

- `migrations/0031_event.up.sql` / `.down.sql`
  - `CREATE SCHEMA event`; `event.food_event` (household_id, name,
    event_date, slot_granularity_minutes CHECK IN (15,30), is_active,
    audit cols) + `idx_food_event_household_date`
  - `event.event_recipe` (food_event_id FK cascade, recipe_id nullable FK,
    meal_type, target_time TIMESTAMPTZ, servings, notes, audit cols)
  - `recipe.recipe_step` += `duration_minutes`, `step_type`, `is_passive`,
    `depends_on_step_number`, `appliance`; composite FK
    `(recipe_id, depends_on_step_number) → (recipe_id, step_number)`
    `ON UPDATE CASCADE` (NO ACTION = end-of-statement check, safe against
    the delete-all/reinsert-all update path)
  - `household.notifications`: extend `kind` CHECK with the three event
    kinds; add `food_event_id` FK ON DELETE SET NULL
  - `lena_app` grants block mirroring 0028/0029 (GRANT + ALTER DEFAULT
    PRIVILEGES for schema `event`)
- `internal/event/` — mirror `internal/mealplan/`:
  - `queries.sql`: CRUD for `food_event` filtered by `household_id`;
    `event_recipe` queries enforce ownership via `JOIN event.food_event …
    AND household_id`; `ListEventRecipesByEvents` batch query;
    `ReassignFoodEventsToHousehold` for the invite-accept merge
  - `service.go`: `NewService`/`WithTx`/`InTx` with `dbtx.NewTimedExecer`,
    `FoodEvent`/`EventRecipe` structs, `ReassignHousehold` (satisfies
    `bff.HouseholdMigration`)
  - `sqlc.yaml` stanza + `sqlc/generate.go` mockgen hook
  - `service_test.go` (gomock) + `integration_test.go` (lifecycle,
    reassign, cross-household isolation)
- `internal/recipe` step-timing plumbing (same phase so columns are
  writable): `RecipeStep` struct fields, `toRecipeStep` mapping,
  `AddRecipeStep`/`UpdateRecipeStep` queries take the new columns;
  `CreateRecipeWithChildren`/`UpdateRecipeWithChildren` thread fields
  through `addRecipeStep`. Exported signatures unchanged this phase.

### Phase 2 — `events-p2`: BFF GraphQL + notifications

- `schema.graphqls`: `FoodEvent`/`EventRecipe`/`FoodEventPage` types,
  inputs (`CreateFoodEventInput`, `UpdateFoodEventInput`,
  `AddEventRecipeInput`, `UpdateEventRecipeInput`), queries
  `foodEvent`/`foodEvents`, mutations `createFoodEvent`/`updateFoodEvent`/
  `deleteFoodEvent`/`addEventRecipe`/`updateEventRecipe`/`removeEventRecipe`
- `RecipeStep` type + `RecipeStepInput` gain the five timing fields;
  `Notification` gains `foodEventId` + new kinds
- `resolver_event.go` mirroring `resolver_mealplan.go`: householdID from
  ctx, batch `loadRecipeChildren` for nested `recipe`, `badInputf`/
  `checkedInt16`/`clamp`/`parseID` helpers, `targetTime.Minute() %
  granularity == 0` validation, `InTx` + `r.notify` fan-out to other
  members on mutations
- `services.go`: `EventReader`/`EventWriter`/`EventService` (+
  `HouseholdMigration`), `CreateNotification` widened for `food_event_id`
- `acceptInvite` gains `EventService.ReassignHousehold` alongside
  mealplan/grocery
- `NewResolver`/`Services`/`cmd/lena/main.go` wiring; mock regen
- `resolver_event_test.go` units + `runEventTests` e2e (multi-member
  visibility, notification assertions, cross-household isolation)

### Phase 3 — `events-p3`: web UI

- `lib/types.ts` + `lib/api.ts`: `FoodEvent`/`EventRecipe` types and all
  CRUD calls; recipe step timing fields in create/update payloads
- `app/events/` page: list, create dialog (name/date/granularity),
  detail with recipe rows (meal type, granularity-constrained time
  picker, servings, notes), delete with confirmation
- Recipe editor: per-step duration/type/passive/depends-on/appliance
- `AdminLayout` notification text + `/events` deep-link; nav link
- Jest coverage for page, api, notification labels

### Deferred — master timeline phase (not this work)

- Engine backwards-schedules from `event_recipe.target_time` over the
  step DAG (`depends_on_step_number`, default = previous step), subtracts
  `duration_minutes`, overlaps `is_passive` steps, contends `appliance`
  resources; Ollama-assisted adjustment advice when appliances are
  oversubscribed; NULL durations fall back to recipe-level prep/cook
  split or flag the recipe unschedulable
- Mobile events UI + timeline land together in that phase
- Persistence (compute-on-read vs cache table) decided then; current
  schema supports either

## Fix-preservation audit

- `household_id` from `currentuser` only; every event query filters it
- Event `ReassignHousehold` inside the `acceptInvite` transaction
- Grants block mirrors 0028/0029 (the e2e 503 fix)
- Notifications inside the producing transaction; `food_event_id` is
  `ON DELETE SET NULL` like `invite_id`
- No cross-domain SQL joins — `recipe` resolves via `RecipeService`
  batch loads
- `RecipeStepInput` extension is additive — old clients omit fields
- `kind` CHECK extended atomically in one migration
- `pageArgs`/`PageInfo`/`clamp`/`checkedInt16`/`badInputf`/`parseID`
  reused

## Verification per phase

`go build ./...`, `go test -race -count=1 ./cmd/... ./internal/...`
(integration in CI), `sqlc generate`, mock regen, `gofmt`, `go vet`,
`golangci-lint`; p3 adds `tsc`, jest, eslint, `next build`,
`docker compose config`.
