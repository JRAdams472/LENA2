# Phase 4 — Unit & Integration Test Review

Branch: `audit` · Base: `main` · Scope: every `*_test.go` under `cmd/` and `internal/`, `internal/bff/mock/`, all `generate.go` directives, `tools/coveragefilter`, and the Go job in `.github/workflows/test.yml`. Frontend (`clients/web`) tests are out of scope for this phase.

No application, test, or configuration code was modified. Findings are numbered `A4-xx` and appended to `docs/20260911-audit-findings.md`.

## Severity-ranked summary

| ID | Severity | Category | Location | Title |
|---|---|---|---|---|
| A4-01 | high | bug (test gap) | `internal/bff/resolver_grocery.go:130-190`, `internal/bff/resolver_grocery_test.go`, `internal/bff/bff_integration_test.go:582-593` | Transactional grocery→pantry sync path has no test (29.8% fn coverage); all tests hit the `Pool == nil` fallback |
| A4-02 | high | poor design (test gap) | `internal/recipeimport/*`, `internal/bff/resolver_recipe_import.go`, `internal/ocrimport/review.go`, `internal/ocrimport/workqueue.go` | Recipe-import pipeline is effectively untested (pkg 20%, resolver file 0%, store 0%, no integration test) |
| A4-03 | high | bug (tautological assertion) | `internal/grocery/integration_test.go:220-231`, `internal/mealplan/integration_test.go` | Cross-user integration tests assert `NoError` on wrong-user UPDATE/DELETE, codifying the silent-success bug (A3-01) |
| A4-04 | medium | security (test gap) | `internal/bff/bff_integration_test.go:51,148` | End-to-end suite runs every mutation as an admin; no HTTP-level non-admin authorization test |
| A4-05 | medium | code smell (weak assertion) | `internal/bff/bff_integration_test.go:684-711` | `doGraphQL` swallows JSON decode errors and only logs GraphQL errors |
| A4-06 | medium | security (test gap) | `internal/bff/nutrition_ocr.go`, `internal/bff/recipe_scan.go`, `internal/platform/ocrclient/` | OCR/scan upload paths and `ocrclient` have zero tests (data-URI parsing, file writes, async fan-out) |
| A4-07 | medium | bug (test gap) | `internal/inventory/service.go:79-194`, `internal/bff/resolver_inventory.go:73-175,1014` | Brand moderation flow (`SubmitBrand`, `SearchBrands`, `PendingBrands`, `Approve/RejectBrand`, `brandWriteError`) is 0% covered |
| A4-08 | medium | antipattern | `internal/*/service_test.go`, `internal/bff/resolver_*_test.go` | Two mock layers mirror the implementation; SQL predicates and `:exec` semantics are never observed by unit tests (353 `gomock.Any()` in resolver tests) |
| A4-09 | medium | code smell | `internal/recipeimport/service_test.go:18-201,252-270` | Hand-rolled `memoryStore` diverges from the SQL store; `Approve` tested only from a pre-set `ready` state; errors discarded with `_` |
| A4-10 | medium | bug (test gap) | all packages | No concurrency tests exist despite four race findings (A2-01, A3-05, A3-06, A3-08); `-race` cannot detect DB-level lost updates |
| A4-11 | low | code smell (flaky pattern) | `internal/bff/resolver_misc_test.go:190-229` | Async-worker test relies on `time.Sleep(50ms)` and asserts a non-event immediately |
| A4-12 | low | antipattern | `internal/platform/testenv/testenv.go`, `internal/*/integration_test.go` | One Postgres container per test function (~8 min wall) with a hard Docker Hub dependency; run failed with `429 Too Many Requests` |
| A4-13 | low | poor design | `.github/workflows/test.yml`, `tools/coveragefilter/main.go` | Single 60% global floor (measured 65.7% filtered / 41.6% raw) hides zero-coverage packages and files |
| A4-14 | low | code smell | `internal/bff/bff_integration_test.go:137-139` | Integration test mutates `Authenticator` private state (`mu`, `jwks[...].fetchedAt`) |
| A4-15 | low | poor design (test gap) | `cmd/lena/main_test.go`, `cmd/lena/main_integration_test.go` | Bootstrap tests cover CORS/health/401 only; shutdown, rate-limiter wiring, import inbox untested |
| A4-16 | low | code smell (test gap) | `internal/{identity,mealplan,userprefs,wine}/service.go` `WithTx`/`InTx` | Transaction binding untested outside `grocery`; A3-12 composition never exercised |

