Repository: `JRAdams472/LENA2`. This plan fixes the bugs, security holes, code smells, and architectural/testing gaps identified in the code review. Work through the issues below. Each item names the file(s) and the intended outcome. Explore the relevant domain packages under `internal/` to find the exact code, since the service and sqlc layers follow a consistent per-domain pattern.

## 1. Fix pagination `total` (return real row counts)
Problem: 7 resolvers in `internal/bff/resolver.go` set `total: int32(len(items))`, which is the current page size, not the total row count. Affected resolvers: `Items`, `Recipes`, `Bottles`, `MealPlans`, `GroceryLists`, `UserBottles`, `UserItems`.

- For each affected domain (`internal/inventory`, `internal/recipe`, `internal/wine`, `internal/mealplan`, `internal/grocery`), add a `COUNT(*)` SQL query in the domain's `sqlc` queries (`.sql` files), regenerate sqlc (or hand-write the query method following the existing generated style), and expose a `Count...(ctx, <same filters>)` method on the domain service that returns `int64`.
- Update each resolver to call the new count method and pass the true total into the `...PageResolver` instead of `int32(len(items))`.
- Ensure filter/scope arguments (e.g. user-scoped `UserBottles`/`UserItems`) are applied identically in the count query and the list query so the count matches the filtered result set.

## 2. Fix "zero means null" helpers
Problem: `int32Ptr`, `float64OrNil`, and `int16OrNil` in `internal/bff/resolver.go` (lines ~74-111) treat `0` as absence and return `nil`, dropping legitimate zero values.

- Drive nullability from the actual DB null state (the underlying `pgtype`/sql null flags carried up from the service layer) rather than value equality.
- Where the service currently flattens nullable columns to plain `float64`/`int` with a zero default, change those to carry an explicit "valid/present" flag (e.g. return the `pgtype` valid bool or a `*float64`/`*int32`) so the resolver can distinguish `0` from unset.
- Update the callers accordingly and remove the value-equality checks. Keep helper behavior only where a value is genuinely optional and zero is truly not meaningful (verify per field).

## 3. Make multi-write mutations transactional
Problem: `CreateRecipe` and `UpdateRecipe` in `internal/bff/resolver.go` (lines ~843-893 and the create equivalent) perform many sequential writes with no enclosing transaction, risking partial/corrupt state.

- Each domain's `internal/<domain>/sqlc/db.go` already provides a `WithTx(tx)` method on the `Queries` struct. Add a transactional path to the `recipe` service (`internal/recipe/service.go`): e.g. a method that begins a `pgx` transaction on the pool, wraps `Queries` via `WithTx`, performs all recipe+items+steps writes, and commits (rolling back on any error).
- Refactor `CreateRecipe`/`UpdateRecipe` orchestration so the full sequence (recipe row, delete existing items/steps, insert new items/steps) runs inside a single service-level transaction rather than as independent BFF calls.
- Audit other multi-write resolvers (e.g. meal plan slot replacement, grocery list item replacement) for the same issue and apply the same transactional pattern where multiple writes must be atomic.

## 4. Stop silently swallowing numeric conversion errors
Problem: in `internal/inventory/service.go`, `numericFromFloat64` (lines ~499-506) returns an empty invalid `pgtype.Numeric` on `Scan` error, and `numericToFloat64`/`toFoodNutrient` (lines ~478-489) return `0` on error.

- Change `numericFromFloat64` to return `(pgtype.Numeric, error)` and propagate the error to callers instead of discarding it.
- Change the read-side helpers to propagate conversion errors (or at minimum log them) rather than silently substituting `0`, so malformed amounts don't become silent zeros in nutrition totals.
- Update all callers to handle the returned error.

## 5. Fix inconsistent user-context handling in `UpdateItem`
Problem: `UpdateItem` in `internal/bff/resolver.go` (lines ~773-781) checks auth via `userFromContext` at the top, then re-fetches with `u, _ := currentuser.FromContext(ctx)`, ignoring the second error.

- Capture the user from the initial `userFromContext` check and reuse it (matching every other resolver), removing the redundant second lookup and the ignored error.

