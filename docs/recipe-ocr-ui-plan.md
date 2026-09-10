# Recipe OCR import — web-driven review & profanity filtering (phased)

Move the recipe OCR pipeline into the web UI across four phase branches: backend foundation, BFF GraphQL, web review UI, and CLI/docs cleanup. Each phase merges to `main` before the next begins.

## Decisions from clarifying questions

- **Review UI:** a new rich review page under `/recipes/pending/[id]`, not the existing `/items/pending` list.
- **Review granularity:** full per-ingredient editor (quantity, unit, catalog item, notes, sections) before approval.
- **Profanity filter:** hardcoded English deny-list + LLM prompt flag, applied to OCR text before and during structuring.
- **CLI:** the existing `cmd/ocrimport` CLI is no longer required; the UI becomes the only path.

## Objective

Make recipe scan import a first-class web workflow:

1. Admin uploads a PNG/JPG/PDF from `/recipes`.
2. The API writes the file to the import inbox, creates a persistent `recipe_import` job, and enqueues background processing.
3. Background worker serially runs OCR → profanity check → LLM structuring → catalog mapping, storing each artifact in Postgres.
4. When ready for review the job appears in `/recipes/pending`.
5. Admin opens `/recipes/pending/[id]`, edits recipe metadata, steps, and ingredient-to-catalog mappings, then approves.
6. Approval calls the existing `RecipeService.CreateRecipeWithChildren` transaction and links the resulting `recipe_id`.
7. Persisted recipes are visible on `/recipes` like any other recipe.
8. Core API still starts without OCR/Ollama; the feature is off when `LENA_OLLAMA_URL`/`LENA_OCR_SERVICE_URL` are unset.

## Branching strategy

Following `AGENTS.md`, each major phase has its own branch and its own PR against `main`. The previous recipe OCR phases ended at `p5`, so this work continues with `p6`–`p9`.

| Phase | Branch | Focus | PR into |
|-------|--------|-------|---------|
| 1 | `phase-recipe-ocr-p6` | DB migration, profanity package, `ocrimport` refactor, `recipeimport` backend service | `main` |
| 2 | `phase-recipe-ocr-p7` | GraphQL schema, BFF resolvers, integration tests | `main` |
| 3 | `phase-recipe-ocr-p8` | Web UI: pending list, review editor, upload feedback | `main` |
| 4 | `phase-recipe-ocr-p9` | Delete CLI, update docs/env, final cleanup and verification | `main` |

A phase is merged only after `go build ./...`, `go test -short ./...`, lint, and the phase-specific verification checklist are green.

---

# Phase 1 — Backend foundation (`phase-recipe-ocr-p6`)

## 1.1 Database migration

**New file:** `migrations/0023_recipe_import.up.sql`

```sql
CREATE TABLE recipe.recipe_import (
    recipe_import_id     BIGSERIAL PRIMARY KEY,
    submitted_by_user_id BIGINT REFERENCES identity.users(user_id) ON DELETE SET NULL,
    source_filename      VARCHAR(500) NOT NULL,
    source_path          TEXT NOT NULL,
    source_hash          VARCHAR(64),
    ocr_text             TEXT,
    ocr_json             JSONB,
    draft_json           JSONB,
    review_json          JSONB,
    profanity_flag       BOOLEAN NOT NULL DEFAULT FALSE,
    profanity_reason     TEXT,
    status               VARCHAR(20) NOT NULL DEFAULT 'pending'
        CONSTRAINT recipe_import_status CHECK (
            status IN (
                'pending','processing','ocred','drafted','reviewing',
                'ready','persisted','rejected','failed','profanity'
            )
        ),
    recipe_id            BIGINT REFERENCES recipe.recipe(recipe_id) ON DELETE SET NULL,
    error_message        TEXT,
    created_by           VARCHAR(100) NOT NULL,
    updated_by           VARCHAR(100),
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ,
    approved_by_user_id BIGINT REFERENCES identity.users(user_id) ON DELETE SET NULL,
    approved_at          TIMESTAMPTZ
);

CREATE INDEX idx_recipe_import_status         ON recipe.recipe_import (status);
CREATE INDEX idx_recipe_import_submitted_by  ON recipe.recipe_import (submitted_by_user_id);
CREATE INDEX idx_recipe_import_recipe_id     ON recipe.recipe_import (recipe_id);
```

