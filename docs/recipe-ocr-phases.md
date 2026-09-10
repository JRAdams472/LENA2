# Recipe OCR Import — Phased Implementation Plan

A permanent, admin-only feature for bulk-importing printed recipes (cookbook pages, recipe cards, scanned PDFs) into LENA2 using local OCR and a local LLM. The feature is **always available once enabled**, not a time-limited import window, because it parallels the existing item/barcode OCR infrastructure.

---

## Decisions locked in

1. **Permanent, admin-gated feature.** Like `CreateRecipe`, the import tool requires an admin bearer token. It is not restricted by calendar or number of recipes.
2. **Local-only OCR and LLM.** Tesseract or PaddleOCR for text, Ollama for structured JSON extraction. No cloud services.
3. **No new database write paths.** Recipes are persisted only through the existing `createRecipe` GraphQL mutation → `RecipeService.CreateRecipeWithChildren` transaction.
4. **Human review is mandatory.** The LLM produces a *draft*; the admin must reconcile every ingredient to a catalog `itemId` and a canonical unit before persistence.
5. **Optional compose profile.** OCR/LLM containers live in an `import` profile or `docker-compose.import.yml` so normal `docker compose up` does not require a GPU or download multi-GB models.
6. **Target hardware.** A single GPU with **16 GB VRAM**, same assumption as `docs/recipe-ocr-import.md`.

---

## Phase overview

| Phase | Name | Goal | Output |
|---|---|---|---|
| p0 | Local infrastructure & importer skeleton | OCR + Ollama containers, config, work queue, CLI skeleton | `docker-compose.import.yml`, `cmd/ocrimport`, `internal/ocrimport` |
| p1 | OCR stage | Convert images/PDFs to plain text | `POST /ocr` endpoint, page text files, layout metadata |
| p2 | LLM structuring | Convert OCR text into `RecipeDraft` JSON | Ollama chat with JSON schema, draft validation, re-prompt loop |
| p3 | Catalog mapping & review | Match ingredients to `inventory.item`, normalize units, produce review report | `MatchItems`, review TUI/web page, admin decisions queue |
| p4 | Persist & import | Call `createRecipe` as admin, handle retries/idempotency | Import runner, success/failure log, one-recipe-per-tx |
| p5 | Hardening, alternatives, docs & CI | PaddleOCR/docTR, vision-LLM, metrics, CI job, README | Production-ready import with observability and tests |

---

## p0 — Local infrastructure & importer skeleton

### Goals

- Add an `import` Docker Compose profile with `ollama` and `ocr` services.
- Define `LENA_OCR_*` and `LENA_OLLAMA_*` env vars in `internal/platform/config/config.go` (default empty = feature off).
- Create a new `internal/ocrimport` package and `cmd/ocrimport` CLI binary.
- Implement a local SQLite/JSON work queue for source pages and intermediate artifacts.
- Wire the importer to the existing GraphQL client pattern.

### Key files

- `docker-compose.import.yml` — `ollama`, `ollama-pull`, `ocr` services.
- `tools/ocr/Dockerfile` — Tesseract-based image (swappable later).
- `internal/platform/config/config.go` — new `OCR` and `Ollama` config blocks.
- `internal/ocrimport/workqueue.go` — page id, source hash, status, artifact paths.
- `cmd/ocrimport/main.go` — `import`, `review`, `persist` subcommands.

### Acceptance

- `docker compose -f docker-compose.yml -f docker-compose.import.yml --profile import up -d` starts and is healthy without breaking the core stack.
- `go run ./cmd/ocrimport --help` works and lists subcommands.
- When `LENA_OCR_URL` or `LENA_OLLAMA_URL` is empty, the tool exits early with a clear "import feature not configured" message.

---

## p1 — OCR stage

### Goals

- Accept single images (`png`, `jpg`) or multi-page PDFs.
- Produce UTF-8 text plus per-line bounding boxes for debugging.
- Preserve layout enough for ingredient lists and instructions to be readable.

### Key work

1. **Pre-processing:** deskew, binarize, rescale to ~300 DPI, produce `tiff/png` before Tesseract.
2. **Tesseract container endpoint:** `POST /ocr` with multipart image; response `{"text": "...", "lines": [{"text":"...", "bbox":...}]}`.
3. **Page segmentation modes:** switch between `--psm 4` (multiline variable), `--psm 6` (single uniform block), and `--psm 11` (sparse text) per page; record which mode was used.
4. **PDF handling:** `pdftoppm` / `pdfimages` to render each page to an image before OCR.
5. **Output artifacts:** for each source page, write `work/<page-id>/page.png`, `page.ocr.txt`, and `page.ocr.json`.

### Acceptance

