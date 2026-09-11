# Phase 2 — BFF & GraphQL API Layer Code Review

Date: 2026-09-11
Branch: `audit` (based on `main` @ `c4ad3c7`)
Scope: `internal/bff/` non-test files (`resolver*.go`, `auth.go`, `errors.go`, `ratelimit.go`,
`graphql_tracer.go`, `nutrition_ocr.go`, `recipe_scan.go`, `services.go`, GraphQL handler wiring in
`resolver.go`) and the server bootstrap/middleware in `cmd/lena/main.go`.

Method: full read of the files above, targeted reads of the domain services / SQL they call when needed to
confirm behaviour (e.g. `internal/recipeimport/service.go`, `internal/recipe/queries.sql`,
`migrations/0003_create_inventory.up.sql`), and a scan for functions longer than 60 lines. No code was
executed or modified. Line numbers refer to the `main` checkout at `c4ad3c7`.

Phase 1 findings (`A1-xx`) are referenced rather than repeated; this report focuses on defects and
code-level quality inside the API layer.

## Severity-ranked summary

| ID | Severity | Category | Title | Location |
|---|---|---|---|---|
| A2-01 | high | bug | Read-modify-write inventory updates without row locks lose updates under concurrency | `internal/bff/resolver_grocery.go:132-199`, `internal/bff/resolver_userprefs.go:209-224` |
| A2-02 | high | bug | Auth middleware collapses every failure (incl. DB/JWKS outages) into an unlogged `401 invalid token` | `internal/bff/auth.go:81-98, 159-165` |
| A2-03 | medium | bug | `Resolver.Shutdown` cancels in-flight background work immediately and never waits for recipe-import drain | `internal/bff/resolver.go:112-136`, `cmd/lena/main.go:117-129` |
| A2-04 | medium | poor design | Authenticator holds one global mutex across network I/O, serialising all authenticated requests behind a slow issuer | `internal/bff/auth.go:195-224` |
| A2-05 | medium | poor design | Every authenticated request performs an identity `UpsertUser` write (plus possible `SetUserRole`) | `internal/bff/auth.go:159-180` |
| A2-06 | medium | bug | `Nutrition` multiplies unit-less `food_nutrient.amount` by recipe/slot quantities in arbitrary units | `internal/bff/resolver_mealplan.go:194-232` |
| A2-07 | medium | antipattern | N+1 query paths remain in single-object and nested resolvers (`groceryList`, `ingredient`, `scaledRecipe`, `recipeImport.recipe`) | `internal/bff/resolver_grocery.go:17-31, 318-327, 364-390`; `resolver_recipe.go:73-130`; `resolver_recipe_import.go:203-214` |
| A2-08 | medium | bug | Unique-violation mapping is inconsistent: `UpdateItem` and admin `CreateBrand` surface `INTERNAL` where sibling mutations return `BAD_USER_INPUT` | `internal/bff/resolver_inventory.go:459-470, 1190-1202` |
| A2-09 | medium | bug | Recipe-import list resolvers skip pagination clamping and report a wrong `totalCount` | `internal/bff/resolver_recipe_import.go:33-76` |
| A2-10 | medium | poor design | No per-request deadline for GraphQL execution; resolvers outlive disconnected clients | `cmd/lena/main.go:179-182, 248-269`, `internal/bff/resolver.go:819-838` |
| A2-11 | medium | error handling | `submitItemNutritionPhoto` returns `true` even when the task is dropped; OCR pipeline creates nutrient types non-atomically | `internal/bff/nutrition_ocr.go:36-62, 115-118`, `internal/bff/resolver.go:83-90` |
| A2-12 | medium | cognitive complexity | 12 resolver functions exceed 60 lines; `Nutrition` (141), `ToggleGroceryItemChecked` (116), `UpdateBottle` (112) | see finding |
| A2-13 | low | bug | `resolveUnitID` maps every service error (incl. DB outage) to `BAD_USER_INPUT "unknown unit"` | `internal/bff/resolver.go:410-416` |
| A2-14 | low | antipattern | Per-item `resolveUnitID` lookups in `parseRecipeChildren` (N queries per mutation) | `internal/bff/resolver_recipe.go:176-213` |
| A2-15 | low | error handling | Recipe-import `Draft()`/`Review()` swallow JSON errors as `null`; `Recipe()` ignores missing user | `internal/bff/resolver_recipe_import.go:181-214` |
| A2-16 | low | antipattern | `recordSelection`/`recordSearch` always return `true`, accept unvalidated `entityType`, and let >500-char terms fail silently | `internal/bff/resolver_analytics.go:16-78` |
| A2-17 | low | code smell | `FrequentBrands`/`FrequentItems` are a 60-line copy-paste; counts fetched twice | `internal/bff/resolver_inventory.go:175-234, 238-318` |
| A2-18 | low | poor design | Pointer-typed update inputs cannot express "clear this nullable field" | `internal/bff/resolver_wine.go:326-397`, `internal/bff/resolver_inventory.go:1146-1152` |
| A2-19 | low | antipattern | Tracer span names come from client-supplied `operationName`; span status carries pre-sanitised error text | `internal/bff/graphql_tracer.go:50-62, 105-110` |
| A2-20 | low | bug | `SubmitRecipeScan` leaves an orphan file in the inbox when the DB insert fails | `internal/bff/recipe_scan.go:61-83` |
| A2-21 | low | poor design | Rate limiter is per-process in-memory and keyed by `RealIP()` from XFF; ineffective across replicas | `internal/bff/ratelimit.go:9-56`, `cmd/lena/main.go:176` |
| A2-22 | low | error handling | Preloaded-map miss in `unitName` becomes an `INTERNAL` error instead of a fallback lookup | `internal/bff/resolver.go:420-434` |

