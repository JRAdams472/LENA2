# Phase 3 — Domain Services Code Review

Date: 2026-09-13
Branch: `audit` (based on `main` @ `c4ad3c7`)
Scope: non-test `.go` files and `queries.sql` in `internal/{inventory,recipe,mealplan,grocery,wine,identity,
userprefs,analytics,ocrimport,recipeimport}`, plus `internal/platform/dbtx` (the transaction seam every
domain uses). Generated code under `*/sqlc/` was inspected for consistency but not reviewed line-by-line
(see "Generated code" below).

Method: full read of every hand-written `service.go` / `store.go` / `catalog.go` / `review.go` /
`workqueue.go`, targeted reads of `queries.sql` and `migrations/` to confirm SQL semantics and DB-level
constraints, and grep-based sweeps for `:exec` vs `:execrows`, `errors.Is`, `InTx`, and cross-package
imports. No code was executed or modified. Line numbers refer to the `main` checkout at `c4ad3c7`.

Phase 1 (`A1-xx`) and Phase 2 (`A2-xx`) findings are referenced rather than repeated. Leads carried into
this phase and their outcome:

| Lead | Outcome |
|---|---|
| A1-01 cross-schema SQL in `recipe`/`analytics` | Confirmed; not re-recorded. Performance angle recorded as A3-14. |
| A1-04 `grocery.Generate` returns an empty list | Confirmed — A3-11. |
| A1-06 non-transactional `inventory.DeleteItem` | Confirmed — A3-07 (plus a module-ownership issue). |
| A1-13 recipe-import pending-list pagination | Confirmed — A3-09. |
| A2-01 lost updates in pantry adjust | Root cause is in `userprefs` (no atomic increment) — A3-06. |
| A2-06 nutrition basis | Domain has no basis column or validation; nothing new at the domain layer beyond what A2-06 records. |
| A2-11 nutrient-type check-then-create race | Domain side confirmed and widened (case-insensitive lookup vs case-sensitive `UNIQUE`) — A3-18. |

## Severity-ranked summary