Totals: 3 high · 7 medium · 6 low.

## Execution results

Command (mirrors CI):

```
go test -race -count=1 -coverprofile=cov.out -covermode=atomic ./cmd/... ./internal/...
go run ./tools/coveragefilter < cov.out > covf.out
go tool cover -func=covf.out | tail -1
```

Run 1 (cold Docker cache): 22 integration test functions across `cmd/lena`, `analytics`, `bff`, `grocery`, `identity`, `inventory`, `mealplan`, `recipe`, `userprefs`, `wine` **failed** with `failed to resolve reference "docker.io/library/postgres:16-alpine": 429 Too Many Requests`. Unit tests passed; filtered coverage 61.7% (1.7 points above the CI floor). See A4-12.

Run 2 (after pulling `postgres:16-alpine` from `mirror.gcr.io`): **all packages pass**, `EXIT=0`. Wall time ≈ 8.5 min dominated by container startup.

| Package | Coverage | Notes |
|---|---|---|
| `cmd/lena` | 66.5% | 7.5% without integration test |
| `cmd/testissuer` | 32.1% | |
| `internal/analytics` | 86.0% | |
| `internal/bff` | 69.2% | `resolver_recipe_import.go` 0%, `nutrition_ocr.go` 0%, `recipe_scan.go` 0% |
| `internal/grocery` | 86.0% | |
| `internal/identity` | 84.7% | |
| `internal/inventory` | 71.7% | brand-moderation functions 0% |
| `internal/inventory/nutritionparse` | 89.2% | |
| `internal/mealplan` | 88.9% | |
| `internal/ocrimport` | 52.5% | `review.go`, `workqueue.go` 0% |
| `internal/platform/config` | 83.3% | |
| `internal/platform/currentuser` | 100% | |
| `internal/platform/dbtx` | 94.3% | |
| `internal/platform/logger` | 81.2% | |
| `internal/platform/ocrclient` | 0.0% | no test files |
| `internal/platform/ollamaclient` | 73.5% | |
| `internal/platform/postgres` | 84.6% | |
| `internal/platform/profanity` | 90.0% | |
| `internal/platform/telemetry` | 89.1% | |
| `internal/platform/testenv` | 0.5% | helper package; `auth.go` 0% (exercised only from other packages) |
| `internal/recipe` | 85.2% | |
| `internal/recipeimport` | 20.0% | `Process`, `Retry`, `Shutdown`, `List`, `NewService`, `store.go` all 0% |
| `internal/userprefs` | 74.1% | `GetUserItemByUserAndItem` 0% (used by A4-01 path) |
| `internal/wine` | 86.1% | |
| **Total (filtered, hand-written only)** | **65.7%** | CI floor 60% |
| **Total (raw, incl. sqlc/mocks)** | **41.6%** | |

215 hand-written functions have 0% coverage; 69 of them are in `internal/bff/resolver_recipe_import.go`.

## Test inventory and methodology

