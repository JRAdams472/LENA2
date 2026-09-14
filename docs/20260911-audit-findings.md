# LENA2 Code Audit — Running Findings Log (2026-09-11)

Branch: `audit`. Baseline: `main` @ `c4ad3c7`.
One entry per finding; IDs are `A<phase>-<nn>`. Full write-ups live in `audit/phase-<N>-*.md`. Later phases append their own section below; do not renumber earlier entries.

Format: `ID | Severity | Category | Location | Summary`

## Phase 1 — Architecture & design (`audit/phase-1-architecture.md`)

| ID | Severity | Category | Location | Summary |
|----|----------|----------|----------|---------|
| A1-01 | High | poor design | `internal/recipe/queries.sql:129-150`; `internal/analytics/queries.sql:66-100` | Cross-domain SQL joins into `mealplan.*` / `recipe.*` violate the documented one-schema-per-module rule |
| A1-02 | High | leaky abstraction | `internal/bff/errors.go:57`; `resolver_inventory.go:1003-1019`; `resolver_recipe.go:92,594,631`; `nutrition_ocr.go:44` | No typed domain errors; BFF imports pgx/pgconn and matches `ErrNoRows` / SQLSTATE 23505 (one via string match) |
| A1-03 | High | antipattern / layering | `internal/bff/resolver_grocery.go:128-132`; `resolver_userprefs.go:211-214` | BFF downcasts service interfaces to concrete `*Service`, runs `dbtx.InTx` itself, and has a non-transactional fallback path used only by tests |
| A1-04 | High | bug / poor design | `internal/grocery/service.go:227-231`; `internal/bff/resolver_grocery.go:86-110` | `generateGroceryList` creates an empty list; aggregation was deferred to "the BFF" and never implemented |
| A1-05 | Medium | poor design | `migrations/0006_create_userprefs.up.sql`; `internal/userprefs/queries.sql`; `internal/inventory/queries.sql:165-167` | `userprefs` owns no schema; its tables sit in `inventory`/`wine`/`recipe` and `inventory` also writes `inventory.user_item` |
| A1-06 | Medium | bug | `internal/inventory/service.go:505-513` | `DeleteItem` executes two dependent deletes outside a transaction |
| A1-07 | Medium | coupling | `internal/recipeimport/service.go:13-21`; `catalog.go:11-14` | `recipeimport` imports sibling domains `recipe` + `inventory`, concrete OCR/Ollama clients, and `platform/config` |
| A1-08 | Medium | poor design | `internal/bff/services.go:36-98,214-260` | Full-surface service interfaces (63/45 methods) instead of role interfaces; 2 877-line generated mock |
| A1-09 | Medium | layering / complexity | `internal/bff/nutrition_ocr.go:20-77`; `recipe_scan.go:19-84` | Domain logic (nutrient-type auto-creation, inbox file writes) implemented in BFF |
| A1-10 | Medium | poor design | `internal/bff/resolver.go:38-71,73-136` | `Resolver` is GraphQL root + DI container + background worker pool + config bag; 14-arg constructor |
| A1-11 | Medium | poor design | `internal/bff/schema.graphqls:153-154,173,196,221,266,346,498,792` | No GraphQL enums; roles/statuses/sources are `String!`; admin-only marked by comments |
| A1-12 | Medium | layering | `internal/platform/testenv/testenv.go:129` | Platform package imports domain `identity` |
| A1-13 | Medium | code smell | `internal/recipeimport/service.go:137-163` | `ListPending` issues up to 8 sequential per-status queries and paginates incorrectly |
| A1-14 | Low | code smell | `internal/platform/validator/validator.go` | Unused package |
| A1-15 | Low | code smell | `internal/ocrimport/workqueue.go`; `review.go:28-52,98-180` | File-based work queue / review IO unreferenced by any binary |
| A1-16 | Low | poor design | `internal/platform/config/config.go:13-106,117-131` | Single flat `Config` across server, OCR, Ollama, import; `ValidateServer` workaround |
| A1-17 | Low | poor design | `internal/bff/resolver.go:379-813` | Bespoke preloading with `if map != nil` dual paths per child resolver instead of a request-scoped DataLoader |
| A1-18 | Low | process | `docs/` | Requested spec docs (`go-rewrite-spec.md`, `lena-go-postgres-rewrite-plan.md`) absent; no ADRs; `architecture-hardening.md` stale |