| ID | Severity | Category | Title | Location |
|---|---|---|---|---|
| A3-01 | high | bug | Every UPDATE/DELETE is `:exec`; services return `nil` when zero rows match, so not-found and ownership failures are silently reported as success | all `queries.sql`; e.g. `internal/grocery/service.go:99-101, 199-222`, `internal/mealplan/service.go:104-119, 193-209, 297-299`, `internal/inventory/service.go:483-513`, `internal/userprefs/service.go:149-151, 259-261`, `internal/wine/service.go:697-731`, `internal/recipe/service.go:173-193, 382-394` |
| A3-02 | high | bug / concurrency | Recipe-import state machine has no guarded transitions: `Approve` is non-atomic and re-runnable (duplicate recipes), `Reject`/`UpdateReview` can be overwritten by the worker | `internal/recipeimport/service.go:166-252, 316-327`, `internal/recipeimport/queries.sql:31-92` |
| A3-03 | high | bug | Recipe-import worker queue is in-memory only: jobs are orphaned on restart, `Process` has no timeout, and `Retry` while `processing` runs two workers on one job | `internal/recipeimport/service.go:330-358` |
| A3-04 | medium | poor design | Domain validation failures are untyped `fmt.Errorf` strings, so the BFF surfaces them as `INTERNAL` instead of `BAD_USER_INPUT` (identity is the only domain with sentinel errors) | `internal/wine/service.go:385-396`, `internal/recipe/service.go:109-114, 413-415`, `internal/analytics/service.go:96-101`, `internal/recipeimport/service.go:166-232` |
| A3-05 | medium | concurrency | Last-admin guard is check-then-act outside a transaction; two concurrent demotions/deactivations can leave zero admins | `internal/identity/service.go:160-213` |
| A3-06 | medium | bug | `userprefs` has no atomic quantity operation; `UpsertUserItem` is a full-row replace and ignores `UserItemID`, forcing the BFF into the lost-update pattern of A2-01 | `internal/userprefs/service.go:61-90`, `internal/userprefs/queries.sql:1-16` |
| A3-07 | medium | bug / poor design | `inventory.DeleteItem` runs two statements without a transaction and deletes `user_item` rows that belong to the `userprefs` module | `internal/inventory/service.go:505-513`, `internal/inventory/queries.sql:165-171` |
| A3-08 | medium | bug / concurrency | `SubmitBrand` is check-then-create on a non-indexed normalized expression; it resurfaces rejected brands and other users' pending brands, and duplicates slip through under concurrency | `internal/inventory/service.go:79-97`, `internal/inventory/queries.sql:19-34` |
| A3-09 | medium | bug | `ListPending` applies the same page offset to eight separate per-status queries; page ≥ 2 skips/duplicates rows and ordering is by status not recency | `internal/recipeimport/service.go:137-163` |
| A3-10 | medium | bug | `AllResolved` treats fuzzy "suggested" matches as resolved and ignores `Status`/`Approved`, so imports auto-advance to `ready` and can be persisted without an admin accepting any match | `internal/ocrimport/review.go:57-64`, `internal/ocrimport/catalog.go:263-268`, `internal/recipeimport/service.go:230-232, 446-449` |
| A3-11 | medium | poor design | `grocery.Generate` is a stub that only creates an empty list; no layer implements meal-plan → grocery aggregation | `internal/grocery/service.go:224-231` |
| A3-12 | medium | poor design | `InTx` on a `WithTx`-bound service opens a *new* transaction on the pool, silently escaping the caller's transaction; `analytics.runInTx` ships a `pool == nil` test seam in production code | `internal/*/service.go` `WithTx`/`InTx` (e.g. `internal/grocery/service.go:34-43`), `internal/inventory/service.go:745-766`, `internal/analytics/service.go:170-178` |
| A3-13 | medium | cognitive complexity / code smell | `recipeimport.Process` is a 105-line pipeline driven by magic status strings; a second, incompatible status vocabulary exists in `ocrimport` | `internal/recipeimport/service.go:147, 203-206, 350-455`, `internal/ocrimport/workqueue.go:18-27` |
| A3-14 | low | performance | Recommendation and import-matching paths are O(catalog) or O(users × recipes) per event | `internal/analytics/queries.sql:66-100`, `internal/recipeimport/catalog.go:41-98`, `internal/ocrimport/catalog.go:222-233` |
| A3-15 | low | bug | `identity.users.email` is not unique and `UpsertUser` blanks `display_name` when the claim is empty (email is guarded, display name is not) | `internal/identity/queries.sql:12-31`, `migrations/0002_create_identity.up.sql:13` |
| A3-16 | low | code smell | `textOrNull` / `optInt8` / `numericFromFloat64` / `optNumeric` re-implemented in eight packages | `internal/{analytics,grocery,identity,inventory,mealplan,recipe,userprefs,wine}/service.go` |
| A3-17 | low | code smell | Dead CLI-era code in `ocrimport`: file-backed `Queue`, `LoadReview`/`SaveReview`, `RenderReviewMarkdown`, `MergeExistingReview`, `BestSuggestion` have no non-test callers | `internal/ocrimport/workqueue.go`, `internal/ocrimport/review.go:27-52, 67-97, 100-175, 185-205` |
| A3-18 | low | bug | Case-insensitive lookups paired with case-sensitive `UNIQUE` constraints (`nutrient_type.name`, `unit.name`/`abbreviation`, `item (name, brand_id)` with NULL brand) let near-duplicates in, after which `:one` queries return an arbitrary row | `internal/inventory/queries.sql:89-92, 183-186, 366-369`, `migrations/0003_create_inventory.up.sql:30, 56` |
| A3-19 | low | code smell | Audit columns applied inconsistently (`UpdateBrand`, `SetUserRole`, `CreateNutrientType` skip `updated_by`/`created_by`); stale comment claims tasting scores have no DB `CHECK` | `internal/inventory/queries.sql:183-186, 268-272`, `internal/identity/queries.sql:33-37`, `internal/wine/service.go:382-384` |
| A3-20 | low | bug | Worker lifecycle edges: unbounded goroutine-per-job fan-out, `close(s.shutdown)` panics on second `Shutdown`, `MarkFailed` error discarded | `internal/recipeimport/service.go:330-347, 513-526` |
| A3-21 | low | poor design | Domain services perform almost no input validation (free-text `MealType`/`Source`, negative `current_qty`/`Quantity`, arbitrary `status` strings); correctness rests on DB `CHECK`s that surface as `INTERNAL` | `internal/mealplan/service.go:134-152`, `internal/grocery/service.go:121-149`, `internal/userprefs/service.go:61-90, 169-200`, `internal/inventory/service.go:177-191, 416-430` |

Totals: 3 high, 10 medium, 8 low.

---

## Findings

### A3-01 — `:exec` UPDATE/DELETE queries hide not-found and ownership failures (high, bug)