Reasoning: the whole review decision is a document (`review_json` JSONB) in v1; no normalized item table is needed yet. `recipe_id` links to the persisted recipe. `source_path` is the absolute inbox path the worker reads.

## 1.2 Profanity filter package

**New files:**
- `internal/platform/profanity/profanity.go`
- `internal/platform/profanity/profanity_test.go`

- `Detector` holds a word-boundary regex built from a default English deny-list plus an optional comma-separated `LENA_PROFANITY_EXTRA_TERMS` list.
- `Check(text string) (bool, []string)` returns whether any term matched and which terms.
- Add `ProfanityExtraTerms string` to `internal/platform/config/config.go`.

## 1.3 Refactor `ocrimport` for server-side reuse

**Files:** `internal/ocrimport/draft.go`, `internal/ocrimport/catalog.go`, `internal/ocrimport/catalog_test.go`, `internal/ocrimport/draft_test.go`

- Add `ProfanityDetected bool` and `ProfanityReason *string` to `RecipeDraft`.
- Update `JSONSchema()`:
  - Add `profanityDetected` (boolean) and `profanityReason` (string).
  - Allow `name`, `items`, `steps` to be present but the validator, not the schema, decides whether they are required.
- Update `ValidateDraft()`:
  - If `ProfanityDetected` is true, skip name/items/steps validation.
  - Otherwise keep the existing checks.
- Decouple `CatalogSnapshot` from `bffclient` types:
  - Introduce small interfaces `CatalogItem`, `CatalogIngredient`, `CatalogUnit` with `ID() string` and `Name() string` (unit also `Abbreviation() string`).
  - `NewCatalogSnapshot(catalog Catalog)` builds indexes from those interfaces.
  - Keep `NormalizeName`, `Similarity`, `ResolveUnit`, `MatchItem`, `MapDraftItem` logic unchanged.
- Update `catalog_test.go` to build a local test catalog using the new interface types instead of `bffclient.Catalog`.
- Update `draft_test.go` with profanity cases.

## 1.4 New `internal/recipeimport` service

**New files:**
- `internal/recipeimport/types.go`
- `internal/recipeimport/catalog.go` (adapter from `inventory.Service` to `ocrimport.CatalogSnapshot`)
- `internal/recipeimport/store.go`
- `internal/recipeimport/sqlc/queries.sql`
- `internal/recipeimport/sqlc/generate.go`
- `internal/recipeimport/service.go`
- `internal/recipeimport/service_test.go`

### Types

`RecipeImport` mirrors the DB row:

```go
type RecipeImport struct {
    ID                int64
    SubmittedByUserID *int64
    SourceFilename    string
    SourcePath        string
    SourceHash        string
    OCRText           string
    OCRJSON           []byte
    DraftJSON         []byte
    ReviewJSON        []byte
    ProfanityFlag     bool
    ProfanityReason   string
    Status            string
    RecipeID          *int64
    ErrorMessage      string
    CreatedBy         string
    UpdatedBy         string
    CreatedAt         time.Time
    UpdatedAt         time.Time
    ApprovedByUserID  *int64
    ApprovedAt        time.Time
}
```

`ReviewRecipe` and `MatchResult` are reused from `internal/ocrimport`.

### Store interface

Methods:
- `Create(ctx, RecipeImport) (*RecipeImport, error)`
- `Get(ctx, id int64) (*RecipeImport, error)`
- `List(ctx, status string, limit, offset int32) ([]RecipeImport, error)`
- `Count(ctx, status string) (int64, error)`
- `UpdateOCR(ctx, id int64, ocrText string, ocrJSON []byte) error`
- `UpdateDraft(ctx, id int64, draftJSON []byte) error`
- `UpdateReview(ctx, id int64, reviewJSON []byte, status string) error`
- `MarkProfanity(ctx, id int64, reason string) error`
- `MarkFailed(ctx, id int64, errorMessage string) error`
- `SetPersisted(ctx, id int64, recipeID int64, approvedByUserID int64) error`
- `SetRejected(ctx, id int64) error`
- `SetPending(ctx, id int64) error`
- `WithTx(tx pgx.Tx) Store`

### Catalog adapter (`internal/recipeimport/catalog.go`)

Implement the `ocrimport` catalog interfaces from the `inventory` service results:

- Load `ListUnits(ctx)`.
- Paged load of `ListItems(ctx, 0, pageSize, offset)` until total reached (status `approved` for catalog matching).
- Paged load of `ListIngredients` and `ListCategories`.
- Map to interface types and feed `ocrimport.NewCatalogSnapshot`.

