# Remediation Phase 3 — Error contract & silent-success

- **Branch:** `audit-review-phase3` (cut from latest `main` after the Phase 2 PR is approved)
- **Theme:** Error contract & silent-success — establish typed domain errors (`ErrNotFound`,
  `ErrConflict`, `ErrValidation`), make every UPDATE/DELETE report zero-row outcomes, purge pgx/pgconn
  from the BFF, and fix the tests that currently codify silent success.
- **Source reports:** `audit/phase-3-domains.md`, `audit/phase-1-architecture.md`,
  `audit/phase-4-tests.md`, `audit/phase-2-bff.md`, `audit/summary.md` (top-15 #2, #11; theme "Silent
  success")
- **Why this phase comes now:** the shared error sentinels introduced here are the contract that
  Phase 4's transaction seam and Phase 5's conditional import transitions return through
  (`ErrConflict` on zero rows). Do not start Phase 4 before this PR is approved.

## Findings in scope

| ID | Severity | One-line summary | Target file(s) |
|---|---|---|---|
| A3-01 | High | Every UPDATE/DELETE is `:exec`; zero-row updates/deletes return `nil`, hiding not-found and ownership failures | all `internal/*/queries.sql` UPDATE/DELETE; e.g. `internal/grocery/service.go:99-101,199-222`, `internal/mealplan/service.go:104-119,193-209`, `internal/inventory/service.go:483-513`, `internal/wine/service.go:697-731` |
| A1-02 | High | No typed domain errors; BFF imports pgx/pgconn and matches `ErrNoRows` / SQLSTATE 23505 (one via string match) | `internal/bff/errors.go:57`; `internal/bff/resolver_inventory.go:1003-1019`; `internal/bff/resolver_recipe.go:92,594,631`; `internal/bff/nutrition_ocr.go:44` |
| A4-03 | High | Cross-user tests `require.NoError` on wrong-user UPDATE/DELETE, codifying the A3-01 silent-success bug | `internal/grocery/integration_test.go:220-231`; `internal/mealplan/integration_test.go` (cross-user test) |
| A3-04 | Medium | Validation errors are untyped strings → BFF returns `INTERNAL`; only `identity` exports sentinels | `internal/wine/service.go:385-396`; `internal/recipe/service.go:109-114,413-415`; `internal/analytics/service.go:96-101`; `internal/recipeimport/service.go:166-232`; `internal/bff/errors.go` (`sanitizeQueryErrors`) |
| A2-08 | Medium | `UpdateItem` and admin `CreateBrand` skip `itemWriteError`/`brandWriteError`; duplicates surface as `INTERNAL` | `internal/bff/resolver_inventory.go:459-470,1190-1202` |

All five IDs exist in the reports with the severities shown; no corrections were needed.

## Remediation steps

1. **A1-02 / A3-04 — introduce the typed error contract (do this first; everything else builds on it).**
   1. Create `internal/platform/domainerr` with `ErrNotFound`, `ErrConflict`, `ErrValidation` (plus a
      small `ValidationError{Field, Msg}` type wrapping `ErrValidation` if field detail is useful). Keep
      `identity`'s existing sentinels but alias/wrap them so `errors.Is(err, domainerr.ErrNotFound)` works.
   2. In every domain service, translate at the sqlc boundary:
      `if errors.Is(err, pgx.ErrNoRows) { return domainerr.ErrNotFound }` and
      `if pgErr.Code == "23505" { return fmt.Errorf("%w: %s", domainerr.ErrConflict, pgErr.ConstraintName) }`.
      Wrap with `%w` everywhere so callers can use `errors.Is`.
   3. Convert the untyped validation strings in `wine/service.go:385-396`, `recipe/service.go:109-114,
      413-415`, `analytics/service.go:96-101` and `recipeimport/service.go:166-232` to
      `fmt.Errorf("%w: ...", domainerr.ErrValidation)`.
   4. Extend `sanitizeQueryErrors` / `errors.go` in the BFF to map `ErrNotFound` → `NOT_FOUND`,
      `ErrConflict` → `CONFLICT`, `ErrValidation` → `BAD_USER_INPUT` once, centrally.
   5. Remove every `github.com/jackc/...` import from `internal/bff` (`errors.go:57`,
      `resolver_inventory.go:1003-1019`, `resolver_recipe.go:92,594,631`, `nutrition_ocr.go:44`) — in
      particular delete the string-match on `"23505"`. Add a `depguard` rule in `.golangci.yml` denying
      `jackc` under `internal/bff` so it cannot regress.
2. **A3-01 — make mutations report zero rows.**
   1. In every `internal/*/queries.sql`, change UPDATE/DELETE statements from `:exec` to `:execrows`
      (or `... RETURNING <pk>` with `:one`), then `sqlc generate`.
   2. In each service method: `if n == 0 { return domainerr.ErrNotFound }`. Start with the call sites the
      report lists (`grocery/service.go:99-101,199-222`, `mealplan/service.go:104-119,193-209`,
      `inventory/service.go:483-513`, `wine/service.go:697-731`) and sweep the rest of the packages
      (`identity`, `recipe`, `userprefs`, `analytics`, `recipeimport`).
   3. Remove the BFF's "read then write" ownership pre-checks that exist only to compensate for silent
      success, now that the domain reports the outcome (keep any check that has a genuinely different
      error message).
3. **A4-03 — fix the tautological cross-user tests in the same PR.**
   1. In `grocery/integration_test.go:220-231` and the `mealplan` cross-user test, replace
      `require.NoError(...)` on wrong-user UPDATE/DELETE with `assert.ErrorIs(err, domainerr.ErrNotFound)`.
   2. Keep the existing "row unchanged" assertions.
   3. Grep every other `integration_test.go` for the same pattern (wrong-user write followed by
      `NoError`) and fix them too — they will start failing after step 2 anyway.
4. **A2-08 — apply the write-error mapping at the two missing call sites.**
   1. With step 1.4 in place, `UpdateItem` (`resolver_inventory.go:459-470`) and admin `CreateBrand`
      (`:1190-1202`) should simply return the service error and let the central mapper produce
      `CONFLICT`. If the central mapping is not yet wired for these resolvers, apply
      `itemWriteError`/`brandWriteError` there as the interim fix.
   2. Add a resolver test asserting a duplicate item / brand yields `CONFLICT`, not `INTERNAL`.

## Co-located Low policy

Fix a Low finding only if it lives in code already being changed for a High/Medium finding in this
phase; never edit code solely to fix a Low. Lows likely to be co-located here: A2-13 (`resolveUnitID`
maps all errors to `BAD_USER_INPUT` — same error-mapping code as A1-02), A2-22 (`unitName` map miss →
`INTERNAL`, only if `resolver.go:420-434` is touched), A3-19 (stale "no DB CHECK" comment in
`wine/service.go:382-384`, adjacent to the A3-04 edit), A2-15 (`Draft()/Review()` swallow decode errors —
only if `resolver_recipe_import.go` is edited for error mapping).

## Verification

- `go build ./...` passes (after `sqlc generate`; commit the regenerated code).
- `go test ./...` passes (integration tests use testcontainers; Docker required).
- `golangci-lint run ./...` and `go vet ./...` report no issues, including the new `depguard` rule.
- Manual: `grep -r "jackc" internal/bff` returns nothing.
- Manual: `grep -rn ":exec$" internal/*/queries.sql` returns no UPDATE/DELETE statements (INSERTs that
  legitimately do not need a count may remain).
- Manual (GraphQL): as user B, `updateMealSlot`/`deleteGroceryListItem` against user A's row returns
  `extensions.code = "NOT_FOUND"`; creating a duplicate item or brand returns `CONFLICT`; an invalid
  `abv` on `updateBottle` returns `BAD_USER_INPUT` (was `INTERNAL`).

## Closing instruction

Open a PR from `audit-review-phase3` into `main` summarising the changes above, then **stop**. Do not
begin Phase 4 until this PR has been reviewed and approved.