**Location.** Every mutation in every `queries.sql` is declared `:exec`; there is not a single `:execrows`
or `:execresult` in the repository (50 UPDATE/DELETE statements). Representative service wrappers:
`internal/grocery/service.go:99-101` (`DeleteGroceryList`), `199-222` (`UpdateGroceryListItem`,
`DeleteGroceryListItem`); `internal/mealplan/service.go:104-119, 193-209, 297-299`;
`internal/inventory/service.go:483-500` (`UpdateItem`), `505-513` (`DeleteItem`);
`internal/userprefs/service.go:149-151, 259-261`; `internal/wine/service.go:697-731`;
`internal/recipe/service.go:173-193, 382-394`; `internal/identity/service.go:122-124, 218-226`.

**Problem.** `pgx` `Exec` does not error when zero rows match. All per-user queries correctly include
`AND user_id = $n` for ownership, but when the row does not exist or belongs to another user the service
returns `nil` and the caller cannot tell. The BFF then returns `true` from `deleteGroceryList`,
`deleteMealSlot`, `deleteUserItem`, `updateBottle`, etc. for IDs that were never touched.

**Why it matters.** (1) Clients get false-positive success for updates/deletes of missing or foreign rows
and never learn the write was a no-op. (2) It defeats the typed-error contract described in
`docs/graphql-bff-orchestration.md` (`ErrNotFound`) and forces the BFF into extra pre-read queries
(A2-07) purely to synthesise a not-found. (3) Where the BFF *does not* pre-read (delete paths), an
attacker probing IDs receives identical responses for "exists but not yours" and "doesn't exist", which
is good, but the legitimate owner also gets no signal when a stale ID is deleted twice.

**Remediation.** Switch mutations to `:execrows`, and in each service `if n == 0 { return ErrNotFound }`
with a package-level sentinel (`var ErrNotFound = errors.New(...)`) that the BFF maps to `NOT_FOUND`.
Alternatively `... RETURNING <pk>` with `:one` and map `pgx.ErrNoRows`. Either way, remove the BFF's
"read then write" pre-checks once the domain reports the outcome.

### A3-02 — Recipe-import transitions are unguarded; `Approve` is non-atomic and re-runnable (high, bug / concurrency)

**Location.** `internal/recipeimport/service.go:214-252` (`Approve`), `166-211` (`UpdateReview`),
`316-327` (`Reject`, `Retry`); `internal/recipeimport/queries.sql:31-92` — every `UPDATE recipe.recipe_import`
is `WHERE recipe_import_id = $1` with no status predicate.

**Problem.**
- `Approve` reads the row, checks `Status` in Go, calls `rec.CreateRecipeWithChildren` (its own
  transaction on the recipe pool), then `store.SetPersisted` in a second, separate statement. If
  `SetPersisted` fails, or two admins approve concurrently, or a client retries after a timeout, the
  recipe is created twice and the import may still read `ready`.
- `Approve` rejects `persisted` and `profanity` but not `rejected` or `failed`; a rejected import can be
  approved if its review JSON happens to be resolved.
- `Reject`, `Retry`, `SetPersisted` and the worker's `UpdateReview(... "processing"/"reviewing"/"ready")`
  all write unconditionally. A worker finishing after an admin clicked Reject flips the row back to
  `reviewing`; `UpdateReview` on a `persisted` import silently edits history.
- `Store.WithTx` exists (`store.go:39-41`) but is never used; the service has a `pool` field it never
  transacts on.

**Why it matters.** Duplicate catalog recipes, lost admin decisions, and an audit trail
(`approved_by_user_id`, `approved_at`) that can disagree with what was persisted.

**Remediation.** Make every transition a conditional update returning the affected count, e.g.
`UPDATE ... SET status='persisted', ... WHERE recipe_import_id=$1 AND status IN ('ready','reviewing')`
declared `:execrows`, and treat `0` as `ErrConflict`. Run `Approve` as one transaction: begin on the pool,
`recipe.Service.WithTx(tx).CreateRecipeWithChildren(...)`, `store.WithTx(tx).SetPersisted(...)`, commit.
Introduce a small `canTransition(from, to)` table so the worker cannot regress a terminal state.

### A3-03 — In-memory import queue: orphaned jobs, no timeouts, re-entrant processing (high, bug)

**Location.** `internal/recipeimport/service.go:330-347` (`EnqueueProcess`), `350-358` (`Process`
status gate), `321-327` (`Retry`).