### Service

Constructor:

```go
func NewService(
    pool dbtx.Pool,
    ocr *ocrclient.Client,
    ollama *ollamaclient.Client,
    inv InventoryReader,
    rec *recipe.Service,
    profanity *profanity.Detector,
    cfg Config,
) *Service
```

`Config` is a small local struct of thresholds:
- `OCRConfidenceThreshold int`
- `ImportAutoAcceptConfidence float64`
- `ImportReviewThreshold float64`
- `ImportWorkerConcurrency int` (new, default 1 to avoid GPU contention)

Public methods:
- `Create(ctx, sourceFilename, sourcePath, sourceHash string, submittedByUserID *int64, createdBy string) (*RecipeImport, error)` — inserts `pending` and enqueues processing.
- `Get(ctx, id int64) (*RecipeImport, error)`
- `List(ctx, status string, page, pageSize int32)` and `ListPending(ctx, page, pageSize int32)` (`pending`/`reviewing`/`ready`/`profanity`/`failed`).
- `UpdateReview(ctx, id int64, review *ocrimport.ReviewRecipe, updatedBy string) (*RecipeImport, error)` — validates item IDs and unit names, stores JSON, sets status `reviewing` or `ready` depending on `AllResolved()`.
- `Approve(ctx, id int64, approvedBy currentuser.User) (*recipe.Recipe, *RecipeImport, error)` — resolves unit/item IDs, builds `recipe.Recipe` + `[]recipe.RecipeItem` + `[]recipe.RecipeStep`, calls `RecipeService.CreateRecipeWithChildren` inside a transaction, then `Store.SetPersisted`.
- `Reject(ctx, id int64) error`.
- `Retry(ctx, id int64) error` — resets to `pending` and enqueues processing.
- `Shutdown(ctx context.Context) error` — drains the worker pool.

Background processing:
- The service owns a bounded worker pool (`workerSem` + `workerWG`).
- `EnqueueProcess(id int64)` submits `Process(id)` to the pool.
- `Process(ctx, id)`:
  1. Load row; skip if not `pending`.
  2. `SetStatus` `processing`.
  3. Read source file from `SourcePath`.
  4. `ocr.ExtractTextResult(ctx, bytes, sourceFilename)`.
  5. Check `MeanConfidence >= OCRConfidenceThreshold`; else fail.
  6. `profanity.Check(ocrText)`; if matched, `MarkProfanity` and stop.
  7. Save `ocr_text`/`ocr_json` and set `ocred`.
  8. Build Ollama system prompt (JSON schema + profanity instruction) and user prompt (`OCR text:\n` + `ocrText`).
  9. `ollama.Chat(...)`; parse JSON into `ocrimport.RecipeDraft`; `ValidateDraft`.
  10. If `draft.ProfanityDetected`, `MarkProfanity` and stop.
  11. Save `draft_json` and set `drafted`.
  12. Build catalog snapshot; `MapDraftItem` each `DraftItem` into `MatchResult`; produce `ReviewRecipe`.
  13. Save `review_json` and set `reviewing`.
- On any error, `MarkFailed` with the error text.

### SQLC setup

- Add a new block to `sqlc.yaml` for `internal/recipeimport/queries.sql` → `internal/recipeimport/sqlc`.
- `queries.sql` contains the CRUD queries from the store interface.
- `internal/recipeimport/sqlc/generate.go`:
  ```go
  //go:generate sqlc generate -f ../../sqlc.yaml
  //go:generate go run go.uber.org/mock/mockgen -source=querier.go -package=mock -destination=mock/querier.go Querier
  ```
- Override `timestamptz` to `time.Time` and `jsonb` to `[]byte` in `sqlc.yaml` for this package.

## 1.5 Phase 1 verification

- `sqlc generate` runs and `internal/recipeimport/sqlc` is created.
- `go test ./internal/recipeimport/...` passes (mocks + integration against test DB).
- `go test ./internal/ocrimport/...` passes after refactor.
- `go test ./internal/platform/profanity/...` passes.
- `go build ./...` still passes (no public API surface changed yet).

---

# Phase 2 — BFF GraphQL integration (`phase-recipe-ocr-p7`)

## 2.1 GraphQL schema

**File:** `internal/bff/schema.graphqls`

Add types:

```graphql
type RecipeImport {
  id: ID!
  status: String!
  sourceFilename: String!
  ocrText: String
  draft: RecipeImportDraft
  review: RecipeImportReview
  recipe: Recipe
  profanityFlag: Boolean!
  profanityReason: String
  errorMessage: String
  createdAt: Time!
  updatedAt: Time
  createdBy: String!
}

type RecipeImportDraft {
  name: String
  description: String
  servings: Int
  prepTimeMinutes: Int
  cookTimeMinutes: Int
  items: [RecipeImportDraftItem!]!
  steps: [RecipeImportDraftStep!]!
  sourceHint: String
}

type RecipeImportDraftItem {
  quantity: Float
  unit: String
  ingredient: String!
  section: String
  notes: String
  isOptional: Boolean!
}

type RecipeImportDraftStep {
  stepNumber: Int!
  instruction: String!
}

type RecipeImportReview {
  pageId: String
  name: String
  description: String
  servings: Int
  prepTimeMinutes: Int
  cookTimeMinutes: Int
  sourceHint: String
  items: [RecipeImportReviewItem!]!
  steps: [RecipeImportReviewStep!]!
  approved: Boolean!
}

type RecipeImportReviewItem {
  draftItem: RecipeImportDraftItem!
  itemId: ID
  itemName: String
  unit: String
  unitId: ID
  confidence: Float!
  suggestions: [RecipeImportSuggestion!]!
  status: String!
  notes: String
  approved: Boolean!
}

type RecipeImportSuggestion {
  id: ID!
  name: String!
  kind: String!
  score: Float!
}

type RecipeImportReviewStep {
  stepNumber: Int!
  instruction: String!
}

type RecipeImportPage {
  items: [RecipeImport!]!
  pageInfo: PageInfo!
}

input RecipeImportReviewInput {
  name: String
  description: String
  servings: Int
  prepTimeMinutes: Int
  cookTimeMinutes: Int
  sourceHint: String
  items: [RecipeImportReviewItemInput!]!
  steps: [RecipeImportReviewStepInput!]!
}

input RecipeImportReviewItemInput {
  ingredient: String!
  quantity: Float
  unit: String
  section: String
  notes: String
  isOptional: Boolean!
  itemId: ID
  itemName: String
  unitId: ID
  approved: Boolean!
}

input RecipeImportReviewStepInput {
  stepNumber: Int!
  instruction: String!
}
```

Add queries:

```graphql
recipeImport(id: ID!): RecipeImport
recipeImports(page: Int = 1, pageSize: Int = 25, status: String): RecipeImportPage!
pendingRecipeImports(page: Int = 1, pageSize: Int = 25): RecipeImportPage!
```

Add mutations:

```graphql
submitRecipeScan(fileBase64: String!): RecipeImport!
updateRecipeImport(id: ID!, input: RecipeImportReviewInput!): RecipeImport!
approveRecipeImport(id: ID!): Recipe!
rejectRecipeImport(id: ID!): RecipeImport!
retryRecipeImport(id: ID!): RecipeImport!
```

`submitRecipeScan` becomes `RecipeImport!` instead of `Boolean!` so the UI can immediately show the job ID.

## 2.2 BFF resolvers

**New file:** `internal/bff/resolver_recipe_import.go`

- `recipeImportResolver` with field resolvers for `RecipeImport`.
- `recipeImportDraftResolver`, `recipeImportDraftItemResolver`, `recipeImportDraftStepResolver`, `recipeImportReviewResolver`, `recipeImportReviewItemResolver`, `recipeImportSuggestionResolver`, `recipeImportReviewStepResolver`.
- `recipeImportPageResolver`.
- For `RecipeImportReviewItem.itemId`/`unitId`/`item`, parse and lazy-load; batch load items/units by IDs in the parent resolver where possible.

Queries/mutations (all admin-only):
- `RecipeImport(ctx, args{ID})` — `requireAdmin`, call `RecipeImportService.Get`.
- `RecipeImports(ctx, args)` — admin-only list.
- `PendingRecipeImports(ctx, args)` — list with `status IN ('reviewing','ready','profanity','failed')`.
- `SubmitRecipeScan(ctx, args{FileBase64})` — validate, write file to `ImportInbox`, call `RecipeImportService.Create`, enqueue process, return resolver.
- `UpdateRecipeImport(ctx, args{ID, Input})` — convert input to `ocrimport.ReviewRecipe`, call `RecipeImportService.UpdateReview`.
- `ApproveRecipeImport(ctx, args{ID})` — call `RecipeImportService.Approve`, return `recipeResolver`.
- `RejectRecipeImport(ctx, args{ID})` — call `RecipeImportService.Reject`.
- `RetryRecipeImport(ctx, args{ID})` — call `RecipeImportService.Retry`.

