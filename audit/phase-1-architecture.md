# Phase 1 — Architecture & Design Review

Repository: `JRAdams472/LENA2` @ `main` (`c4ad3c7`)
Date: 2026-09-11
Scope: stated architecture (`docs/`) vs. actual code; modular-monolith boundaries under `internal/`; BFF orchestration (`internal/bff/resolver.go`, `services.go`, `schema.graphqls`); shared `internal/platform/` packages.

No application code was modified. Line numbers refer to the commit above.

---

## Severity-ranked summary

| # | Severity | Category | Location | Title |
|---|----------|----------|----------|-------|
| A1-01 | **High** | poor design | `internal/recipe/queries.sql:129-150`, `internal/analytics/queries.sql:66-100` | Cross-domain SQL joins violate the documented "no cross-domain SQL" rule |
| A1-02 | **High** | poor design / leaky abstraction | `internal/bff/errors.go:57`, `resolver_inventory.go:1003-1019`, `resolver_recipe.go:92,594,631`, `nutrition_ocr.go:44` | No domain-typed errors; the BFF matches on `pgx.ErrNoRows` / `pgconn.PgError` codes |
| A1-03 | **High** | antipattern / layering violation | `internal/bff/resolver_grocery.go:128-132`, `resolver_userprefs.go:211-214` | BFF type-asserts service interfaces to concrete `*grocery.Service`/`*userprefs.Service` and opens pgx transactions itself, with a divergent non-transactional fallback path |
| A1-04 | **High** | bug / poor design | `internal/grocery/service.go:227-231`, `internal/bff/resolver_grocery.go:86-110` | `generateGroceryList` never aggregates recipe items — creates an empty list; docs describe it as generating a shopping list |
| A1-05 | **Medium** | poor design | `internal/userprefs/queries.sql`, `internal/inventory/queries.sql:165-167`, `migrations/0006_create_userprefs.up.sql` | `userprefs` has no schema of its own; its tables live in `inventory`, `wine`, `recipe` schemas and `inventory` mutates a `userprefs`-owned table |
| A1-06 | **Medium** | bug | `internal/inventory/service.go:505-513` | `DeleteItem` runs two dependent statements outside a transaction despite `dbtx.InTx` being available |
| A1-07 | **Medium** | poor design / coupling | `internal/recipeimport/service.go:13-21`, `catalog.go:11-14` | `recipeimport` depends on two sibling domains (`recipe`, `inventory`) plus platform HTTP clients and `platform/config` — it is an orchestrator living in the domain tier |
| A1-08 | **Medium** | poor design | `internal/bff/services.go:36-98` (63 methods), `:214-260` (45 methods) | Service interfaces mirror the entire concrete struct instead of role-scoped interfaces the docs prescribe; mocks are 2 877 generated lines |
| A1-09 | **Medium** | layering violation / cognitive complexity | `internal/bff/nutrition_ocr.go:20-77`, `recipe_scan.go:19-84` | Business logic (OCR parsing → nutrient-type creation, filesystem inbox writes) implemented inside the BFF instead of a domain/service |
| A1-10 | **Medium** | poor design | `internal/bff/resolver.go:38-71` | `Resolver` mixes GraphQL root, DI container, background job scheduler and config bag (`ImportInbox`, `*MaxBytes`, `Pool`) |
| A1-11 | **Medium** | poor design | `internal/bff/schema.graphqls:153-154,173,196,221,266,346,498,792` | Stringly-typed API: no `enum`s for `role`, `status`, `source`, `reason`, `entityType`; admin-only fields are only marked in comments |
| A1-12 | **Medium** | poor design | `internal/platform/testenv/testenv.go:129` | Platform package imports a domain package (`identity`) — inverted dependency direction |
| A1-13 | **Medium** | code smell | `internal/recipeimport/service.go:137-163` | `ListPending` emulates an `IN (...)` filter with up to 8 sequential queries and mis-paginates (per-status offset) |
| A1-14 | **Low** | code smell | `internal/platform/validator/validator.go` | Dead package — `validator.V` is never referenced outside its own tests |
| A1-15 | **Low** | code smell | `internal/ocrimport/workqueue.go`, `review.go:28-52,98-180` | Filesystem work-queue / review-file code is unreferenced by any binary (CLI removed, library left behind) |
| A1-16 | **Low** | poor design | `internal/platform/config/config.go:13-106` | Single flat `Config` for server, OCR pipeline, Ollama, import thresholds; `ValidateServer` is a partial workaround |
| A1-17 | **Low** | poor design | `internal/bff/resolver.go:379-813` | Hand-rolled per-list "children" preloading (`itemChildren`, `recipeChildren`, `bottleChildren`) instead of a request-scoped loader; nil-map fallback creates two execution paths per field |
| A1-18 | **Low** | poor design | `docs/` | Documentation drift: several referenced spec docs do not exist; `architecture-hardening.md` describes state that has since changed; no ADR directory |