**Problem.**
- Job IDs are enqueued only via a goroutine at `Create`/`Retry` time. There is no startup sweep, so after
  a restart every `pending`/`processing`/`ocred`/`drafted` row stays there forever until an admin
  manually hits `retryRecipeImport`.
- `Process` runs under `context.WithCancel(context.Background())` with no deadline. A hung OCR or Ollama
  call holds a worker slot indefinitely (`ImportWorkerConcurrency` defaults to 1), and `Shutdown` never
  cancels it — it only stops *queued* jobs from starting.
- `Process` accepts `processing` as a resumable status (line 355). `Retry` on a job that is mid-flight
  sets it back to `pending` and enqueues a second worker; both then write OCR/draft/review JSON to the
  same row.

**Why it matters.** Silent pipeline stalls after deploys, one stuck upstream call disabling the whole
import feature, and interleaved writes producing inconsistent `ocr_json`/`draft_json`/`review_json`.

**Remediation.** On `NewService`/start, `SELECT recipe_import_id FROM recipe.recipe_import WHERE status IN
('pending','processing',...)` and re-enqueue. Claim jobs with `UPDATE ... SET status='processing' WHERE id=$1
AND status='pending' RETURNING ...` so only one worker proceeds. Derive the worker context from a service
lifetime context (cancelled in `Shutdown`) with `context.WithTimeout` per stage.

### A3-04 — Untyped validation errors surface as `INTERNAL` (medium, poor design)

**Location.** `internal/wine/service.go:385-390` (`checkScore`), `393-396` (percentage);
`internal/recipe/service.go:109-114` (`ScaleRecipe`), `413-415` (`SetRating`);
`internal/analytics/service.go:96-101`; `internal/recipeimport/service.go:166-196` (`UpdateReview` item/unit
validation), `219-232` (`Approve` state checks). Contrast `internal/identity/service.go:73-77`, the only
domain that exports sentinel errors, and `internal/bff/errors.go:54-57`, which only recognises `clientError`
and `pgx.ErrNoRows`.

**Problem.** Domain-level validation returns `fmt.Errorf("… must be between 1 and 5")` and similar. The
BFF's `sanitizeQueryErrors` cannot classify these, so they are logged as server errors and returned to the
client as `INTERNAL`. The BFF has partially compensated by duplicating the same rules in resolvers
(`resolver.go:220`), which is the drift that A1-02/A1-09 describe.

**Why it matters.** Users receive "internal error" for their own bad input; error-rate dashboards are
polluted; and the domain rules are enforced in two places that will diverge.

**Remediation.** Add `ErrValidation`/`ErrConflict`/`ErrNotFound` sentinels (or a small `ValidationError`
type) to each domain (or a shared `internal/platform/domainerr`), wrap with `%w`, and extend
`sanitizeQueryErrors` to map them. Then delete the duplicated checks in the BFF.

### A3-05 — Last-admin guard is a TOCTOU outside a transaction (medium, concurrency)

**Location.** `internal/identity/service.go:160-177` (`checkAdminMutation`), `181-213`
(`AdminSetRole`, `AdminSetActive`).

**Problem.** `CountActiveAdmins` is read, compared with `<= 1`, and then `SetUserRole`/`SetUserActive`
executes as a separate statement outside any transaction. With two active admins A and B, concurrent
"demote B" and "demote A" requests both observe `n = 2`, both pass, and the system ends with no admin.
`GetByID` → check → write is likewise unlocked, so a protected-email change or a role change between the
read and the write is not seen.

**Why it matters.** Loss of all administrative access requires a DB intervention to recover; the guard
exists precisely to prevent this.

**Remediation.** Run the guard and the write in `InTx`, and make the write itself conditional:
`UPDATE identity.users SET role=$2 WHERE user_id=$1 AND ($2='admin' OR (SELECT COUNT(*) FROM identity.users
WHERE role='admin' AND is_active) > 1)` as `:execrows`, returning `ErrLastAdmin` on zero rows. Also drop the
duplicated self-check (lines 182-184 and 161-163 test the same condition).

### A3-06 — `userprefs` exposes no atomic quantity adjustment (medium, bug)

**Location.** `internal/userprefs/service.go:61-90` (`UpsertUserItem`); `internal/userprefs/queries.sql:1-16`.

**Problem.** The only pantry write is a full-row `INSERT … ON CONFLICT (user_id, item_id) DO UPDATE SET
current_qty = EXCLUDED.current_qty, …`. There is no `current_qty = current_qty + $n` path, so the BFF must
read, add in Go, and upsert — the lost-update race recorded in A2-01. The `UserItem.UserItemID` field is
accepted but ignored (the upsert keys on `(user_id, item_id)`), and every upsert also overwrites `min_qty`,
`purchase_at`, `expires_at`, `notes`, `is_favorite`, so a caller that only wants to bump quantity must
faithfully echo every other column or clobber it.