- 20 representative recipe-card scans produce text that a human can read and that the LLM can structure in p2.
- Multi-column cookbook pages do not interleave ingredient columns in the raw text for at least 80% of samples.
- Tesseract confidence per word is recorded; pages with mean confidence < 50% are flagged for re-scan review.

---

## p2 — LLM structuring

### Goals

- Send OCR text to Ollama and receive a JSON `RecipeDraft` matching the `CreateRecipeInput` shape, but with string ingredient names.
- Validate the draft against a JSON schema; re-prompt once on validation failure.
- Convert fractional quantities to decimals; extract sections (crust, filling), prep/cook times, and servings.

### Key work

1. **Ollama chat prompt:** system prompt instructs the model to *transcribe, not improve*, and to return `null` for missing fields.
2. **JSON schema for `RecipeDraft`:**
   ```json
   {
     "name": "string",
     "description": "string | null",
     "servings": "integer | null",
     "prepTimeMinutes": "integer | null",
     "cookTimeMinutes": "integer | null",
     "items": [ { "quantity": "number | null", "unit": "string | null", "ingredient": "string", "section": "string | null", "notes": "string | null", "isOptional": "boolean" } ],
     "steps": [ { "stepNumber": "integer", "instruction": "string" } ],
     "sourceHint": "string | null"
   }
   ```
3. **Validation & re-prompt:** use `github.com/invopop/jsonschema` or similar to validate Ollama output; append validation errors to the prompt for one retry.
4. **Output:** `work/<page-id>/draft.json` and `draft.log`.
5. **Model selection:** default `qwen2.5:7b-instruct` with `temperature: 0.1`, `num_ctx: 8192`.

### Acceptance

- 80% of recipe cards produce valid `RecipeDraft` JSON on the first call; 95% produce valid JSON after one re-prompt.
- No hallucinated ingredients — if a line cannot be parsed, it is emitted as an item with `quantity: null`, `unit: null`, and the full raw text in `notes`.
- Servings, prep, and cook times are `null` when the page does not state them.

---

## p3 — Catalog mapping & review

### Goals

- Normalize units to existing `inventory.unit` names/abbreviations.
- Match OCR'd ingredient strings to `inventory.item` rows; present unmatched items for admin review.
- Allow the admin to accept, reject, or create missing catalog items inline.

### Key work

1. **Unit normalization table** in the importer: `tablespoons` → `tbsp`, `teaspoons` → `tsp`, `ounces` → `oz`, `pounds` → `lb`, etc. Unknown units become review items.
2. **Catalog snapshot:** at import start, page in `ListItems`, `ListIngredients`, `ListUnits`, and `ListCategories` and build an in-memory index.
3. **Fuzzy matching:**
   - exact normalized `item.name` match → auto-accept;
   - `item.ingredient` canonical mapping → auto-accept;
   - fuzzy score ≥ 0.92 → auto-accept;
   - 0.75–0.92 → suggest top 3;
   - < 0.75 → unmatched.
4. **Review report:** per-recipe Markdown or small local web page showing:
   - source image crop;
   - OCR text;
   - each proposed `itemId`, confidence, unit, quantity;
   - actions: Accept / Suggest / Create item / Reject recipe.
5. **Review decision persistence:** a `work/<page-id>/review.json` file records every decision so a re-run (new model, re-scan) can replay them.

### Acceptance

- The importer does not call `createRecipe` until every item has a resolvable `itemId` and unit.
- Review decisions survive re-running the pipeline against the same source page hash.
- Admin can create a missing `inventory.item` through the existing `createItem` admin mutation and the review page automatically uses the returned `id`.

---

## p4 — Persist & import

### Goals

- Convert a fully reviewed `RecipeDraft` into `CreateRecipeInput` and call the existing `createRecipe` mutation.
- Record success/failure per recipe; make the run idempotent and resumable.
- Handle GraphQL errors and partial failures without corrupting the database.

### Key work

1. **GraphQL client as admin:** obtain an OIDC token for an admin user or use an existing `e2e` test issuer in local dev; send `Authorization: Bearer <admin_token>`.
2. **CreateRecipe conversion:** map `RecipeDraft` to `CreateRecipeInput { name, description, servings, prepTimeMinutes, cookTimeMinutes, items, steps }`.
3. **One-recipe-per-call:** each recipe is a single `createRecipe` mutation. The server wraps it in one transaction.
4. **Idempotency:** before calling, check `GetRecipeByName` (to be added) or use `sourceHint` + source page hash. If the recipe already exists, skip unless `--force` is passed.
5. **Failure handling:** if `createRecipe` returns a validation error (unknown unit, bad `itemId`, duplicate name), write the error to `work/<page-id>/error.json` and continue with the next recipe.
6. **Batch runner:** `ocrimport import --inbox ./import/inbox` runs p1→p2→p3→p4 with `--auto-accept-confidence 0.92` and stops before p4 if any recipe needs review.

