# Remediation Phase 4 — Concurrency, atomicity & tx seam

- **Branch:** `audit-review-phase4` (cut from latest `main` after the Phase 3 PR is approved)
- **Theme:** Concurrency, atomicity & tx seam — give multi-domain writes a first-class transaction
  seam, replace check-then-act patterns with single atomic SQL statements, remove the BFF's concrete
  downcasts and nil-pool fallbacks, and add the concurrency tests that prove the races are gone.
- **Source reports:** `audit/phase-2-bff.md`, `audit/phase-1-architecture.md`,
  `audit/phase-3-domains.md`, `audit/phase-4-tests.md`, `audit/summary.md` (top-15 #4, #14; theme
  "Check-then-act races")
- **Prerequisite:** Phase 3 typed errors (`ErrNotFound`/`ErrConflict`) — the conditional statements
  below return `ErrConflict`/`ErrLastAdmin` on zero rows. The `UnitOfWork` seam built here is in turn a
  prerequisite for Phase 5's atomic recipe-import `Approve`.

## Findings in scope

| ID | Severity | One-line summary | Target file(s) |
|---|---|---|---|
| A2-01 | High | Read-modify-write pantry updates inside `InTx` without `FOR UPDATE`/atomic SQL; concurrent toggles double-apply and lose updates | `internal/bff/resolver_grocery.go:132-199`; `internal/bff/resolver_userprefs.go:209-224` |
| A1-03 | High | BFF downcasts service interfaces to concrete `*Service`, runs `dbtx.InTx` itself, and has a non-transactional fallback path used only by tests | `internal/bff/resolver_grocery.go:128-132`; `internal/bff/resolver_userprefs.go:211-214`; `internal/platform/dbtx` |
| A4-01 | High | Transactional grocery→pantry sync never executed under test (29.8% fn coverage); unit tests use `Pool == nil`, integration toggles a manual item | `internal/bff/resolver_grocery.go:130-190`; `internal/bff/resolver_grocery_test.go`; `internal/bff/bff_integration_test.go:559-593` |
| A3-05 | Medium | Last-admin guard is check-then-act outside a transaction; concurrent demotions can leave zero admins | `internal/identity/service.go:160-213`; `internal/identity/queries.sql` |
| A3-06 | Medium | No atomic quantity adjust; full-row upsert ignores `UserItemID` — root cause of A2-01 | `internal/userprefs/service.go:61-90`; `internal/userprefs/queries.sql:1-16` |
| A3-07 | Medium | `DeleteItem` is two statements without a tx; inventory deletes `user_item` rows owned by userprefs | `internal/inventory/service.go:505-513`; `internal/inventory/queries.sql:165-171` |
| A1-06 | Medium | `DeleteItem` executes two dependent deletes outside a transaction (architecture-report view of A3-07) | `internal/inventory/service.go:505-513` |
| A3-08 | Medium | `SubmitBrand` check-then-create; resurfaces rejected/foreign pending brands; normalized name unindexed and not unique | `internal/inventory/service.go:79-97`; `internal/inventory/queries.sql:19-34`; new migration |
| A3-12 | Medium | `InTx` on a tx-bound service opens a new pool transaction; `runInTx` nil-pool test seam in production | all `WithTx`/`InTx` (e.g. `internal/grocery/service.go:34-43`); `internal/inventory/service.go:745-766`; `internal/analytics/service.go:170-178` |
| A4-10 | Medium | No concurrency tests for A2-01/A3-05/A3-06/A3-08; `-race` cannot detect DB-level lost updates | repository-wide (`internal/*/integration_test.go`, `internal/bff/bff_integration_test.go`) |

**Roadmap corrections / additions.** All IDs supplied in the roadmap exist with the severities shown.
**A1-06 (Medium)** was not assigned to any phase in the roadmap; it is the architecture report's view of
the same defect as A3-07 and is added here so it is closed by the same change.

## Remediation steps

1. **A3-12 — fix the `WithTx`/`InTx` composition trap first (foundation for everything below).**
   1. Have `WithTx(tx)` set a `tx pgx.Tx` field on the returned service and make `InTx` reuse it
      (`if s.tx != nil { return fn(s) }`), or use `tx.Begin` to open a savepoint. Apply to every domain
      service that exposes the pair (`grocery`, `identity`, `inventory`, `mealplan`, `userprefs`, `wine`,
      `analytics`, `recipe`).
   2. Remove `runInTx`'s nil-pool branch (`inventory/service.go:745-766`, `analytics/service.go:170-178`
      and equivalents); tests use a fake `dbtx.Pool` instead.
2. **A1-03 — a first-class unit-of-work seam.**
   1. Option (a), preferred: add a `UnitOfWork` interface to `platform/dbtx` —
      `InTx(ctx, func(ctx context.Context) error) error` — that stores the `pgx.Tx` in `ctx`; each
      service's querier resolves `dbtx.FromContext(ctx)` (falling back to the pool). The BFF then never
      sees `pgx.Tx`, and mocks implement `UnitOfWork` trivially.
      Option (b): a dedicated composite service (e.g. `internal/pantry`) owning "check grocery item ⇒
      adjust pantry" as one atomic operation.
   2. Delete the `*grocery.Service`/`*userprefs.Service` type assertions at `resolver_grocery.go:128-132`
      and `resolver_userprefs.go:211-214` and the non-transactional fallback branch.