**Why it matters.** This is the domain-side root cause of A2-01; the fix belongs here, not in the BFF.

**Remediation.** Add `AdjustUserItemQuantity(ctx, userID, itemID, delta, by)` backed by
`INSERT … ON CONFLICT DO UPDATE SET current_qty = GREATEST(0, inventory.user_item.current_qty + EXCLUDED.current_qty),
updated_by=…, updated_at=now() RETURNING *`. Consider a `CHECK (current_qty >= 0)` (none exists today).

### A3-07 — `DeleteItem` is two statements without a transaction and crosses module ownership (medium, bug / poor design)

**Location.** `internal/inventory/service.go:505-513`; `internal/inventory/queries.sql:165-171`
(`DeleteUserItemsByItem`, `DeleteItem`).

**Problem.** `DeleteUserItemsByItem` commits before `DeleteItem` runs. If the second statement fails (FK
from `recipe.recipe_item`, `grocery.grocery_list_item`, `mealplan.meal_slot_item` — none of which are
cleaned up here), every user's pantry row for that item is already gone and the item remains. The
package comment in `userprefs/service.go:1-4` states `inventory.user_item` is owned by `userprefs`; the
inventory module writing to it is the same boundary violation as A1-01, just in the delete direction.

**Why it matters.** Irreversible data loss on a failed delete; unclear ownership makes future schema
changes to `user_item` risky.

**Remediation.** Wrap in `s.InTx`; have the BFF orchestrate `userprefs.DeleteUserItemsByItem` and
`inventory.DeleteItem` in one transaction via `WithTx`, or add `ON DELETE CASCADE`/`RESTRICT` explicitly and
let the DB decide.

### A3-08 — `SubmitBrand` check-then-create with lossy normalization (medium, bug / concurrency)

**Location.** `internal/inventory/service.go:79-97`; `internal/inventory/queries.sql:19-25`
(`FindBrandByNormalizedName`), `27-34` (`SearchBrands`).

**Problem.**
- `FindBrandByNormalizedName` matches *any* status. A brand an admin rejected is returned to the next
  submitter as "existing", so rejection is not sticky; a pending brand submitted by user X is returned to
  user Y even though `ListBrandsVisible` hides it from Y.
- The DB `UNIQUE` is on the raw `name` (`migrations/0003_create_inventory.up.sql:14`), not on the
  normalized expression. Two concurrent submits of "Bush's" and "Bushs" both miss the lookup and both
  insert; afterwards the `:one` query returns whichever row the planner finds first.
- `lower(regexp_replace(name, …))` is not indexed, so every submit and every `SearchBrands` keystroke is a
  sequential scan over `inventory.brand`.

**Why it matters.** Moderation can be bypassed by resubmitting; catalog duplicates; and search latency grows
linearly with the catalog.

**Remediation.** Add a generated column `name_normalized` with a unique index (or a unique expression
index), filter the lookup by `status <> 'rejected'` and `(status='approved' OR submitted_by_user_id=$2)`,
and handle `23505` on insert as `ErrConflict`. `SearchBrands` can then use a trigram or prefix index on the
normalized column.

### A3-09 — `ListPending` pagination is wrong by construction (medium, bug)

**Location.** `internal/recipeimport/service.go:137-163`.

**Problem.** For each of eight statuses the same `LIMIT pageSize OFFSET (page-1)*pageSize` is issued.
Page 1 returns the first `pageSize` rows of `pending`, then `processing`, … until full — so a single status
with more than `pageSize` rows hides every other status. Page 2 skips the first `pageSize` rows *of every
status* rather than the first `pageSize` rows of the union, so rows are both skipped and repeated across
pages. Ordering is by status order, not `created_at`. The BFF then reports `total = len(items)` (A2-09).

**Why it matters.** The admin review queue is the only way imports get approved; items become invisible
once one status bucket grows.

**Remediation.** One query: `WHERE status = ANY($1::varchar[]) ORDER BY created_at DESC LIMIT $2 OFFSET $3`
plus a matching `COUNT(*)`. Delete the Go-side merge.

### A3-10 — `AllResolved` lets fuzzy suggestions pass the human gate (medium, bug)