---

## 0. Documentation vs. reality

### 0.1 Documents requested vs. found

| Requested | Status |
|-----------|--------|
| `docs/go-rewrite-spec.md` | **Not present** on `main` (never existed under this name in history; closest: `docs/architecture-hardening.md`, `docs/postgres-data-model.md`) |
| `docs/graphql-bff-orchestration.md` | Present — used as the architectural baseline below |
| `docs/lena-go-postgres-rewrite-plan.md` | **Not present** (git history shows planning docs were imported in `89eae1d` and later renamed/removed) |
| ADRs | **None** — no `docs/adr/` directory or ADR-style files |

Docs actually reviewed: `graphql-bff-orchestration.md`, `architecture-hardening.md`, `issue-remediation.md`, `postgres-data-model.md`, `graphql-schema.md`, `testing.md`, `recipe-ocr-import.md`.

### 0.2 Stated architecture (from `docs/graphql-bff-orchestration.md`)

1. `internal/bff` is the **only** package allowed to touch more than one domain per request.
2. Each module's sqlc queries may only read tables in **its own schema**; no cross-domain joins.
3. Domain services expose **small, role-scoped interfaces** (`CatalogReader`, `CatalogWriter`).
4. Domain services return **typed errors** (`ErrNotFound`, `ErrValidation`, `ErrConflict`) that the BFF maps to GraphQL extension codes.
5. N+1 is mitigated via request-scoped cache / DataLoader.
6. Per-user mutations derive `user_id` from `currentuser`, never from client input.

### 0.3 Scorecard

| Principle | Verdict | Evidence |
|-----------|---------|----------|
| BFF is the only cross-domain orchestrator | **Partially violated** | `recipeimport` orchestrates `recipe` + `inventory` (A1-07); `recipe` and `analytics` read `mealplan` tables in SQL (A1-01) |
| No cross-domain SQL | **Violated** | A1-01, A1-05 |
| Small role interfaces | **Not implemented** | A1-08 |
| Typed domain errors | **Not implemented** | A1-02 — zero `ErrNotFound`/`ErrConflict` definitions anywhere in `internal/` |
| N+1 mitigation | **Implemented (bespoke)** | `loadItemChildren`, `loadRecipeChildren`, `loadBottleChildren` (A1-17) |
| user_id from context only | **Held** | Verified: all per-user resolvers call `userFromContext`/`requireAdmin`; no `userId` inputs in schema |
| Transactions via `dbtx` | **Held in domains**, leaked into BFF | A1-03 |

Items 1–4 of `architecture-hardening.md` (pool interface, batching, OTel, query-cost controls) have all been addressed since that doc was written; the doc is now stale (A1-18).

---

## 1. Modular-monolith boundary analysis

### 1.1 Compile-time dependency graph (non-test, non-generated)

```
cmd/lena ──► bff, all domains, platform/*
bff ───────► analytics grocery identity inventory mealplan ocrimport recipe recipeimport userprefs wine
             platform/{currentuser,dbtx}  + github.com/jackc/pgx (!)
recipeimport ► ocrimport, recipe (!), inventory (!), platform/{config,currentuser,dbtx,ocrclient,ollamaclient,profanity}
identity, inventory, wine, recipe, mealplan, grocery, userprefs, analytics ► own sqlc + platform/dbtx only  ✔
ocrimport ──► (stdlib only)  ✔
platform/testenv ► identity (!)
```