Totals: 2 high, 10 medium, 10 low. No critical.

---

## Findings

### A2-01 — Read-modify-write inventory updates without row locks lose updates under concurrency

- **Category:** bug
- **Severity:** high
- **Location:** `internal/bff/resolver_grocery.go:132-199` (`ToggleGroceryItemChecked`), `internal/bff/resolver_userprefs.go:209-224` (`AdjustUserItem` transactional path, `applyUserItemDelta`)

**Description.** Both mutations open a transaction via `dbtx.InTx`, `SELECT` the current row(s) with plain
reads (`GetGroceryListItemByID`, `GetUserItemByUserAndItem`), compute the new quantity in Go
(`existing.CurrentQty ± current.QuantityNeeded`, `existing.CurrentQty + delta`), then `UpsertUserItem`.
Postgres' default `READ COMMITTED` isolation does not prevent two concurrent transactions from reading
the same snapshot and both writing; nothing uses `SELECT ... FOR UPDATE`, an atomic
`SET current_qty = current_qty + $1`, or a version column. For the toggle, two concurrent calls both
read `IsChecked=false`, both flip to `true`, and both add `QuantityNeeded` to the pantry — a double
increment plus an idempotency violation (the second call should have unchecked). `ToggleGroceryItemChecked`
also reads the grocery item a third time *outside* the transaction (line 121) purely to decide which
code path to take.

**Why it matters.** Pantry quantities are the core data of the app; double-tap on mobile or a retry after
a network blip corrupts them silently. The transaction wrapper gives a false sense of safety here — it
protects against partial writes but not against lost updates.

**Remediation.** Move both operations into the owning services as single SQL statements (e.g.
`UPDATE userprefs.user_item SET current_qty = GREATEST(current_qty + $3, 0) ... RETURNING *`, and
`UPDATE grocery.grocery_list_item SET is_checked = NOT is_checked ... RETURNING *`) or lock the row with
`FOR UPDATE` inside the transaction. Make the toggle explicit (`setGroceryItemChecked(id, checked: Boolean!)`)
so retries are idempotent. This also removes the concrete-type assertions flagged in A1-03.

---

### A2-02 — Auth middleware collapses every failure into an unlogged `401 invalid token`

- **Category:** bug
- **Severity:** high
- **Location:** `internal/bff/auth.go:81-98` (`Middleware`), `159-165` (`UpsertUser`), `176-179` (`SetUserRole`), `195-224` (`keySetForIssuer`)

**Description.** `Middleware` calls `authenticate` and, on *any* error, returns
`echo.NewHTTPError(http.StatusUnauthorized, "invalid token")` without logging `err`. `authenticate`
returns errors for genuinely bad tokens (wrong issuer/audience, bad signature) but also for
infrastructure failures: OIDC discovery HTTP errors, `jwk.Fetch` failures, and — most importantly —
`identity.UpsertUser` / `SetUserRole` database errors (`auth.go:159-165, 176-179`). All of these are
reported to the client as `401 invalid token`.

**Why it matters.** During a database blip or an IdP outage every logged-in user receives 401s, which
SPA clients typically treat as "session expired → sign out". Operators see a spike in 401s with no
server-side log line explaining why, so the outage is indistinguishable from a credential-stuffing
attack. The `/metrics` endpoint shares this middleware (`main.go:245`), so scraping also breaks
silently.

