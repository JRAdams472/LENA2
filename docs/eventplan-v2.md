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

### Phase 4 — `events-p4`: timeline engine + step snapshots + web timeline

- `internal/event/timeline.go` — pure compute-on-read engine: backwards-
  schedules each recipe's step DAG from `target_time` (default edge =
  previous step; `depends_on_step_number` overrides), rounds durations up
  to the event's slot granularity, estimates NULL durations as one slot
  (flagged `estimated`), marks dependency cycles/free-form slots
  `unschedulable`, and flags overlapping `appliance` usage as `conflicts`
- Migration `0032` + `event.event_recipe_step` — per-slot step snapshot:
  linking a recipe copies its steps; all event-context step edits write
  only to the snapshot so the original recipe is never altered. The
  timeline schedules the snapshot, so a later recipe edit/delete can't
  move a laid-out plan. `syncEventRecipeSteps` re-copies; unlinking keeps
  the snapshot as free-form steps. Added late in the phase after review
  flagged that event-context edits would otherwise hit shared recipe rows
- GraphQL `eventTimeline(foodEventId)` → `EventTimeline` /
  `EventTimelineRecipe` / `TimelineStep` (computed on read — the cache-
  table persistence option was dropped in favor of compute-on-read);
  `EventRecipe.steps` exposes the snapshot; `addEventRecipeStep` /
  `updateEventRecipeStep` / `removeEventRecipeStep` /
  `syncEventRecipeSteps` mutations
- Web: `getEventTimeline` API + a lazy "Generate Timeline" section on
  `/events/[id]` showing per-recipe step tables, start-by times, and
  conflict warnings; expandable per-slot step editor (add/edit/delete,
  sync-from-recipe) writing only to the snapshot
- Migration `0033` + `event.event_recipe_item` — per-slot ingredient
  snapshot (same isolation guarantee as steps). `event_recipe.
  base_servings` freezes the recipe's servings at link time so
  `quantity × servings ÷ base_servings` scales correctly even if the
  recipe's serving count changes later — the same rule `expandPlanLines`
  uses for meal-plan grocery scaling. Added after the user confirmed
  servings scaling is in scope. Item mutations mirror the step ones;
  `syncEventRecipeSteps` was widened into `syncEventRecipe` (steps +
  items + base servings), and the web slot expander shows scaled
  quantities with add/edit/delete and a servings scale hint

### Remaining deferred work

- Ollama-assisted adjustment advice when appliances are oversubscribed
  (the `internal/platform/ollamaclient` infrastructure already exists)
- Mobile events UI + timeline view

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

### Final phase — `events-closeout` (after the last feature phase merges)

Run once the last implementation PR has merged, before any new plan:

- **Branch cleanup** — delete all merged `events-*` branches locally and
  on GitHub; confirm each PR state with `gh` first.
- **Coverage check** — no new feature may sit at 0% coverage; every new
  service, resolver, or page needs at least one unit or integration test.
- **Audit regression check** — re-walk the corrected findings in
  `audit/summary.md` to confirm no remediation regressed.
- **Follow-up notes** — record improvements or feature ideas inspired by
  the shipped work (e.g. append to `docs/newfeatures.md`).
- **README update** — refresh `README.md` so features and architecture
  match what shipped.

This close-out phase is a standing convention — every future plan ends
with the same five steps.