Phase 1 totals: 0 critical · 4 high · 9 medium · 5 low.

## Phase 2 — BFF & GraphQL API layer (`audit/phase-2-bff.md`)

| ID | Severity | Category | Location | Summary |
|----|----------|----------|----------|---------|
| A2-01 | High | bug | `internal/bff/resolver_grocery.go:132-199`; `resolver_userprefs.go:209-224` | Read-modify-write pantry updates inside `InTx` without `FOR UPDATE`/atomic SQL; concurrent toggles double-apply and lose updates |
| A2-02 | High | bug | `internal/bff/auth.go:81-98,159-165` | Auth middleware turns every failure (DB/JWKS outages included) into an unlogged `401 invalid token` |
| A2-03 | Medium | bug | `internal/bff/resolver.go:112-136`; `cmd/lena/main.go:117-129` | `Shutdown` cancels `bgCtx` before waiting and never waits on recipe-import drain; in-flight analytics/OCR work is aborted |
| A2-04 | Medium | poor design | `internal/bff/auth.go:195-224` | Global mutex held across OIDC discovery + JWKS fetch (and on cache hits); slow issuer stalls all auth; fetch uses request ctx; no stale-while-revalidate |
| A2-05 | Medium | poor design | `internal/bff/auth.go:159-180` | `UpsertUser` (and possibly `SetUserRole`) executed on every request; no caching of resolved user |
| A2-06 | Medium | bug (possible) | `internal/bff/resolver_mealplan.go:194-232` | `Nutrition` multiplies basis-less `food_nutrient.amount` by quantities in arbitrary units; totals dimensionally meaningless |
| A2-07 | Medium | antipattern (N+1) | `resolver_grocery.go:17-31,318-327,364-390`; `resolver_recipe.go:73-130`; `resolver_recipe_import.go:203-214` | Single-object / nested paths (`groceryList`, `ingredient`, `scaledRecipe.items.item`, `recipeImport.recipe`) fall through to per-row lazy queries |
| A2-08 | Medium | bug | `internal/bff/resolver_inventory.go:459-470,1190-1202` | `UpdateItem` and admin `CreateBrand` skip `itemWriteError`/`brandWriteError`; duplicates surface as `INTERNAL` |
| A2-09 | Medium | bug | `internal/bff/resolver_recipe_import.go:33-76` | Recipe-import lists have no `pageSize` upper clamp, echo raw page args in `pageInfo`, and report `total = len(items)` |
| A2-10 | Medium | poor design | `cmd/lena/main.go:179-182,248-269`; `internal/bff/resolver.go:819-838` | No per-request deadline/cost limit around `Exec`; resolvers outlive disconnected clients; bind errors bypass GraphQL error shape |
| A2-11 | Medium | error handling | `internal/bff/nutrition_ocr.go:36-62,115-118`; `resolver.go:83-90` | `submitItemNutritionPhoto` returns `true` when `runAsync` drops the task; check-then-create nutrient types race; actor recorded as `"ocr-system"` |
| A2-12 | Medium | cognitive complexity | `resolver_mealplan.go:103-243`; `resolver_grocery.go:112-227`; `resolver_wine.go:310-419`; `resolver_recipe.go:52-140`; `resolver_inventory.go:1128-1213,238-318` | 12 resolvers >60 lines; domain algorithms, manual PATCH merging and hand-rolled batching inline |
| A2-13 | Low | bug | `internal/bff/resolver.go:410-416` | `resolveUnitID` maps all service errors to `BAD_USER_INPUT "unknown unit"` |
| A2-14 | Low | antipattern (N+1) | `internal/bff/resolver_recipe.go:176-213` | One `GetUnitByName` query per recipe item on create/update |
| A2-15 | Low | error handling | `internal/bff/resolver_recipe_import.go:181-214` | `Draft()`/`Review()` return `nil` on JSON decode error without logging; `Recipe()` ignores missing user |
| A2-16 | Low | antipattern | `internal/bff/resolver_analytics.go:16-78` | Analytics mutations always return `true`; `entityType` unvalidated; >500-char terms fail silently; users can inflate global popularity |
| A2-17 | Low | code smell | `internal/bff/resolver_inventory.go:175-234,238-318` | `FrequentBrands`/`FrequentItems` duplicated blend logic; selection counts queried twice |
| A2-18 | Low | poor design | `resolver_wine.go:326-397`; `resolver_inventory.go:1146-1152`; `resolver_recipe.go:250-310` | Pointer update inputs cannot clear nullable fields (e.g. `brandId`, `abv`) |
| A2-19 | Low | antipattern | `internal/bff/graphql_tracer.go:50-62,105-110` | Span names from client `operationName` (unbounded cardinality); span status carries pre-sanitised error text; only first error recorded |
| A2-20 | Low | bug | `internal/bff/recipe_scan.go:61-83` | File written before DB insert; failure leaves orphan file in inbox |
| A2-21 | Low | poor design | `internal/bff/ratelimit.go:9-56`; `cmd/lena/main.go:176,262-264` | In-memory per-process limiter keyed on XFF-derived IP; single budget regardless of operation cost |
| A2-22 | Low | error handling | `internal/bff/resolver.go:420-434` | Preloaded-map miss in `unitName` returns `INTERNAL` instead of falling back |

