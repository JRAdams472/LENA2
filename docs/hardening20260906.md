# Hardening — 2026-09-06

Code audit findings and the concrete remediation plan (phase-24 and follow-on audit-remediation phases).

> **Scope note:** Mobile client findings (anything under `clients/mobile`) are out of scope for these remediation phases. The mobile app is scheduled for a near-future rewrite with new specs and will be addressed separately; ignore those findings when planning the hardening phases below.

## Findings

### Bugs / correctness

Unvalidated rating with silent integer truncation. RateRecipe casts the incoming GraphQL int32 straight to int16 with no range check, even though the doc comment claims a "1-5 star rating." A client can send 0, -1, or 40000 and it is persisted (and truncated on the int16 cast). resolver_recipe.go:347 The generic int16Ptr helper has the same silent narrowing problem. resolver.go:137-143

unitName silently returns an empty string on a cache miss. When a preloaded units map is present but the requested unitID isn't in it, the function returns "", nil instead of an error. A missing batch-load entry becomes an empty unit name in the response rather than a detected failure. resolver.go:291-296

os.Exit skips deferred cleanup. defer pool.Close() and the telemetry shutdown defer never run when the server goroutine calls os.Exit(1) on a start error (or any of the earlier os.Exit paths). Connections and trace flushing are abandoned. main.go:74-77 main.go:56-67

Incomplete struct mapping in inventory create paths. CreateFoodNutrient returns a FoodNutrient with ItemID and Name left zero-valued, and CreateFoodFlavor similarly omits ItemID/Name. Callers relying on the returned value get partially-populated objects. service.go:472-475 service.go:505-508

numericToFloat64 conflates NULL with 0. An invalid/NULL numeric returns 0, nil, so a null amount and a real zero are indistinguishable downstream. service.go:844-847

### Security-relevant

Ignored error on audience extraction. token.Audience() returns an error that is discarded before the allowlist check. If extraction ever fails this proceeds with an empty audience slice. auth.go:118-121 The email/name claim reads similarly discard errors. auth.go:128-130

/metrics is registered without the authenticator. Only /graphql is wrapped in authenticator.Middleware(); /metrics is exposed unauthenticated, which may leak operational data depending on deployment. main.go:160-170

### Code smells / maintainability

Reinvented standard-library helpers. contains / containsAny duplicate slices.Contains, and slices is already used elsewhere in the package. SonarQube-style linters flag hand-rolled reimplementations. auth.go:232-248

Fire-and-forget goroutines with no bound or lifecycle. recordEventAsync and computeOverlapAsync spawn detached goroutines on context.Background(); there is no concurrency limit and no wait on shutdown, so in-flight analytics work is silently dropped when the process exits. The computeOverlapAsync comment itself acknowledges this should move to a job queue. resolver.go:73-87 resolver.go:93-109