**Remediation.** Introduce typed auth errors (e.g. `errTokenInvalid`, `errIdentityStore`,
`errKeyDiscovery`); return 401 only for token problems and 503 (with `Retry-After`) for dependency
failures. Log every non-token failure at `WARN`/`ERROR` with request ID and issuer. Consider a bounded
negative cache for repeated invalid tokens so the IdP is not re-queried by garbage-token floods.

---

### A2-03 — `Resolver.Shutdown` cancels background work immediately and never waits for recipe-import drain

- **Category:** bug
- **Severity:** medium
- **Location:** `internal/bff/resolver.go:112-136`, `cmd/lena/main.go:117-129`

**Description.** The doc comment says `Shutdown` exists "so analytics writes and recipe import workers
are not silently dropped on process exit", but the first statement is `r.bgCancel()` (line 114). Every
in-flight `runAsync` task derives its context from `bgCtx` (line 100), so the cancel aborts running
analytics inserts, overlap computations and nutrition OCR with `context.Canceled` before the
subsequent `bgWG.Wait()` gets a chance to let them finish. Separately, `RecipeImportService.Shutdown`
is launched in a goroutine (lines 123-127) whose completion is *not* part of the `select` on lines 131-134;
`Shutdown` returns as soon as the resolver's own waitgroup drains, and `main.go` then returns and the
process exits regardless of whether the import workers finished.

**Why it matters.** Graceful shutdown is exactly the moment when in-flight work should be allowed to
complete. As written, a rolling deploy reliably drops the last few seconds of analytics events and can
interrupt an OCR/LLM import mid-write.

**Remediation.** Wait first, cancel on deadline: run `bgWG.Wait()` in the goroutine, `select` on
`done`/`ctx.Done()`, and call `bgCancel()` only in the timeout branch. Include the recipe-import shutdown
in the same waitgroup (or return its error) so `Shutdown` truly reports completion.

---

### A2-04 — Authenticator global mutex held across network I/O

- **Category:** poor design
- **Severity:** medium
- **Location:** `internal/bff/auth.go:195-224` (`keySetForIssuer`), `227-235` (`cachedSetHasKey`)

**Description.** `keySetForIssuer` takes `a.mu` and holds it for the whole function — including cache
hits and, on miss/expiry, the OIDC discovery request *and* the JWKS fetch (two round-trips, each bounded
only by the 10 s HTTP client timeout at line 72). A single map protects all issuers. Because every
request calls `keySetForIssuer` (cache-hit path), a slow or unreachable IdP for issuer A blocks token
verification for issuer B and for every request whose key is already cached. The fetch uses the
*request* context, so if the client that happened to trigger the refresh disconnects, the shared refresh
fails for all waiters. When the 1 h TTL expires (line 60) there is no stale-while-revalidate: an IdP
outage at that moment is a total authentication outage.

**Why it matters.** Availability of the entire API is coupled to IdP tail latency; the comment on lines
196-197 documents the intent (single-flight) but the implementation is a global stop-the-world.

**Remediation.** Use `sync.RWMutex` (or `atomic.Pointer`) for cache reads, a per-issuer
`singleflight.Group` for refreshes, a detached context with its own timeout for the fetch, and serve
the stale set while a refresh is in progress. `jwk.Cache` from lestrrat-go/jwx already implements most
of this.

---

### A2-05 — Every authenticated request performs an identity write

- **Category:** poor design
- **Severity:** medium
- **Location:** `internal/bff/auth.go:159-180`

**Description.** After signature verification `authenticate` calls `identity.UpsertUser` on every request
(and `SetUserRole` on every request for admin emails whose role is not yet admin). The result is not
cached; there is no per-token or per-subject memoisation.

**Why it matters.** An `INSERT ... ON CONFLICT DO UPDATE` per GraphQL call turns the hottest read path
into a write path, contends on the `identity.user` row for active users, bloats WAL, and makes the
database a hard dependency for reading even cached catalog data. It also means the request-level rate
limiter (which runs *after* auth, `main.go:262-264`) cannot protect the identity table.

**Remediation.** Cache the resolved `currentuser.User` keyed by token hash (or `iss+sub`) for the token's
remaining lifetime (bounded by a few minutes), refreshing `last_login_at` at most once per interval.
Alternatively move the upsert to a `/session` bootstrap mutation and treat the JWT alone as sufficient
for reads.

---

### A2-06 — `Nutrition` multiplies unit-less nutrient amounts by quantities in arbitrary units

- **Category:** bug (possible)
- **Severity:** medium
- **Location:** `internal/bff/resolver_mealplan.go:194-232`; schema `migrations/0003_create_inventory.up.sql:61-68`