Phase 2 totals: 0 critical · 2 high · 10 medium · 10 low.

## Phase 3 — Domain services (`audit/phase-3-domains.md`)

| ID | Severity | Category | Location | Summary |
|----|----------|----------|----------|---------|
| A3-01 | High | bug | all `internal/*/queries.sql` UPDATE/DELETE; e.g. `grocery/service.go:99-101,199-222`, `mealplan/service.go:104-119,193-209`, `inventory/service.go:483-513`, `wine/service.go:697-731` | Every mutation is `:exec`; zero-row updates/deletes return `nil`, hiding not-found and ownership failures |
| A3-02 | High | bug / concurrency | `internal/recipeimport/service.go:166-252,316-327`; `recipeimport/queries.sql:31-92` | Unguarded status transitions; `Approve` non-atomic and re-runnable (duplicate recipes); admin decisions overwritable by worker |
| A3-03 | High | bug | `internal/recipeimport/service.go:330-358` | In-memory job queue: orphaned jobs after restart, no per-job timeout, `Retry` during `processing` runs two workers |
| A3-04 | Medium | poor design | `wine/service.go:385-396`; `recipe/service.go:109-114,413-415`; `analytics/service.go:96-101`; `recipeimport/service.go:166-232` | Validation errors are untyped strings → BFF returns `INTERNAL`; only `identity` exports sentinels |
| A3-05 | Medium | concurrency | `internal/identity/service.go:160-213` | Last-admin guard is check-then-act outside a transaction; concurrent demotions can leave zero admins |
| A3-06 | Medium | bug | `internal/userprefs/service.go:61-90`; `userprefs/queries.sql:1-16` | No atomic quantity adjust; full-row upsert ignores `UserItemID` — root cause of A2-01 |
| A3-07 | Medium | bug / poor design | `internal/inventory/service.go:505-513`; `inventory/queries.sql:165-171` | `DeleteItem` two statements without tx; inventory deletes `user_item` rows owned by userprefs |
| A3-08 | Medium | bug / concurrency | `internal/inventory/service.go:79-97`; `inventory/queries.sql:19-34` | `SubmitBrand` check-then-create; resurfaces rejected/foreign pending brands; normalized name unindexed and not unique |
| A3-09 | Medium | bug | `internal/recipeimport/service.go:137-163` | `ListPending` applies same offset to eight per-status queries; pages skip/duplicate rows |
| A3-10 | Medium | bug | `internal/ocrimport/review.go:57-64`; `ocrimport/catalog.go:263-268`; `recipeimport/service.go:230-232,446-449` | `AllResolved` accepts fuzzy "suggested" matches; imports auto-mark `ready` and persist without admin acceptance |
| A3-11 | Medium | poor design | `internal/grocery/service.go:224-231` | `Generate` creates an empty list; no layer implements aggregation (confirms A1-04) |
| A3-12 | Medium | poor design | all `WithTx`/`InTx` (e.g. `grocery/service.go:34-43`); `inventory/service.go:745-766`; `analytics/service.go:170-178` | `InTx` on a tx-bound service opens a new pool transaction; `runInTx` nil-pool test seam in production |
| A3-13 | Medium | cognitive complexity | `internal/recipeimport/service.go:350-455`; `ocrimport/workqueue.go:18-27` | 105-line `Process` driven by magic status strings; two incompatible status vocabularies |
| A3-14 | Low | performance | `analytics/queries.sql:66-100`; `recipeimport/catalog.go:41-98`; `ocrimport/catalog.go:222-233` | O(users×recipes) overlap query per recipe create; full catalog load + fuzzy sweep per import job |
| A3-15 | Low | bug | `internal/identity/queries.sql:12-31`; `migrations/0002_create_identity.up.sql:13` | Email not unique; `UpsertUser` nulls `display_name` on empty claim |
| A3-16 | Low | code smell | `internal/{analytics,grocery,identity,inventory,mealplan,recipe,userprefs,wine}/service.go` | pgtype helper functions duplicated in eight packages |
| A3-17 | Low | code smell | `internal/ocrimport/workqueue.go`; `ocrimport/review.go:27-52,67-205` | Dead CLI-era file-backed queue/review code with no non-test callers |
| A3-18 | Low | bug | `inventory/queries.sql:89-92,183-186,366-369`; `migrations/0003_create_inventory.up.sql:30,56` | Case-insensitive lookups vs case-sensitive `UNIQUE`; `:one` returns arbitrary row on near-duplicates |
| A3-19 | Low | code smell | `inventory/queries.sql:183-186,268-272`; `identity/queries.sql:33-37`; `wine/service.go:382-384` | Inconsistent audit columns; stale "no DB CHECK" comment |
| A3-20 | Low | bug | `internal/recipeimport/service.go:330-347,513-526` | Goroutine-per-job fan-out; double `Shutdown` panics; `MarkFailed` error discarded |
| A3-21 | Low | poor design | `mealplan/service.go:134-152`; `grocery/service.go:121-149`; `userprefs/service.go:61-90,169-200`; `inventory/service.go:177-191,416-430` | Near-zero domain validation; relies on DB `CHECK`s that surface as `INTERNAL` |