**Location.** `internal/ocrimport/review.go:57-64`; `internal/ocrimport/catalog.go:263-268`
(`MatchItem` sets `ItemID` for `Status == "suggested"`); `internal/recipeimport/service.go:446-449`
(worker marks `ready`), `230-232` (`Approve` only checks `AllResolved`).

**Problem.** `AllResolved` returns true when every item has a non-empty `ItemID` and `Unit`. `MatchItem`
populates `ItemID` for a fuzzy match whose score merely clears `ImportReviewThreshold` (status
`"suggested"`), and `MapDraftItem` always fills `Unit` when the unit resolves. Consequently a draft whose
every ingredient fuzzy-matched at, say, 0.6 is written as `ready` with no admin interaction, and `Approve`
persists it. Neither `MatchResult.Approved` nor `ReviewRecipe.Approved` nor `Status` is consulted anywhere
in the server path. `AllResolved` also ignores `UnitID`, which `buildRecipe` (lines 272-280) needs.

**Why it matters.** Wrong catalog items end up in published recipes; the review step the pipeline is
designed around is optional in practice.

**Remediation.** Define resolved as `Status == "accepted" || Approved` *and* `UnitID != ""`; have
`UpdateReview` set `Approved` per item; require `review.Approved` in `Approve`.

### A3-11 — `grocery.Generate` is a stub (medium, poor design)

**Location.** `internal/grocery/service.go:224-231`.

**Problem.** The doc comment promises "seeds it from a meal plan … totals and pantry subtraction are
calculated in Go", the body creates an empty list, and the inline comment defers aggregation to "the
BFF/resolver layer" — which (Phase 2) does not implement it either. `generateGroceryList` therefore returns
an empty list to every caller.

**Why it matters.** A headline feature is non-functional, and the misleading doc comment will mislead the
next implementer about where the logic lives.

**Remediation.** Implement in the BFF (the only layer allowed to read mealplan + recipe + userprefs):
load slots/items for the plan, expand recipes via `recipe.ListRecipeItemsByRecipes`, aggregate by
`(item_id, unit_id)`, subtract `userprefs.ListUserItems`, then bulk-insert via a new
`grocery.AddGroceryListItems(ctx, []GroceryListItem)` inside one transaction. Until then, make `Generate`
return an explicit `ErrNotImplemented` or remove it.

### A3-12 — `WithTx`/`InTx` composition trap and a test seam in production (medium, poor design)

**Location.** Pattern repeated in every domain, e.g. `internal/grocery/service.go:34-43`;
`internal/inventory/service.go:745-766` (`SetItemNutrients`); `internal/analytics/service.go:170-178`
(`runInTx`).

**Problem.** `WithTx` copies the service but keeps `pool`; `InTx` always begins a *new* transaction on
`pool`. So `inv.WithTx(tx).SetItemNutrients(...)` runs its delete+insert in a different transaction from
`tx`, and a caller that composes `WithTx` services inside `dbtx.InTx` and then calls any method that itself
uses `InTx` gets two independent transactions with no error. Nothing in the API prevents or documents
this. `analytics.runInTx` adds a `pool == nil` branch so unit tests can skip transactions — a test-only
code path compiled into the service.

**Why it matters.** Atomicity guarantees that look correct at the call site are silently broken; the
analytics seam means the transactional path is never exercised by the unit tests that rely on it.

**Remediation.** Have `WithTx` set a `tx pgx.Tx` field and make `InTx` reuse it (`if s.tx != nil { return
fn(s) }`), or use `tx.Begin` for a savepoint. Remove `runInTx`'s nil-pool branch and test with a fake
`dbtx.Pool` instead.

### A3-13 — `Process` is a long, string-driven pipeline with two status vocabularies (medium, cognitive complexity / code smell)

**Location.** `internal/recipeimport/service.go:350-455` (105 lines, 11 early returns, 8 sequential
store writes), magic strings at `103, 147, 203-206, 219-222, 355, 446`; `internal/ocrimport/workqueue.go:18-27`
declares `ocrimport.Status` constants that lack `processing`/`profanity` and are unused by `recipeimport`.
`Process` also uses `store.UpdateReview(ctx, id, ri.ReviewJSON, "processing", …)` (line 360) to change
status, re-writing `review_json` as a side effect.

**Why it matters.** Status typos compile; adding a state requires touching six call sites and the SQL;
the mismatch between the two enumerations invites bugs when the CLI-era types are reused.

**Remediation.** One `type Status string` with constants and a transition table in `recipeimport`; split
`Process` into `runOCR`, `runDraft`, `runMapping` each returning the next state; add a dedicated
`SetStatus` store method.