- 52 test files, ~16.2k lines. Unit tests use `gomock` mocks of sqlc `Querier` (domain) and of the BFF service interfaces (`internal/bff/mock/services.go`, 2,877 generated lines from `internal/bff/generate.go`). Integration tests use Testcontainers Postgres 16 via `testenv.NewTestDB` and are gated by `testing.Short()`.
- Generated code (`internal/*/sqlc/`, `internal/*/sqlc/mock/`, `internal/bff/mock/`) was reviewed only for provenance and breadth; nothing in it is reported as a defect. Directives are consistent (`mockgen -source=services.go` for BFF; sqlc per domain).
- Per-function coverage was cross-referenced against Phase 1–3 findings to determine which known defects the suite could catch. Result: none of the high-severity findings from A2/A3 (A2-01, A2-02, A3-01, A3-02, A3-03) would be caught by the current suite.

## Findings

### A4-01 — Transactional grocery→pantry sync path is untested
- Category: bug (test gap) · Severity: high
- Location: `internal/bff/resolver_grocery.go:130-190`; `internal/bff/resolver_grocery_test.go` (all `Toggle*` tests construct `&Resolver{GroceryService: g}` with `Pool == nil`); `internal/bff/bff_integration_test.go:559-593` (toggles a *manual* item, so `it.ItemID == nil`).
- Description: `ToggleGroceryItemChecked` branches on `r.Pool != nil && ... && it.ItemID != nil`. Every unit test takes the fallback branch; the single integration toggle uses a manual grocery item and therefore also skips the transaction. Function coverage is 29.8%; `userprefs.GetUserItemByUserAndItem` (called only inside the transaction) is 0%.
- Why it matters: this is the code carrying the lost-update race (A2-01) and the non-atomic quantity math (A3-06). The suite gives green while the most complex mutation in the BFF has never executed under test.
- Remediation: add an integration case that creates a catalog item, adds a grocery item with `itemId`, toggles twice, and asserts pantry `currentQty` moves by `quantityNeeded` and returns to the original. Add a rollback case (force failure after the grocery update, assert nothing persisted). Once A2-01 is fixed, add a two-goroutine concurrent toggle test asserting the final quantity.

### A4-02 — Recipe-import pipeline is effectively untested
- Category: poor design (test gap) · Severity: high
- Location: `internal/recipeimport/service.go` (`Process:350`, `Retry:321`, `Shutdown:513`, `List:121`, `NewService:69`, `structuredDraft:457` all 0%); `internal/recipeimport/store.go` (0%, no integration test); `internal/bff/resolver_recipe_import.go` (0%, 69 functions); `internal/ocrimport/review.go` (`AllResolved`, `MergeExistingReview`, `BestSuggestion` 0%); `internal/ocrimport/workqueue.go` (0%).
- Description: the only tests are five service tests against an in-memory fake (A4-09). The SQL store, the GraphQL surface (`pendingRecipeImports`, `approveRecipeImport`, `rejectRecipeImport`, `retryRecipeImport`), the worker loop, OCR→LLM→profanity→draft pipeline, and the on-disk work queue have no tests at all.
- Why it matters: Phase 3 rated this domain worst (A3-02 non-atomic approve, A3-03 in-memory queue, A3-09 pagination, A3-10 `AllResolved`, A3-13 magic-string pipeline). None of those can regress-fail today. The 0%-covered resolver file is also admin-only mutation surface.
- Remediation: (1) integration test for `store.go` against Testcontainers covering every status transition and `ListPending` pagination; (2) resolver tests for the four admin mutations including `requireAdmin` denial; (3) unit tests for `Process` with fake OCR/LLM clients covering profanity, OCR failure, unparsable LLM output; (4) unit tests for `review.go` (`AllResolved` with fuzzy suggestions — the A3-10 case) and `workqueue.go` persistence round-trip.

### A4-03 — Cross-user tests codify silent success on wrong-user mutations
- Category: bug (tautological assertion) · Severity: high
- Location: `internal/grocery/integration_test.go:220-231`; `internal/mealplan/integration_test.go` (same pattern in `TestIntegrationMealPlanCrossUserDenied`).
- Description:
  ```go
  require.NoError(t, svc.UpdateGroceryListItem(ctx, gli.GroceryListItemID, userB, ...))
  ...
  require.NoError(t, svc.DeleteGroceryListItem(ctx, gli.GroceryListItemID, userB))
  ```
  The tests then verify the row is unchanged. The `NoError` requirement means the suite *asserts* that a wrong-user update/delete returns success — the exact behaviour Phase 3 flagged as a bug (A3-01: all UPDATE/DELETE are `:exec` and ignore row counts).