Phase 3 totals: 0 critical · 3 high · 10 medium · 8 low.

## Phase 4 — Unit & integration tests (`audit/phase-4-tests.md`)

Execution: `go test -race -count=1 -coverprofile ... ./cmd/... ./internal/...` — all packages pass once `postgres:16-alpine` is available locally (first run: 22 integration tests failed on Docker Hub `429 Too Many Requests`). Coverage: 65.7% filtered (hand-written code, CI floor 60%), 41.6% raw. 215 hand-written functions at 0%.

| ID | Severity | Category | Location | Summary |
|----|----------|----------|----------|---------|
| A4-01 | High | bug (test gap) | `bff/resolver_grocery.go:130-190`; `bff/resolver_grocery_test.go`; `bff/bff_integration_test.go:559-593` | Transactional grocery→pantry sync never executed under test (29.8% fn coverage); unit tests use `Pool == nil`, integration toggles a manual item |
| A4-02 | High | poor design (test gap) | `internal/recipeimport/*` (20%; `Process`, `Retry`, `Shutdown`, `store.go` 0%); `bff/resolver_recipe_import.go` (0%, 69 funcs); `ocrimport/review.go`, `workqueue.go` (0%) | Recipe-import pipeline, SQL store, and admin GraphQL surface effectively untested |
| A4-03 | High | bug (tautological assertion) | `grocery/integration_test.go:220-231`; `mealplan/integration_test.go` cross-user test | Tests `require.NoError` on wrong-user UPDATE/DELETE, codifying the A3-01 silent-success bug |
| A4-04 | Medium | security (test gap) | `bff/bff_integration_test.go:51,148` | E2E suite runs every mutation as admin; no HTTP-level non-admin authorization test |
| A4-05 | Medium | code smell | `bff/bff_integration_test.go:684-711` | `doGraphQL` discards decode errors and only logs GraphQL errors; partial-success responses pass |
| A4-06 | Medium | security (test gap) | `bff/nutrition_ocr.go`; `bff/recipe_scan.go`; `platform/ocrclient/` (no tests) | OCR/scan upload paths (data-URI parsing, inbox writes, async fan-out) 0% covered |
| A4-07 | Medium | bug (test gap) | `inventory/service.go:79-194`; `bff/resolver_inventory.go:73-175,1014` | Brand moderation flow (`SubmitBrand`, `SearchBrands`, `PendingBrands`, `Approve/RejectBrand`, `brandWriteError`) 0% covered |
| A4-08 | Medium | antipattern | `internal/*/service_test.go`; `bff/resolver_*_test.go` (353 `gomock.Any()`) | Two mock layers mirror the implementation; SQL predicates and `:exec` semantics never observed by unit tests |
| A4-09 | Medium | code smell | `recipeimport/service_test.go:18-201,252-270` | Hand-rolled `memoryStore` diverges from SQL store; `Approve` tested only from pre-set `ready`; setup errors discarded |
| A4-10 | Medium | bug (test gap) | repository-wide | No concurrency tests for A2-01/A3-05/A3-06/A3-08; `-race` cannot detect DB-level lost updates |
| A4-11 | Low | code smell (flaky) | `bff/resolver_misc_test.go:190-229` | Async-worker test relies on `time.Sleep(50ms)` and asserts a non-event |
| A4-12 | Low | antipattern | `platform/testenv/testenv.go`; every `integration_test.go` | One Postgres container per test function (~8 min wall); unauthenticated Docker Hub pull failed with 429 |
| A4-13 | Low | poor design | `.github/workflows/test.yml`; `tools/coveragefilter/main.go` | Single 60% global floor hides 0% files/packages; filter is path-based only |
| A4-14 | Low | code smell | `bff/bff_integration_test.go:137-139` | Test mutates `Authenticator` private state instead of injecting a clock |
| A4-15 | Low | poor design (test gap) | `cmd/lena/main_test.go`; `cmd/lena/main_integration_test.go` | Bootstrap tests cover CORS/health/401 only; shutdown, rate-limit wiring, body limits untested |
| A4-16 | Low | code smell (test gap) | `{identity,mealplan,userprefs,wine}/service.go` `WithTx`/`InTx` (0%) | Transaction binding untested outside grocery; A3-12 composition never exercised |

