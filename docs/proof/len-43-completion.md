# LEN-43 / P3 — Go complexity: ai, domains, cmd

Phase 3 of the LEN-28 SonarQube megaplan. All remaining Go findings outside
`internal/bff` cleared: **SonarQube rescan reports 0 `go:S3776`, `go:S107`,
`go:S1192`, and `go:S1186` findings project-wide**. Repo-wide open issues
dropped **145 → 109** (remaining issues are TypeScript/Docker/Python, scoped
to P4/P5).

## What changed (pure refactor — behavior unchanged)

### S3776 cognitive-complexity splits (all → ≤15)

| Function | Was | Approach |
|---|---|---|
| `cmd/lena/main.go` `newServer` | 33 | `serverServices` bundle + `newDomainServices`, `wireAI`, `newEcho`, `registerAuthRoutes` |
| `cmd/allergencurate/main.go` `proposeBatch` | 22 | prompt build, provider-reply parse, proposal conversion |
| `ai/service.go` `Ask` | — | tool-call processing loop extracted |
| `ai/suggest.go` | — | meal preparation + suggestion filtering |
| `ai/suggest_allergens.go` `filterAllergenProposals` | — | target index + per-proposal validation |
| `ai/suggest_drinks.go` | — | pairing/cocktail filtering |
| `ai/suggest_event.go` `filterEventFixes` | 30 | step index + per-action validation |
| `ai/tools/tastes.go` (tool closure) | 50 | `householdTastes`, `userTastes`, name resolution, `parseLimitArg` |
| `ai/tools/validate.go` `validateValue` | 35 | object/scalar/array/bound validators; integer wording preserved |
| `ai/tools/pantry.go` `pantryRows` | 43 | metadata lookups + `toPantryRow` per-row conversion |
| `ai/tools/recipes.go` `attachIngredients` | 39 | ID collection, name maps, row grouping |
| `ai/tools/cellar.go` `cocktailRows` | 37 | ID selection, name maps, ingredient grouping |
| `ai/tools/events.go` `buildEventTimeline` | 25 | slot construction + output conversion |
| `ai/tools/allergens.go` `allergenCandidates` | 31 | lookups bundle, flag mapping, per-item conversion |
| `event/timeline.go` `scheduleRecipe` | — | step-placement loop extracted |
| `event/timeline.go` `topoOrder` | 23 | edge construction + Kahn drain |
| `grocery/store.go` `RouteGroups` | 53 | rank build, aisle assignment, comparator, `assembleRouteGroups` + per-item placement |
| `grocery/store.go` `ReorderListItems` | 19 | aisle validation extracted |
| `idempotency.go` `resolveConflict` | 20 | per-row inspection extracted |
| `nutritionparse/parser.go` `Parse` | 18 | `lineNutrient` per-line parse |
| `inventory/service.go` `SearchItems` | 19 | searched-term tier + remainder page |
| `notifier/service.go` `categoryAllowed` | 16 | duplicated pref loop collapsed |
| `notifier/sweep.go` `sweepMealReminders` | 24 | signal loading + per-slot emission |
| `session/service.go` `Refresh` | 28 | refresh-reuse check + rotation transaction |
| `ocrimport/draft.go` `ValidateDraft` | 46 | scalar/item/step validators |
| `ocrimport/catalog.go` `MatchItem` | 36 | exact-match + fuzzy-sweep stages |
| `ocrimport/parse.go` `splitLeadingQuantity` | 35 | `leadingQty` quantity prefix + unit-word stages |
| `ocrimport/similarity.go` `jaroWinkler` | 31 | match scan, transpositions, prefix boost |
| `testutil/testutil.go` `execFile` | 16 | plain-statement exec helper |
| `app/recipeembed/service.go` `ingredientNames` | 18 | `entityNames` name-map loading |

### S107 parameter-list fixes

- `notifier/sweep.go` — the 8-parameter reminder helper now takes a
  `recipeReminder`/`itemReminder`/`slotReminders` context bundle.

### S1192 string constants

- `notifier/sweep.go` — repeated `"Mon Jan 2"` date-format literal extracted.
- `testutil/testutil.go` — `testProvider` constant for `"test-provider"`.

## Verification

- `go build ./...` ✓
- `go vet ./internal/... ./cmd/...` ✓
- `go test` — all touched packages pass (`internal/ai`, `ai/tools`, `event`,
  `grocery`, `idempotency`, `identity`, `inventory`, `nutritionparse`,
  `notifier`, `ocrimport`, `session`, `app/recipeembed`, `testutil`)
- `golangci-lint run ./...` — 0 issues
- `gofmt` — clean
- SonarQube rescan: `go:S3776`/`S107`/`S1192`/`S1186` **0 findings**
  project-wide; total open **145 → 109**; no new issues introduced