**Files to modify:**
- `internal/bff/services.go` — add `RecipeImportService` interface.
- `internal/bff/resolver.go` — add `RecipeImportService` to `Resolver` and `NewResolver` signature; update `SubmitRecipeScan` in `internal/bff/recipe_scan.go` to use the service and return `*recipeImportResolver`.
- `internal/bff/resolver_recipe.go` — ensure `recipeResolver` can be returned from `approveRecipeImport`.
- `internal/bff/generate.go` — regenerate `mock/services.go` after interface change.

## 2.3 Wire into the API server

**File:** `cmd/lena/main.go`

- Build `profanity.Detector` from `cfg.ProfanityExtraTerms`.
- Build `*ollamaclient.Client` only if `cfg.OllamaURL != ""`.
- Build `*recipeimport.Service` with `ocrclient`, `ollamaclient`, `inventorySvc`, `recipeSvc`, `profanity.Detector`, and local config.
- Pass the service to `bff.NewResolver`.
- In shutdown sequence, call `recipeImportSvc.Shutdown(ctx)` before closing the pool.

## 2.4 Phase 2 verification

- `go generate ./...` regenerates mocks.
- `go test ./internal/bff/...` passes, including new `resolver_recipe_import_test.go`.
- `go test -short ./...` passes.
- Manual GraphQL tests:
  - `submitRecipeScan` returns a `RecipeImport` in `pending` then reaches `reviewing`.
  - `approveRecipeImport` creates a recipe and returns it.
  - `updateRecipeImport`/`rejectRecipeImport`/`retryRecipeImport` behave correctly.

---

# Phase 3 — Web review UI (`phase-recipe-ocr-p8`)

## 3.1 Types and API client

**Files:** `clients/web/lib/types.ts`, `clients/web/lib/api.ts`

Add `RecipeImport`, `RecipeImportDraft`, `RecipeImportReview`, `RecipeImportReviewItem`, `RecipeImportSuggestion`, `RecipeImportReviewStep` types.

Add API methods:
- `getRecipeImport(id: number): Promise<RecipeImport>`
- `getPendingRecipeImports(page, pageSize): Promise<PagedResult<RecipeImport>>`
- `getUnits(): Promise<Unit[]>` (queries the existing `units` GraphQL field)
- `submitRecipeScan(fileBase64: string): Promise<RecipeImport>`
- `updateRecipeImport(id, input): Promise<RecipeImport>`
- `approveRecipeImport(id): Promise<Recipe>`
- `rejectRecipeImport(id): Promise<RecipeImport>`
- `retryRecipeImport(id): Promise<RecipeImport>`

Add `RECIPE_IMPORT_FIELDS` GraphQL fragment in `api.ts`.

## 3.2 Navigation

**File:** `clients/web/app/components/AdminLayout.tsx`

Under the `Recipes` group add an admin-only child:

```ts
{ label: "Pending Reviews", href: "/recipes/pending", adminOnly: true }
```

## 3.3 Pending list page

**New file:** `clients/web/app/recipes/pending/page.tsx`

- Admin-only check via `useMe`.
- Query `pendingRecipeImports` with `useQuery`, poll every 5 seconds (`refetchInterval: 5000`).
- Table columns: source filename, draft name (from `review.name` or `draft.name`), status chip, updated at, actions.
- Actions: open detail (`/recipes/pending/[id]`), reject, retry (for `failed`/`profanity`).

## 3.4 Review detail page

**New file:** `clients/web/app/recipes/pending/[id]/page.tsx`

Sections:
- Header with status chip, source filename, error/profanity banner.
- Collapsible panel showing `ocrText`.
- Recipe metadata form: name, description, servings, prep/cook times.
- Steps editor: add/remove/edit `stepNumber` and `instruction`.
- Ingredient editor table with one row per `review.items`:
  - raw ingredient text (read-only)
  - quantity input
  - unit autocomplete from `getUnits()`
  - item search autocomplete from `api.searchItems(term)` (client-side filter over all approved items)
  - suggestion chips: clicking a suggestion fills `itemId`/`itemName`
  - status chip
  - notes input
  - `isOptional` checkbox
  - section input