**Description.** `addNutrients` computes `t.amount += n.Amount * quantity`. `inventory.food_nutrient.amount`
has no basis column (per 100 g? per serving? per one `unit_id` of the item?), and `quantity` comes
either from `meal_slot_item.quantity` (with its own `unit_id`, line 82-84) or `recipe_item.quantity *
scale` (also with its own `unit_id`). Nothing in the resolver or the SQL converts units, so "2 cups
flour" and "200 g flour" contribute identically to the total. The OCR path (`nutrition_ocr.go`) stores
values straight from a nutrition label, i.e. per labelled serving, which is different again.

**Why it matters.** The `nutrition` query returns numbers with correct names and units that are
dimensionally meaningless; users will trust them. This is domain logic living in the BFF (see A1-09),
so the defect is also in the wrong layer to be tested.

**Remediation.** Define the basis explicitly (add `per_quantity NUMERIC` + `per_unit_id` to
`food_nutrient`, or standardise on per-100 g and require item net weight), do unit conversion in the
inventory/mealplan domain, and move the aggregation to a `mealplan` service method so it can be
unit-tested with fixtures.

---

### A2-07 — Remaining N+1 paths in single-object and nested resolvers

- **Category:** antipattern (N+1)
- **Severity:** medium
- **Location:**
  - `internal/bff/resolver_grocery.go:17-31` (`GroceryList`), `318-327` (`Items` lazy branch), `375-390` (`Item` lazy branch), `364-373` (`Ingredient` — always lazy)
  - `internal/bff/resolver_recipe.go:73-130` (`ScaledRecipe` builds `recipeChildren` without `itemChildren`)
  - `internal/bff/resolver_recipe_import.go:203-214` (`recipeImportResolver.Recipe`), `internal/bff/resolver_recipe_import.go:102-117` (`ApproveRecipeImport` returns a `recipeResolver` with `rc == nil`)

**Description.** The list resolvers batch well (Phase 1, A1-17), but the dual "preloaded map or lazy
call" pattern means every path that constructs a child resolver without the preload struct silently
degrades to one query per row:

- `groceryList(id)` constructs `groceryListResolver` with no `ch`, so `items { item { brand category unit } unitOfMeasure ingredient }` issues `ListGroceryListItems` + per-item `GetItemByID` + per-item `GetUnitByID` + per-item `GetIngredientByID` + per-item brand/category/unit lookups. `Ingredient` has *no* preloaded branch at all, so even `groceryLists` pays N queries.
- `scaledRecipe` populates `rc.items`/`rc.units` but not `rc.itemChildren`; `items { item { brand category unit } }` then goes through `itemResolver` with `ch == nil` → 3 queries per ingredient.
- `recipeImports { items { recipe { ... } } }` does `GetRecipeByID` per import and returns a `recipeResolver` with `rc == nil`, so every nested field fans out again.

**Why it matters.** A 30-item grocery list detail view is ~150 round-trips; the pattern is invisible in
tests that use mocks.

**Remediation.** Make preloading the *only* path: build `itemChildren`/`recipeChildren` in a shared
helper used by single-object, list and mutation-return resolvers, and delete the lazy branches (turn
`if r.ch != nil` into a hard invariant). Longer term, adopt a request-scoped DataLoader (A1-17) so the
question no longer depends on which root resolver was called.

---

### A2-08 — Inconsistent unique-violation mapping across inventory mutations

- **Category:** bug
- **Severity:** medium
- **Location:** `internal/bff/resolver_inventory.go:459-470` (admin `CreateBrand`), `1190-1202` (`UpdateItem`); compare `775, 877, 952, 117` (callers of `itemWriteError`/`brandWriteError`)

**Description.** `CreateItem`, `SubmitItem`, `SubmitBrand` and `UpdateBrand` wrap the service error in
`itemWriteError`/`brandWriteError`, which translate `pgconn` unique violations into
`BAD_USER_INPUT` ("an item with that UPC already exists" etc.). `UpdateItem` and the admin `CreateBrand`
return the raw error, so the same duplicate-UPC / duplicate-name condition surfaces as
`INTERNAL "internal server error"` and is logged as a server fault by `sanitizeQueryErrors`
(`errors.go`).

**Why it matters.** The admin UI cannot tell the user what went wrong on edit, and every duplicate edit
pollutes error logs/alerts. It is also a symptom of the A1-02 problem: the mapping lives in the BFF, so
each call site has to remember to apply it.

**Remediation.** Have the inventory service return `inventory.ErrConflict`/`ErrValidation` and map once
in `errors.go`; until then apply `itemWriteError`/`brandWriteError` in the two missing call sites.

---

### A2-09 — Recipe-import list resolvers skip pagination clamping and report a wrong `totalCount`