- Why it matters: fixing A3-01 (return `ErrNotFound` on zero rows) will break these tests, and reviewers may "fix the tests" back. Tests should encode intended behaviour, not current behaviour.
- Remediation: change these to `assert.ErrorIs(err, ErrNotFound)` (or `pgx.ErrNoRows` as the reads already do) at the same time A3-01 is fixed; keep the "row unchanged" assertions.

### A4-04 — End-to-end suite runs entirely as admin
- Category: security (test gap) · Severity: medium
- Location: `internal/bff/bff_integration_test.go:51` (`AdminEmails: {"auth@example.com", "user-a@example.com"}`), `:148-149` (`tokA` used for every mutation; `tokB` only for two read isolation checks).
- Description: `createBrand`, `createCategory`, `createItem`, wine reference mutations, etc. are admin-only. Because user A is in `AdminEmails`, there is no HTTP-level test that a normal user receives a forbidden error for any admin mutation. Resolver unit tests cover `requireAdmin` for inventory (14 cases), identity (3), recipe (2), wine (2), and nothing for recipe-import (A4-02).
- Why it matters: `requireAdmin` is the only authorization control (Phase 6 scope). Middleware/handler-level regressions (e.g. admin flag not propagated from `authenticate` into context) would go unnoticed because unit tests inject `currentuser.User{IsAdmin:true}` directly.
- Remediation: add a `tokB`-driven block that attempts one admin mutation per domain and asserts the GraphQL error code; add one case asserting the error is `forbidden`, not `unauthenticated`.

### A4-05 — `doGraphQL` masks transport and GraphQL errors
- Category: code smell (weak assertion) · Severity: medium
- Location: `internal/bff/bff_integration_test.go:684-711`.
- Description: a JSON decode failure is discarded (`io.Copy(io.Discard, ...)`) and returns a zero `graphqlResponse`; GraphQL `errors` are `t.Logf`'d, never failed. Callers then `decodeData` into a struct and check a few fields, so a response of `{"data":null,"errors":[...]}` only fails indirectly via `require.NotEmpty(ID)`, with the real cause buried in logs.
- Why it matters: partial-success responses (data plus errors — common in nested resolvers) pass silently; e.g. the `mealPlan { slots { items } }` query at `:515` would pass if `items` errored and returned `null` alongside `slots`.
- Remediation: make `doGraphQL` `require.NoError` on decode and `require.Empty(gr.Errors)` by default, with an explicit `expectErrors` variant for negative tests.

### A4-06 — OCR/scan upload paths have zero tests
- Category: security (test gap) · Severity: medium
- Location: `internal/bff/nutrition_ocr.go:20,79`; `internal/bff/recipe_scan.go:19,85,91,111,131`; `internal/platform/ocrclient/` (no `_test.go`).
- Description: `SubmitRecipeScan` parses a client-supplied data URI, derives a file extension from media type, hashes bytes, writes to the import inbox and enqueues processing; `SubmitItemNutritionPhoto` fans out to the async worker. All 0%. The `ocrclient` HTTP client has no tests (contrast `ollamaclient` at 73.5%).
- Why it matters: these are the input-handling paths Phase 6 must assess for path traversal, unbounded body size, and SSRF; without tests, hardening cannot be regression-guarded. A2-11 (success reported when async work dropped) is likewise untestable today.
- Remediation: table tests for `splitDataURI`/`extensionForMediaType` (malformed URI, unsupported media type, oversized payload, media type with parameters); `httptest`-backed `ocrclient` tests (non-200, timeout, malformed JSON); a resolver test with a fake OCR client and `t.TempDir()` inbox asserting file name is not client-controlled.

