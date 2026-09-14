# Remediation Phase 7 — BFF robustness & DoS controls

- **Branch:** `audit-review-phase7` (cut from latest `main` after the Phase 6 PR is approved)
- **Theme:** BFF robustness & DoS controls — request deadlines and cost budgets, correct shutdown
  ordering, a non-blocking auth key cache, cached user resolution, honest async submission, elimination
  of remaining N+1 paths, and a structural clean-up of the `Resolver` god-object, service interfaces and
  stringly-typed GraphQL contract.
- **Source reports:** `audit/phase-2-bff.md`, `audit/phase-6-security.md`,
  `audit/phase-1-architecture.md`, `audit/phase-3-domains.md`, `audit/summary.md` (top-15 #9; theme
  "Platform hygiene")

## Findings in scope

| ID | Severity | One-line summary | Target file(s) |
|---|---|---|---|
| A2-03 | Medium | `Shutdown` cancels `bgCtx` before waiting and never waits on recipe-import drain; in-flight analytics/OCR work is aborted | `internal/bff/resolver.go:112-136`; `cmd/lena/main.go:117-129` |
| A2-04 | Medium | Global mutex held across OIDC discovery + JWKS fetch (and on cache hits); slow issuer stalls all auth; fetch uses request ctx; no stale-while-revalidate | `internal/bff/auth.go:195-224` |
| A2-05 | Medium | `UpsertUser` (and possibly `SetUserRole`) executed on every request; no caching of resolved user | `internal/bff/auth.go:159-180` |
| A2-07 | Medium | Single-object / nested paths (`groceryList`, `ingredient`, `scaledRecipe.items.item`, `recipeImport.recipe`) fall through to per-row lazy queries | `internal/bff/resolver_grocery.go:17-31,318-327,364-390`; `internal/bff/resolver_recipe.go:73-130`; `internal/bff/resolver_recipe_import.go:203-214` |
| A2-10 | Medium | No per-request deadline/cost limit around `Exec`; resolvers outlive disconnected clients; bind errors bypass GraphQL error shape | `cmd/lena/main.go:179-182,248-269`; `internal/bff/resolver.go:819-838` |
| A6-05 | Medium | No query cost limit or per-request deadline; depth/length limits do not bound cardinality; some lists unclamped | `cmd/lena/main.go:249-269`; `internal/platform/config/config.go:42-55` |
| A2-11 | Medium | `submitItemNutritionPhoto` returns `true` when `runAsync` drops the task; check-then-create nutrient types race; actor recorded as `"ocr-system"` | `internal/bff/nutrition_ocr.go:36-62,115-118`; `internal/bff/resolver.go:83-90` |
| A2-12 | Medium | 12 resolvers >60 lines; domain algorithms, manual PATCH merging and hand-rolled batching inline | `internal/bff/resolver_mealplan.go:103-243`; `resolver_grocery.go:112-227`; `resolver_wine.go:310-419`; `resolver_recipe.go:52-140`; `resolver_inventory.go:1128-1213,238-318` |
| A1-08 | Medium | Full-surface service interfaces (63/45 methods) instead of role interfaces; 2 877-line generated mock | `internal/bff/services.go:36-98,214-260` |
| A1-10 | Medium | `Resolver` is GraphQL root + DI container + background worker pool + config bag; 14-arg constructor | `internal/bff/resolver.go:38-71,73-136` |
| A1-11 | Medium | No GraphQL enums; roles/statuses/sources are `String!`; admin-only marked by comments | `internal/bff/schema.graphqls:153-154,173,196,221,266,346,498,792` |
| A3-21 | **Low** (roadmap said Medium) | Near-zero domain validation; relies on DB `CHECK`s that surface as `INTERNAL` | `internal/mealplan/service.go:134-152`; `grocery/service.go:121-149`; `userprefs/service.go:61-90,169-200`; `inventory/service.go:177-191,416-430` |
| A6-11 | **Low** (roadmap said Medium) | Member-triggered OCR jobs unbounded per user; can saturate worker pool and sidecar | `internal/bff/nutrition_ocr.go:116-119`; `internal/bff/resolver.go:95-110` |