- **Category:** bug
- **Severity:** medium
- **Location:** `internal/bff/resolver_recipe_import.go:33-57` (`RecipeImports`), `60-76` (`PendingRecipeImports`); `internal/recipeimport/service.go:121-129, 137-149`

**Description.** Every other paged resolver uses `clamp(args.PageSize, 1, 100)` (e.g. `resolver_mealplan.go:43-44`).
`RecipeImports`/`PendingRecipeImports` pass `args.Page`/`args.PageSize` through unchanged; the service
only lower-bounds them, so `pageSize: 2147483647` is legal and yields an unbounded `LIMIT`. The returned
`pageInfo` echoes the raw arguments (`page: 0` when the service actually used page 1).
`PendingRecipeImports` sets `total: int64(len(items))`, i.e. the size of the current page, not the
count, so clients cannot compute page counts. (The service's per-status query loop and its pagination
semantics belong to Phase 3.)

**Why it matters.** Admin-only, so DoS exposure is limited, but the `pageInfo` contract is broken for
the review-queue UI, and the inconsistency shows the clamp is not enforced centrally.

**Remediation.** Reuse `clamp` (or, better, a shared `pageArgs()` helper that all paged resolvers must
call) and add a `CountPending` service method. Consider enforcing the clamp in the GraphQL schema via
a `PageInput` type with validation.

---

### A2-10 — No per-request deadline for GraphQL execution

- **Category:** poor design
- **Severity:** medium
- **Location:** `cmd/lena/main.go:179-182` (server timeouts), `248-269` (GraphQL route), `internal/bff/resolver.go:819-838` (`NewGraphQLHandler`)

**Description.** The only time bounds are `http.Server.ReadTimeout`/`WriteTimeout` (15 s / 30 s by
default). Those close the socket but do **not** cancel `c.Request().Context()`; `parsed.Exec` keeps
running every resolver and DB query until the whole operation finishes. `MaxDepth(15)` and a 4 M body
limit bound the shape of a query but not its cost (a depth-3 query over `items(pageSize:100) { ... }`
with the lazy paths in A2-07 can run for a long time). There is also no `context.WithTimeout` around
`Exec`, no per-resolver statement timeout on the pool, and no Echo `Timeout` middleware.

Two smaller handler-wiring issues live here as well: `c.Bind` errors (`resolver.go:830`) are returned to
Echo's default HTTP error handler, so malformed JSON produces `{"message":"..."}` with a 4xx rather than
a GraphQL-shaped `errors[]`; and execution errors always return HTTP 200 (correct per the GraphQL
over-HTTP spec, but the request logger at `main.go:188-222` therefore cannot distinguish failed
operations without inspecting the body).

**Why it matters.** Slow queries pile up as orphaned goroutines and DB connections once clients give up
and retry, which is how a modest traffic spike turns into pool exhaustion.

**Remediation.** Wrap `Exec` in `context.WithTimeout(ctx, cfg.GraphQLTimeout)` (and return a
`GraphQL` error with code `TIMEOUT`), set `statement_timeout` on the pool, and add a cost/complexity
limit (graphql-go lacks one; count resolved fields in `TraceField` and abort past a budget). Return
bind errors in GraphQL shape with `BAD_USER_INPUT`.

---

### A2-11 — `submitItemNutritionPhoto` reports success for dropped work; OCR pipeline creates nutrient types non-atomically

- **Category:** error handling
- **Severity:** medium
- **Location:** `internal/bff/nutrition_ocr.go:115-118`, `36-62`; `internal/bff/resolver.go:83-90`

**Description.** The mutation calls `r.runAsync(...)` and unconditionally returns `true`. `runAsync`
returns silently (after a `WARN` log) when the 16-slot semaphore is full, so under load the photo is
discarded and the client is told it was accepted. Inside the task, `processNutritionPhoto` loops over
parsed labels doing `GetNutrientTypeByName` → `CreateNutrientType` (one or two queries per nutrient, and
a check-then-create race: two concurrent OCR jobs for the same new nutrient name will collide on the
unique index and fail the whole task). `SetItemNutrients` is attributed to the literal
`"ocr-system"` (line 62), losing the submitting user, and the task runs under the 30 s background
context with the request user no longer available.

**Why it matters.** Users get no feedback that nutrition data never arrived; there is no retry, no
persisted job, and no way to attribute bad OCR data to the upload that produced it. The `ocrimport`
package already contains an unreferenced work-queue abstraction (A1-15) that would solve this.

