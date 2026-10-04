# LEN-37 — LEN-29 P3 authz gaps: proof of completion

PR: https://github.com/JRAdams472/LENA2/pull/TBD
Parent: LEN-29 (OWASP Top 10 review). Findings: #2 (recipe category authz), #3 (invite targeting).

## What shipped

### Finding 2 — `setRecipeCategories` is now admin-only
- `internal/bff/schema.graphqls` — `@admin` directive added (documentary; enforcement is resolver-side per codebase convention).
- `internal/bff/resolver_recipe.go` — `SetRecipeCategories` now calls `requireAdmin` instead of `userFromContext`; recipes are a global catalog, so member edits leaked across households.
- `clients/web/app/recipes/[id]/page.tsx` — category edit panel + `recipeCategoryGroups` fetch are admin-gated (`enabled: isAdmin`). Members still see the read-only category chips at the top of the page (pre-existing surface, user-approved UX).
- `clients/mobile/lib/screens/edit_recipe_screen.dart` — the category toggle now reverts its optimistic state and shows a snackbar when the mutation is rejected, instead of leaving the UI claiming an unsaved change.
- `docs/recipe-categories-plan.md` — "members-assign" design note annotated as superseded by LEN-29/LEN-37.

### Finding 3 — invite targeting hardened
`internal/bff/resolver_household.go` `InviteHouseholdMember`:
- Target must be `IsActive && IsSearchable` — mirrors the existing `SearchUsers` semantics, closes the "invite arbitrary user ID" bypass of the discoverability opt-in.
- Every target-side rejection (unknown ID, inactive, unsearchable, already a household mate, duplicate pending invite) returns the identical `BAD_USER_INPUT: cannot invite this user` — no enumeration signal.
- New `invites *userRateLimiter` on `Resolver` (10/min/user, lazily built like `uploads`/`aiCalls`) — invite spam / sequential ID probing is throttled before any store call.
- Caller-side rejections stay distinct: self-invite, no household, household full, rate-limited (`BUSY`).

## Tests

- `resolver_recipe_categories_test.go` — success + exclusivity cases moved to the admin context; new "rejects non-admin members" case asserts FORBIDDEN before any store call.
- `resolver_household_test.go` — new `GenericTargetErrors` table test proves unknown/inactive/unsearchable/duplicate-pending all return byte-identical errors; new `RateLimited` test exercises the limiter; fixtures updated to active+searchable targets.
- `bff_integration_test.go` — `inviteAndAccept` and the events invite now opt the target into `isSearchable` via `updateMyProfile` (matches real UX).
- `tools/wiki-shots/seed_demo.py` — seeder opts the demo member in; "already joined" catch updated to the new generic error.
- Web jest — new member test asserts chips render, picker absent, and no `recipeCategoryGroups` request fires.

## Verification

- `go build ./...` clean; `go test ./internal/bff/` green including the testcontainers integration suite
- `golangci-lint run ./internal/bff/...` — 0 issues
- Web: `tsc --noEmit` clean; jest `__tests__/app/recipes/` 51/51
- Mobile: `flutter test` 104/104; `dart analyze` — 3 pre-existing info-level lints only
- E2e impact assessed: e2e specs mint the seeded admin user (`e2e-user-1`) so the picker remains; no e2e spec exercises invites by raw user ID