**Roadmap corrections.** The roadmap listed **A3-21** and **A6-11** as Medium; both are **Low** in the
reports (`audit/phase-3-domains.md`, `audit/phase-6-security.md`, `docs/20260911-audit-findings.md`).
They stay in this phase because their code is edited here anyway (A6-11 shares `nutrition_ocr.go:115-119`
and the async runner with A2-11; A3-21 shares the domain-service methods that A2-12 pushes logic
into), so they are handled under the co-located-Low policy rather than as primary drivers. Neither
counts toward the 51-Medium reconciliation. All other IDs exist with the severities shown.

## Remediation steps

1. **A2-10 / A6-05 — deadline, cost budget and error shape.**
   1. Wrap `Exec` in `context.WithTimeout(ctx, cfg.GraphQLTimeout)` (new config field, e.g. 10 s) and
      return a GraphQL error with code `TIMEOUT` when it fires; also set `statement_timeout` on the pgx
      pool so abandoned queries die server-side.
   2. Add a cost/complexity budget: count resolved fields in `TraceField` (or pre-parse with
      `graphql-go/graphql/language`) and abort past `cfg.GraphQLMaxCost`; depth/length limits alone do
      not bound cardinality.
   3. Clamp every list resolver through one shared `pageArgs()` helper (audit each `resolver_*.go` for
      unclamped `limit`/`pageSize`).
   4. Return body-bind errors in GraphQL error shape with `BAD_USER_INPUT` (`main.go:248-269`).
2. **A2-03 — shutdown waits before cancelling.**
   1. In `resolver.go:112-136` run `bgWG.Wait()` in a goroutine, `select` on `done`/`ctx.Done()`, and call
      `bgCancel()` only in the timeout branch.
   2. Include the recipe-import worker shutdown in the same wait (or return its error) so `Shutdown`
      truly reports completion; verify `main.go:117-129` orders HTTP drain → resolver drain → pool close.
3. **A2-04 — non-blocking auth key cache.**
   1. Replace the global `sync.Mutex` in `auth.go:195-224` with `sync.RWMutex`/`atomic.Pointer` for
      reads and a per-issuer `singleflight.Group` for refreshes.
   2. Fetch with a detached context and its own timeout (not the request ctx); serve the stale key set
      while a refresh is in flight. `jwk.Cache` from lestrrat-go/jwx already implements most of this —
      prefer it. Wrap bodies in `io.LimitReader` and disable redirects (co-located Low A6-13).
4. **A2-05 — cache the resolved user.**
   1. Cache `currentuser.User` keyed by token hash (or `iss+sub`) for the token's remaining lifetime,
      bounded to a few minutes; refresh `last_login_at` at most once per interval.
   2. Alternatively move the upsert to a `/session` bootstrap mutation and treat the JWT alone as
      sufficient for reads. Keep the Phase 2 issuer-scoped bootstrap semantics intact.
5. **A2-11 / A6-11 — honest async submission with per-user bounds.**
   1. Make `runAsync` return `bool`/error; `submitItemNutritionPhoto` surfaces saturation as a GraphQL
      error (`code: "BUSY"`) — or, better, persist the job (status `queued`) and return its ID so the
      client can poll. Never return `true` for dropped work.
   2. Add a per-user in-flight cap (one OCR job per user), backpressure returning `PENDING`/`REJECTED`,
      and a dedicated lower rate limit for upload mutations (A6-11, co-located).
   3. The nutrient-type race and actor attribution halves of A2-11 were closed in Phase 2 (A6-01); if any
      `GetNutrientTypeByName` → create pair remains, replace it with an `INSERT … ON CONFLICT DO NOTHING
      RETURNING` `GetOrCreateNutrientType` restricted to admin callers.
6. **A2-07 — preloading is the only path.**
   1. Build `itemChildren`/`recipeChildren` in a shared helper used by single-object, list and
      mutation-return resolvers; delete the lazy branches at `resolver_grocery.go:17-31,318-327,364-390`,
      `resolver_recipe.go:73-130`, `resolver_recipe_import.go:203-214` and turn `if r.ch != nil` into a
      hard invariant (or fall back with a `WARN` log — co-located A2-22).
   2. Longer term (only if cheap here): request-scoped DataLoader (Low A1-17).
