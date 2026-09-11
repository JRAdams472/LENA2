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