### A3-14 — O(catalog) and O(users × recipes) work per event (low, performance)

**Location.** `internal/analytics/queries.sql:66-100` (`IngredientOverlapScores`);
`internal/recipeimport/catalog.go:41-98` (`newCatalogSnapshot`); `internal/ocrimport/catalog.go:222-233`
(`MatchItem` fuzzy sweep).

**Problem.** Every `createRecipe` runs a Jaccard computation over all users' meal-slot history joined to
all recipe items (async, but on the shared pool). Every import job pages the *entire* approved item and
ingredient catalog 100 rows at a time (passing `userID = 0` to a visibility-filtered query) and then scores
each draft ingredient against every catalog entry with Jaro-Winkler.

**Why it matters.** Fine at current scale; degrades linearly-to-quadratically and competes with request
traffic since it shares the pool and has no limits.

**Remediation.** Cache the catalog snapshot per worker with a TTL or invalidation on item approval; add a
dedicated `ListApprovedItems` query; bound `IngredientOverlapScores` to recently active users or move it
to a scheduled job.

### A3-15 — Email not unique; `UpsertUser` blanks `display_name` (low, bug)

**Location.** `internal/identity/queries.sql:12-31`; `migrations/0002_create_identity.up.sql:13`.

**Problem.** Uniqueness is `(provider, external_subject)` only; the same email via two issuers is two
accounts, and `LENA_ADMIN_EMAILS` promotion (BFF) matches by email, so both become admin. The upsert guards
`email` against an empty claim (`CASE WHEN EXCLUDED.email = '' …`) but sets `display_name =
EXCLUDED.display_name` unconditionally, so a token without a name claim nulls a previously stored name.

**Remediation.** Apply the same `CASE`/`COALESCE` to `display_name`; decide whether email should be
unique per provider or globally and enforce it (security implications deferred to Phase 6).

### A3-16 — Helper functions duplicated across eight packages (low, code smell)

**Location.** `textOrNull`, `optInt8`/`optInt64`, `optInt4`, `optNumeric`, `numericFromFloat64`,
`numericToFloat64` in `internal/{analytics,grocery,identity,inventory,mealplan,recipe,userprefs,wine}/service.go`.

**Remediation.** Move to `internal/platform/pgconv` (or similar); the functions have no domain semantics.

### A3-17 — Dead CLI-era code in `ocrimport` (low, code smell)

**Location.** `internal/ocrimport/workqueue.go` (entire file: file-backed `Queue`, `queue.json`);
`internal/ocrimport/review.go:27-52` (`LoadReview`/`SaveReview`), `67-97` (`MergeExistingReview`), `100-175`
(`RenderReviewMarkdown`), `185-205` (`BestSuggestion`).

**Problem.** None of these symbols are referenced outside `ocrimport` and its tests; the server pipeline
stores everything in `recipe.recipe_import`. They carry filesystem I/O (`os.WriteFile`, `MkdirAll`) that
Phase 6 must otherwise treat as attack surface.

**Remediation.** Delete, or move to a `cmd/ocrimport` tool if the CLI is still wanted.

### A3-18 — Case-insensitive lookups vs case-sensitive uniqueness (low, bug)

**Location.** `internal/inventory/queries.sql:89-92` (`GetNutrientTypeByName`), `183-186`
(`CreateNutrientType`), `366-369` (`GetUnitByName`); `migrations/0003_create_inventory.up.sql:30, 56`.

**Problem.** `nutrient_type.name` is `UNIQUE` but compared with `lower()`, so "Protein" and "protein"
can both exist (the OCR path in A2-11 creates types from free text), after which the `:one` lookup returns
an arbitrary one. `GetUnitByName` matches `name OR abbreviation` with `:one`; a unit whose abbreviation
equals another unit's name yields nondeterministic results. `item UNIQUE (name, brand_id)` does not
constrain rows with `brand_id IS NULL`.

**Remediation.** Unique indexes on `lower(name)`; `UNIQUE NULLS NOT DISTINCT` (PG15+) or a partial unique
index for brand-less items; make `GetUnitByName` prefer exact name before abbreviation, or return `:many`
and disambiguate.

### A3-19 — Inconsistent audit columns and stale comments (low, code smell)

**Location.** `internal/inventory/queries.sql:268-272` (`UpdateBrand` no `updated_by`/`updated_at`),
`183-186` (`CreateNutrientType` no `created_by`); `internal/identity/queries.sql:33-37` (`SetUserRole` no
`updated_by`); `internal/wine/service.go:382-384` says tasting-score columns "have no DB CHECK constraint"
but `migrations/0018_domain_check_constraints.up.sql:27-31` added them.