Eight of ten domain packages are cleanly isolated at compile time. The two exceptions are `recipeimport` (A1-07) and the test helper `platform/testenv` (A1-12).

### 1.2 Table-ownership matrix (from `internal/*/queries.sql`)

| Package | Tables referenced | Foreign-schema tables |
|---------|-------------------|-----------------------|
| identity | `identity.users` | — |
| inventory | 10× `inventory.*` incl. `inventory.user_item` | `inventory.user_item` is owned by **userprefs** (A1-05) |
| wine | 9× `wine.*` | — |
| recipe | `recipe.recipe*`, `recipe.recipe_rating` | **`mealplan.meal_slot`, `mealplan.meal_plan`** (A1-01) |
| userprefs | — | **`inventory.user_item`, `wine.user_bottle`, `recipe.user_recipe_preference`** (A1-05) |
| mealplan | 3× `mealplan.*` | — |
| grocery | 2× `grocery.*` | — |
| analytics | 4× `analytics.*` | **`mealplan.meal_slot`, `mealplan.meal_plan`, `recipe.recipe_item`** (A1-01) |
| recipeimport | `recipe.recipe_import` | table lives in the `recipe` schema, not its own |

---

## 2. Findings

### A1-01 — Cross-domain SQL joins violate the "no cross-domain SQL" rule
- **Category:** poor design (boundary violation) · **Severity:** High
- **Files:** `internal/recipe/queries.sql:129-150` (`ListRatingRecencySuggestions`); `internal/analytics/queries.sql:66-100` (`IngredientOverlapScores`); consumers `internal/recipe/service.go:483`, `internal/analytics/service.go:184-189`
- **Problem:** `recipe` LEFT JOINs `mealplan.meal_slot`/`mealplan.meal_plan`; `analytics` joins `mealplan.*` and `recipe.recipe_item`. `docs/graphql-bff-orchestration.md §2` states explicitly: "Each module's sqlc queries may only read tables in its own schema."
- **Why it matters:** The schema of `mealplan` can no longer change without breaking `recipe` and `analytics`, which is exactly the coupling the rule exists to prevent. It also silently makes `recipe`/`analytics` dependent on `mealplan`'s per-user authorization semantics (`mp.user_id`) without the compile-time visibility an import would give. The invariant is currently enforced by convention only.
- **Remediation:**
  1. Move the recency scoring into the BFF: `mealplan` exposes `LastPlannedDates(ctx, userID, recipeIDs) map[int64]time.Time`; `recipe` exposes `ListRatedAtLeast(ctx, userID, minRating)`; the BFF combines and scores (the scoring is ~10 lines of arithmetic).
  2. For `IngredientOverlapScores`, either (a) accept that `analytics` is a read-model that may denormalise from other domains and **document that exception** in an ADR, or (b) publish `RecipeCreated`/`MealSlotAdded` events into an `analytics`-owned projection table so the Jaccard query stays inside `analytics.*`.
  3. Add a guard test: parse each `internal/<d>/queries.sql`, extract `schema.table` tokens, fail if any schema other than the package's own appears (allow-list for documented exceptions). This turns the doc rule into CI.