## 6. Fix CORS wildcard-with-credentials
Problem: in `cmd/lena/main.go` (lines ~91-101), when `CORS_ALLOWED_ORIGINS` is `*`, `AllowOriginFunc` reflects any origin while `AllowCredentials` stays `true` — a credential-leak/CSRF vector.

- When origins are wildcarded, set `AllowCredentials: false` (do not reflect arbitrary origins with credentials). Alternatively, reject `*` outright and require an explicit origin allowlist.
- Keep `AllowCredentials: true` only when an explicit origin list is configured.

## 7. Add authorization to global catalog mutations
Problem: catalog-mutating resolvers (`CreateBrand`, `CreateItem`, `UpdateItem`, `DeleteItem`, `CreateCategory`, `CreateNutrientType`, etc.) in `internal/bff/resolver.go` only check authentication, not authorization, so any authenticated user can mutate shared global data.

- Introduce a role/permission concept. Add a `Roles` (or `IsAdmin`) field to `currentuser.User` (`internal/bff/auth.go` around lines 120-126) populated from the identity flow / token claims.
- Add a helper (e.g. `requireAdmin(ctx)`) and apply it to all global-catalog mutation resolvers.
- If roles cannot be sourced from the identity provider yet, add a persisted role on the user record in `internal/identity` and populate `currentuser.User` from it. Document the chosen mechanism in code comments.

## 8. Scope the startup DB context correctly
Problem: in `cmd/lena/main.go` (lines ~40-48) the 10s timeout context for `NewPool` is deferred-cancelled at the end of `main`, keeping it alive for the whole process.

- Scope the timeout context to just the `postgres.NewPool` call (create it, defer-cancel in a narrow block or cancel immediately after pool creation) so it does not linger for the process lifetime.

## 9. Address N+1 fan-out in `Nutrition`
Problem: `Nutrition` in `internal/bff/resolver.go` (lines ~394-438) issues per-slot, per-item, and per-recipe queries with no batching.

- Add batch-fetch methods to the relevant services (e.g. fetch food nutrients for a set of item IDs in one query, fetch recipes/recipe-items for a set of recipe IDs in one query) and refactor `Nutrition` to collect the needed IDs first, then batch-load, then compute totals in memory.
- Keep cross-domain orchestration in the BFF (no cross-domain SQL joins), consistent with the existing architecture.

## 10. Harden JWKS caching against key rotation
Problem: `keySetForIssuer` in `internal/bff/auth.go` (lines ~131-154) caches for one hour and does not handle key rotation mid-window.

- On signature-verification failure, force a JWKS re-fetch (cache bust) and retry once, or replace the manual cache with the jwx auto-refreshing key set (`jwk.Cache`/`NewCache`). Ensure concurrency safety is preserved.

## 11. Remove or wire up `GoogleClientID`
Problem: `GoogleClientID` in `internal/platform/config/config.go` (lines ~14-16) is `required:"true"` but is only referenced in config and its test — it is not consumed by application logic.

- If it is genuinely unused, remove the field (and its reference in `internal/platform/config/config_test.go`), or at minimum drop `required:"true"` so it doesn't block startup.
- If it is intended for token audience validation, wire it into the auth audience/validation flow instead.

## 12. Add missing test coverage
Problem: domain service packages under `internal/` (e.g. `internal/inventory`, `internal/recipe`) lack their own unit tests; the pagination bug indicates paths aren't asserted against realistic data volumes.

- Add unit tests for the new/changed service methods: `Count...` methods (assert total ≠ page size when rows > page size), the transactional recipe create/update (assert rollback leaves prior state intact on mid-sequence failure), and the numeric conversion helpers (assert errors are propagated, not swallowed).
- Add BFF resolver tests asserting `total` reflects the full row count across multiple pages for each affected paginated resolver.
- Add a CORS regression test asserting credentials are not allowed when origins are wildcarded, and authorization tests asserting non-admin users are rejected from catalog mutations.

## Verification
- Run `go build ./...` and `go test ./...` at the repo root; regenerate sqlc if queries were added.
- Ensure all new SQL queries are reflected in the generated `sqlc` code and that existing tests still pass.