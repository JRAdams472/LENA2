# Recipe OCR Import Operator Guide

The recipe OCR import feature lets an admin bulk-import printed recipes (cookbook pages, recipe cards, scanned PDFs) into LENA2 using only local services: a containerized OCR engine extracts text, a local Ollama model structures the text into a `RecipeDraft`, a mapping step reconciles free-text ingredients and units against the existing `inventory` catalog, and the existing admin `createRecipe` mutation persists the result.

- **Admin-only** — every stage that writes or approves is gated behind an admin bearer token.
- **Local-only** — no cloud OCR or cloud LLM is used. The only external network activity is the one-time `ollama pull` of model weights.
- **Human review is mandatory** — the LLM produces a draft; the admin must reconcile every ingredient to a catalog `itemId` and a canonical unit before persistence.

---

## Enabling the feature

The feature is off by default. Set the environment variables in your `.env` file or shell, then start the optional `import` compose profile.

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
LENA_API_URL=http://api:8080
LENA_IMPORT_ADMIN_TOKEN=<admin bearer token>
```

Start the import services:

```bash
docker compose -f docker-compose.yml -f docker-compose.import.yml --profile import up -d
```

This starts `ollama`, `ollama-pull` (one-shot model download), and the `ocr` service. The core LENA2 stack runs without these services; the `api` does not depend on Ollama.

---

## Workflow

Scanned pages can be submitted in two ways:

1. **Web upload (admin only)** — on the `/recipes` page, click *Upload Recipe Scan*, choose a PNG, JPG, or PDF, and the file is written to `LENA_IMPORT_INBOX`.
2. **Filesystem drop** — copy files directly into `./import/inbox`.

Then run the pipeline one stage at a time:

```bash
# 1. Enqueue source pages
go run ./cmd/ocrimport import --inbox ./import/inbox

# 2. Extract text from each page
go run ./cmd/ocrimport ocr

# 3. Ask the local LLM to structure OCR text into RecipeDraft JSON
go run ./cmd/ocrimport draft

# 4. Map draft ingredients to the catalog and produce a review report
go run ./cmd/ocrimport review

# 5. Review the generated `work/<page-id>/review.md` files and edit
#    `work/<page-id>/review.json` to set `itemId` and `unit` for each item.
#    Once resolved, set `approved: true` in the same JSON file.

# 6. Persist approved recipes to LENA2 via createRecipe
go run ./cmd/ocrimport persist
```

Each stage writes artifacts into `LENA_IMPORT_WORK_DIR/<page-id>/`:

- `ocr.txt` and `ocr.json` — raw OCR text and structured line/confidence output.
- `draft.json` and `draft.log` — the LLM `RecipeDraft` and the raw prompt/response log.
- `review.json` and `review.md` — mapping decisions and a human-readable review report.
- `persisted.json` — record of a successfully imported recipe.
- `error.json` — record of a failed stage, including the original GraphQL error.

---

## Artifacts per stage

| Stage | Input | Output | Status |
|---|---|---|---|
| `import` | `inbox/*.png`, `*.jpg`, `*.pdf` | `work/queue.json` entries and per-page work dirs | `pending` |
| `ocr` | source image/PDF | `ocr.txt`, `ocr.json` | `ocred` or `failed` |
| `draft` | `ocr.txt` | `draft.json`, `draft.log` | `drafted` or `failed` |
| `review` | `draft.json` | `review.json`, `review.md` | `reviewing` or `ready` |
| `persist` | `review.json` + admin approval | `persisted.json` or `error.json` | `persisted` or `failed` |

The work queue is a local `queue.json` file; it is not part of the LENA2 Postgres schema.

---

## Review and approval

The `review` command uses fuzzy string matching to propose catalog items:

- **≥ 0.92** — auto-accepted.
- **0.75–0.92** — suggested; the top 3 candidates appear in `review.md`.
- **< 0.75** — unmatched and flagged for manual review.

For each unmatched or suggested line, edit `review.json` and set:

- `itemId` — the catalog `inventory.item` id.
- `unit` — the canonical unit name that `resolveUnitID` accepts (e.g. `cup`, `tbsp`, `oz`, `each`).
- `approved: true` — mark the entire recipe ready for persistence.

Re-running `ocrimport review` preserves existing `itemId`/`unit` decisions and only re-maps items that are still empty, so a model change or re-scan does not lose manual work.

---

## Idempotency

`ocrimport persist` skips a page if any of the following are true:

- `persisted.json` already exists for the page (use `--force` to override).
- A recipe with the same name already exists in LENA2 (unless `--force` is used).

Duplicate-name checks are case-insensitive and performed client-side by listing existing recipes before calling `createRecipe`.

---

## Commands

```
Usage: ocrimport <command> [flags]

Commands:
  import   Scan --inbox and enqueue source pages
  ocr      Run the local OCR service over pending source pages
  draft    Convert OCR text to a RecipeDraft JSON using the local LLM
  review   Map draft ingredients to the catalog and produce a review report
  persist  Persist approved reviews to LENA2 via createRecipe
  version  Print version
  help     Show this help
```

---

## Troubleshooting

- **`OCR is not configured.`** — set `LENA_OCR_URL`.
- **`Ollama is not configured.`** — set `LENA_OLLAMA_URL`.
- **`API URL is not configured.`** — set `LENA_API_URL` for `review`/`persist`.
- **`Admin token is not configured.`** — `persist` requires `LENA_IMPORT_ADMIN_TOKEN` because `createRecipe` is admin-only.
- **Low OCR confidence** — pages with mean word confidence below `LENA_OCR_CONFIDENCE_THRESHOLD` are marked `failed`; rescan or adjust preprocessing.
- **GPU not present** — Ollama falls back to CPU and a 7B model becomes very slow. The importer should warn if the model is not GPU-resident.

---

## Further reading

- `docs/recipe-ocr-phases.md` — phased implementation plan.
- `docs/recipe-ocr-import.md` — original design doc with architecture decisions and risks.
- `docker-compose.import.yml` — optional import service definitions.