Effectively-dead rate-limiter fallback branch. The limiter runs after the authenticator, so the ip: fallback for "no user in context" is unreachable in the wired-up path (unauthenticated requests are already 401'd). It is either dead code or a sign the middleware ordering intent isn't matched by the code. ratelimit.go:30-35

Nondeterministic ID lists in ScaledRecipe. loadItemChildren deliberately sorts ID slices before querying for deterministic SQL, but the hand-rolled batch loading inside ScaledRecipe builds itemIDs/unitIDs from map iteration without sorting, contradicting that convention. resolver_recipe.go:106-120 resolver.go:366-372

Inconsistent nil-telemetry handling. newServer guards with if tel != nil, but main treats a Setup error as fatal and otherwise assumes non-nil, so the two files disagree on whether tel can be nil. main.go:58-62 main.go:160-163

Startup panic in the handler constructor. NewGraphQLHandler panics on a schema-parse error rather than returning it, coupling schema validity to a panic path. resolver.go:690-693

## Resolution Plan (phase-24 → executed in phases 25–26)

Fix all findings — input-range validation at the GraphQL boundary, an authenticated `/metrics` endpoint, a bounded shutdown-aware async worker, deterministic ID ordering, and lifecycle/constructor cleanups — verified by `go build ./...`, `go test ./...` (short mode), `golangci-lint`, and `sqlc generate` for touched queries.

**Execution note (2026-09-07):** `phase-24` merged as the audit register only; the fixes below landed later. `phase-25` shipped the auth/authorization items (LENA-001/002/003/011/029 plus cross-user integration tests) and step 7 below. `phase-26` implemented the rest: steps 2–6, 8–14. `phase-27` then shipped the two remaining Criticals: LENA-004 (HTTP read/write/idle timeouts on `e.Server` plus a `BodyLimit` middleware on `/graphql`, configurable via `LENA_HTTP_*` / `LENA_GRAPHQL_BODY_LIMIT`) and LENA-006 (`sanitizeQueryErrors` in `internal/bff/errors.go` masks internal resolver errors behind "internal server error" + `extensions.code`, marks client-safe errors UNAUTHENTICATED/FORBIDDEN/BAD_USER_INPUT/NOT_FOUND, and logs the original with the request id). `phase-28` tackles the remaining Major findings that are still in scope for the Go backend: LENA-013 (analytics upserts in a transaction), LENA-018 (grocery FK `ON DELETE SET NULL`), LENA-019 (wine `oak_integration` nullable end-to-end), LENA-020/021 (single-row userprefs lookups and SQL-driven recency scoring), and LENA-022 (remove full GraphQL query text from trace spans).

### Context verified during exploration

- `recipe.Service.SetRating` **does** validate 1–5 (`internal/recipe/service.go:414`), but `resolver_recipe.go:347` casts `int32 → int16` *before* the check, so values like `65537` wrap to `1` and pass. Same wrap-around applies to `int16Ptr` (wine acidity/tannin/body/sweetness/percentage — **no DB CHECK constraints** on those columns, `migrations/0004_create_wine.up.sql`) and `int16(args.Input.Intensity)` for food/wine flavors (DB has `CHECK 1–5`, so those currently fail late at the DB instead of at the boundary).
- `inventory.food_nutrient.amount` is `NUMERIC(10,4) NOT NULL` (`migrations/0003_create_inventory.up.sql:64`), so a `!Valid` numeric is genuinely an error condition, not a real zero.
- `CreateFoodNutrient`/`CreateFoodFlavor` use `RETURNING *`, which cannot produce `Name` (it comes from a join to `nutrient_type`/`flavor_profile`). Fix = rewrite the queries as `WITH ins AS (INSERT ... RETURNING *) SELECT ... JOIN` so one round-trip returns the joined name, then `sqlc generate` (sqlc is installed at `C:\Users\aipal\go\bin\sqlc.exe`).
- `newServer` is called with `tel = nil` in `cmd/lena/main_integration_test.go:42` — the nil-guard is load-bearing for tests; the fix is making the contract explicit and consistent, not removing the check.
- `NewGraphQLHandler` call sites: `cmd/lena/main.go:166`, `internal/bff/bff_integration_test.go:65`, `internal/bff/handler_limits_test.go:32,40,51`.
- Lint config `.golangci.yml` enables errcheck, govet, gosec (G104/G115 excluded), staticcheck, etc.

### Decisions (confirmed)

1. `/metrics` → wrap with `authenticator.Middleware()` (no scraper access wanted).
2. Async work → bounded + shutdown-aware worker (no new infra; not a job-queue refactor).
3. Wine int16 fields → resolver int16-range check + domain bounds in the wine service (acidity/tannin/body/sweetness 1–5, percentage 0–100).
4. Branch → `phase-24` off `main`.

### Implementation steps

#### 1. Branch & setup
- `git checkout -b phase-24` from `main`. Do not commit to `main` (AGENTS.md).

#### 2. Input validation — silent int16 truncation
`internal/bff/resolver.go`:
- Replace `int16Ptr(v *int32) *int16` with a validating pair:
  - `checkedInt16(v int32, field string, min, max int16) (int16, error)` — rejects `v < min || v > max` (covers wrap-around since min/max are int16-representable).
  - `checkedInt16Ptr(v *int32, field string, min, max int16) (*int16, error)` — nil-preserving wrapper.
- In `RateRecipe` (`resolver_recipe.go:347`): validate `args.Rating` via `checkedInt16(v, "rating", 1, 5)` before calling `SetRating`.
- `resolver_inventory.go:719` (`AddFoodFlavor`) and the wine `AddBottleFlavorProfile` call site: validate intensity 1–5.
- `resolver_wine.go:280-283, 348-360, 407`: route all wine `int16Ptr` uses through `checkedInt16Ptr` — acidity/tannin/body/sweetness 1–5, grape percentage 0–100.
- Defense in depth in `internal/wine/service.go`: validate the same bounds in `CreateBottle`/`UpdateBottle` (1–5 when non-nil) and `AddBottleGrapeVariety` (0–100 when non-nil), matching the existing `SetRating` style (`fmt.Errorf("... must be between ...")`).
- Update/extend unit tests in `resolver_recipe_test.go`, `resolver_wine_test.go`, `service_test.go` for boundary and wrap-around cases (e.g. rating 0, 6, 65537, -1).

#### 3. `unitName` cache-miss silent empty string
`internal/bff/resolver.go:290-302`:
- When `units != nil` and `unitID` is absent, return `fmt.Errorf("unit %d missing from preloaded set", unitID)` instead of `"", nil`. The lazy path (nil map) is unchanged.
- This surfaces through `unitNamePtr` (grocery/mealplan paths) automatically.
- Add/adjust a test asserting the miss produces an error.

#### 4. `os.Exit` skipping deferred cleanup
`cmd/lena/main.go`:
- Refactor `main` into `func main() { os.Exit(run()) }` + `func run() int`. All defers (`pool.Close`, telemetry `Shutdown`) live inside `run` so they execute on every exit path.
- Replace the `os.Exit(1)` inside the server goroutine with an error sent on a channel; `run` `select`s on `sig` vs `serverErr` and returns 1 on server error after graceful cleanup.
- Because `newServer` must now return errors (item 10) and the resolver for shutdown drain (item 6), change signature to `newServer(cfg, pool, log, tel) (*echo.Echo, *bff.Resolver, error)`; update `main_integration_test.go:42`.

#### 5. Incomplete struct mapping in inventory creates
`internal/inventory/queries.sql` + `service.go`:
- Rewrite `CreateFoodNutrient` and `CreateFoodFlavor` as CTEs that join the reference table so `food_id`, name, amount/intensity come back in one round-trip:
  ```sql
  -- name: CreateFoodNutrient :one
  WITH ins AS (
    INSERT INTO inventory.food_nutrient (food_id, nutrient_id, amount, created_by)
    VALUES ($1, $2, $3, $4)
    RETURNING food_id, nutrient_id, amount
  )
  SELECT ins.food_id, nt.nutrient_id, nt.name, nt.unit, ins.amount
  FROM ins JOIN inventory.nutrient_type nt ON ins.nutrient_id = nt.nutrient_id;
  ```
  (analogous for `CreateFoodFlavor` joining `flavor_profile`.)
- Run `sqlc generate`; update `service.go` mappings to populate `ItemID`/`Name`/`Unit` fully; update `internal/inventory/sqlc/mock` mocks if regenerated; adjust `service_test.go`/`resolver_inventory_test.go` expectations and the integration test at `integration_test.go:294-310`.

#### 6. `numericToFloat64` NULL vs 0
`internal/inventory/service.go:844-853`:
- Treat `!n.Valid` as an error: `return 0, fmt.Errorf("convert numeric to float64: value is NULL")`. Column is `NOT NULL`, so this only fires on genuine data corruption — never conflates with 0.
- Update `service_test.go:860` ("returns zero for invalid numeric") to expect an error.

#### 7. Auth ignored errors
`internal/bff/auth.go:118-130`:
- `audience, err := token.Audience()` — return `fmt.Errorf("token audience: %w", err)` on error (aud claim extraction failure rejects the token rather than proceeding with an empty slice).
- `token.Get("email", &email)` / `token.Get("name", &name)` — check errors; on type-mismatch/decode failure return an error. (Missing claims still decode to empty string with nil error — UpsertUser tolerates empty email; no behavior change there beyond surfacing real errors.)
- Add `auth_test.go` cases: token with malformed `aud`, wrong-typed `email` claim → 401.

#### 8. `/metrics` unauthenticated
`cmd/lena/main.go:160-163`:
- Register `e.GET("/metrics", echo.WrapHandler(tel.MetricsHandler()), authenticator.Middleware())` — same auth as `/graphql` (rate limiter not needed; auth middleware only).
- Add an integration assertion in `main_integration_test.go` that `/metrics` without a token returns 401 when telemetry is wired — since tests pass `tel=nil`, add the check either via a test that constructs telemetry or a unit test asserting the route registration logic.

#### 9. `contains`/`containsAny` reinvention
`internal/bff/auth.go:232-248`:
- Delete `contains`; replace call sites with `slices.Contains` (add `"slices"` import).
- Rewrite `containsAny` as a loop over `values` calling `slices.Contains(allowed, v)` (or `slices.ContainsFunc` intersect) — or keep the tiny helper name but implement via `slices.Contains`.

#### 10. Fire-and-forget goroutines → bounded, shutdown-aware worker
`internal/bff/resolver.go` + `cmd/lena/main.go`:
- Add to `Resolver` an internal worker: `bg struct{ ctx context.Context; cancel context.CancelFunc; sem chan struct{}; wg sync.WaitGroup }` initialized in `NewResolver` with a bounded `sem` (e.g. capacity 16).
- `recordEventAsync`/`computeOverlapAsync` become methods `(r *Resolver)` that acquire `sem` (drop + log if full — analytics must never block the request path), run under `context.WithTimeout(r.bg.ctx, …)` instead of `context.Background()`, and `wg.Done` on exit.
- Add `func (r *Resolver) Shutdown(ctx context.Context) error` — `cancel()` then wait on `wg` or ctx timeout; log abandoned count.
- Call sites updated to `r.recordEventAsync(...)` (`resolver_recipe.go:231,236,350`, `resolver_mealplan.go:364`).
- In `main` `run()`: after `e.Shutdown(shutdownCtx)`, call `resolver.Shutdown(drainCtx)` so in-flight analytics complete before `pool.Close`/telemetry shutdown. `newServer` returns the resolver (item 4).
- Unit test: N+limit submissions never exceed the bound; Shutdown drains.

#### 11. Dead rate-limiter fallback
`internal/bff/ratelimit.go:30-35`:
- Keep the `ip:` fallback as defense-in-depth (ordering could change; the middleware is also reusable standalone), but fix the doc comment to say the fallback is a safety net for misordered wiring, and add a test in `ratelimit_test.go` asserting the identifier is `user:<id>` when a user is present (guards the intended ordering).

#### 12. Nondeterministic ID lists in `ScaledRecipe`
`internal/bff/resolver_recipe.go:106-127`:
- `slices.Sort(itemIDs)` and `slices.Sort(unitIDs)` after building from the sets (slices already imported in that file), matching the `distinctIDs`/`loadItemChildren` convention.

#### 13. Nil-telemetry contract
- Keep `newServer`'s `if tel != nil` (required by `main_integration_test.go` which passes `nil`).
- Make the contract explicit: comment on `telemetry.Setup` that it never returns `(nil, nil)`; comment in `newServer` that nil telemetry is supported for tests and disables `/metrics` + HTTP metrics middleware. No behavioral change.

#### 14. `NewGraphQLHandler` panic
`internal/bff/resolver.go:688-693`:
- Change signature to `NewGraphQLHandler(r *Resolver, schemaOpts ...graphql.SchemaOpt) (echo.HandlerFunc, error)`; return `fmt.Errorf("parse graphql schema: %w", err)` instead of `panic`.
- Update call sites: `main.go` (inside `newServer`, now error-returning), `bff_integration_test.go:65`, `handler_limits_test.go:32,40,51` (use `require.NoError`).

### Files to modify

- `cmd/lena/main.go` — `run()` refactor, error-returning `newServer` returning resolver, `/metrics` auth, shutdown drain order.
- `cmd/lena/main_integration_test.go` — `newServer` signature; `/metrics` 401 assertion if feasible.
- `internal/bff/resolver.go` — int16 helpers, `unitName` miss error, async worker + `Resolver.Shutdown`, `NewGraphQLHandler` error return.
- `internal/bff/resolver_recipe.go` — rating validation, sorted ID slices, worker method calls.
- `internal/bff/resolver_inventory.go` — intensity validation.
- `internal/bff/resolver_wine.go` — checked int16 fields.
- `internal/bff/resolver_mealplan.go` — worker method call.
- `internal/bff/auth.go` — audience/claim error handling, `slices.Contains`.
- `internal/bff/ratelimit.go` — comment fix; test addition.
- `internal/wine/service.go` — domain bounds for acidity/tannin/body/sweetness (1–5), percentage (0–100).
- `internal/inventory/service.go` — `numericToFloat64` NULL error, full mapping for create paths.
- `internal/inventory/queries.sql` + regenerated `sqlc` package + `sqlc/mock` — CTE create queries.
- Tests: `auth_test.go`, `ratelimit_test.go`, `resolver_recipe_test.go`, `resolver_wine_test.go`, `resolver_inventory_test.go`, `inventory/service_test.go`, `wine/service_test.go`, `handler_limits_test.go`, `bff_integration_test.go`.
- `docs/hardening20260906.md` — resolution notes (this file).

### Verification

- [ ] `go build ./...`
- [ ] `sqlc generate` produces clean diff; `go vet ./...` clean
- [ ] `go test ./... -short` (unit/mocks) green
- [ ] `go test ./...` integration tests green if testenv DB available (docker-compose)
- [ ] `golangci-lint run` clean (errcheck will catch any new ignored errors)
- [ ] Manual: `rateRecipe` with `rating: 65537` → GraphQL error; `/metrics` without token → 401; SIGTERM log shows worker drain before pool close.
- [ ] New unit tests: rating bounds incl. wrap-around, int16Ptr bounds, unitName miss, NULL numeric error, auth audience error, worker bound+drain, handler constructor error propagation.

### Risks / considerations

- **sqlc regeneration**: CTE + INSERT in `:one` queries is supported by sqlc, but the generated row type name changes — mocks in `internal/inventory/sqlc/mock` regenerate too. Verify `sqlc.yaml` config before running; if regen is problematic, fallback is a second `SELECT` in the service method (still one extra round-trip only on create).
- **Domain bounds for wine fields (1–5 / 0–100)**: assumed from the 1–5 intensity convention elsewhere; wrong bounds would reject previously-storable data. No DB CHECKs exist, so service validation is the only enforcement.
- **Metrics auth**: Prometheus scrapers will now need a bearer token from a trusted issuer — deployment docs may need a note that scraping requires a service account token.
- **Behavior change**: `rateRecipe` with out-of-range rating now returns a GraphQL error instead of a DB/validation error later — same observable outcome for 0/6, but wrap-around values now reject instead of silently persisting.
- **Worker drop semantics**: analytics events are best-effort by design; a full semaphore drops + logs rather than blocking — confirmed acceptable since analytics already tolerates loss.

---

## Phase 29 — Infrastructure, CI/CD and Docker hardening

Scope: the remaining in-scope Major findings that are not the mobile rewrite and not performance work.

### Issues to remediate

1. **LENA-023**: gate `latest` Docker image pushes on successful test results.
2. **LENA-024**: consolidate the two overlapping CI workflows, pin Go version from `go.mod`, and pin `golangci-lint`.
3. **LENA-025**: harden `docker-compose.yml` — remove host port mappings for DB/API/Seq, require `POSTGRES_PASSWORD`, pin image tags, tighten CORS/SSL defaults.
4. **LENA-026**: harden `clients/web/Dockerfile` — run as `USER node`, use `npm ci` only, pin base image tags/digests.

### Verification

- `docker compose config` is valid.
- `go build ./...` and `go test -short ./...` still pass (Go code is not touched, but workflows must invoke the same commands).
- Web image builds (`docker build clients/web`) and the web Dockerfile lint passes.

## Phase 30 — Remaining Major findings (`phase-30`)

Scope: the last two in-scope Major findings. LENA-027/028 (mobile client) remain out of scope per the scope note at the top — the mobile app is scheduled for a rewrite. After this phase every non-mobile Major and Critical finding in `docs/code-audit.md` is remediated; only Minor/Info findings and the performance/observability work below remain.

**Status (2026-09-07):** implemented on `phase-30` — verified by `go build ./...`, `go vet ./...`, `go test -short` (bff/platform/cmd green), web `jest` (293 tests) + `eslint` + `tsc --noEmit` clean. `golangci-lint` is not installed locally; CI runs it.

### Issues to remediate

1. **LENA-007** — Rate limiter ordering and client-IP extraction:
   - Add an IP-keyed limiter (`bff.IPRateLimiter`) that runs **before** `authenticator.Middleware()` on `/graphql`, so unauthenticated floods (incl. the JWKS-refresh amplification in LENA-003) are throttled. The existing user-keyed `GraphQLRateLimiter` stays after auth.
   - New config knobs `LENA_IP_RATE_LIMIT_PER_MINUTE` (default 300) / `LENA_IP_RATE_LIMIT_BURST` (default 60) — looser than the per-user limit since an IP can front multiple users (NAT).
   - Set `e.IPExtractor = echo.ExtractIPFromXFFHeader()` so `c.RealIP()` honours `X-Forwarded-For` from Caddy. Echo's default trust set (loopback + private nets) matches the compose deployment where the only reachable peer is the Caddy container on the docker network; port 8080 is no longer published to the host (LENA-025).
   - Middleware order on `/graphql`: `BodyLimit` → `IPRateLimiter` → `authenticator` → `GraphQLRateLimiter`.
2. **LENA-017** — Google ID token in `localStorage` + no renewal:
   - Move the token to `sessionStorage` (per-tab, cleared on tab close — the "at minimum" option in the audit hint). On read, migrate any legacy `localStorage["lena_id_token"]` value into `sessionStorage` and delete the localStorage copy so existing sessions are cleaned up.
   - Add silent re-auth: a `SilentReAuth` component inside `GoogleOAuthProvider` calls `useGoogleOneTapLogin` with `auto_select: true` and `disabled: isAuthenticated`. When the token expires or a 401 signs the user out, `disabled` flips and GIS re-prompts; users with an active Google session get a fresh ID token without interaction.
   - `api.ts` default token getter reads `sessionStorage`.
   - `e2e/auth.setup.ts`: `context.storageState` does not persist `sessionStorage`, so the saved auth file keeps seeding `localStorage` (the app migrates it on first load). Construct the storage-state JSON explicitly so the token is present in the saved file even though the app deletes it from the live page.

### Files to modify

- `internal/platform/config/config.go` — `IPRateLimitPerMinute`/`IPRateLimitBurst`.
- `internal/bff/ratelimit.go` — `IPRateLimiter`, shared store constructor.
- `cmd/lena/main.go` — `e.IPExtractor`, middleware order.
- `internal/bff/ratelimit_test.go` — per-IP limiting, IP independence, disabled.
- `.env.example` — document the new knobs.
- `clients/web/app/auth/AuthProvider.tsx` — sessionStorage store + migration.
- `clients/web/app/auth/SilentReAuth.tsx` — new component.
- `clients/web/app/providers.tsx` — `GoogleOAuthProvider` must wrap `AuthProvider` (GIS context needed by `SilentReAuth`); render `SilentReAuth`.
- `clients/web/lib/api.ts` — default getter.
- Tests: `__tests__/app/auth-gate.test.tsx`, `__tests__/app/login/page.test.tsx`, `__tests__/components/AdminLayout.test.tsx`, `__tests__/app/providers.test.tsx` (add `useGoogleOneTapLogin` mock), `e2e/auth.setup.ts`.

### Verification

- `go build ./...`, `go test ./... -short`, `golangci-lint run`.
- `npm test` / `npx eslint` in `clients/web`.
- e2e: `npx playwright test` (requires the compose stack).
- Manual: burst of unauthenticated `/graphql` POSTs → 429 after the IP budget; sign in, delete the sessionStorage token's validity (or wait for expiry / revoke) → One Tap auto_select silently renews without showing the login screen.

### Risks

- One Tap `auto_select` only signs in silently when the user has exactly one eligible Google session that previously consented; otherwise the One Tap UI may appear (dismissible, `cancel_on_tap_outside`). That is the documented GIS behaviour, not a bug.
- `sessionStorage` is still readable by same-tab XSS; the full fix (HttpOnly cookie via a BFF session endpoint) is a larger architectural change and remains a candidate follow-up.
- IP-based limiting can share a bucket across users behind the same NAT/proxy — mitigated by the looser IP budget.

---

## Follow-on phase — Performance Metrics & Observability (`phase-31`)

Motivation: the Playwright e2e suite now takes >5 minutes and there is no data showing where that time goes — or whether API latency is drifting. This phase adds request-level and dependency-level performance metrics so slowdowns are measurable before they are felt. It also naturally covers audit findings LENA-049 (`HTTPMetrics` silently degrading), LENA-050 (telemetry `Setup` without timeout), and LENA-063 (pgxpool without explicit sizing/statement timeout) where they overlap.

### Implementation steps

1. **GraphQL field/operation timing** (`internal/bff`, `internal/platform/telemetry`):
   - Record a histogram `graphql_resolver_duration_ms` labeled by `{operation_type (query|mutation), field}` using graphql-go's tracer/response-extension hooks (or a thin wrapper around `exec.Resolve` if hooks are insufficient).
   - Keep label cardinality bounded: field name only — never argument values or raw query text (consistent with LENA-022 / LENA-070).
2. **Domain service + SQL timing**:
   - Histogram `domain_query_duration_ms{service, query}` around generated `sqlc.Querier` calls via a single reusable `timedQuerier`-style decorator (e.g. in `internal/platform/dbtx`) applied across `grocery`, `mealplan`, `inventory`, `recipe`, `wine`, `identity`, `analytics` — rather than hand-instrumenting every method.
   - Simpler alternative: pgx query tracing via a `pgxpool` tracer hook so Postgres spans land in existing OTel traces. Evaluate both; prefer whichever touches fewer files.
3. **HTTP metrics verification**: `tel.HTTPMetrics` and the (now authenticated) `/metrics` endpoint exist — verify histograms actually emit buckets, surface instrument-creation errors instead of silently degrading (LENA-049), and add a bounded `route` label if missing.
4. **CI/e2e timing visibility**:
   - Emit per-test durations in CI via a Playwright JUnit/JSON reporter artifact so slow tests are identifiable without downloading traces.
   - GitHub step timing is sufficient for suite-level duration — do not over-build a custom metric for this.
5. **Out of scope**: dashboards/alerts (Seq/Grafana) — follow-up once data exists; pgxpool sizing changes (LENA-063) are adjacent but land in their own fix.
6. **Tests**: unit test the timing decorator (correct labels, error propagation); a resolver-timing test asserting a histogram sample exists after a query; a cardinality guard test asserting labels contain no argument data.
7. **Verification**: `go build ./...`, `go test ./...` (incl. testcontainers integration tests), `golangci-lint run`; hit `/metrics` after a few GraphQL calls and confirm the new series appear with no per-user or high-cardinality labels.