**Remediation.** Have `runAsync` return a `bool`/error and surface saturation as a GraphQL error
(`code: "BUSY"`), or — better — persist the job (status `queued`) and return its ID so the client can
poll. Use `GetOrCreateNutrientType` (`INSERT ... ON CONFLICT DO NOTHING RETURNING`) in the inventory
service and pass the user's email as actor.

---

### A2-12 — Long, deeply nested resolver functions

- **Category:** cognitive complexity
- **Severity:** medium
- **Location:**

| Function | File:lines | Length |
|---|---|---|
| `Nutrition` | `resolver_mealplan.go:103-243` | 141 |
| `ToggleGroceryItemChecked` | `resolver_grocery.go:112-227` | 116 |
| `UpdateBottle` | `resolver_wine.go:310-419` | 110 |
| `ScaledRecipe` | `resolver_recipe.go:52-140` | 89 |
| `UpdateItem` | `resolver_inventory.go:1128-1213` | 86 |
| `FrequentItems` | `resolver_inventory.go:238-318` | 81 |
| `RecommendedRecipes` | `resolver_recipe.go:395-463` | 69 |
| `MealPlans` | `resolver_mealplan.go:35-100` | 66 |
| `AddGroceryItem` | `resolver_grocery.go:244-309` | 66 |
| `DeleteBottle`-adjacent admin flows | `resolver_wine.go:1091-1103` and siblings | — |

**Description.** Three recurring shapes drive the length:

1. **Business logic in the resolver** — `Nutrition` and `ToggleGroceryItemChecked` contain the actual
   domain algorithm (nutrient aggregation with recipe-scaling and override rules; pantry
   adjust-on-check with four branches nested inside a transaction closure inside a type-assertion
   guard, five levels deep at `resolver_grocery.go:161-192`).
2. **Manual PATCH merging** — `UpdateItem`, `UpdateBottle`, `UpdateRecipe` (`resolver_recipe.go:250-310`),
   `UpdateBrand` each spend 40-70 lines copying `existing.X` unless `args.Input.X != nil`. This is the
   same 8-line block repeated per field.
3. **Hand-rolled batching** — `ScaledRecipe`, `MealPlans`, `FrequentItems` rebuild ID sets, sort them and
   call the `load*` helpers in slightly different orders instead of one shared preload function.

**Why it matters.** These are the functions most likely to hide the bugs above (A2-01, A2-06, A2-07 all
sit inside them) and the hardest to unit-test through mocks.

**Remediation.** Push (1) into the domain services (`mealplan.Service.NutritionSummary`,
`grocery.Service.ToggleChecked`); replace (2) with a `Patch` struct on the service (`UpdateItem(ctx, id,
inventory.ItemPatch{Name: *string, ...})`) or a generic `coalesce` helper; and collapse (3) into a
single `preloadRecipeGraph(ctx, recipeIDs, extraItemIDs)` used by every recipe-returning resolver.
Target ≤ 40 lines per resolver.

---

### A2-13 — `resolveUnitID` reports infrastructure errors as bad user input

- **Category:** bug
- **Severity:** low
- **Location:** `internal/bff/resolver.go:410-416`

**Description.** Any error from `GetUnitByName` — including a pool timeout — becomes
`badInputf("unknown unit %q", name)`, so the client is told its unit name is wrong and the real error
is neither logged nor surfaced.

**Remediation.** Only translate `pgx.ErrNoRows` (or a future `inventory.ErrNotFound`); wrap and return
anything else.

---

### A2-14 — Per-item unit lookups in `parseRecipeChildren`

- **Category:** antipattern (N+1 on write)
- **Severity:** low
- **Location:** `internal/bff/resolver_recipe.go:176-213`; callers `216-246` (`CreateRecipe`), `250-310` (`UpdateRecipe`)

**Description.** The loop calls `resolveUnitID` for every recipe item — one `GetUnitByName` query per
ingredient, before the transaction that writes the recipe even starts. A 40-ingredient recipe is 40
round-trips plus the transaction. The unit catalog is tiny and effectively static.

**Remediation.** Collect distinct unit names, call one `GetUnitsByNames` query (or load the whole unit
table once per request / cache it in-process with a short TTL), then map.

---

### A2-15 — Recipe-import JSON accessors swallow errors; missing user ignored

- **Category:** error handling
- **Severity:** low
- **Location:** `internal/bff/resolver_recipe_import.go:181-201` (`Draft`, `Review`), `203-214` (`Recipe`)