- Actions: Save, Approve (disabled until all items have `itemId` and `unit`), Reject, Retry.

Approve calls `approveRecipeImport`; on success navigate to `/recipes/[recipeId]` or `/recipes`.

## 3.5 Recipes upload feedback

**File:** `clients/web/app/recipes/page.tsx`

Update `handleUpload` so that after `submitRecipeScan` returns the `RecipeImport`, the success message links to `/recipes/pending` and shows the import ID.

## 3.6 Phase 3 verification

- `npm run test` in `clients/web` passes for new pages.
- Manual end-to-end:
  1. Admin uploads a scan from `/recipes`.
  2. `/recipes/pending` shows the job and reaches `reviewing`.
  3. Detail page shows OCR text and proposed recipe.
  4. Map ingredients and approve.
  5. Recipe appears in `/recipes`.

---

# Phase 4 — Cleanup and docs (`phase-recipe-ocr-p9`)

## 4.1 Remove CLI workflow

Since the UI is the only path now:

- **Delete:** `cmd/ocrimport/main.go` and the `cmd/ocrimport` directory.
- **Delete:** `internal/ocrimport/workqueue.go` (file-queue no longer used).
- **Delete:** `internal/ocrimport/persist.go` (server-side `Approve` handles persistence).
- **Delete or archive:** `docs/recipe-ocr-phases.md` (replaced by this plan and `docs/recipe-ocr-usage.md`).
- **Delete if unused:** `internal/platform/bffclient` package (if no other code references it after the above removals).

## 4.2 Configuration and docs

- **Update:** `docs/recipe-ocr-usage.md` — rewrite for UI workflow; remove `go run ./cmd/ocrimport` steps.
- **Update:** `.env.example` — remove `LENA_IMPORT_WORK_DIR`, `LENA_API_URL`, `LENA_IMPORT_ADMIN_TOKEN`; add `LENA_PROFANITY_EXTRA_TERMS`; keep `LENA_OCR_*`, `LENA_OLLAMA_*`, `LENA_IMPORT_*` thresholds.
- **Update:** `internal/platform/config/config.go` — remove or deprecate `ImportWorkDir`, `APIURL`, `ImportAdminToken` if no longer referenced.
- **Update:** `docker-compose.import.yml` if needed — keep `ollama`/`ocr` services; no CLI volume requirements beyond the existing `./import:/data/import` in `api`.

## 4.3 Final tests and verification

- Full `go test -short ./...`.
- `npm run test` in `clients/web`.
- Manual end-to-end:
  1. Fresh `docker compose -f docker-compose.yml -f docker-compose.import.yml --profile import up -d --build`.
  2. Upload the existing `scan-*.pdf`.
  3. `/recipes/pending` reaches `reviewing`.
  4. Detail page shows "Vegetable Enchilada Casserole".
  5. Map each ingredient to an existing catalog item and unit.
  6. Approve; recipe appears in `/recipes`.
  7. Upload a scan with profanity; job reaches `profanity` status and cannot be approved.

---

# Cross-phase risks and considerations

- **Long background tasks.** OCR + LLM can take minutes. The `recipeimport.Service` has its own worker pool so it does not starve the analytics `runAsync` semaphore. `Shutdown` drains it before the DB pool closes.
- **GPU contention.** `ImportWorkerConcurrency` defaults to 1 so only one recipe is structured at a time on the 16 GB VRAM budget.
- **Catalog snapshot size.** `ListItems` is paged; large catalogs may take a while to snapshot. A follow-up can add a server-side `searchItems` query and avoid full snapshots.
- **JSON schema change.** Adding `profanityDetected`/`profanityReason` to `RecipeDraft` requires updating `ocrimport` tests and any prompt-cached integration tests.
- **GraphQL breaking change.** `submitRecipeScan` changes from `Boolean!` to `RecipeImport!`; this is acceptable because it is only used by the new upload flow.
- **File storage.** Source scans remain on disk at `LENA_IMPORT_INBOX`; the DB stores the path. The existing `api` container already mounts `./import:/data/import`.
- **Profanity false positives.** The deny-list uses word boundaries, but medical/culinary terms may still trigger. The LLM layer is a second opinion; the admin can retry if a false positive occurs. A future improvement is a per-language/configurable list.
- **CLI removal.** Deleting `cmd/ocrimport` removes the only non-web path. If host-side batch import is needed later, the `recipeimport` package can be wrapped in a new CLI, but this is out of scope for these phases.