### A1-02 — No domain-typed errors; BFF leaks pgx/pgconn
- **Category:** poor design / leaky abstraction · **Severity:** High
- **Files:** `internal/bff/errors.go:9,57`; `internal/bff/resolver_inventory.go:16-17,807,1003-1019`; `internal/bff/resolver_recipe.go:15,92,594,631`; `internal/bff/nutrition_ocr.go:12,44`; every `internal/<domain>/service.go` (no `var Err…` declared anywhere)
- **Problem:** `docs/graphql-bff-orchestration.md §6` promises `ErrNotFound`/`ErrValidation`/`ErrConflict`. None exist. Instead the BFF imports `github.com/jackc/pgx/v5` and `pgconn`, matches `errors.Is(err, pgx.ErrNoRows)` and `pgErr.Code == "23505"`. `nutrition_ocr.go:44` goes further and does `strings.Contains(err.Error(), pgx.ErrNoRows.Error())`.
- **Why it matters:** The persistence library is now part of the BFF's contract. Swapping drivers, adding a cache in front of a service, or returning a not-found from in-memory logic all break error classification silently (the default branch masks everything as `INTERNAL`). `itemWriteError` guesses which unique constraint fired from a generic 23505, which is brittle as constraints are added. String matching on error text is a defect waiting to happen (wrapped errors with altered text).
- **Remediation:** Introduce `internal/platform/domainerr` (or per-package sentinels) with `ErrNotFound`, `ErrConflict`, `ErrValidation`; have each service translate at the sqlc boundary (`if errors.Is(err, pgx.ErrNoRows) { return ErrNotFound }`, `if pgErr.Code == "23505" { return fmt.Errorf("%w: %s", ErrConflict, pgErr.ConstraintName) }`). Remove all `jackc` imports from `internal/bff`. Enforce with a `depguard` rule in `.golangci.yml`.

### A1-03 — BFF type-asserts to concrete services and manages pgx transactions
- **Category:** antipattern / layering violation · **Severity:** High
- **Files:** `internal/bff/resolver_grocery.go:112-150` (`ToggleGroceryItemChecked`, assertion at 128-132), `internal/bff/resolver_userprefs.go:211-214` (`IncrementUserItem`); `internal/bff/resolver.go:39` (`Pool dbtx.Pool` on the resolver)
- **Problem:**
  ```go
  g, okG := r.GroceryService.(*grocery.Service)
  up, okU := r.UserPrefsService.(*userprefs.Service)
  if r.Pool != nil && okG && okU && it.ItemID != nil {
      dbtx.InTx(ctx, r.Pool, func(tx pgx.Tx) error { gTx := g.WithTx(tx); upTx := up.WithTx(tx); ... })
  }
  // "Fallback for tests without a real transactional pool."
  ```
  The interfaces in `services.go` are defeated by downcasting; the BFF holds a DB pool and a `pgx.Tx`; and each mutation has two code paths (transactional vs. fallback) selected by whether the injected value is a mock.
- **Why it matters:** (1) Unit tests exercise a code path that production never runs, so the transactional path is only covered by integration tests. (2) Production behaviour changes if someone wraps a service (decorator for metrics/caching) — the assertion fails and the mutation silently degrades to non-atomic. (3) The BFF is now coupled to pgx types, contradicting "The BFF only knows these interfaces; it does not know SQL."
- **Remediation:** Give the multi-domain write a first-class home. Options: (a) a `UnitOfWork` interface in `platform/dbtx` — `InTx(ctx, func(ctx context.Context) error)` that stores the tx in `ctx`, with each service's querier resolving `dbtx.FromContext(ctx)` — so the BFF never sees `pgx.Tx` and mocks can implement `UnitOfWork` trivially; or (b) a dedicated composite service (`internal/pantry` or similar) that owns "check grocery item ⇒ adjust pantry" as one atomic operation. Delete the fallback branch.

### A1-04 — `generateGroceryList` creates an empty list
- **Category:** bug / poor design · **Severity:** High (functional gap in a headline feature)
- **Files:** `internal/grocery/service.go:227-231`; `internal/bff/resolver_grocery.go:86-110`; `docs/graphql-schema.md:240,301`; `internal/bff/schema.graphqls:152`
- **Problem:** `Generate` is documented as "generates a grocery list from a meal plan". Implementation:
  ```go
  // Phase 4 provides the list container only; full aggregation against
  // recipe items and user stock is left for the BFF/resolver layer.
  return s.CreateGroceryList(ctx, userID, &mealPlanID, by)
  ```
  The BFF resolver does not perform the promised aggregation either. The result is an empty `GroceryList` linked to the plan.
