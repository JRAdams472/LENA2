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
