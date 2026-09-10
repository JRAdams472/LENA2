# Recipe OCR Import Operator Guide

The recipe OCR import feature lets an admin bulk-import printed recipes (cookbook pages, recipe cards, scanned PDFs) into LENA2 using only local services: a containerized OCR engine extracts text, a local Ollama model structures the text into a `RecipeDraft`, and the web review UI reconciles free-text ingredients and units against the existing `inventory` catalog before persisting the recipe.

- **Admin-only** — upload, review, and approval are gated behind the admin role.
- **Local-only** — no cloud OCR or cloud LLM is used. The only external network activity is the one-time `ollama pull` of model weights.
- **Human review is mandatory** — the LLM produces a draft; the admin must reconcile every ingredient to a catalog `itemId` and a canonical unit before persistence.

---

## Enabling the feature

The feature is off by default. Set the environment variables in your `.env` file or shell, then start the optional `import` compose profile.

```env
# OCR
LENA_OCR_SERVICE_URL=http://ocr:8000   # empty = feature disabled
LENA_OCR_CONFIDENCE_THRESHOLD=50

# Ollama
LENA_OLLAMA_URL=http://ollama:11434    # empty = feature disabled
LENA_OLLAMA_MODEL=qwen2.5:7b-instruct
LENA_OLLAMA_TEMPERATURE=0.1
LENA_OLLAMA_NUM_CTX=8192
LENA_OLLAMA_VISION_MODEL=              # optional, e.g. qwen2.5-vl:7b

# Importer
LENA_IMPORT_INBOX=./import/inbox
LENA_RECIPE_SCAN_MAX_BYTES=20971520
LENA_IMPORT_AUTO_ACCEPT_CONFIDENCE=0.92
LENA_IMPORT_REVIEW_THRESHOLD=0.75
LENA_IMPORT_WORKER_CONCURRENCY=1
LENA_PROFANITY_EXTRA_TERMS=            # comma-separated extra deny-list terms
```

Start the import services:

```bash
docker compose -f docker-compose.yml -f docker-compose.import.yml --profile import up -d
```

This starts `ollama`, `ollama-pull` (one-shot model download), and the `ocr` service. The core LENA2 stack runs without these services; the `api` starts and remains healthy when OCR or Ollama are not configured.

`docker-compose.import.yml` exposes `ocr` on `127.0.0.1:${LENA_OCR_HOST_PORT:-8000}` and `ollama` on `127.0.0.1:${LENA_OLLAMA_HOST_PORT:-11434}`. If you already have Ollama running locally on port `11434`, set `LENA_OLLAMA_HOST_PORT` to another port (e.g. `11435`) in `.env` and point `LENA_OLLAMA_URL` at `http://localhost:11435`.

---

## Workflow

1. **Upload** — on the `/recipes` page, click *Upload Recipe Scan*, choose a PNG, JPG, or PDF, and the file is written to `LENA_IMPORT_INBOX` and enqueued for background processing.
2. **Review** — open `/recipes/pending` to see imports in progress. The list refreshes every 5 seconds. Click *Review* on a completed import to open the editor.
3. **Edit** — correct the recipe name, description, servings, prep/cook times, ingredients, and steps. For each ingredient, set the catalog `itemId` and `unitId` (or unit name). Add or remove ingredients and steps as needed.
4. **Approve** — once every ingredient is reconciled, click *Save Review* then *Approve*. The recipe is persisted through the existing recipe service and appears under `/recipes`.
5. **Retry / Reject** — a failed, profanity-flagged, or rejected import can be retried from the pending list. Rejected imports are removed from the active queue.

---

## Review and approval

The review UI uses fuzzy string matching to propose catalog items:

- **≥ `LENA_IMPORT_AUTO_ACCEPT_CONFIDENCE`** — auto-accepted during the background draft stage.
- **`LENA_IMPORT_REVIEW_THRESHOLD` – auto-accept** — suggested; shown in the review editor for the admin to pick.
- **< `LENA_IMPORT_REVIEW_THRESHOLD`** — unmatched and flagged for manual review.

For each unmatched or suggested line, set:

- `itemId` — the catalog `inventory.item` id.
- `unitId` — the catalog `inventory.unit` id, or the canonical unit name.
- `approved` — mark the ingredient resolved.

Saving the review re-runs catalog validation and moves the import to `ready` when every item is resolved.

---

## Idempotency

- The API skips a file whose SHA-256 hash already exists in `recipe.recipe_import` (a duplicate upload returns the existing import).
- Approval creates a new recipe; re-approving an already `persisted` import is rejected by the service.
- Retrying resets the import to `pending` and re-runs the full OCR/draft/mapping pipeline.

---

## Troubleshooting

- **`OCR is not configured.`** — set `LENA_OCR_SERVICE_URL` to the OCR service address, or leave it empty to disable the feature.
- **`Ollama is not configured.`** — set `LENA_OLLAMA_URL`, or leave it empty to disable structured extraction.
- **Low OCR confidence** — pages with mean word confidence below `LENA_OCR_CONFIDENCE_THRESHOLD` are marked `failed`; rescan or adjust preprocessing.
- **Profanity detected** — the import is held in `profanity` status. Review the `profanityReason` and edit the offending text, then retry.
- **GPU not present** — Ollama falls back to CPU and a 7B model becomes very slow. Consider reducing `LENA_OLLAMA_NUM_CTX` or using a smaller model on CPU-only hosts.

---

## Logs

When running with Docker Compose, application logs are shipped to Seq:

- Browse the Seq UI at `http://localhost:5341`.
- Filter by `tag = lena2-api` or `tag = lena2-web`.
- For short-lived container output, you can also use `docker compose logs api`.

---

## Further reading

- `docs/recipe-ocr-ui-plan.md` — implementation plan for the web review UI.
- `docs/recipe-ocr-import.md` — original design doc with architecture decisions and risks.
- `docker-compose.import.yml` — optional import service definitions.
