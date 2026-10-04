# LEN-23 — P4 AI allergen flagging: completion proof

**PR:** https://github.com/JRAdams472/LENA2/pull/254
**Ticket:** LEN-23 (LEN-16 P4)

## What shipped

AI-proposed ingredient/item allergen flags held in an admin review queue — **nothing is applied automatically**. Accept writes a real `contains`/`may_contain` flag under the reviewer's attribution; Dismiss keeps the audit row.

### Backend

- Migration `0048` — `inventory.allergen_suggestion` (pending/accepted/dismissed, recipe provenance, suggester/reviewer attribution, audit columns). Partial unique index: one pending proposal per (target, allergen).
- sqlc queries + domain service — `CreateAllergenSuggestions` (ErrNoRows dedup skip), `ListAllergenSuggestions`, `GetAllergenSuggestion`, `AcceptAllergenSuggestion` (flag write + status flip in one `dbtx.InTx`), `DismissAllergenSuggestion` (`WHERE status='pending'` guards double-review).
- AI tool `get_recipe_allergen_candidates` — recipe lines with target kind/id/name, household ingredient overrides resolved, existing flags, active allergen registry.
- `ai.Service.SuggestAllergens` — `prepare` → `runPrepared` → validate: target must come from the recipe, allergen must be active, already-flagged pairs and (target, allergen) dupes drop.
- GraphQL — `suggestRecipeAllergens`, `allergenSuggestions(status)`, `acceptAllergenSuggestion`, `dismissAllergenSuggestion`; all `@admin`.
- `tools.go` — pins mockgen so `go mod tidy` can't break codegen.

### Web

- `/inventory/allergen-suggestions` admin page — recipe autocomplete → "Suggest flags" → notice with created count; review table with target, allergen, flag kind, recipe, rationale, status chips, Accept/Dismiss on pending rows; status filter (pending/accepted/dismissed/all); AI-unavailable banner when no provider. Nav entry under Inventory.

## Live UAT (debug stack, real ollama provider)

- `suggestRecipeAllergens(recipeId:"1")` → 1 pending row created.
- Re-ran on the same recipe → `[]` — pending dedup verified.
- `acceptAllergenSuggestion(id:"1")` → `ingredient_allergen` row written with `created_by = aipaloovik@gmail.com`; status `accepted` + `reviewedAt` set.
- Re-accept → `NOT_FOUND` (pending-only WHERE clause verified).
- `suggestRecipeAllergens(recipeId:"5")` → 3 proposals; `dismissAllergenSuggestion(id:"3")` → `dismissed`, zero flags written on the item.
- Web: recipe picker → "Suggest flags" → "1 suggestion added to the queue" notice; Dismiss click flipped row to dismissed; status tabs verified.

### Bugs caught by real-DB UAT (not visible to mocks)

1. `SetAllergenSuggestionStatus` — Postgres `42P08` (inconsistent `$2` type across `status = $2` and the CASE literal) → `::varchar` cast.
2. `suggestRecipeAllergens` missing from `aiProviderFieldPattern` → would have run on the interactive GraphQL timeout and died during a cold model load → added to the pattern + test row.

## Tests

- `go test ./...` green — new: 4 ai suggest tests, 9 inventory subtests (create/dedup/accept-tx/accept-rollback/dismiss), 6 resolver tests (admin gate on every op).
- jest 556/556 (+5 page tests: queue render, accept, dismiss, suggest flow, reviewed-rows-hide-actions).
- `golangci-lint` 0 issues; `tsc` clean; `eslint` 0 problems.

## Notes

- Local 7B model proposals are low quality (expected at this size) — the review gate exists precisely for this; a larger provider improves proposals with no design changes.
- LEN-16 remaining: LEN-24 (P5 close-out).