**Remediation.** Add the columns to the statements; update the comment (and drop the Go check or keep it
as the typed-error source per A3-04).

### A3-20 — Worker lifecycle edge cases (low, bug)

**Location.** `internal/recipeimport/service.go:330-347, 513-526`.

**Problem.** `EnqueueProcess` spawns one goroutine per job that blocks on the semaphore, so a burst of
uploads creates unbounded parked goroutines. `Shutdown` does `close(s.shutdown)` unguarded — a second call
panics. `MarkFailed`'s error is discarded, so a job whose processing *and* failure-marking both fail stays
`processing` forever (compounding A3-03).

**Remediation.** Use a buffered channel queue with a fixed worker loop; `sync.Once` around close; log the
`MarkFailed` error.

### A3-21 — Near-zero domain-level input validation (low, poor design)

**Location.** `internal/mealplan/service.go:134-152` (`MealType` free text), `internal/grocery/service.go:121-149`
(`Source` free text, quantity sign), `internal/userprefs/service.go:61-90, 169-200` (`CurrentQty`, `Quantity`
may be negative; no `CHECK` on `user_item.current_qty`), `internal/inventory/service.go:177-191, 416-430`
(`status` string accepted verbatim, relies on `CHECK`).

**Problem.** Services accept whatever the BFF passes; correctness depends on `migrations/0018` `CHECK`s
and on the BFF re-implementing rules. When a `CHECK` fires it is a `pgconn` error that reaches the client
as `INTERNAL` (A3-04).

**Remediation.** Validate at the domain boundary (enums for `MealType`/`Source`/status, non-negative
quantities) and return `ErrValidation`; keep the `CHECK`s as a backstop.

---

## Generated code (sqlc)

- All `internal/*/sqlc/*.go` carry the `// Code generated by sqlc. DO NOT EDIT. … sqlc v1.31.1` header;
  no hand edits were detected. `sqlc.yaml` enables `emit_interface`, `emit_json_tags`,
  `emit_empty_slices` and a `timestamptz → time.Time` override uniformly across domains.
- Every query uses positional/named parameters; no string concatenation into SQL was found in any
  `queries.sql` (full SQL-injection verification is scheduled for Phase 6).
- Smells originate in the SQL, not the generator: unnamed parameters produce `Column1`
  (`recipeimport/sqlc` `ListRecipeImportsParams`) and `RegexpReplace` (`inventory/sqlc`
  `SearchBrandsParams`); use `sqlc.arg(name)` as `analytics/queries.sql` already does.
- Generated code is out of scope for the findings above; every `A3-xx` refers to hand-written Go or SQL.

## Observations not raised as findings

- `internal/platform/dbtx.InTx` is correct and small; it does not recover panics inside `fn`, which is
  acceptable given Echo's recovery middleware but means a panicking resolver leaks a connection until the
  deferred `Rollback` runs (it does run).
- `identity` is the model domain for error handling (`ErrSelfModification`, `ErrProtectedUser`,
  `ErrLastAdmin`) and for copying the receiver in `WithTx` so `protected` survives.
- `recipe.ScaleRecipe` and `recipe.CreateRecipeWithChildren`/`UpdateRecipeWithChildren` are correctly
  transactional and are the right shape for other domains to follow.
- `analytics.UpsertUserSelectionCount`/`UpsertGlobalSelectionCount` use `select_count + 1` in SQL — the
  atomic pattern that A3-06 asks for in `userprefs`.
- `recipe.ListRatingRecencySuggestions` joins `meal_slot` for all users before filtering on `meal_plan`;
  results are correct (non-matching plans yield NULL and drop out of `MAX`) but the join is wider than
  necessary.

## Suggested remediation order

1. A3-01 (`:execrows` + `ErrNotFound`) and A3-04 (typed errors) together — they unblock the BFF cleanup
   promised in A1-02/A1-03 and remove most pre-read queries.
2. A3-02 / A3-03 / A3-10 — the recipe-import pipeline's correctness issues; small SQL changes with large
   behavioural impact.
3. A3-05, A3-06, A3-07, A3-08 — the remaining concurrency/atomicity defects.
4. A3-09, A3-11, A3-12 — pagination, the grocery stub, and the transaction seam.
5. Low-severity clean-ups (A3-13 to A3-21) opportunistically alongside the above.
