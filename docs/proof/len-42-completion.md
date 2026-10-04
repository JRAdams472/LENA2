# LEN-42 / P2 — Go complexity: bff package

Phase 2 of the LEN-28 SonarQube megaplan. All `internal/bff` findings cleared:
**SonarQube rescan reports 0 open issues in `internal/bff`**. Repo-wide open
issues dropped **179 → 145**.

## What changed (pure refactor — behavior unchanged)

### S3776 cognitive-complexity splits (all → ≤15)

| Function | Was | Approach |
|---|---|---|
| `resolver.go` `loadItemChildren` | 23 | `newItemChildren`, `collectItemIDSets`, `loadCatalogRows`, `mergedIngredientIDs`, shared `sortedIDs[V]` generic |
| `resolver.go` `loadRecipeInventoryChildren` | 32 | `recipeItemIDs`/`recipeItemIngredientIDs`/`recipeItemUnitIDs` collectors + `recipeChildLoaders` param object (also clears S107) |
| `resolver.go` `NewGraphQLHandler` | 22 | `decodeGraphQLRequest`, `badRequestErrorBody`, `dedupMutation`, `execGraphQL` |
| `resolver_grocery.go` `GenerateGroceryList` | 27 | `generateGroceryListTx`, `planListTarget`, `planGroceryNeeds`, `writeGeneratedLines`, `recordGroceryNeedEvents` |
| `resolver_grocery.go` `expandPlanLines` | 39 | `planExpansion` index struct, `expandSlotItems`, `expandSlotRecipeLines` |
| `resolver_grocery.go` `aggregateGroceryNeeds` | 20 | `groceryNeedKey` type + `accumulateGroceryNeeds`, `sortGroceryNeedKeys`, `groceryNeedFor` |
| `resolver_grocery.go` `groceryNeedLines` | 25 | `stockCoversNeed`, `pantryCover` |
| `resolver_grocery.go` `SuggestedRestockItems` | 28 | `lowStockItemIDs`, `restockCandidates`, `engagementRanked`, `approvedItemResolvers` |
| `resolver_allergy.go` `loadAllergyContext` | 36 | `newAllergyContext`, `resolveContextItems`, `flagIngredientIDs`, `loadFlagRows`, `populateFlagMaps`, `loadMemberRecords`, `loadAllergenRegistry` |
| `resolver_household.go` `acceptInvite` | 18 | `mergeIntoHousehold`, `notifyJoin` |
| `resolver_household.go` `LeaveHousehold` | 19 | `promoteSuccessor`, `cancelPendingInvitesFromUser` |
| `resolver_identity.go` `UpdateMyProfile` | 26 | `profileNameField`, `profileBackupEmail`, `profileBirthdate` |
| `resolver_mealplan.go` `allergenSet` | 25 | `allergenInputs` (batch-hit vs lazy-load split) |
| `resolver_recipe.go` `Recipes` | 19 | `applyEngagementFilters`, `runRecipeSearch`, `runSemanticSearch` |
| `resolver_recipe.go` `parseRecipeChildren` | 19 | `parseRecipeItemIDs`, `resolveBrandOnlyIngredients`, `buildRecipeItems`, `buildRecipeSteps` |
| `auth.go` `verifyOIDCToken` | 16 | `parseVerifiedToken` (JWKS load + kid-miss refresh retry) |
| `upload_sniff.go` `sniffUpload` | 17 | `normalizeDeclaredType`, `sniffImageUpload`, `sniffPDFUpload` |

### S107 parameter-list fixes

- `loadRecipeChildren` (10 params) and `loadRecipeInventoryChildren` (8 params)
  now take a `recipeChildLoaders` services bundle; `Resolver.childLoaders()`
  builds it at the 8 call sites.

### S1192 string constants

- `resolver_ai.go`: `msgAIUnavailable`, `msgAIRateLimited`, `msgMaxSuggestions`,
  `msgAIToolsUnavailable`
- `resolver_household.go`: `msgCannotInvite` (enumeration-proof invite
  rejection — message text unchanged)
- `errors.go`: `msgServingsPositive` shared by recipe/event/mealplan resolvers
- `session_handler.go`: `sessionPath`, `msgSessionsUnavailable`,
  `msgInvalidRequestBody`
- `upload_sniff.go`: `mediaTypeJPEG/PNG/PDF`
- `"2006-01-02"` → stdlib `time.DateOnly` everywhere (event, mealplan,
  identity, resolver)

### S1186

- `graphql_tracer.go`: nested comment explaining the no-op trivial-field
  finish func.

## Verification

- `go build ./...` ✓
- `go vet ./...` ✓
- `go test ./internal/... ./cmd/...` — all packages pass incl. testcontainers
  (bff 9.9s, recipeimport 39.6s)
- `golangci-lint run ./internal/bff/...` — 0 issues
- `gofmt -l internal/bff/` — clean
- SonarQube rescan: `internal/bff` open issues **0** (was ~30); total open
  **179 → 145**; no new issues introduced