Phase 4 totals: 0 critical · 3 high · 7 medium · 6 low.

## Phase 5 — Docker, deployment & CI (`audit/phase-5-docker-deploy.md`)

Note: the task brief's premise that `.github/workflows/` is empty is stale — `test.yml` (build/vet/govulncheck/gofmt/race tests/coverage gate/lint/web/e2e/Flutter/publish) and `cleanup.yml` exist. Findings below audit the pipeline that exists.

| ID | Severity | Category | Location | Summary |
|----|----------|----------|----------|---------|
| A5-01 | High | security | `docker-compose.yml:6-8,24,59`; `docs/deployment.md:32,45,104` | API and migrations run as the PostgreSQL superuser; documented `lena_app` least-privilege role is not implemented |
| A5-02 | High | bug | `Dockerfile:22-26`; `docker-compose.yml:70-71`; `bff/recipe_scan.go:61` | Bind mount `./import:/data/import` overrides image `chown`; `nobody` cannot write inbox → recipe-scan uploads fail on fresh checkout |
| A5-03 | Medium | security | `docker-compose.yml:60,62`; `test.yml:265`; `clients/web/Dockerfile:13-16` | `dummy` fallbacks for Google client ID / auth audiences let a non-functional auth config start silently |
| A5-04 | Medium | security | `docker-compose.yml:66` | Personal e-mail hard-coded as default `LENA_PROTECTED_EMAILS` |
| A5-05 | Medium | poor design | all `Dockerfile`s; `docker-compose.import.yml:5,30`; `go.mod:3` | Mutable image tags, no digests, `ollama:latest`; builder Go 1.27 vs `go.mod` 1.26.6; mixed Alpine bases |
| A5-06 | Medium | poor design | `Dockerfile:9-10`; `.dockerignore` | `COPY . .` ships whole repo into builder; `.dockerignore` misses `clients/mobile`, `tools`, `import`, `audit`, CSVs |
| A5-07 | Medium | security | `docker-compose.yml:82-88,103-107,127-129,138-139` | Seq UI (5341) and GELF UDP (12201) published on all interfaces, unauthenticated; logs leave network as plaintext UDP |
| A5-08 | Medium | poor design | `docker-compose.yml:75-76,82-88,100-107` | GELF driver + `seq-gelf` mandatory for `api`/`web`; daemon-side `localhost` breaks on Desktop/rootless/remote Docker |
| A5-09 | Medium | security | `.github/workflows/test.yml` (all `uses:`), `:42` | Actions pinned by major tag not SHA; `govulncheck@latest`; no top-level `permissions:` |
| A5-10 | Medium | bug | `docker-compose.yml:32-49`; `migrations/seed/` | Seed job re-runs every `up`, outside migration versioning; API blocked on seed success; CSV unreferenced |
| A5-11 | Low | security | `docker-compose.yml:24,59`; `config.go:36-39` | `sslmode=disable` default; `OTLPInsecure` defaults `true` |
| A5-12 | Low | poor design | `Caddyfile`; `docker-compose.yml:112-113` | No TLS, security headers, body-size limit, or timeouts at edge; `/ready` not routed |
| A5-13 | Low | code smell | `docker-compose*.yml` `container_name` | Fixed container names prevent parallel stacks |
| A5-14 | Low | code smell | `README.md:225,264`; `docs/deployment.md` | Docs reference non-existent `ci.yml`/`docker.yml`, `lena_app`, `LENA_DB_PASSWORD`, `auto_https off`, unprefixed env names |
| A5-15 | Low | poor design | `test.yml:134-195,223-268` | E2E and publish build separately (published image ≠ tested image); `needs` omits `lint`; no cache/SBOM/provenance/scan; Docker Hub pulls unauthenticated |
| A5-16 | Low | poor design | `tools/ocr/Dockerfile`; `tools/ocr/app.py:120-146` | OCR sidecar runs as root; no upload size/page cap; 300 dpi rasterisation unbounded |
| A5-17 | Low | code smell | `scripts/run-local.ps1:35-42,59-66` | Windows-only helper exports all `.env` secrets; infinite health loop; no POSIX equivalent |
| A5-18 | Low | poor design | `Dockerfile:15`; `docker-compose.yml:77-81`; `main.go:224-234` | `curl` in runtime image only for probe; compose probes `/health` (no DB check) instead of `/ready`; no image `HEALTHCHECK` |

Phase 5 totals: 0 critical · 2 high · 8 medium · 8 low.