### Acceptance

- `go run ./cmd/ocrimport import --inbox ./import/inbox` processes 100 pages without crashing; failed recipes are logged with the original `createRecipe` error.
- Re-running against the same inbox does not create duplicate recipes.
- `go test ./cmd/ocrimport/...` passes (happy path, validation failure, duplicate skip).

---

## p5 — Hardening, alternatives, docs & CI

### Goals

- Make the importer reliable enough for real cookbooks.
- Add support for multi-column layouts and a vision-LLM alternative.
- Document the feature and add CI.

### Key work

1. **OCR alternatives:**
   - Add a `tools/ocr-paddle` or `tools/ocr-doctr` Dockerfile.
   - Allow `LENA_OCR_ENGINE=tesseract|paddle|doctr`.
   - Compare error rates on a shared 50-page benchmark set before switching the default.
2. **Vision-LLM alternative:** a `LENA_OLLAMA_VISION_MODEL` optional env var that skips OCR and sends the image directly to `qwen2.5-vl:7b` or `llama3.2-vision:11b`. Document the tradeoffs (slower, no intermediate text, better layout).
3. **Batching and resumability:** process pages in a `pageSize` of N, with `workqueue` checkpointing after every page so a restart resumes from the last completed page.
4. **Observability:** emit metrics for `ocr_pages`, `llm_drafts`, `recipes_persisted`, `review_items`; add spans for each stage.
5. **Tests:**
   - Unit tests for unit normalization, fuzzy matching, and JSON validation.
   - Integration tests using a mocked Ollama HTTP server.
   - Playwright or Jest tests for the review web page.
6. **CI job:** add `ocr-import` build and unit tests to `.github/workflows/test.yml`.
7. **Docs:** update `README.md` with the feature, add `docs/recipe-ocr-usage.md` for operators, and update `docs/graphql-schema.md` if any new admin queries (e.g. `GetRecipeByName`) were added.

### Acceptance

- 95% of a 100-page cookbook produces at least a valid draft after review.
- The import suite passes in CI without a GPU (OCR and LLM mocked or CPU-only).
- `docker compose -f docker-compose.yml -f docker-compose.import.yml --profile import up` is documented and works on a fresh clone.

---

## Verification by phase

| Phase | Verify |
|---|---|
| p0 | `docker compose -f docker-compose.yml -f docker-compose.import.yml --profile import up -d` healthy; `go build ./cmd/ocrimport` clean |
| p1 | OCR endpoint returns text for 20 sample scans; confidence ≥ 50% threshold |
| p2 | 95% valid `RecipeDraft` after one re-prompt on sample set |
| p3 | All `RecipeDraft` items map or are flagged; admin can complete review |
| p4 | 100-page import batch runs without corrupting the database; duplicates skipped |
| p5 | CI green on `ocrimport` unit tests; `golangci-lint` clean; operator docs merged |

---

## Risks

1. **GPU not present.** Ollama falls back to CPU and a 7B model becomes unusably slow for bulk import. The importer should warn if `ollama ps` shows the model on CPU.
2. **Catalog bloat from auto-created items.** Creating missing ingredients is admin-only and per-line, but a bad batch could pollute `inventory.item`. Add a `--no-create-items` flag and a review gate.
3. **LLM hallucination.** Low temperature, strict JSON schema, and mandatory human review are the mitigations.
4. **Multi-column bleed.** OCR engines may interleave columns; keep Stage 1 swappable and use bounding boxes for ordering.
5. **Long recipes exceed context.** Multi-page recipes are concatenated; set `num_ctx` high (8k–16k) and prompt for no more than the page says.

---

## Env vars (new)

```env
# OCR
LENA_OCR_URL=http://ocr:8000           # empty = feature disabled
LENA_OCR_ENGINE=tesseract              # tesseract | paddle | doctr
LENA_OCR_CONFIDENCE_THRESHOLD=50

# Ollama
LENA_OLLAMA_URL=http://ollama:11434    # empty = feature disabled
LENA_OLLAMA_MODEL=qwen2.5:7b-instruct
LENA_OLLAMA_TEMPERATURE=0.1
LENA_OLLAMA_NUM_CTX=8192
LENA_OLLAMA_VISION_MODEL=              # optional, e.g. qwen2.5-vl:7b

# Importer
LENA_IMPORT_WORK_DIR=./import/work
LENA_IMPORT_AUTO_ACCEPT_CONFIDENCE=0.92
LENA_IMPORT_REVIEW_THRESHOLD=0.75
```

---

## Relationship to other docs

- `docs/recipe-ocr-import.md` contains the original, fully detailed design (stages, containerization, model selection, risks).
- `docs/recipe-ocr-phases.md` (this file) is the implementation schedule and acceptance checklist.