- **Why it matters:** The client receives a successful mutation that did not do what the schema, docs, and README promise. The TODO is hidden in a comment inside a domain service and has survived several phases (the `docs/eventplan.md`/`recipe-ocr-*` features shipped after it).
- **Remediation:** Implement the aggregation in the BFF (it is legitimately cross-domain: `mealplan` slots → `recipe` items → `userprefs` stock → `grocery` items) inside one `InTx` unit of work, or mark the mutation `@deprecated`/remove it until implemented. Track as an explicit issue rather than an inline comment.

### A1-05 — `userprefs` has no schema; ownership of per-user tables is split
- **Category:** poor design · **Severity:** Medium
- **Files:** `migrations/0006_create_userprefs.up.sql:1,20,38`; `internal/userprefs/queries.sql` (all); `internal/inventory/queries.sql:165-167`; `internal/inventory/service.go:505-513`
- **Problem:** `postgres-data-model.md §1` lists one Postgres schema per domain, but `userprefs` tables were placed in `inventory.user_item`, `wine.user_bottle`, `recipe.user_recipe_preference`. Consequently `inventory` also writes to `inventory.user_item` (`DeleteUserItemsByItem`), so two Go packages mutate one table.
- **Why it matters:** Table ownership is ambiguous; the "one schema per module" rule cannot be checked mechanically (A1-01 guard test would need special cases); a future `userprefs` change (e.g. soft-delete) can be bypassed by `inventory.DeleteItem`.
- **Remediation:** Either create a `userprefs` schema and migrate the three tables (FKs across schemas are fine in Postgres), or explicitly re-assign ownership: `user_item` → `inventory`, `user_bottle` → `wine`, `user_recipe_preference` → `recipe`, and dissolve `userprefs` into per-domain "per-user" query files. Whichever is chosen, `inventory.DeleteItem` should go through the owning service or rely on `ON DELETE CASCADE`.

### A1-06 — `inventory.DeleteItem` is not atomic
- **Category:** bug · **Severity:** Medium
- **File:** `internal/inventory/service.go:505-513`
- **Problem:** Two statements (`DeleteUserItemsByItem`, `DeleteItem`) executed on `s.q` (pool), not inside `s.InTx`. If the second fails (FK from `recipe.recipe_item`, `mealplan.meal_slot_item`, `grocery.grocery_list_item`), every user's pantry row for that item is already gone.
- **Why it matters:** Data loss on a failed admin action; the service *has* `InTx` (line 41) and does not use it here.
- **Remediation:** Wrap in `s.InTx`, or replace the manual pre-delete with `ON DELETE CASCADE` on `inventory.user_item.item_id` and drop `DeleteUserItemsByItem`.

### A1-07 — `recipeimport` is an orchestrator in the domain tier
- **Category:** poor design / coupling · **Severity:** Medium
- **Files:** `internal/recipeimport/service.go:13-21,47-66`; `internal/recipeimport/catalog.go:11-14`
- **Problem:** The package imports `recipe` (writes recipes), `inventory` (reads catalog), `platform/ocrclient`, `platform/ollamaclient`, `platform/profanity` and `platform/config`. It holds `*ocrclient.Client`/`*ollamaclient.Client` as concrete pointers (not interfaces), and `ConfigFromPlatform(*config.Config)` couples it to the global config struct.
- **Why it matters:** It is the one domain package that can break when `recipe` or `inventory` change, and the one package the BFF rule ("only bff orchestrates") does not cover. Concrete client pointers prevent substituting a fake OCR/LLM in unit tests without HTTP servers.
- **Remediation:** Either (a) acknowledge it as an *application service* and move it to `internal/app/recipeimport` (or under `bff/`), so the layering diagram is honest; or (b) keep it a domain but depend on narrow interfaces (`OCRExtractor`, `Structurer`) defined locally, and accept `Config` values rather than `*config.Config`. Document the choice in an ADR.