**Description.** `Draft()`/`Review()` return `nil` when `json.Unmarshal` fails, making a corrupt
`draft_json`/`review_json` row indistinguishable from "no draft yet"; the error is not logged. `Recipe()`
does `u, _ := currentuser.FromContext(ctx)` and constructs a `recipeResolver` with a zero `User`, so
`isFavorite`/`myRating` under an admin-only field would query for `user_id = 0` if the context were ever
missing (defensive today because `requireAdmin` guards the root, but it silently hides wiring mistakes).

**Remediation.** Return `(*T, error)` from `Draft`/`Review` and log the decode failure with the import
ID; use `userFromContext` in `Recipe()` and return the `UNAUTHENTICATED` error.

---

### A2-16 — Analytics mutations are fire-and-forget with unvalidated input

- **Category:** antipattern
- **Severity:** low
- **Location:** `internal/bff/resolver_analytics.go:16-58`, `60-78`

**Description.** `recordSelection`/`recordSearch` log the service error and return `true`. `entityType` is
any string; unknown values default to `item_selected`/`item_searched` but the raw `EntityType` string is
persisted, so the analytics table can contain arbitrary entity types. `term` is not length-limited in
the resolver while the column is `VARCHAR(500)` (`migrations` analytics schema line 9), so long terms
fail on insert and are dropped silently. `entityID` existence is not verified. Because these counters
feed `FrequentItems`/`FrequentBrands` (`resolver_inventory.go:181-188`), any authenticated user can
inflate global popularity rankings — noted here as an API-integrity issue; the abuse angle is deferred
to Phase 6.

**Remediation.** Validate `entityType` against the enum in `analytics` (return `BAD_USER_INPUT`),
truncate/limit `term` to 500 runes, return `false` (or an error) when the write fails, and consider
deduplicating selection events per user/entity/day in the service.

---

### A2-17 — `FrequentBrands` / `FrequentItems` duplication and redundant count queries

- **Category:** code smell
- **Severity:** low
- **Location:** `internal/bff/resolver_inventory.go:175-234`, `238-318`

**Description.** Lines 190-217 and 253-280 are byte-for-byte identical blend/dedupe logic. `FrequentItems`
then calls `loadItemSelectionCounts` and `loadBrandSelectionCounts` (lines 301-306), re-querying the
personal/global counts it already received from `TopUserSelections`/`TopGlobalSelections` — four
analytics queries where two suffice — and finally passes *both* the `counts` map and `ch` to
`itemResolver`, which prefers `ch` when present so the first pair of results is discarded.

**Remediation.** Extract `blendSelections(personal, global []analytics.Count, limit) ([]int64,
map[int64]countPair)` and reuse it; seed `ch.itemCounts` from the blend result instead of re-querying.

---

### A2-18 — Pointer-typed update inputs cannot clear nullable fields

- **Category:** poor design
- **Severity:** low
- **Location:** `internal/bff/resolver_wine.go:326-397`, `internal/bff/resolver_inventory.go:1146-1152`, `internal/bff/resolver_recipe.go:250-310`

**Description.** graphql-go delivers both "field omitted" and "field: null" as a nil pointer, and the
resolvers treat nil as "keep existing". Consequently `abv`, `acidity`, `oakIntegration`, `brandId`,
`servings`, `description`, etc. can be set but never reset to `NULL` through the API. `UpdateItem`'s
`brandID` is the clearest case: `optionalID` is only invoked when the pointer is non-nil, so a brand can
never be detached.

**Remediation.** Either expose explicit `clearX: Boolean` flags / a `[String!]` `clearFields` argument, or
switch the update inputs to a `Nullable[T]` wrapper (graphql-go supports `graphql.NullString` etc.) that
distinguishes absent from null.

---

### A2-19 — Tracer uses client-controlled span names and pre-sanitised error text

- **Category:** antipattern
- **Severity:** low
- **Location:** `internal/bff/graphql_tracer.go:50-62`, `105-110`

**Description.** The query span is named `"graphql " + operationName`, and `operationName` is whatever
the client sent (bounded only by the 8 KiB query-length limit). Span names are typically indexed by
tracing backends, so unique names per request produce unbounded cardinality. `finishGraphQLSpan` sets
the status message to `errs[0].Error()` *before* `sanitizeQueryErrors` runs, so raw pgx/pgconn messages
(including SQL fragments and constraint names) flow into the tracing backend. Only the first of possibly
many errors is recorded. (Metric labels are safe: `operation_type` and `field` are schema-bounded.)

**Remediation.** Use a fixed span name (`graphql.operation`) with `operationName` as an attribute,
truncated to e.g. 64 chars; record the sanitized error code rather than the raw message, and record all
errors (or a count).

---

### A2-20 — `SubmitRecipeScan` leaves orphan files when the DB insert fails

- **Category:** bug
- **Severity:** low
- **Location:** `internal/bff/recipe_scan.go:61-83`