3. **A3-06 — atomic pantry adjustment in `userprefs`.**
   1. Add `AdjustUserItemQuantity(ctx, userID, itemID, delta, by)` backed by
      `INSERT … ON CONFLICT DO UPDATE SET current_qty = GREATEST(0, inventory.user_item.current_qty +
      EXCLUDED.current_qty), updated_by = …, updated_at = now() RETURNING *`.
   2. Fix the full-row upsert in `userprefs/queries.sql:1-16` so it honours `UserItemID`.
   3. Consider `CHECK (current_qty >= 0)` (none exists today) in the same migration as step 5.
4. **A2-01 — single-statement grocery toggle + pantry adjust.**
   1. Move the two operations into the owning services as single SQL statements:
      `UPDATE grocery.grocery_list_item SET is_checked = NOT is_checked … RETURNING *` (or better an
      explicit `setGroceryItemChecked(id, checked: Boolean!)` so retries are idempotent — schema change
      requires updating `clients/web` and `clients/mobile` callers, or keep the old mutation as a thin
      wrapper) and the `AdjustUserItemQuantity` from step 3.
   2. Compose them inside the `UnitOfWork` from step 2; if any read-modify-write remains, lock the row
      with `FOR UPDATE`.
   3. Apply the same treatment to `resolver_userprefs.go:209-224` (`adjustUserItemQuantity`).
5. **A3-08 — brand dedup without the race.**
   1. Migration: generated column `name_normalized` on `inventory.brand` with a unique (expression) index.
   2. `SubmitBrand` lookup: filter by `status <> 'rejected'` and `(status = 'approved' OR
      submitted_by_user_id = $2)` so rejected/foreign pending brands are not resurfaced.
   3. Insert and handle `23505` as `domainerr.ErrConflict` (Phase 3 sentinel) instead of the pre-check.
   4. Point `SearchBrands` at the normalized column (prefix/trigram index optional).
6. **A3-05 — last-admin guard as a conditional update.**
   1. Run guard + write in `InTx` and make the write conditional, declared `:execrows`:
      `UPDATE identity.users SET role = $2 WHERE user_id = $1 AND ($2 = 'admin' OR (SELECT COUNT(*) FROM
      identity.users WHERE role = 'admin' AND is_active) > 1)`; return `ErrLastAdmin` on zero rows. Apply
      the same to the deactivate path.
   2. Drop the duplicated self-check (`service.go:182-184` vs `161-163`).
7. **A3-07 / A1-06 — atomic `DeleteItem` that respects ownership.**
   1. Either wrap `DeleteUserItemsByItem` + `DeleteItem` in `s.InTx` **and** route the `user_item`
      delete through `userprefs` via the unit of work (the BFF orchestrates), or replace the manual
      pre-delete with an explicit `ON DELETE CASCADE` (or `RESTRICT`) on `inventory.user_item.item_id`
      and drop `DeleteUserItemsByItem` from `inventory/queries.sql:165-171`.
   2. Record the chosen ownership rule (feeds Phase 6, A1-05).
8. **A4-01 / A4-10 — tests that exercise the seam and the races.**
   1. Integration test (`bff_integration_test.go`): create a catalog item, add a grocery item with
      `itemId`, toggle twice, assert pantry `currentQty` moves by `quantityNeeded` and returns to the
      original.
   2. Rollback test: force a failure after the grocery update and assert nothing persisted.
   3. Concurrency tests (N goroutines against the same row, assert the invariant): concurrent toggles →
      exact final quantity (A2-01/A3-06); concurrent demotions → exactly one admin remains (A3-05);
      concurrent `SubmitBrand` with the same normalized name → one brand row (A3-08).
   4. Replace the `Pool == nil` unit tests in `resolver_grocery_test.go` with tests against a fake
      `UnitOfWork`.

## Co-located Low policy

Fix a Low finding only if it lives in code already being changed for a High/Medium finding in this
phase; never edit code solely to fix a Low. Lows likely to be co-located here: A4-16 (tx binding untested
outside grocery — add a binding test per domain while touching each `WithTx`), A3-18 (case-insensitive
lookups vs `UNIQUE` on `inventory.brand`/`item` — if the A3-08 migration already adds a normalized
unique index, extend it), A3-16 (duplicated pgtype helpers — only if a helper file is already being
edited).

## Verification

- `go build ./...` passes (after `sqlc generate`; commit regenerated code and the new migration).
- `go test ./...` passes, including the new concurrency tests, with `-race`.
- `golangci-lint run ./...` and `go vet ./...` report no issues.
- Manual: `grep -rn "\.(\*grocery.Service)\|\.(\*userprefs.Service)" internal/bff` returns nothing;
  `grep -rn "Pool == nil" internal` returns nothing.
- Manual: run 20 parallel `toggleGroceryListItem` mutations on one item via a small script and assert
  `currentQty` ends at the expected value.
- Manual: two admins, two concurrent `setUserRole(member)` calls → one succeeds, one returns the
  last-admin error.
- Manual: `submitBrand("Acme ")` and `submitBrand("acme")` concurrently → one row in `inventory.brand`.

## Closing instruction

Open a PR from `audit-review-phase4` into `main` summarising the changes above, then **stop**. Do not
begin Phase 5 until this PR has been reviewed and approved.