### A1-08 — Service interfaces are full-surface mirrors, not role interfaces
- **Category:** poor design · **Severity:** Medium
- **Files:** `internal/bff/services.go:36-98` (`InventoryService`, 63 methods), `:214-260` (`WineService`, 45), `:124-148` (`RecipeService`, 24); `internal/bff/mock/services.go` (2 877 lines generated)
- **Problem:** The docs prescribe `CatalogReader`/`CatalogWriter`-style role interfaces. The code instead declares one interface per domain that lists every exported method of the concrete struct, asserted by `var _ InventoryService = (*inventory.Service)(nil)`. Interfaces are defined by the consumer (good) but are not narrowed.
- **Why it matters:** Interface Segregation is lost: every resolver test mock must satisfy 60+ methods; adding a service method forces regenerating a 3k-line mock; per-type resolvers (`itemResolver`, `groceryListItemResolver`) receive the full `InventoryService` when they need one or two reads, which obscures what a resolver can actually do.
- **Remediation:** Split into read/write and per-aggregate interfaces (`ItemReader`, `CatalogAdmin`, `BottleReader`, …); embed them in the root `Resolver` struct; pass only the narrow interface into child resolvers. Mocks shrink accordingly.

### A1-09 — Business logic implemented in the BFF
- **Category:** layering violation / cognitive complexity · **Severity:** Medium
- **Files:** `internal/bff/nutrition_ocr.go:20-77` (`processNutritionPhoto`), `internal/bff/recipe_scan.go:19-84` (`SubmitRecipeScan`)
- **Problem:** `processNutritionPhoto` decides how OCR labels map to nutrient types, *creates catalog `nutrient_type` rows* as a side effect, and writes them with a hard-coded actor `"ocr-system"`. `SubmitRecipeScan` performs `os.MkdirAll`/`os.WriteFile` into the import inbox and computes the source hash — filesystem I/O that duplicates what `recipeimport` already knows about the inbox.
- **Why it matters:** Domain rules (nutrient-type auto-creation, audit actor) are untestable without the GraphQL layer and invisible to the `inventory` package that owns nutrient types. The inbox path convention is now split between `recipeimport` and `bff`.
- **Remediation:** Move `processNutritionPhoto` into `inventory` (e.g. `inventory.Service.ApplyNutritionLabel(ctx, itemID, parsed []nutritionparse.Nutrient, by)`) and the file-write into `recipeimport.Service.Submit(ctx, mediaType, bytes, by)`; the BFF keeps only decode/size-check/auth.