**Description.** The image is written to the inbox (line 72) before `RecipeImportService.Create` (line 78).
If the insert fails (DB down, constraint), the function returns the error but the file remains, with no
record pointing at it, and nothing sweeps the directory. Path construction itself is safe (generated
name, fixed inbox — no traversal); file-permission and inbox-ownership questions are covered in Phases
5/6.

**Remediation.** `os.Remove(path)` on the error path, or write to a temp name and rename after the row is
committed. Better: let `recipeimport.Service.Create` own the file write (A1-09) so cleanup lives with the
transaction.

---

### A2-21 — Rate limiter is per-process, in-memory, and keyed on proxy-supplied IP

- **Category:** poor design
- **Severity:** low
- **Location:** `internal/bff/ratelimit.go:9-56`, `cmd/lena/main.go:176, 262-264`

**Description.** `RateLimiterMemoryStore` is process-local, so limits scale with replica count and reset
on restart. `IPRateLimiter` keys on `c.RealIP()`, which `main.go:176` derives from `X-Forwarded-For`
with Echo's default trust options (loopback/private ranges); correctness depends entirely on Caddy
always being the only ingress. The per-user limiter runs *after* authentication (`main.go:262-264`), so
it does not protect the auth path (A2-05) — the IP limiter does, but only at 300/min per IP. The whole
`/graphql` surface shares one budget regardless of operation cost (a `me { id }` and a 100-item
`items` page cost the same token). Trust-boundary/spoofing analysis is deferred to Phase 6.

**Remediation.** Document the single-ingress assumption or pass explicit `TrustIPRange` options; if the
service is ever scaled horizontally, back the store with Postgres/Redis. Consider cost-weighted limiting
using the field count from `TraceField`.

---

### A2-22 — Preloaded-map miss in `unitName` becomes an internal error

- **Category:** error handling
- **Severity:** low
- **Location:** `internal/bff/resolver.go:420-434`

**Description.** When `units != nil` but the ID is absent, `unitName` returns
`fmt.Errorf("unit %d missing from preloaded set")` — an `INTERNAL` error to the client — instead of
falling back to `GetUnitByID`. Any root resolver that forgets to merge a unit source into the map (as
`MealPlans` has to do by hand at `resolver_mealplan.go:78-97`) turns a data-shape oversight into a
user-visible 500-equivalent on an otherwise valid query.

**Remediation.** Fall back to the lazy lookup (and log at `WARN`) on a map miss, or make the preload
helpers responsible for collecting unit IDs from every entity type so the map is complete by construction.

---

## Observations that did not become findings

- **Error sanitisation (`errors.go`)** is sound: client-facing messages are whitelisted, everything else
  logs with the request ID and returns `INTERNAL`. Its direct dependency on `pgx.ErrNoRows` is A1-02.
- **Authorisation checks** are present and consistent: every root resolver starts with
  `userFromContext`/`requireAdmin`; per-user services receive `u.UserID`; `Item()` filters by
  `itemVisibleTo`; `canModifyItem` restricts non-admins to their own pending items. Deeper review is
  Phase 6.
- **Context propagation** is consistent inside request handling: `c.Request().Context()` flows through
  `Exec`, resolvers and services. Background work deliberately uses a detached `bgCtx` (appropriate),
  with the shutdown defect noted in A2-03.
- **`recipe_scan.go`** input handling (base64/data-URI decode, size cap defaulting to 20 MiB, random
  filenames, `0700`/`0600` modes) is careful; only the orphan-file path (A2-20) was flagged.
- **`RecommendedRecipes`** merges scores from two sources on the same `[0,1]` scale (verified in
  `internal/recipe/queries.sql:129-150`), so the direct comparison is valid.
- **Server bootstrap** (`main.go`) has sensible defaults: header/read/write/idle timeouts, body limit,
  depth and query-length limits, `Recover`, request IDs, authenticated `/metrics`, DB-pinging `/ready`,
  CORS that disables credentials for wildcard origins.

## Suggested remediation order

1. A2-01 (lost updates) and A2-02 (auth error semantics) — correctness and operability defects with
   user-visible impact.
2. A2-03, A2-04, A2-05 — shutdown and authenticator behaviour under load.
3. A2-07, A2-08, A2-09, A2-10 — API-contract consistency and N+1 removal, ideally alongside the
   DataLoader work from A1-17.
4. A2-06, A2-11, A2-12 — move nutrition, grocery-toggle and OCR logic into their domains; this shrinks
   the longest resolvers and unblocks unit testing (Phase 4).
5. Remaining lows as part of routine cleanup.
