# LENA2 Code Quality & Security Audit

| | |
|---|---|
| **Repository** | `JRAdams472/LENA2` |
| **Branch / commit** | `main` @ `672c61c` (Merge pull request #40 from JRAdams472/phase-23) |
| **Audit date** | 2026-09-07 |
| **Style** | SonarQube-style issue register (severity: Blocker / Critical / Major / Minor / Info; priority: P0–P3) |
| **Scope** | Entire repository: `cmd/`, `internal/` (all domains, `bff`, `platform`), `migrations/`, every domain `queries.sql` + generated `sqlc` output, `tools/`, `Dockerfile`, `docker-compose*.yml`, `Caddyfile`, `.github/workflows/`, `clients/web` (Next.js), `clients/mobile` (Flutter) |

This document is a standalone, complete issue register. It supersedes the coverage of the earlier partial review; `docs/issue-remediation.md` (the remediation plan derived from that partial review) is left unchanged for historical context. Findings from that review were **re-validated against `main`** and are only listed below if they still exist (see "Prior findings re-validated" at the end).

No source code was modified as part of this audit.

## Tools run

| Tool | Version | Command | Result |
|---|---|---|---|
| Go | `go1.27.1 linux/amd64` | `go build ./...` | exit 0, no output |
| Go vet | `go1.27.1` | `go vet ./...` | exit 0, no findings |
| golangci-lint | `v2.13.2` (config: `.golangci.yml` — errcheck, govet, ineffassign, misspell, staticcheck, unused, gosec; G104/G115 excluded) | `golangci-lint run ./...` | `0 issues.` |
| Go test | `go1.27.1` | `go test ./... -count=1` | all 20 packages with tests pass (integration tests use testcontainers Postgres) |
| Node / npm | `v24.19.0` | `npm ci` (clients/web) | ok (deprecation warnings for `glob@10`, `eslint@9.39.5`, `whatwg-encoding@3`) |
| TypeScript | project `typescript` | `npx tsc --noEmit` (clients/web) | exit 0 |
| ESLint | `eslint@9.39.5` + `eslint-config-next@16.3.3` | `npm run lint` (clients/web) | exit 0 |
| Jest | `jest@30` | `npm test` (clients/web) | 23 suites / 292 tests pass |
| npm audit | — | `npm audit --omit=dev` (clients/web) | 0 vulnerabilities |
| Flutter / Dart | **not available** | `flutter analyze` (clients/mobile) | **could not be run** — no Flutter/Dart toolchain on the audit machine; `clients/mobile` was reviewed manually only |

Note: because the automated tooling was clean, **every issue below comes from manual review**. The configured linters are conservative (see LENA-071) and do not catch authorization, transaction, or data-mapping defects.

## Manually reviewed areas

- `cmd/lena/main.go`, `cmd/testissuer/`
- `internal/bff/*` (all resolvers, `auth.go`, `ratelimit.go`, `graphql_tracer.go`, `services.go`, `schema.graphqls`, mocks)
- `internal/{analytics,grocery,identity,inventory,mealplan,recipe,userprefs,wine}/service.go` and `queries.sql`, plus generated `sqlc` models/queries where relevant
- `internal/platform/{config,currentuser,dbtx,logger,postgres,telemetry,testenv,validator}`
- `migrations/0001`–`0016` up/down SQL, `migrations/seed`
- `tools/coveragefilter`
- `Dockerfile`, `clients/web/Dockerfile`, `docker-compose.yml`, `docker-compose.e2e.yml`, `Caddyfile`, `.dockerignore`, `.env.example`
- `.github/workflows/{ci,docker,test}.yml`
- `clients/web` (`app/**`, `lib/api.ts`, `lib/types.ts`, config files, tests)
- `clients/mobile/lib/**`, `pubspec.yaml`, `analysis_options.yaml`

## Summary by severity

| Severity | Count |
|---|---|
| Blocker | 1 |
| Critical | 5 |
| Major | 22 |
| Minor | 31 |
| Info | 14 |
| **Total** | **73** |

Category breakdown: Security 24 · Bug 23 · Code Smell 17 · Architecture 4 · Test Gap 5.

---

## Blocker

### LENA-001 — Meal-plan and grocery child objects can be read/modified/deleted by any authenticated user (IDOR)
- **Files:** `internal/bff/resolver_mealplan.go:335-371` (`AddMealSlot`), `:374-386` (`RemoveMealSlot`), `:389-426` (`AddMealSlotItem`), `:429-441` (`RemoveMealSlotItem`); `internal/bff/resolver_grocery.go:100-122` (`ToggleGroceryItemChecked`), `:125-137` (`DeleteGroceryItem`), `:140-182` (`AddGroceryItem`); `internal/mealplan/queries.sql:37-45, 59-72, 74-77, 99-101`; `internal/grocery/queries.sql:27-30, 46-63`; `internal/mealplan/service.go` (`AddMealSlot`, `DeleteMealSlot`, `AddMealSlotItem`, `DeleteMealSlotItem`); `internal/grocery/service.go:170-216`
- **Category:** Security · **Severity:** Blocker · **Priority:** P0
- **Description:** These mutations only verify that *a* user is authenticated, then pass the client-supplied `slotID` / `slotItemID` / `groceryListItemID` / `mealPlanID` / `groceryListID` straight to SQL that filters solely on the primary key (`WHERE slot_id = $1`, `WHERE grocery_list_item_id = $1`, `INSERT ... (meal_plan_id, ...)`). Ownership is enforced for the parent (`GetMealPlanByID(..., user_id)`, `GetGroceryListByID(..., user_id)`) but never for children, so any member can add to, toggle, or delete rows in another user's meal plans and grocery lists by guessing sequential BIGSERIAL IDs.
- **Remediation hint:** Add `user_id` to every child query via a join on the parent (`... USING mealplan.meal_plan mp WHERE ms.meal_plan_id = mp.meal_plan_id AND mp.user_id = $2`), thread `userID` through the service signatures, and add negative integration tests that assert a second user gets "not found".

---

## Critical

### LENA-002 — Admin bootstrap trusts an unverified `email` claim
- **Files:** `internal/bff/auth.go:128-145`; `internal/platform/config/config.go:18-20`
- **Category:** Security · **Severity:** Critical · **Priority:** P0
- **Description:** Admin promotion compares `u.Email` (taken directly from the token's `email` claim) against `ADMIN_EMAILS`, without checking `email_verified`. Any accepted issuer that can assert an arbitrary, unverified email (or a non-Google issuer added to `AUTH_ISSUERS`) yields a persisted `admin` role. `UpsertUser` also overwrites `email` on every login (`identity/queries.sql:24-26`), so the persisted row follows the token.
- **Remediation hint:** Require `email_verified == true` before using `email` for promotion (and ideally restrict promotion to a specific issuer); compare case-insensitively.

### LENA-003 — Unauthenticated callers can force outbound OIDC discovery + JWKS fetches on every request (amplification / DoS)
- **Files:** `internal/bff/auth.go:101-116, 160-183`; `cmd/lena/main.go:166-170`
- **Category:** Security · **Severity:** Critical · **Priority:** P0
- **Description:** On any signature failure the authenticator force-refreshes the JWKS (`keySetForIssuer(ctx, issuer, true)`), which performs two outbound HTTP calls (discovery + JWKS). There is no single-flight, negative cache, or back-off, and the rate limiter is registered *after* authentication, so an unauthenticated attacker sending garbage tokens with a valid `iss` triggers unbounded fetches against the IdP and adds ~100s of ms latency per request to the API.
- **Remediation hint:** Use `jwk.Cache`/`jwk.NewCachedSet` with a minimum refresh interval, single-flight refreshes, and only refresh when the token's `kid` is unknown; move IP-based rate limiting before auth.

### LENA-004 — HTTP server has no timeouts or body-size limit
- **Files:** `cmd/lena/main.go:71-78, 107, 166-170`; `internal/bff/resolver.go:694-705`
- **Category:** Security · **Severity:** Critical · **Priority:** P0
- **Description:** `echo.New()` + `e.Start(addr)` runs a `net/http` server with zero `ReadHeaderTimeout`/`ReadTimeout`/`WriteTimeout`/`IdleTimeout`, and no `BodyLimit` middleware. `graphql.MaxQueryLength` bounds only the `query` string after the whole JSON body has been read by `c.Bind`. The service is exposed directly on host port 8080 (see LENA-025). Slowloris and large-body requests can exhaust connections/memory.
- **Remediation hint:** Configure `e.Server.ReadHeaderTimeout`/`ReadTimeout`/`WriteTimeout`/`IdleTimeout` and add `middleware.BodyLimit("64K")` in front of `/graphql`.

### LENA-005 — Unchecked `int32 → int16` narrowing lets out-of-range GraphQL input wrap into "valid" values
- **Files:** `internal/bff/resolver_recipe.go:347` (`int16(args.Rating)`); `internal/bff/resolver_inventory.go:719` and `internal/bff/resolver_wine.go:450` (`int16(args.Input.Intensity)`); `internal/bff/resolver_mealplan.go:261, 301, 354` (day-of-week); `internal/bff/resolver.go:137-144` (`int16Ptr`, used for wine acidity/tannin/body/sweetness/percentage)
- **Category:** Bug · **Severity:** Critical · **Priority:** P0
- **Description:** GraphQL `Int` is `int32`; the conversions are unchecked and gosec G115 is disabled in `.golangci.yml`. `rating: 65537` wraps to `1` and passes the service-level 1–5 check (`recipe/service.go`) and the DB `CHECK (rating BETWEEN 1 AND 5)`, silently storing a wrong value; the same applies to flavor intensity, wine tasting scores, grape percentage and `dayOfWeek` (which additionally has no range check at all — see LENA-034).
- **Remediation hint:** Add a shared `toInt16(v int32) (int16, error)` helper that rejects values outside `[math.MinInt16, math.MaxInt16]` (and domain ranges), and re-enable G115.

### LENA-006 — Resolver errors are returned verbatim to GraphQL clients (internal detail leakage)
- **Files:** `internal/bff/resolver.go:694-705`; all `internal/*/service.go` `fmt.Errorf("...: %w", err)` wrappers
- **Category:** Security · **Severity:** Critical · **Priority:** P1
- **Description:** `parsed.Exec` serialises every resolver `error.Error()` into the `errors[].message` field, so clients receive raw pgx/Postgres messages (`no rows in result set`, constraint names such as `user_bottle_user_bottle_key`, column names, SQLSTATE details). No error presenter / classification exists, and no `extensions.code` is set (the web client's `UNAUTHENTICATED`/`UNAUTHORIZED` handling in `clients/web/lib/api.ts:107-111` is therefore dead).
- **Remediation hint:** Wrap resolvers with an error presenter that maps known sentinel/domain errors to safe messages + `extensions.code`, logs the original with the request/trace ID, and returns a generic "internal error" for everything else.

---

## Major

### LENA-007 — Rate limiter is applied after authentication, so unauthenticated traffic is never rate-limited (dead IP fallback)
- **Files:** `cmd/lena/main.go:166-170`; `internal/bff/ratelimit.go:19-37`
- **Category:** Security · **Severity:** Major · **Priority:** P1
- **Description:** Echo route-level middlewares run in registration order: `authenticator.Middleware()` rejects unauthenticated requests before `GraphQLRateLimiter` is reached, so the `"ip:" + c.RealIP()` branch is unreachable in production and brute-force / JWKS-refresh floods (LENA-003) are unthrottled. `c.RealIP()` also trusts `X-Forwarded-For` by default with no `IPExtractor` configured.
- **Remediation hint:** Register an IP-keyed limiter before auth and a user-keyed limiter after; set `e.IPExtractor = echo.ExtractIPFromXFFHeader(echo.TrustLinkLocal(false), ...)` matching the Caddy deployment.

### LENA-008 — `/metrics` is served without authentication
- **Files:** `cmd/lena/main.go:160-163`; `docker-compose.yml:79-80` (port 8080 published)
- **Category:** Security · **Severity:** Major · **Priority:** P1
- **Description:** The Prometheus endpoint is registered on the root Echo instance with no auth middleware. Caddy does not route it, but the API container publishes `8080:8080` to the host, so pool statistics, per-route request rates and Go runtime internals are readable by anyone reaching the host.
- **Remediation hint:** Bind metrics to a separate internal listener/port, or guard the route with a bearer/basic-auth middleware or an IP allowlist; stop publishing 8080 to the host.

### LENA-009 — `os.Exit` in server goroutine and early exits skip deferred cleanup
- **Files:** `cmd/lena/main.go:56-67, 71-78`
- **Category:** Bug · **Severity:** Major · **Priority:** P1
- **Description:** `os.Exit(1)` on `e.Start` failure (line 76) and on telemetry setup failure (line 61, after `defer pool.Close()`) terminates the process without running `pool.Close()` or `tel.Shutdown()`, so buffered traces are lost and DB connections are not released. Shutdown also does not wait for in-flight `recordEventAsync`/`computeOverlapAsync` goroutines (LENA-012).
- **Remediation hint:** Send the start error on a channel and `select` on it alongside the signal channel, then fall through to the normal shutdown path; wrap `main` in a `run() error` function.

### LENA-010 — GraphQL schema parse failure panics at startup
- **Files:** `internal/bff/resolver.go:688-693`
- **Category:** Bug · **Severity:** Major · **Priority:** P1
- **Description:** `NewGraphQLHandler` calls `panic(err)` when `graphql.ParseSchema` fails. Called from `newServer`, the panic bypasses the logger and all deferred cleanup in `main`, and gives an unstructured crash instead of a clear startup error.
- **Remediation hint:** Return `(echo.HandlerFunc, error)` (or a `Must…` variant used only in tests) and surface the error through `main`'s normal error path.

### LENA-011 — Ignored `token.Audience()`, `email` and `name` extraction errors; empty email persisted
- **Files:** `internal/bff/auth.go:118-121, 128-135`; `internal/identity/service.go:65-75`; `internal/identity/queries.sql:12-30`
- **Category:** Bug · **Severity:** Major · **Priority:** P1
- **Description:** `audience, _ := token.Audience()` and `_ = token.Get("email"/"name", ...)` discard errors. A token with no `aud` fails closed (empty slice never matches) but the message is misleading; a token whose `email` claim is malformed or missing is accepted and persisted with `email = ''`, `created_by = ''`, and the client then sees `isAuthenticated=false` in the web app because it requires an email.
- **Remediation hint:** Check the returned errors; reject tokens without a usable, verified email if the product requires one, and never write empty strings to `created_by`.

### LENA-012 — Fire-and-forget analytics/recommendation goroutines are untracked
- **Files:** `internal/bff/resolver.go:71-109` (`recordEventAsync`, `computeOverlapAsync`); `internal/bff/resolver_analytics.go` (`RecordSelection`, `RecordSearch` log-and-ignore)
- **Category:** Architecture · **Severity:** Major · **Priority:** P2
- **Description:** Each call spawns a detached goroutine on `context.Background()` with no `WaitGroup`, cancellation, or concurrency cap; `e.Shutdown` returns while they may still be writing, and a burst of mutations can create an unbounded number of goroutines/DB connections (each `RecordEvent` opens a transaction). `computeOverlapAsync` scores *every* user for each new recipe (30 s budget) inline in the API process.
- **Remediation hint:** Route events through a bounded channel + worker owned by the server (drained on shutdown), or a job table; expose `Wait()` for graceful shutdown.

### LENA-013 — `ComputeIngredientOverlapSuggestions` performs N upserts without a transaction
- **Files:** `internal/analytics/service.go:174-204`
- **Category:** Bug · **Severity:** Major · **Priority:** P2
- **Description:** The loop issues one `UpsertRecipeRecommendation` per user row on the plain pool (`s.q`, not `InTx`). A failure mid-way leaves a partially written recommendation set (`return n, err`), and hundreds of round-trips are made for a single logical operation.
- **Remediation hint:** Wrap in `s.InTx` and/or use a single set-based `INSERT ... SELECT ... ON CONFLICT` query.

### LENA-014 — `numericToFloat64` conflates SQL `NULL` with `0`
- **Files:** `internal/inventory/service.go:844-853`; `internal/inventory/service.go:746-751` (`toUnit` also swallows the conversion error)
- **Category:** Bug · **Severity:** Major · **Priority:** P2
- **Description:** `if !n.Valid { return 0, nil }` makes a NULL nutrient amount / conversion factor indistinguishable from a real zero; other domains (`grocery`, `mealplan`, `userprefs`, `wine`) were rewritten to keep pointer semantics, so `inventory` is now the inconsistent one. `toUnit` additionally drops `ToBaseFactor` silently if `Float64Value` errors.
- **Remediation hint:** Return `(*float64, error)` for nullable columns (or error on NULL for `NOT NULL` columns) and propagate the conversion error.

### LENA-015 — `unitName` returns `""` with no error when a preloaded unit is missing
- **Files:** `internal/bff/resolver.go:290-302`
- **Category:** Bug · **Severity:** Major · **Priority:** P2
- **Description:** When the batch map is non-nil but lacks `unitID` (e.g. FK to an inactive/deleted unit, or a batch-loading bug), the resolver silently yields an empty unit name instead of falling back to `GetUnitByID` or erroring. Clients cannot distinguish "no unit" from "unit exists".
- **Remediation hint:** Fall back to `inv.GetUnitByID` on a map miss, or return an explicit error.

### LENA-016 — Incomplete struct mapping in `inventory` create paths
- **Files:** `internal/inventory/service.go:453-476` (`CreateFoodNutrient` returns without `ItemID`, `Name`, `Unit`), `:494-509` (`CreateFoodFlavor` returns without `ItemID`, `Name`); `internal/wine/service.go:383-397` (`AddBottleGrapeVariety` returns without `BottleID`, `Name`); `internal/bff/resolver_wine.go:631-636` (`bottleGrapeVarietyResolver.GrapeVariety` fabricates a partial `GrapeVariety`)
- **Category:** Bug · **Severity:** Major · **Priority:** P2
- **Description:** Mutation payloads return half-populated objects (zero IDs, empty names, empty units), so the GraphQL response for `createFoodNutrient`/`addFoodFlavor`/`addBottleGrapeVariety` is misleading and differs from the subsequent query result. The wine resolver hides `Description`/`IsActive` by constructing a fake `GrapeVariety`.
- **Remediation hint:** Re-read the joined row (or use a `RETURNING`+join query) and map every field; in the resolver fetch the real `GrapeVariety`.

### LENA-017 — Google ID token persisted in `localStorage` (web)
- **Files:** `clients/web/app/auth/AuthProvider.tsx:39, 72-90`; `clients/web/lib/api.ts:49-51`
- **Category:** Security · **Severity:** Major · **Priority:** P1
- **Description:** The bearer token is stored in `window.localStorage["lena_id_token"]`, readable by any XSS payload and by all scripts on the origin. No refresh flow exists, so the app silently signs out after the ~1 h Google token lifetime.
- **Remediation hint:** Keep the token in memory / a `HttpOnly` cookie set by a BFF session endpoint, or at minimum `sessionStorage`; implement silent re-auth via GIS `prompt`.

### LENA-018 — `grocery_list.meal_plan_id` FK has no `ON DELETE` action, breaking meal-plan deletion
- **Files:** `migrations/0008_create_grocery.up.sql:1-10`; `internal/mealplan/queries.sql:33-35`; `internal/bff/resolver_mealplan.go:319-333`
- **Category:** Bug · **Severity:** Major · **Priority:** P2
- **Description:** Once a grocery list has been generated from a plan, `deleteMealPlan` fails with a raw FK-violation error (leaked to the client per LENA-006). No code path handles this.
- **Remediation hint:** Add `ON DELETE SET NULL` (list survives, loses provenance) or `ON DELETE CASCADE`, in a new migration.

### LENA-019 — Wine `oak_integration` NULL/false conflation
- **Files:** `internal/wine/service.go:568-582, 601, 690, 809-811, 848-850`
- **Category:** Bug · **Severity:** Major · **Priority:** P2
- **Description:** `boolOrNull` always returns `Valid: true`, so `oak_integration` can never be written as NULL, and `toBottle` maps a NULL column to `false`. The domain type is a plain `bool` although the struct comment promises "nullable tasting-note fields are pointers so a real 0 is distinguishable from 'not set'".
- **Remediation hint:** Make `OakIntegration *bool` end-to-end (schema already nullable) or make the column `NOT NULL DEFAULT false`.

### LENA-020 — `findUserItem` / `findUserBottle` load up to 100 000 rows to find one record
- **Files:** `internal/bff/resolver_userprefs.go:291-314`
- **Category:** Bug · **Severity:** Major · **Priority:** P2
- **Description:** Every `upsertUserItem`/`upsertUserBottle` mutation fetches the user's entire pantry/cellar (`ListUserItems(ctx, userID, 100_000, 0)`) and scans it in Go, even though unique `(user_id, item_id)`/`(user_id, bottle_id)` constraints exist and single-row lookups are trivially expressible in SQL. Silently wrong for users with > 100 000 rows.
- **Remediation hint:** Add `GetUserItemByUserAndItem` / `GetUserBottleByUserAndBottle` queries (or `INSERT ... ON CONFLICT ... RETURNING`).

### LENA-021 — `ListRatingRecencySuggestions` sorts/limits in Go after loading all rows
- **Files:** `internal/recipe/service.go:504-537`
- **Category:** Code Smell · **Severity:** Major · **Priority:** P2
- **Description:** All rows matching `rating >= minRating` for a user are pulled, aggregated in a map, sorted with `sort.Slice`, and truncated to `limit` in memory. Also uses `sort.Slice` where `slices.SortFunc` is the stdlib idiom.
- **Remediation hint:** Move aggregation/ordering/`LIMIT` into SQL.

### LENA-022 — Full GraphQL query text recorded as a span attribute
- **Files:** `internal/bff/graphql_tracer.go:30-37`
- **Category:** Security · **Severity:** Major · **Priority:** P2
- **Description:** `graphql.operation.query` stores the raw query string (up to 8 KiB by default) on every trace. Queries embed inline literals (search terms, notes, emails in filters) that are shipped in plaintext to the OTLP collector (see LENA-030), bypassing the PII redaction in `platform/logger`.
- **Remediation hint:** Record only `operationName`/operation type and a hash of the document, or apply the same redaction/allowlist policy.

### LENA-023 — Docker images are pushed to GHCR before tests run
- **Files:** `.github/workflows/docker.yml:1-75`; `.github/workflows/test.yml`
- **Category:** Architecture · **Severity:** Major · **Priority:** P2
- **Description:** The `Docker` workflow builds and pushes `latest` on every push to `main` with no `needs:` dependency on the `Test`/`CI` jobs and no `workflow_run` gating, so a red build still publishes a deployable image tagged `latest`.
- **Remediation hint:** Merge into one workflow with `needs: [go, web, e2e]`, or trigger on `workflow_run` success; require branch protection.

### LENA-024 — Two overlapping CI workflows with different Go versions and scopes
- **Files:** `.github/workflows/ci.yml:14-31`; `.github/workflows/test.yml:24-58`; `go.mod` (`go 1.26.0`)
- **Category:** Code Smell · **Severity:** Major · **Priority:** P2
- **Description:** `ci.yml` uses Go `1.27`, `go build ./...`, no `-race`/`-count=1`, and `golangci-lint version: latest`; `test.yml` uses Go `1.26.0`, path-scoped `./cmd/... ./internal/...`, `-race`, coverage gating. Results can diverge (e.g. `./...` includes `clients/web/node_modules/flatted/golang` — see LENA-045) and `latest` linter is non-reproducible.
- **Remediation hint:** Delete `ci.yml` (or make it the single source of truth), pin the Go version from `go.mod` via `go-version-file`, and pin the golangci-lint version.

### LENA-025 — Compose stack exposes database, API and Seq directly on the host with default credentials
- **Files:** `docker-compose.yml:5-12, 79-80, 104-121`; `Caddyfile`
- **Category:** Security · **Severity:** Major · **Priority:** P2
- **Description:** Postgres (`5432`, default password `change-me`), the API (`8080`, bypassing Caddy and exposing `/metrics`, `/ready`), and Seq UI (`5341`, no auth, `ACCEPT_EULA`) are published on all interfaces. `LENA_CORS_ALLOWED_ORIGINS` defaults to `*`, `sslmode=disable` is hard-coded, and `seq`, `seq-input-gelf`, `migrate/migrate` images are unpinned (`latest`).
- **Remediation hint:** Drop host port mappings for `db`, `api`, `seq`; require `POSTGRES_PASSWORD` (no default); pin image tags/digests; set an explicit CORS origin default.

### LENA-026 — Web Dockerfile runs as root and falls back to `npm install`
- **Files:** `clients/web/Dockerfile:5-6, 20-31`
- **Category:** Security · **Severity:** Major · **Priority:** P2
- **Description:** The runtime stage has no `USER`, so Next.js runs as root; the builder uses `npm ci || npm install`, silently ignoring lockfile drift. The API Dockerfile correctly uses `USER nobody`, so the two images are inconsistent.
- **Remediation hint:** Add `USER node`, use `npm ci` only, and consider pinning base image digests.

### LENA-027 — Mobile client has no sign-in flow and cannot authenticate
- **Files:** `clients/mobile/lib/graphql_config.dart:1-24`; `clients/mobile/lib/main.dart`; `clients/mobile/pubspec.yaml`
- **Category:** Bug · **Severity:** Major · **Priority:** P2
- **Description:** `getIdToken()` reads `id_token` from `flutter_secure_storage`, but nothing in `lib/` ever writes it (no `google_sign_in`, no login screen). Every GraphQL call therefore fails with 401 against the BFF. The API URL is also hard-coded to `http://localhost:8080/graphql` (plaintext, emulator-only).
- **Remediation hint:** Add a Google sign-in flow that stores the ID token, and inject the API URL via `--dart-define`.

### LENA-028 — Mobile client has zero tests, no platform folders, no lockfile
- **Files:** `clients/mobile/` (no `test/`, `android/`, `ios/`, `pubspec.lock`); `.github/workflows/*` (no Flutter job)
- **Category:** Test Gap · **Severity:** Major · **Priority:** P2
- **Description:** ~2 400 lines of Dart have no unit/widget tests, are not analysed or built in CI, and the project cannot be built as shipped. `flutter_lints ^3.0.0` is two majors behind.
- **Remediation hint:** Add `flutter analyze` + `flutter test` CI job, commit platform scaffolding and `pubspec.lock`, upgrade `flutter_lints`.

---

## Minor

### LENA-029 — Admin-email comparison is case-sensitive
- **Files:** `internal/bff/auth.go:140, 232-239`
- **Category:** Bug · **Severity:** Minor · **Priority:** P2
- **Description:** `contains(a.cfg.AdminEmails, u.Email)` uses exact string equality; `Admin@Example.com` in the token vs `admin@example.com` in config never promotes.
- **Remediation hint:** Normalise both sides with `strings.EqualFold`/lower-casing.

### LENA-030 — OTLP trace exporter hard-codes `WithInsecure()`
- **Files:** `internal/platform/telemetry/telemetry.go:44-47`
- **Category:** Security · **Severity:** Minor · **Priority:** P2
- **Description:** Traces (including the query text from LENA-022) are always sent over plaintext gRPC; there is no way to enable TLS via config.
- **Remediation hint:** Make TLS configurable (e.g. honour `OTEL_EXPORTER_OTLP_INSECURE`).

### LENA-031 — `/ready` leaks the database error string
- **Files:** `cmd/lena/main.go:153-158`
- **Category:** Security · **Severity:** Minor · **Priority:** P3
- **Description:** The unauthenticated readiness probe returns `err.Error()` from `pool.Ping`, which can include host names, user names and driver internals.
- **Remediation hint:** Log the error and return a fixed `{"status":"not ready"}` body.

### LENA-032 — CORS origin list is split without trimming, inconsistent with `splitAndTrim`
- **Files:** `cmd/lena/main.go:185-191, 196-206`
- **Category:** Bug · **Severity:** Minor · **Priority:** P2
- **Description:** `strings.Split(allowedOrigins, ",")` keeps whitespace, so `"https://a.example, https://b.example"` silently rejects the second origin, while the neighbouring `splitAndTrim` helper exists for exactly this purpose. The default `http://localhost` also omits the port used by the web app.
- **Remediation hint:** Use `splitAndTrim` and document an explicit default.

### LENA-033 — `nil`-telemetry check is inconsistent with construction
- **Files:** `cmd/lena/main.go:58-62, 160-163`; `internal/platform/telemetry/telemetry.go:33-73`
- **Category:** Code Smell · **Severity:** Minor · **Priority:** P3
- **Description:** `tel` can never be nil in `main` (setup failure exits), yet `newServer` branches on `tel != nil` to register metrics, and `HTTPMetrics()` uses the global meter irrespective of `tel`. Tests pass `nil`, so the production and test wiring differ.
- **Remediation hint:** Make `Telemetry` a value with a no-op mode, or always register metrics and let the provider decide.

### LENA-034 — No range validation for `dayOfWeek`, `weekStartDayOfWeek`, `servings`, `quantity`, `intensity`, `percentage`
- **Files:** `internal/bff/resolver_mealplan.go:244-269, 271-317, 335-371`; `internal/bff/resolver_inventory.go:719`; `internal/bff/resolver_wine.go:450`; `migrations/0003`, `0004`, `0007`, `0008` (no `CHECK` constraints)
- **Category:** Bug · **Severity:** Minor · **Priority:** P2
- **Description:** Beyond the wrap-around issue (LENA-005), nothing prevents `dayOfWeek = 9`, negative servings/quantities, `intensity = 500`, or grape percentages summing past 100. Only `recipe_rating` and `identity.users.role` have DB `CHECK`s.
- **Remediation hint:** Add resolver/service validation and matching `CHECK` constraints in a migration.

### LENA-035 — `grocery_list_item` allows a row with neither `item_id`, `ingredient_id` nor `manual_item_name`
- **Files:** `migrations/0008_create_grocery.up.sql:12-25`; `migrations/0011_ingredient.up.sql`; `internal/bff/resolver_grocery.go:140-182`
- **Category:** Bug · **Severity:** Minor · **Priority:** P2
- **Description:** All three identifying columns are nullable and neither the resolver nor the schema requires at least one, so "empty" grocery items can be created; `source` is a free-form `VARCHAR(50)` with no enum check.
- **Remediation hint:** Add a `CHECK (item_id IS NOT NULL OR ingredient_id IS NOT NULL OR manual_item_name IS NOT NULL)` and validate in the resolver.

### LENA-036 — Missing indexes on high-traffic FK columns
- **Files:** `migrations/0007_create_mealplan.up.sql` (`meal_slot.meal_plan_id`, `meal_slot.recipe_id`, `meal_slot_item.slot_id`); `migrations/0008_create_grocery.up.sql` (`grocery_list.user_id`, `grocery_list.meal_plan_id`, `grocery_list_item.grocery_list_id`); `migrations/0011_ingredient.up.sql` (all `ingredient_id` FKs); `migrations/0012` (`unit_id` FKs)
- **Category:** Code Smell · **Severity:** Minor · **Priority:** P3
- **Description:** `ListMealSlotItemsByPlan(s)`, `ListGroceryListItemsByLists`, `ListGroceryLists` and cascade deletes all filter/join on un-indexed FK columns; Postgres does not index FKs automatically.
- **Remediation hint:** Add B-tree indexes for each FK used in `WHERE`/`JOIN`.

### LENA-037 — `GenerateGroceryList` is a stub and does not verify plan ownership
- **Files:** `internal/grocery/service.go:218-225`; `internal/bff/resolver_grocery.go:83-97`
- **Category:** Bug · **Severity:** Minor · **Priority:** P2
- **Description:** `Generate` only creates an empty list linked to `mealPlanID` ("Phase 4 provides the list container only"); it neither aggregates recipe items nor checks that the plan belongs to the caller, so a user can link a list to another user's plan ID (existence oracle via FK error) and the GraphQL mutation name over-promises.
- **Remediation hint:** Verify ownership with `GetMealPlanByID(ctx, id, userID)` and either implement aggregation or rename/document the mutation.

### LENA-038 — `Regions` resolver swallows the country lookup error
- **Files:** `internal/bff/resolver_wine.go:86-110`
- **Category:** Bug · **Severity:** Minor · **Priority:** P3
- **Description:** `if c, err := r.WineService.GetCountryByID(...); err == nil { ... }` treats DB failures the same as "not found" and falls back to lazy per-row loading (an N+1 path).
- **Remediation hint:** Distinguish `pgx.ErrNoRows` from other errors and return the latter.

### LENA-039 — Preloaded-catalog misses return `nil, nil` for non-null GraphQL fields
- **Files:** `internal/bff/resolver_userprefs.go:346, 411`
- **Category:** Bug · **Severity:** Minor · **Priority:** P3
- **Description:** When a batch map lacks the referenced item/bottle the resolver returns `nil, nil`; graphql-go then reports a generic "graphql: got nil for non-null" error for the whole object instead of a meaningful message.
- **Remediation hint:** Return an explicit error or fall back to a single fetch.

### LENA-040 — Analytics events ignore errors (`RecordSelection`, `RecordSearch` always return `true`)
- **Files:** `internal/bff/resolver_analytics.go` (both mutations)
- **Category:** Bug · **Severity:** Minor · **Priority:** P3
- **Description:** DB errors are logged and the mutation still returns `true`, so clients cannot detect failures and tests cannot assert them. Fire-and-forget is acceptable, but the contract (`Boolean!`) claims success.
- **Remediation hint:** Return the error, or change the schema to a `Void`/acknowledgement type and document best-effort semantics.

### LENA-041 — Audit columns store the user's email (`created_by`/`updated_by`)
- **Files:** All `internal/*/service.go` (`by string` parameters populated with `u.Email` in every resolver); all migrations (`created_by VARCHAR(100)`)
- **Category:** Security · **Severity:** Minor · **Priority:** P3
- **Description:** PII is copied into every table (including analytics), making right-to-erasure hard and contradicting the log redaction of `email`. `VARCHAR(100)` is shorter than `identity.users.email VARCHAR(320)`, so long emails would fail inserts.
- **Remediation hint:** Store `user_id` (FK) in audit columns instead of email.

### LENA-042 — `identity.users.email` is not unique and is overwritten on every login
- **Files:** `migrations/0002_create_identity.up.sql`; `internal/identity/queries.sql:12-30`
- **Category:** Security · **Severity:** Minor · **Priority:** P3
- **Description:** Two `(provider, subject)` rows may share an email, and `ON CONFLICT ... DO UPDATE SET email = EXCLUDED.email` lets a token change the persisted email at will. Combined with LENA-002 this widens the admin-bootstrap surface.
- **Remediation hint:** Add a unique index on `(provider, lower(email))` or at least on `lower(email)` where verified.

### LENA-043 — Reinvented stdlib helpers (`contains`, `containsAny`, `int16Ptr`, `textOrNull`, `optInt64/optInt8/optInt2`, `numericFromFloat64`)
- **Files:** `internal/bff/auth.go:232-248`; `internal/inventory/service.go:779-784, 825-859`; `internal/wine/service.go:815-850`; `internal/grocery/service.go`, `internal/mealplan/service.go`, `internal/userprefs/service.go`, `internal/recipe/service.go` (each has its own copy)
- **Category:** Code Smell · **Severity:** Minor · **Priority:** P3
- **Description:** `contains` duplicates `slices.Contains`; `containsAny` duplicates `slices.ContainsFunc`. The pgtype conversion helpers are copy-pasted into six domain packages with subtly different names/semantics (e.g. `optInt64` vs `optInt8`), which is how the NULL/0 inconsistency in LENA-014/LENA-019 arose.
- **Remediation hint:** Use `slices.*`; move pgtype helpers into `internal/platform/pgconv` with one tested implementation.

### LENA-044 — Dead code: `platform/validator`, `Config.GoogleClientID`, unused service methods
- **Files:** `internal/platform/validator/validator.go` (exported `V` has no callers); `internal/platform/config/config.go:14-17`; `internal/recipe/service.go:383-391` (`UpdateRecipeStep`); `internal/userprefs/service.go:92, 185, 274` (`GetUserItemByID`, `GetUserBottleByID`, `DeleteRecipeFavorite`); `internal/mealplan/queries.sql:59-68` (`UpdateMealSlot` – no service/resolver caller)
- **Category:** Code Smell · **Severity:** Minor · **Priority:** P3
- **Description:** Unused packages/config and service methods add surface area and misleading test coverage; `GetUserItemByID` is the very query `findUserItem` (LENA-020) should be using.
- **Remediation hint:** Remove or wire up; keep `unused` linting for exported symbols via `deadcode`.

### LENA-045 — `go test/build ./...` traverses `clients/web/node_modules`
- **Files:** `go.mod` (module root), `clients/web/node_modules/flatted/golang/pkg/flatted`; `.github/workflows/ci.yml:20-26`
- **Category:** Code Smell · **Severity:** Minor · **Priority:** P3
- **Description:** A third-party Go package inside `node_modules` is compiled and vetted as part of the module (`go test ./...` output lists it), which is slow and can break the build when dependencies change; `test.yml` works around it with path scoping while `ci.yml` does not.
- **Remediation hint:** Add a `go.work`/nested `go.mod` under `clients/`, or exclude via `-skip`/path scoping consistently.

### LENA-046 — `IdentifierExtractor` trusts `X-Forwarded-For` without a configured IP extractor
- **Files:** `internal/bff/ratelimit.go:30-35`; `cmd/lena/main.go:107-113`
- **Category:** Security · **Severity:** Minor · **Priority:** P2
- **Description:** `c.RealIP()` honours client-supplied `X-Forwarded-For`/`X-Real-IP` by default. Behind Caddy that is fine only if Caddy overwrites the header; with port 8080 exposed directly (LENA-025) any client can spoof its identity for IP-keyed limits.
- **Remediation hint:** Set `e.IPExtractor` with trusted proxy ranges.

### LENA-047 — GraphQL introspection and unrestricted operations for all authenticated users
- **Files:** `internal/bff/resolver.go:688-705`; `internal/bff/schema.graphqls`
- **Category:** Security · **Severity:** Minor · **Priority:** P3
- **Description:** graphql-go serves full introspection to any member; combined with LENA-006 error verbosity this eases enumeration. No per-operation complexity limit exists beyond depth/length.
- **Remediation hint:** Disable introspection in production (`graphql.DisableIntrospection()`) and consider complexity limits.

### LENA-048 — Interaction event `metadata`/`search_term` stored without size or content policy
- **Files:** `migrations/0014_create_analytics.up.sql:3-13`; `internal/analytics/service.go` (`RecordEvent`)
- **Category:** Security · **Severity:** Minor · **Priority:** P3
- **Description:** `search_term VARCHAR(500)` and free-form `metadata JSONB` are written from client input with the user's ID and no retention policy; analytics tables can accumulate PII indefinitely.
- **Remediation hint:** Define retention/anonymisation and limit accepted metadata keys.

### LENA-049 — `HTTPMetrics` silently degrades to a no-op if instrument creation fails
- **Files:** `internal/platform/telemetry/http.go:17-24`; `internal/platform/telemetry/telemetry.go:82-88` (`Shutdown` ignores errors)
- **Category:** Bug · **Severity:** Minor · **Priority:** P3
- **Description:** Instrument-creation errors are swallowed and metrics quietly disappear; `Shutdown` discards flush errors so lost spans are invisible.
- **Remediation hint:** Log the errors (or return them from a constructor).

### LENA-050 — Telemetry `Setup` uses `context.Background()` without timeout
- **Files:** `cmd/lena/main.go:58`
- **Category:** Bug · **Severity:** Minor · **Priority:** P3
- **Description:** `otlptracegrpc.New` may block on connection setup; unlike the DB pool (which now has a 10 s bound) telemetry setup can hang startup indefinitely when the collector is unreachable.
- **Remediation hint:** Use a bounded context as for `postgres.NewPool`.

### LENA-051 — Duplicated token-key literal and dead `extensions.code` branch in web API client
- **Files:** `clients/web/lib/api.ts:47-51, 105-112`; `clients/web/app/auth/AuthProvider.tsx:39`
- **Category:** Code Smell · **Severity:** Minor · **Priority:** P3
- **Description:** `"lena_id_token"` is hard-coded in two modules; the `UNAUTHENTICATED`/`UNAUTHORIZED` check can never fire because the BFF never sets `extensions.code` (LENA-006), so a member calling an admin mutation sees a raw "forbidden: admin role required" string.
- **Remediation hint:** Export the key from one module; agree an error-code contract with the BFF.

### LENA-052 — Web UI has no role awareness; admin CRUD is rendered for every user
- **Files:** `clients/web/app/components/AdminLayout.tsx`, `clients/web/app/inventory/**`, `clients/web/app/wine/**`, `clients/web/app/recipes/page.tsx`; `internal/bff/schema.graphqls` (no `me { isAdmin }` field)
- **Category:** Architecture · **Severity:** Minor · **Priority:** P3
- **Description:** The BFF enforces `requireAdmin`, but the client cannot learn the caller's role, so members see create/edit/delete controls that fail on submit with a leaked error string.
- **Remediation hint:** Expose a `viewer { isAdmin }` query and hide admin actions client-side (server checks stay authoritative).

### LENA-053 — `lib/api.ts` is a 2 252-line monolith
- **Files:** `clients/web/lib/api.ts`
- **Category:** Code Smell · **Severity:** Minor · **Priority:** P3
- **Description:** All GraphQL documents, types and fetch helpers for every domain live in one file with hand-written query strings and no codegen, making schema drift undetectable until runtime.
- **Remediation hint:** Split per domain and adopt GraphQL codegen against `schema.graphqls`.

### LENA-054 — `eslint-disable react-hooks/set-state-in-effect` suppressions
- **Files:** `clients/web/app/inventory/items/page.tsx:51`; `clients/web/app/recipes/page.tsx:60`
- **Category:** Code Smell · **Severity:** Minor · **Priority:** P3
- **Description:** Two components suppress the rule instead of deriving state during render or using `useMemo`, risking redundant renders.
- **Remediation hint:** Derive the value or reset via key/`useMemo`.

### LENA-055 — Stale `.env.example` references a different backend
- **Files:** `clients/web/.env.example`
- **Category:** Code Smell · **Severity:** Minor · **Priority:** P3
- **Description:** Points to `LENA.API` on `http://localhost:5059` (a REST predecessor), whereas the app expects the GraphQL BFF at `/graphql` or `:8080`. New developers will misconfigure the client.
- **Remediation hint:** Update to `NEXT_PUBLIC_API_BASE_URL=http://localhost:8080/graphql`.

### LENA-056 — `cmd/testissuer` is built by the main module and shares the production image build context
- **Files:** `cmd/testissuer/main.go`; `cmd/testissuer/Dockerfile`; `Dockerfile:9` (`COPY . .`)
- **Category:** Security · **Severity:** Minor · **Priority:** P3
- **Description:** A token-minting OIDC issuer ("never deploy it") lives in the production module, is compiled by `go build ./...`, and its source is copied into the API image build context. Misconfiguring `AUTH_ISSUERS` to include it in a non-e2e environment yields full auth bypass.
- **Remediation hint:** Move it to a separate module (`tools/` or `e2e/`) excluded from `.dockerignore`, and fail startup if a non-HTTPS issuer is configured outside a dev flag.

### LENA-057 — Caddy serves plain HTTP only and adds no security headers
- **Files:** `Caddyfile:1-13`
- **Category:** Security · **Severity:** Minor · **Priority:** P3
- **Description:** `{$CADDY_ADDR:-:80}` disables automatic HTTPS; no `header` directives (HSTS, `X-Content-Type-Options`, CSP, `Referrer-Policy`). Bearer tokens travel in cleartext in the default stack.
- **Remediation hint:** Configure a hostname for automatic TLS and add a `header` block.

### LENA-058 — Workflows lack `permissions:` and `timeout-minutes`; actions pinned by tag only
- **Files:** `.github/workflows/ci.yml`, `.github/workflows/test.yml` (no `permissions`), all three workflows (`actions/*@v4/v5`, `golangci-lint-action@v7`, no SHA pins, no timeouts)
- **Category:** Security · **Severity:** Minor · **Priority:** P3
- **Description:** Default `GITHUB_TOKEN` permissions are broader than needed; mutable tags allow supply-chain drift; the e2e job can hang for the 6 h default.
- **Remediation hint:** Add `permissions: contents: read`, `timeout-minutes`, and pin actions to commit SHAs.

### LENA-059 — No integration tests for cross-user authorization boundaries
- **Files:** `internal/bff/bff_integration_test.go`, `internal/bff/resolver_*_test.go`, `clients/web/e2e/*`
- **Category:** Test Gap · **Severity:** Minor · **Priority:** P1
- **Description:** No test exercises "user B mutates user A's slot/item/list" — which is why LENA-001 went unnoticed. e2e authenticates as one user (`docker-compose.e2e.yml`, rate limit disabled) and only checks the admin-vs-member catalog path.
- **Remediation hint:** Add table-driven negative tests for every child mutation with a second seeded user.

---

## Info

### LENA-060 — `.golangci.yml` excludes G115 and G104 and enables a small linter set
- **Files:** `.golangci.yml`
- **Category:** Code Smell · **Severity:** Info · **Priority:** P2
- **Description:** Disabling G115 (integer overflow) hides LENA-005; G104 (unhandled errors) hides ignored-error patterns. `revive`, `errorlint`, `gocritic`, `bodyclose`, `noctx`, `sqlclosecheck` are not enabled.
- **Remediation hint:** Re-enable G115/G104 with targeted `//nolint` where justified and add the listed linters.

### LENA-061 — Go coverage baseline set at 60 %
- **Files:** `.github/workflows/test.yml:14-18`; `tools/coveragefilter/main.go`
- **Category:** Test Gap · **Severity:** Info · **Priority:** P3
- **Description:** Generated code is filtered out, but the remaining hand-written threshold is low for a service whose bugs are in error paths; `tools/coveragefilter` itself has no tests.
- **Remediation hint:** Ratchet the baseline upward per PR; add a small test for the filter.

### LENA-062 — No Playwright/e2e coverage for member (non-admin) user journeys beyond rejection
- **Files:** `clients/web/e2e/*`, `docker-compose.e2e.yml:20-30`
- **Category:** Test Gap · **Severity:** Info · **Priority:** P3
- **Description:** e2e-other@example.com is only used to prove catalog mutations are rejected; pantry/cellar/meal-plan/grocery flows run solely as the admin user.
- **Remediation hint:** Add member-role happy-path scenarios.

### LENA-063 — `pgxpool` has no explicit pool sizing or statement timeout
- **Files:** `internal/platform/postgres/postgres.go:13-30`
- **Category:** Architecture · **Severity:** Info · **Priority:** P3
- **Description:** Pool limits, `MaxConnLifetime`, `HealthCheckPeriod` and `statement_timeout` rely on pgx defaults; long-running analytics queries (LENA-012) can monopolise connections.
- **Remediation hint:** Expose pool settings in `Config` and set `statement_timeout` via `RuntimeParams`.

### LENA-064 — `logger` redaction is substring-based and only covers top-level keys
- **Files:** `internal/platform/logger/logger.go:9-34, 52-58`
- **Category:** Security · **Severity:** Info · **Priority:** P3
- **Description:** `"address"` redacts `ip_address`/`addr`-like keys (over-redaction) while values embedded in `error` strings (e.g. `"promote admin user: … email …"`) are not redacted (under-redaction). `AddSource: true` in production leaks file paths.
- **Remediation hint:** Redact by explicit key set; sanitize errors before logging.

### LENA-065 — `dbtx.InTx` ignores rollback errors and offers no isolation options
- **Files:** `internal/platform/dbtx/dbtx.go:43-59`
- **Category:** Code Smell · **Severity:** Info · **Priority:** P3
- **Description:** `_ = tx.Rollback(ctx)` is conventional, but `InTx` cannot start `SERIALIZABLE`/read-only transactions, which `RecordEvent`'s read-modify-write counters may eventually need.
- **Remediation hint:** Accept `pgx.TxOptions` variadically.

### LENA-066 — `CreateBrand` has no `created_by` while every sibling catalog entity does
- **Files:** `internal/inventory/service.go:51-58`; `migrations/0003_create_inventory.up.sql` (brand table)
- **Category:** Code Smell · **Severity:** Info · **Priority:** P3
- **Description:** Brands lack the audit columns/`by` parameter used consistently elsewhere.
- **Remediation hint:** Align the schema and service signature.

### LENA-067 — Migrations backfill unmatched units to `'each'` silently
- **Files:** `migrations/0012_units_and_recipe_item_sections.up.sql:36-70`
- **Category:** Bug · **Severity:** Info · **Priority:** P3
- **Description:** Any free-text unit that did not match a name/abbreviation was replaced by `each` with no record of the original value, so historical recipe quantities may now be wrong with no way to recover.
- **Remediation hint:** For future data migrations, preserve the legacy column (`unit_legacy`) or log unmatched values.

### LENA-068 — `docker-compose.yml` seeds the DB on every `up`
- **Files:** `docker-compose.yml:35-55`
- **Category:** Code Smell · **Severity:** Info · **Priority:** P3
- **Description:** `db-seed` re-runs `psql -f` for every seed file each start; it only works because the seed uses `ON CONFLICT DO NOTHING`, which is an implicit contract.
- **Remediation hint:** Make seeding idempotent by convention (document it) or run it via `migrate`.

### LENA-069 — API image installs `curl` only for the compose healthcheck
- **Files:** `Dockerfile:12-16`; `docker-compose.yml:82-87`
- **Category:** Code Smell · **Severity:** Info · **Priority:** P3
- **Description:** Adds an extra binary (and CVE surface) to the runtime image solely so compose can `curl /health`.
- **Remediation hint:** Use a Go `-healthcheck` sub-command or a distroless image with a static probe.

### LENA-070 — `graphql.operation.query` also stored for failed parses
- **Files:** `internal/bff/graphql_tracer.go:30-45`
- **Category:** Security · **Severity:** Info · **Priority:** P3
- **Description:** Malformed/oversized queries from unauthenticated-adjacent clients are recorded verbatim in spans, increasing exporter volume under abuse (LENA-003/LENA-004).
- **Remediation hint:** Truncate or hash.

### LENA-071 — Automated tooling produced zero findings while manual review found 70+
- **Files:** `.golangci.yml`, `clients/web/eslint.config.mjs`
- **Category:** Test Gap · **Severity:** Info · **Priority:** P2
- **Description:** Clean linter output gave false confidence; static analysis is not configured to detect authz, transaction, or narrowing issues, and no SAST/secret-scanning (e.g. `gosec` with G115, `govulncheck`, `semgrep`, dependabot) runs in CI.
- **Remediation hint:** Add `govulncheck`, dependabot/renovate, and a semgrep ruleset for "SQL without user_id" patterns.

### LENA-072 — Web client decodes JWT without validation to derive the user
- **Files:** `clients/web/app/auth/AuthProvider.tsx:41-60, 95-99`
- **Category:** Security · **Severity:** Info · **Priority:** P3
- **Description:** Displaying `email` from an unverified payload is acceptable for UI, but `isAuthenticated` is derived purely from presence of an unexpired token; server rejection is only discovered on the first request.
- **Remediation hint:** Consider a `viewer` query on load to confirm the session.

### LENA-073 — No root `README.md`; onboarding relies on `AGENTS.md` and per-client READMEs
- **Files:** repository root
- **Category:** Code Smell · **Severity:** Info · **Priority:** P3
- **Description:** There is no top-level description of how to run migrations, seed, start the stack, or configure OIDC — the compose defaults (`dummy` audience, `*` CORS) are the de-facto docs.
- **Remediation hint:** Add a root README covering env vars from `config.go` and the compose profiles.

---

## Prior findings re-validated against `main`

| Prior finding | Status on `main` @ `672c61c` | Listed as |
|---|---|---|
| Rating validation / int narrowing in `resolver_recipe.go` | Still present (service validates 1–5, but wrap-around defeats it) | LENA-005 |
| `unitName` empty-string-on-miss | Still present | LENA-015 |
| `os.Exit` skipping deferred cleanup in `main.go` | Still present | LENA-009 |
| Incomplete struct mapping in `inventory/service.go` create paths | `CreateItem`/`CreateIngredient` are now complete; `CreateFoodNutrient`/`CreateFoodFlavor` still partial | LENA-016 |
| `numericToFloat64` null/zero conflation | Still present in `inventory`; other domains fixed | LENA-014 |
| Ignored `token.Audience()` / email / name claim errors | Still present | LENA-011 |
| Unauthenticated `/metrics` | Still present | LENA-008 |
| Reinvented `contains`/`containsAny` | Still present | LENA-043 |
| Fire-and-forget analytics goroutines | Still present (now with timeout + recover, still untracked) | LENA-012 |
| Dead rate-limiter IP fallback | Still present | LENA-007 |
| Nondeterministic ID lists | **Fixed** — `distinctIDs` now `slices.Sort`s; not listed | — |
| Nil-telemetry inconsistency | Still present | LENA-033 |
| Startup `panic` in `NewGraphQLHandler` | Still present | LENA-010 |

Other items from `docs/issue-remediation.md` that were verified as fixed and therefore not listed: unbounded pagination (count queries now exist), catalog mutations without admin check (`requireAdmin` in place), recipe child writes outside a transaction (now `InTx`), N+1 nutrition loading (batch queries), JWKS never refreshed (refresh-on-failure exists — but see LENA-003 for the new problem it introduces), DB startup context never cancelled (now scoped), wildcard CORS with credentials (credentials disabled for `*`).

## Limitations / areas not assessed

- **Flutter static analysis was not run.** No Flutter/Dart SDK was available; `clients/mobile` findings (LENA-027, LENA-028) are from reading source only. Widget-level bugs, deprecated API usage and lint violations were not assessed.
- **Playwright e2e suite was not executed** (requires the full Docker compose stack including the test issuer); results were inferred from the workflow definition and test sources.
- **Runtime/load behaviour** (goroutine growth under load, actual JWKS fetch amplification factor, pool exhaustion) was reasoned about from code, not measured.
- **Generated code** (`internal/*/sqlc/*.go`, gomock mocks) was consulted for types and SQL text but not audited line-by-line; issues in hand-written SQL are attributed to `queries.sql`.
- **Seed data** (`migrations/seed/0001_reference_data.sql`) was not validated for content correctness (e.g. unit conversion factors were spot-checked only).
- **Dependency vulnerability status for Go** was not checked with `govulncheck` (not part of the requested tool set); `npm audit --omit=dev` on the web client reported 0 vulnerabilities on the audit date.
- **Third-party infrastructure** (GHCR settings, branch protection, Seq/Caddy production configuration) is outside the repository and was not reviewed.