### A4-07 — Brand moderation flow is 0% covered
- Category: bug (test gap) · Severity: medium
- Location: `internal/inventory/service.go:79 SubmitBrand, :123 ListBrandsVisible, :137 SearchBrands, :154 ListPendingBrands, :167 CountPendingBrands, :177 SetBrandStatus, :194 GetBrandsByIDs`; `internal/bff/resolver_inventory.go:73 SearchBrands, :110 SubmitBrand, :124 PendingBrands, :145 ApproveBrand, :150 RejectBrand, :154 setBrandStatus, :175 FrequentBrands, :1014 brandWriteError`.
- Description: the entire user-submitted-brand workflow (submit → pending → approve/reject, visibility filtering, search) has no unit or integration test, while the analogous item moderation (`ApproveItem`/`RejectItem`) is at 100%.
- Why it matters: Phase 3 flagged the check-then-create race and lossy normalisation in `SubmitBrand` (A3-08); `ListBrandsVisible` decides what non-admins can see; `brandWriteError` maps unique violations (A2-08 inconsistency).
- Remediation: mirror the item-moderation tests for brands; add an integration test that submits a duplicate brand under different casing/whitespace and asserts the intended outcome; test `brandWriteError` mapping of `23505`.

### A4-08 — Double mock layer mirrors implementation; SQL semantics never observed by unit tests
- Category: antipattern · Severity: medium
- Location: `internal/*/service_test.go` (mock `sqlc.Querier`, e.g. `internal/inventory/service_test.go:668-678`), `internal/bff/resolver_*_test.go` (mock service interfaces; 353 `gomock.Any()` occurrences, 104 in `resolver_inventory_test.go` alone).
- Description: domain tests assert the exact `sqlc.*Params` struct passed to the querier and echo back canned rows; BFF tests mock the domain services. Both layers therefore test "the code calls the next layer with these arguments" — a restatement of the implementation. Behaviour that lives in SQL (ownership predicates, `RETURNING`, unique constraints, zero-row `:exec`) is only reachable via integration tests, which are thin per package (8 inventory, 2 grocery, 2 mealplan, 1 identity, 0 recipeimport).
- Why it matters: refactoring a query (e.g. splitting a param struct) breaks dozens of tests with no behaviour change, while genuine bugs (A3-01, A3-05, A3-07's cross-schema delete at `service.go:505-513`, tested only as two mocked calls) are invisible. High line coverage (86–89% in several domains) overstates confidence.
- Remediation: keep a thin set of gomock tests for error wrapping; move behavioural coverage to integration tests using a shared container (A4-12); in BFF tests prefer `gomock.Eq`/matchers on the fields the resolver is responsible for computing rather than `Any()`.

### A4-09 — `recipeimport` fake store diverges from real store; happy-path-only `Approve`
- Category: code smell · Severity: medium
- Location: `internal/recipeimport/service_test.go:18-149` (`memoryStore`), `:203-270`.
- Description: `memoryStore.Count` ignores the status filter; `UpdateOCR`/`UpdateDraft` set statuses the SQL store may not; there are no transition guards, so the fake cannot reveal A3-02. `TestService_Approve` writes `"ready"` directly into the store and only checks the success path; there is no test that approving a `pending`, `rejected`, or already `persisted` import fails, nor that a `CreateRecipeWithChildren` error leaves the import un-persisted. Tests build `&Service{store: ...}` literals, bypassing `NewService` and its config/worker wiring, and discard errors (`ri, _ := svc.Create(...)` at `:220,234,260`).
- Why it matters: the fake encodes assumptions about the store rather than testing them; with no store integration test (A4-02) nothing checks the two agree.
- Remediation: replace `memoryStore` with the real store over Testcontainers for transition tests, or contract-test both implementations with a shared table; add negative `Approve` cases and a writer-failure case; use `require.NoError` on setup calls.

### A4-10 — No concurrency tests for known race conditions
- Category: bug (test gap) · Severity: medium
- Location: repository-wide (0 `t.Parallel`, no goroutine-based tests outside `resolver_misc_test.go:190`).
- Description: Phases 2–3 identified read-modify-write races in grocery toggle (A2-01), pantry adjust (A3-06), last-admin guard (A3-05), and brand submission (A3-08). No test exercises any operation concurrently. CI's `-race` flag detects in-process memory races only, not database lost updates.
- Why it matters: the fixes for these findings (atomic SQL, `SELECT ... FOR UPDATE`, unique constraints) need tests that fail before and pass after; otherwise regressions are undetectable.
- Remediation: for each race, an integration test running N goroutines against the same row and asserting the invariant (final quantity, exactly one admin remains, one brand row).

### A4-11 — Timing-dependent async worker test
- Category: code smell (flaky pattern) · Severity: low
- Location: `internal/bff/resolver_misc_test.go:212` (`time.Sleep(50 * time.Millisecond)`), `:221-222`.
- Description: after saturating the worker the test sleeps 50 ms, submits `asyncWorkerCap` more tasks, and immediately asserts `dropped == 0`. On a slow or loaded runner the blocked goroutines may not have started, and the drop assertion is a non-event check taken before any task could have run even if queued.
- Why it matters: `-race` slows execution 5–10×; a false pass (or rare false fail) hides regressions in the drop policy (A2-11).
- Remediation: use a `sync.WaitGroup`/channel to confirm all `cap` tasks are running before submitting more; expose or observe the drop counter/log via a test hook and assert it equals the number of extra submissions.

### A4-12 — One container per test function; hard Docker Hub dependency
- Category: antipattern · Severity: low
- Location: `internal/platform/testenv/testenv.go` (`NewTestDB`), every `internal/*/integration_test.go` (each `TestIntegration*` calls `NewTestDB`).
- Description: 25+ integration functions each start a fresh `postgres:16-alpine`, run migrations, and tear down (`inventory` 118 s, `wine` 99 s, `recipe` 83 s). The image is pulled from Docker Hub unauthenticated; the first audit run failed 22 tests with `429 Too Many Requests`. There is no `TestMain`, shared container, image pre-pull, or mirror configuration in CI.
- Why it matters: slow feedback discourages adding the integration tests recommended above, and rate-limit failures present as red CI unrelated to code.
- Remediation: start one container per package in `TestMain` (or one per `go test` run via a Testcontainers reuse label) and isolate tests with per-test schemas or `TRUNCATE`; pin by digest and pull via a mirror (`mirror.gcr.io`) or GHCR copy in CI; consider `t.Parallel()` once shared.

### A4-13 — Single global coverage floor hides zero-coverage surface
- Category: poor design · Severity: low
- Location: `.github/workflows/test.yml` (`GO_COVERAGE_MIN: "60"`), `tools/coveragefilter/main.go`.
- Description: the gate is one repository-wide number (65.7% today, 61.7% when integration tests fail). Whole files at 0% (`resolver_recipe_import.go`, `recipe_scan.go`, `nutrition_ocr.go`, `ocrclient`, `recipeimport/store.go`) pass unnoticed because large well-tested domains lift the mean. The filter excludes generated code by path only, so a hand-written file placed under `mock/` would also be excluded.
- Why it matters: the metric cannot fall when new untested code is added to already-large packages; the 5.7-point margin is consumed by any Testcontainers failure.
- Remediation: enforce per-package minimums (e.g. `go tool cover -func` parsed per directory) and a "no 0% hand-written file" check; publish the HTML report as a CI artifact.

### A4-14 — Test reaches into `Authenticator` private state
- Category: code smell · Severity: low
- Location: `internal/bff/bff_integration_test.go:137-139`.
- Description: to test JWKS rotation the test locks `authenticator.mu` and rewrites `authenticator.jwks[issuer.URL].fetchedAt`.
- Why it matters: couples the test to cache internals (which Phase 2 recommended restructuring in A2-04); the refresh-interval logic itself is not tested — it is bypassed.
- Remediation: inject a clock (`now func() time.Time`) into `Authenticator` and advance it in the test; add a case asserting a rotation inside the min-refresh window is *rejected* (rate-limit behaviour).

### A4-15 — Server bootstrap tests are thin
- Category: poor design (test gap) · Severity: low
- Location: `cmd/lena/main_test.go` (`TestBuildCORSConfig`, `TestSplitAndTrim` only), `cmd/lena/main_integration_test.go:62-95`.
- Description: `main.go` is 7.5% covered by unit tests; the integration test checks `/health`, `/ready`, 401, and `me`. Graceful shutdown ordering (A2-03), rate-limiter middleware attachment and its config (`ratelimit.go` is unit-tested but not its wiring), body-size limits, and import-inbox directory handling are not exercised.
- Remediation: extend `TestIntegration` with a rate-limit burst assertion (429), an oversized body (413), and a shutdown test that verifies in-flight resolvers complete.

### A4-16 — Transaction binding untested outside grocery
- Category: code smell (test gap) · Severity: low
- Location: `internal/identity/service.go:50,59`, `internal/mealplan/service.go:34,41`, `internal/userprefs/service.go:36,43`, `internal/wine/service.go:32,39` (`WithTx`/`InTx` 0%); `internal/grocery/service_intx_test.go`.
- Description: only `grocery` has a `stubPool`/`stubTx` test proving writes go through the transaction. The BFF's cross-service composition (`g.WithTx(tx)` + `up.WithTx(tx)` inside `dbtx.InTx`) — and the A3-12 hazard of calling `InTx` on an already-bound service — is untested anywhere.
- Remediation: extract the `stubPool`/`stubTx` helpers into `internal/platform/dbtx/dbtxtest` and add one binding test per domain; add a test that `svc.WithTx(tx).InTx(...)` reuses `tx` (or errors) once A3-12 is resolved.

## Observations not raised as findings

- Positive: `internal/recipe/integration_test.go` includes genuine rollback tests (`CreateRecipeWithChildrenRollback`, `UpdateRecipeWithChildrenRollback`); `dbtx_test.go` covers begin/commit/rollback error paths; `auth_test.go` covers key rotation, unverified-email admin promotion, and rejection matrix; `ratelimit_test.go` covers per-user vs per-IP identity and `X-Forwarded-For`.
- `handler_limits_test.go` verifies max depth and query length (relevant to A2-10) but not complexity/cost or per-request deadline — consistent with the feature not existing.
- `internal/platform/validator` has no statements (interface-only) and reports `[no statements]`; harmless.
- Generated mocks (`internal/bff/mock/services.go`, `internal/*/sqlc/mock`) are regenerated from source and were not reviewed line-by-line; their breadth reflects A1-08 (large interfaces), not a test defect.
- CI runs the full (non-`-short`) suite on GitHub-hosted runners with Docker, so integration tests do execute in CI; the earlier premise that `.github/workflows/` is empty is stale and will be addressed in Phase 5.

## Suggested remediation order

1. A4-03 together with the A3-01 fix (change assertions to expect not-found).
2. A4-01 and A4-10 — integration + concurrency tests around grocery/pantry before touching the transaction code.
3. A4-02 / A4-09 — store integration tests and resolver tests for recipe import (highest-risk untested area).
4. A4-12 — shared container per package; this makes 1–3 cheap enough to sustain.
5. A4-04, A4-06, A4-07 — authorization and input-handling coverage ahead of Phase 6 fixes.
6. A4-05, A4-08, A4-13 — assertion hygiene and coverage gating.