7. **A2-12 — shrink the long resolvers (≤ 40 lines each).**
   1. Push domain algorithms into services: `mealplan.Service.NutritionSummary` (done in Phase 6 if
      A2-06 landed), `grocery.Service.ToggleChecked`/`SetChecked` (Phase 4).
   2. Replace manual PATCH merging in `resolver_wine.go:310-419`, `resolver_inventory.go:1128-1213` and
      `resolver_recipe.go:250-310` with a `Patch` struct on the service (`inventory.ItemPatch{Name *string,
      …}`) or a generic `coalesce` helper; while there, validate at the domain boundary (enums, non-negative
      quantities) returning `ErrValidation` (A3-21, co-located).
   3. Collapse hand-rolled batching into one `preloadRecipeGraph(ctx, recipeIDs, extraItemIDs)` used by
      every recipe-returning resolver.
8. **A1-10 / A1-08 — decompose `Resolver` and the service interfaces.**
   1. Extract `bff.Services{…}` (struct of interfaces) and `bff.Options{…}` (limits, inbox path); extract
      the async runner into `platform/async` and reuse it for `recipeimport`'s worker pool.
      `NewResolver(services, options, runner)` replaces the 14-arg constructor.
   2. Split the full-surface interfaces in `services.go:36-98,214-260` into role interfaces
      (`ItemReader`, `CatalogAdmin`, `BottleReader`, …); embed them in `Services` and pass only the narrow
      interface into child resolvers; regenerate mocks (they shrink accordingly).
9. **A1-11 — typed GraphQL contract.**
   1. Introduce `enum Role`, `enum ApprovalStatus`, `enum ImportStatus`, `enum EntityType`,
      `enum GroceryItemSource`, `enum RecommendationReason` in `schema.graphqls`; map to the Go constants
      introduced in Phase 5 (`recipeimport.Status`) and elsewhere.
   2. Add a `directive @admin` (documentation/codegen only) so authorization intent is machine-readable.
   3. Coordinate with `clients/web` and `clients/mobile`: enums serialise as the same strings, so existing
      clients keep working, but regenerate any typed client code.

## Co-located Low policy

Fix a Low finding only if it lives in code already being changed for a High/Medium finding in this
phase; never edit code solely to fix a Low. Lows co-located here by construction: A3-21 and A6-11
(reclassified from the roadmap — see corrections above), A6-13 (JWKS body limits — same code as
A2-04), A2-22 (`unitName` map miss — same code as A2-07), A2-19 (span naming — only if
`graphql_tracer.go` is touched for the A2-10 cost counter), A2-21 (limiter design — only if
`ratelimit.go` is edited for the upload limit), A4-11 (timing-dependent async worker test — rewrite
while changing `runAsync`), A1-17 (DataLoader — only if adopted for A2-07), A2-14/A2-17/A2-18 (per-row
unit lookups, duplicated blend logic, non-clearable pointer inputs — only within resolvers already being
refactored for A2-12).

## Verification

- `go build ./...` passes (after regenerating mocks and GraphQL schema artefacts).
- `go test ./...` passes with `-race`.
- `golangci-lint run ./...` and `go vet ./...` report no issues; `clients/web` `npm run lint` and
  typecheck pass if the schema change touched generated client types.
- Manual: a deliberately slow resolver (or `pg_sleep` behind a debug flag) → request fails with
  `TIMEOUT` at the configured deadline and the DB statement is cancelled (`pg_stat_activity` shows none
  after the deadline).
- Manual: a deeply-nested, wide query exceeding the cost budget is rejected before execution.
- Manual: send `SIGTERM` while an OCR job is in flight → the job completes (or hits its own stage timeout)
  before the process exits; no "context canceled" from the background worker in logs.
- Manual: point one issuer's JWKS URL at a slow endpoint → requests for the other issuer are not
  delayed; after the slow issuer recovers, stale keys were served in the meantime.
- Manual: 100 sequential authenticated requests produce ≤ 1 `UpsertUser` write (check
  `identity.users.updated_at`/query log).
- Manual: saturate the async pool (submit > `cap` OCR photos) → the excess return `BUSY`/`PENDING`, never
  `true`; the same user cannot have more than one in-flight job.
- Manual: `groceryList(id)`, `ingredient(id)`, `recipeImport(id).recipe` each issue a bounded number of
  SQL statements (check with `log_statement=all` or the OTel trace) regardless of child count.

## Closing instruction

Open a PR from `audit-review-phase7` into `main` summarising the changes above, then **stop**. Do not
begin Phase 8 until this PR has been reviewed and approved.