### A1-10 — `Resolver` is a god-object
- **Category:** poor design · **Severity:** Medium
- **File:** `internal/bff/resolver.go:38-71` (struct), `:69` (`NewResolver`), `:73-136` (async runner)
- **Problem:** The root resolver struct is simultaneously: GraphQL root (`Me`, `Items`, …), DI container (10 services), a bounded background worker pool (`runAsync`, `Shutdown`, `bgSem`), raw DB access (`Pool`), an HTTP client (`OCRClient`), and configuration (`NutritionPhotoMaxBytes`, `RecipeScanMaxBytes`, `ImportInbox`). `NewResolver` takes 14 positional parameters.
- **Why it matters:** Construction is error-prone (positional `int, int, string`), and the resolver cannot be reasoned about as "schema → services" glue. Background job semantics (drop-on-saturation, 16-worker cap) are an infrastructure concern buried in the API layer.
- **Remediation:** Extract `bff.Services{…}` (struct of interfaces) and `bff.Options{…}` (limits, inbox); extract the async runner into `platform/async` (or reuse for `recipeimport`'s own worker pool, which duplicates the pattern at `recipeimport/service.go:90-91`). `NewResolver(services, options, runner)`.

### A1-11 — Stringly-typed GraphQL contract
- **Category:** poor design · **Severity:** Medium
- **File:** `internal/bff/schema.graphqls:153-154` (`entityType: String!`), `:173` (`role: String!`), `:196,221,266,792` (`role`/`status: String!`), `:346` (`reason`), `:498` (`source`)
- **Problem:** The schema defines **zero** `enum` types. Roles (`member`/`admin`), item/brand/import statuses, analytics entity types, recommendation reasons, grocery item sources are all `String`. Admin-only operations are documented only via `#` comments.
- **Why it matters:** Clients cannot rely on the type system; typos (`"Admin"`) reach resolvers; the Next.js client must duplicate the value sets. Generated TypeScript types become `string` instead of unions.
- **Remediation:** Introduce `enum Role`, `enum ApprovalStatus`, `enum ImportStatus`, `enum EntityType`, `enum GroceryItemSource`, `enum RecommendationReason`. Consider a schema `directive @admin` (even if only used for documentation/codegen) so authorization intent is machine-readable.

### A1-12 — Platform package depends on a domain
- **Category:** layering violation · **Severity:** Medium (test-only code, but breaks the dependency rule)
- **File:** `internal/platform/testenv/testenv.go:129` (`identity.NewService(pool)`); `internal/platform/testenv/auth.go`
- **Problem:** `platform/*` is the bottom layer, yet `testenv` imports `internal/identity` to seed users. A domain package's integration test that imports `testenv` therefore transitively depends on `identity`; `identity`'s own tests cannot use `testenv` without an import cycle risk.
- **Remediation:** Move `testenv` to `internal/testutil` (outside `platform`), or have it seed users with raw SQL / accept a `UserSeeder` func from the caller.

### A1-13 — `ListPending` emulates `IN` with sequential queries
- **Category:** code smell · **Severity:** Medium
- **File:** `internal/recipeimport/service.go:137-163`
- **Problem:** Eight sequential `store.List(ctx, status, pageSize, offset)` calls, concatenated and truncated. Applying the same `offset` to each status means page 2 skips different rows per status — pagination is incorrect, and total count (`Count`) does not match.
- **Remediation:** Add a sqlc query `ListByStatuses(statuses text[], limit, offset)` with `WHERE status = ANY($1)` and a matching count.

### A1-14 — Dead `platform/validator` package
- **Category:** code smell · **Severity:** Low
- **File:** `internal/platform/validator/validator.go` (9 lines) — `validator.V` has no non-test references (`grep -rn "validator\." internal cmd`).
- **Remediation:** Delete, or actually adopt for input structs in the BFF (which currently hand-roll checks like `checkedInt16`).

### A1-15 — Unreferenced filesystem work-queue in `ocrimport`
- **Category:** code smell · **Severity:** Low
- **Files:** `internal/ocrimport/workqueue.go` (141 lines, `Queue`, `Open`, `Add`, `SetStatus`), `internal/ocrimport/review.go:28-52` (`LoadReview`/`SaveReview`), `:98-180` (`RenderReviewMarkdown`)
- **Problem:** No binary under `cmd/` and no server code references these; they were the host-CLI pipeline (`docs/recipe-ocr-usage.md`) whose command no longer exists in the tree. `docker-compose.import.yml` still exists, suggesting the CLI is expected.
- **Remediation:** Either restore/document the CLI entrypoint or delete the file-based queue so `ocrimport` is purely the pure-function matching library it otherwise is.

### A1-16 — Monolithic `Config`
- **Category:** poor design · **Severity:** Low
- **File:** `internal/platform/config/config.go:13-106,117-131`; `internal/recipeimport/service.go:32`
- **Problem:** One struct spans HTTP server, auth, telemetry, GraphQL limits, OCR, Ollama, import thresholds, profanity. `ValidateServer()` exists because the struct is shared with a CLI that needs a different required-set. `recipeimport.ConfigFromPlatform` copies four fields out again.
- **Remediation:** Nest sub-structs (`Server`, `Auth`, `Telemetry`, `GraphQL`, `Import`) using envconfig's struct embedding; pass `cfg.Import` to `recipeimport` directly.

### A1-17 — Bespoke preloading with nil-map fallbacks
- **Category:** poor design · **Severity:** Low
- **Files:** `internal/bff/resolver.go:379-813`; pattern repeated in `resolver_grocery.go:375-390`, `resolver_mealplan.go:528-545`
- **Problem:** Every child resolver has the shape `if r.ch != nil { lookup map } else { call service }`. The batching works (the N+1 from `architecture-hardening.md` §2 is fixed) but each field has two execution paths, and the preload functions must anticipate exactly which fields a query will request (over-fetching brands, categories, nutrients, flavors, units for every item list whether or not selected).
- **Remediation:** Adopt a request-scoped DataLoader (`github.com/vikstrous/dataloadgen`, already suggested in the docs) keyed by `(domain, id)`; child resolvers then always call the loader, eliminating the dual paths and loading only selected fields.

### A1-18 — Documentation drift / missing ADRs
- **Category:** poor design (process) · **Severity:** Low
- **Files:** `docs/architecture-hardening.md` (describes pre-fix state: "resolver.go is a ~2,300-line god file", "OTel packages are all indirect", "no rate limiting" — all since addressed); `docs/issue-remediation.md` (items 1–12 mostly completed but no status markers); no `docs/adr/`.
- **Remediation:** Add `docs/adr/` with at least: ADR-001 modular monolith & no-cross-domain-SQL (with the analytics exception if kept), ADR-002 error contract between domains and BFF, ADR-003 transaction/unit-of-work seam. Archive or annotate stale docs.

---

## 3. Platform package assessment

| Package | Responsibility | Verdict | Notes |
|---------|----------------|---------|-------|
| `config` | env → struct | Adequate | A1-16; note envconfig prefix `lena` means both `LENA_X` and `X` are accepted — docs use both forms inconsistently |
| `logger` | slog JSON + PII redaction | Good | Key-substring redaction (`"notes"`, `"address"`) may over-redact legitimate fields; acceptable trade-off |
| `postgres` | pool + otelpgx | Good | Single responsibility |
| `dbtx` | pool interface, `InTx`, timed querier | Good | Well-designed seam; only misuse is A1-03/A1-06. `NewTimedExecer` silently returns unwrapped inner on meter error — fine |
| `telemetry` | OTel providers, pool gauges, HTTP metrics | Good | Takes `*pgxpool.Pool` concretely (needed for `Stat()`); acceptable |
| `validator` | — | Dead | A1-14 |
| `currentuser` | context carrier | Good | Minimal; `IsAdmin` bool rather than roles slice will need revisiting if more roles appear |
| `ocrclient`, `ollamaclient` | HTTP clients | Adequate | Concrete structs, no interfaces — consumers (`recipeimport`) cannot fake them (A1-07). `ollamaclient.New` hard-codes a 5-minute timeout (`client.go:35`; `NewWithTimeout` exists) |
| `profanity` | regex deny-list | Adequate | Stateless; fine as platform |
| `testenv` | integration harness | Misplaced | A1-12 |

---

## 4. What is working well

- Eight of ten domain packages are compile-time isolated and depend only on their own sqlc output and `platform/dbtx`.
- `dbtx.Pool` + `WithTx`/`InTx` give every domain a uniform, testable transaction seam (the `architecture-hardening.md` item 1 is done).
- All per-user access is derived from `currentuser` in context; the schema exposes no `userId` inputs.
- Batch-by-ID service methods exist across all domains and list resolvers use them; the documented N+1 is gone.
- Query depth, query length, body limit, IP and per-user rate limits, HTTP timeouts and CORS credential handling are all wired in `cmd/lena/main.go`.
- Telemetry is real (otelecho, otelpgx, Prometheus `/metrics` behind auth, per-sqlc-query histograms).

---

## 5. Recommended order of remediation (architecture only)

1. A1-02 typed errors + depguard ban on `jackc/*` in `internal/bff` — small, unlocks A1-03.
2. A1-03 unit-of-work seam; delete fallback branches.
3. A1-01/A1-05 boundary: decide schema ownership, add the SQL-schema guard test, refactor the two cross-domain queries or record the exception in an ADR.
4. A1-04 implement or retire `generateGroceryList`.
5. A1-08 role interfaces; A1-10 split `Resolver`.
6. A1-11 enums; A1-18 ADRs.
