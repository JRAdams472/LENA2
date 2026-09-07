# Recipe OCR Import Plan (Local OCR + Local LLM via Ollama)

Bulk-import a large collection of printed recipes (scanned cookbook pages, recipe cards, photos) into LENA2 with **everything running locally**: a containerized OCR engine produces raw text, a local Ollama model structures that text into LENA2's recipe shape, a mapping step reconciles free-text ingredients/units against the existing `inventory` catalog with a human in the loop, and the result is persisted through the existing admin-only `CreateRecipe` path. No cloud OCR, no cloud LLM, no new write paths into Postgres.

**Status: future feature — design only. Nothing in this document is implemented.** It is written so a later phase can pick it up without re-deriving the architecture; where it names Go symbols, GraphQL types, or compose services it is describing *existing* code that the import must reuse, or *proposed* additions clearly marked as such.

## Decisions locked in
- **Fully local**: OCR and LLM inference run in containers on the same Docker network as `api`/`db`. Neither is exposed on a public port. The only external network activity is the one-time `ollama pull` of model weights.
- **Reuse, don't bypass**: recipes are written *only* via `Resolver.CreateRecipe` → `RecipeService.CreateRecipeWithChildren` (single transaction). The importer never talks to Postgres directly and never inserts into `recipe.*` tables itself.
- **Human review is mandatory** before anything is persisted. The LLM's output is a *proposal*; unmatched ingredients, unknown units, and suspicious quantities are surfaced for an admin to fix or accept.
- **OCR and structuring stay separate stages** by default. A vision-LLM that does both is documented as an alternative, not the default, because it competes for the same 16 GB VRAM budget.
- **Disabled by default**: the feature activates only when new `LENA_OLLAMA_*`/`LENA_OCR_*` env vars are set, mirroring how `OTLPEndpoint` (`LENA_OTEL_EXPORTER_OTLP_ENDPOINT`) is empty-by-default and simply turns trace export off.
- **Hardware assumption**: a single GPU with **16 GB VRAM**. Every model recommendation below is chosen to fit that with headroom for KV cache.

## Current state (relevant findings)
- **Recipe write path is already atomic and admin-only.** `Resolver.CreateRecipe` (`internal/bff/resolver_recipe.go`) calls `requireAdmin(ctx)` (`internal/bff/resolver.go`), converts inputs via `parseRecipeChildren`, then calls `RecipeService.CreateRecipeWithChildren` (`internal/recipe/service.go`), which wraps `createRecipe` + `addRecipeItem` + step inserts in one `withTx`; any failure rolls the whole recipe back (covered by `TestIntegrationCreateRecipeWithChildrenRollback`).
- **Inputs are strict.** `parseRecipeChildren` rejects any recipe item whose `itemId` does not parse (`parseID`) and any unit that `resolveUnitID` cannot resolve. `resolveUnitID` trims the string and calls `InventoryService.GetUnitByName`, which is backed by the sqlc query `GetUnitByName` in `internal/inventory/queries.sql`: `WHERE lower(name) = lower($1) OR lower(abbreviation) = lower($1)`. So "Cup", "cup", and "c" all resolve, but "cups", "1/2 c." or "heaping tbsp" do **not** — unit normalization must happen *before* calling the mutation.
- **`RecipeItemInput` requires a catalog `itemId: ID!`** (`internal/bff/schema.graphqls`), with optional `ingredientId`, plus `quantity: Float!`, `unit: String!`, `section`, `displayOrder`, `notes`, `isOptional`. There is no "free-text ingredient" escape hatch on recipes (unlike `grocery_list_item.manual_item_name`). Every OCR'd ingredient line must therefore be mapped to an `inventory.item` row — this is the hard part of the pipeline.
- **No server-side fuzzy search exists.** `internal/inventory/queries.sql` has `ListItems` (paged, `ORDER BY name`), `GetItemsByIDs`, `ListIngredients`, `ListUnits`, and exact/ID lookups only; there is no `ILIKE`/trigram/full-text item search and `pg_trgm` is not enabled in any migration. The web client filters the full item list in JS (see `docs/surprise-recommendations-plan.md` "Current state"). Ingredient matching will have to be done in the importer over a snapshot of the catalog, or a search query added later.
- **Recipe names are unique.** `recipe.recipe.name VARCHAR(200) NOT NULL UNIQUE` (`migrations/0005_create_recipe.up.sql`) — a re-import of the same page will fail on the second insert rather than silently duplicating, which is useful but produces a poor error unless the importer checks first. There is no `GetRecipeByName` query today.
- **Catalog creation is also admin-only and already exposed.** `Resolver.CreateItem` (`internal/bff/resolver_inventory.go`) takes `CreateItemInput { name, brandId, upc12, upc14, categoryId: ID!, unit: String! }` behind `requireAdmin`; `createIngredient` and `createCategory` mutations exist as well (there is no `createUnit` mutation — the unit catalog is seed data, so unknown units cannot be created by the importer and must be normalized to existing ones). The importer can auto-create missing items through these rather than through SQL.
- **Config pattern.** `config.Config` (`internal/platform/config/config.go`) is loaded with `envconfig.Process("lena", &cfg)`, so every field is a `LENA_*` variable; optional integrations use `default:""` and are treated as "off" when empty.
- **Compose pattern.** `docker-compose.yml` uses one container per concern (`db`, `db-migrate`, `db-seed`, `api`, `web`, `caddy`, `seq`, `seq-gelf`), named volumes (`pg_data`, `seq_data`), `healthcheck` blocks, and `depends_on` with `condition: service_healthy` / `service_completed_successfully`. Only `caddy` is meant to be the public entry point; the current hardening effort (`docs/hardening20260906.md`, phases 25–26) is tightening what else is reachable.
- **Prior AI notes.** `docs/ai-integration.md` already assumes a local Ollama instance (general models like `llama3.x`/`qwen2.5`/`deepseek-r1` are sufficient for culinary tasks, no specialist model needed). This plan is consistent with that and would share the same `ollama` container.

## Pipeline architecture

```
 scanned pages ──► [1] OCR container ──► raw text ──► [2] Ollama (JSON mode) ──► RecipeDraft JSON
                                                                                       │
                                                                                       ▼
                                             [3] Mapping: ingredient → itemId, unit → canonical unit
                                                          │ (unmatched → review queue, admin fixes / auto-creates)
                                                          ▼
                                             [4] Persist: CreateRecipe mutation (admin JWT)
                                                          → RecipeService.CreateRecipeWithChildren (one tx)
```

Each stage produces a file on disk (or a row in a small local SQLite/JSON work-queue owned by the importer, *not* in the LENA2 Postgres schema) so the run is resumable and every intermediate artifact is inspectable. A typical run over a few hundred pages is expected to be dominated by LLM inference time, so stages 1–2 should be batchable and restartable without re-OCRing.

### Stage 1 — OCR (local, containerized)
Input: image or PDF page. Output: plain UTF-8 text plus, where the engine supports it, per-line bounding boxes (useful for column ordering and for showing the admin "where did this come from" during review).

- **Default: Tesseract** (`tesseract-ocr` 5.x with the `eng` traineddata, LSTM engine), packaged as its own small service/container exposing a trivial HTTP endpoint (e.g. `POST /ocr` multipart image → `{ "text": "...", "lines": [...] }`), or invoked as a one-shot CLI container over a mounted `./import/inbox` directory. Simple, well understood, CPU-only (leaves the whole GPU to Ollama), and good enough for typewritten recipe cards and single-column pages. Pre-processing (deskew, binarize, upscale to ~300 DPI, `--psm 4`/`6` page-segmentation modes) matters more than engine choice for card scans.
- **Better for cookbook pages: PaddleOCR or docTR.** Multi-column cookbook layouts, ingredient tables, and mixed font sizes trip up Tesseract's reading-order heuristics. PaddleOCR (PP-OCRv4, with its layout-analysis module) and docTR (detection + recognition with layout export) both preserve block structure far better and are also fully local. They are heavier images and can optionally use the GPU; if run on GPU they must be scheduled so they do not overlap with Ollama inference on the 16 GB card (or pinned to CPU). Recommendation: start with Tesseract, measure the error rate on a sample of ~20 real pages, and switch the OCR container to PaddleOCR/docTR only if column bleed is a recurring failure — the Stage 2 contract (raw text in) does not change.
- **Keep OCR a separate step/container.** It has different dependencies (C++/Leptonica or Python/PyTorch) from the Go API, different scaling characteristics (CPU-bound, embarrassingly parallel), and is the stage most likely to be swapped. It must not be compiled into the `api` image.
- Output is written verbatim to `./import/work/<page-id>.ocr.txt` alongside the source image so the admin can compare during review.

### Stage 2 — Structuring (Ollama, JSON mode)
Input: OCR text for one recipe (pages that span a recipe are concatenated by the operator or by a simple heuristic; the importer should not try to be clever about recipe boundaries in v1). Output: a `RecipeDraft` JSON document.

- Call the local Ollama HTTP API (`POST /api/chat` or `/api/generate`) with **`"format"` set to a JSON schema** (Ollama's structured-output mode; `"format": "json"` is the minimum, but a full schema is strongly preferred so field names and types are enforced by the runtime rather than by prompt wording). Set `temperature` low (0–0.2) — this is extraction, not generation.
- The schema mirrors `CreateRecipeInput` in `internal/bff/schema.graphqls`, but with ingredient references as **strings**, since the LLM cannot know catalog IDs:

  ```json
  {
    "name": "string",
    "description": "string | null",
    "servings": "integer | null",
    "prepTimeMinutes": "integer | null",
    "cookTimeMinutes": "integer | null",
    "items": [
      {
        "quantity": "number | null",
        "unit": "string | null",
        "ingredient": "string",
        "section": "string | null",
        "notes": "string | null",
        "isOptional": "boolean"
      }
    ],
    "steps": [ { "stepNumber": "integer", "instruction": "string" } ],
    "sourceHint": "string | null"
  }
  ```

  `ingredient` is the bare noun phrase ("all-purpose flour", "unsalted butter"); preparation notes ("softened", "finely chopped") go to `notes`; fractions are converted to decimals ("1 1/2" → `1.5`); ranges ("2–3 cloves") take the lower bound with the range recorded in `notes`. `sourceHint` captures anything the model saw that looks like a book/page reference and is used only for idempotency and review, never persisted to `recipe.recipe`.
- The system prompt must instruct the model to **transcribe, not improve**: no inventing missing servings or times, `null` when the page does not say. This is the first line of defence against hallucination (see Risks).
- Ollama's response is validated against the schema again in the importer (never trust the model to have honoured it), and drafts that fail validation are re-prompted once with the validation error appended, then parked for manual attention.
- Draft written to `./import/work/<page-id>.draft.json`.

### Stage 3 — Mapping (ingredients → `itemId`, units → canonical unit; human review)
This is where most of the engineering effort and all of the human effort goes.

**Unit normalization.** Build a small, explicit normalization table in the importer that maps common printed forms to the exact `name`/`abbreviation` strings that `GetUnitByName` will accept (plural → singular, trailing periods stripped, "tablespoons"/"Tbsp."/"T" → `tbsp`, "teaspoons"/"tsp." → `tsp`, "ounces"/"oz." → `oz`, "pounds"/"lbs" → `lb`, "cloves"/"pinch"/"to taste" → handled as count units or `null` quantity + note, etc.). Load `ListUnits` once at start-up and verify every normalization target actually exists in `inventory.unit`; anything that does not normalize is a **review item**, because `resolveUnitID` will otherwise reject the whole recipe. Do not silently coerce an unknown unit to a guess.

**Ingredient → catalog item matching.** Because `RecipeItemInput.itemId` is required and there is no server-side search, the importer snapshots the catalog (`ListItems` paged to exhaustion, plus `ListIngredients` and `ListCategories`) and matches locally:
1. Normalize both sides (lowercase, strip punctuation/brand words/quantifiers, singularize).
2. Exact normalized match on `item.name` → accept.
3. Fuzzy match (token-set ratio / Jaro-Winkler / trigram similarity — any pure-Go implementation is fine) with two thresholds: **≥ 0.92 auto-accept**, **0.75–0.92 suggest** (top 3 candidates shown to the reviewer), **< 0.75 unmatched**.
4. Optionally use `inventory.ingredient` as an intermediate: match the phrase to a generic `Ingredient`, then pick that ingredient's canonical/default `Item` and also populate `RecipeItemInput.ingredientId`. This gives brand-agnostic matching and lines up with how `RecipeItem.ingredient` is already modelled.
5. Everything not auto-accepted lands in the **review queue**.

**Human-in-the-loop review.** The importer emits a review report per recipe (Markdown or a simple local web page — format is an implementation choice) listing: each ingredient line, the OCR text it came from, the proposed `itemId`/name and confidence, the normalized unit, and any quantity that looks implausible (see Risks). The admin can:
- accept a suggested match;
- pick a different existing item (by ID or name);
- **create the missing catalog item** — the importer calls the existing admin `createItem` mutation (`Resolver.CreateItem`, `CreateItemInput { name, categoryId, unit, brandId?, upc12?, upc14? }`) with a reviewer-chosen `categoryId` and default `unit`, then uses the returned `id` as `itemId`. This is opt-in per line (or per run with a flag); it must never happen automatically for low-confidence matches, because items are *global* catalog data shared by every user;
- mark the line `isOptional` / edit `notes`;
- reject the whole recipe (e.g. OCR was garbage; re-scan).

A recipe is only eligible for Stage 4 when **every** item has an `itemId` and a resolvable unit, and the admin has explicitly approved it. Review decisions are stored in the importer's work queue so re-running the pipeline (after a re-scan or a model change) replays them instead of asking again.

### Stage 4 — Persist (existing admin path only)
- The importer authenticates as an **admin** user (obtain an OIDC bearer token for an account listed in `LENA_ADMIN_EMAILS` or already holding the `admin` role) and calls the existing `createRecipe(input: CreateRecipeInput!)` GraphQL mutation on `/graphql`, one recipe per call.
- Server-side this is exactly today's path: `requireAdmin` → `parseRecipeChildren` (final authoritative validation of `itemId` and unit) → `RecipeService.CreateRecipeWithChildren` → one transaction writing `recipe.recipe`, `recipe.recipe_item`, `recipe.recipe_step`. If anything is rejected the whole recipe rolls back and the importer marks it `failed` with the GraphQL error for the admin.
- **Do not bypass this.** Specifically, do not: write to Postgres from the importer; add an "import" SQL script; call the `recipe` package directly from a new `cmd/` binary with its own pool; or relax `parseRecipeChildren`'s unit/ID checks to make import easier. The transaction boundary, admin check, audit columns (`created_by = admin email`), and analytics hooks (`EventRecipeCreated` recorded inside `CreateRecipe`, per `docs/surprise-recommendations-plan.md` Phase 20) all live on the existing path and must apply to imported recipes too.
- Successful persistence records the returned `Recipe.id` in the work queue next to the source page hash (idempotency, below).

## Ollama containerization and model selection (16 GB VRAM)

### Proposed `ollama` service (not yet added)
Follows the existing service pattern in `docker-compose.yml` — dedicated container, named volume, healthcheck, `depends_on`:

```yaml
services:
  ollama:
    image: ollama/ollama:latest          # pin a tag in the real change
    container_name: lena2-ollama
    volumes:
      - ollama_data:/root/.ollama         # model weights survive container recreation
    environment:
      OLLAMA_KEEP_ALIVE: "10m"            # keep the model loaded between recipes in a batch
      OLLAMA_MAX_LOADED_MODELS: "1"       # one model at a time on a 16 GB card
    # NOTE: no `ports:` — reachable only as http://ollama:11434 on the compose network
    healthcheck:
      test: ["CMD-SHELL", "ollama list >/dev/null 2>&1 || exit 1"]
      interval: 15s
      timeout: 5s
      retries: 10
    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              count: 1
              capabilities: [gpu]

  ollama-pull:                            # one-shot warm-up, like db-migrate / db-seed
    image: ollama/ollama:latest
    container_name: lena2-ollama-pull
    entrypoint: ["/bin/sh", "-c"]
    command: ["ollama pull ${LENA_OLLAMA_MODEL:-qwen2.5:7b-instruct}"]
    environment:
      OLLAMA_HOST: http://ollama:11434
    depends_on:
      ollama:
        condition: service_healthy

  ocr:
    build: ./tools/ocr                    # Tesseract by default; swappable for PaddleOCR/docTR
    container_name: lena2-ocr
    # no `ports:`; http://ocr:8000 on the compose network only

volumes:
  ollama_data:
```

- **GPU passthrough** uses the NVIDIA Container Toolkit (`nvidia-container-runtime`) via `deploy.resources.reservations.devices` with `driver: nvidia` and `capabilities: [gpu]`; the host needs the NVIDIA driver + toolkit installed. Without it, Ollama silently falls back to CPU and a 7B model becomes unusably slow for bulk import — the importer should check `GET /api/ps` / `ollama ps` and warn if the model is not GPU-resident.
- These services should live in an **optional compose profile** (`profiles: ["import"]`) or a separate `docker-compose.import.yml` overlay, so `docker compose up` for normal development does not pull multi-GB images or require a GPU.
- `api` should **not** `depends_on: ollama`; the import is an admin tool, and the core app must start and run without the LLM present.

### Model recommendations (must fit 16 GB VRAM, including KV cache)
| Model | Approx. VRAM (default Q4 quant) | Role |
|---|---|---|
| `qwen2.5:7b-instruct` | ~5–6 GB | **Default.** Strong at structured/JSON extraction, good instruction following, lots of headroom for long OCR context (set `num_ctx` to 8k–16k for multi-page recipes). |
| `llama3.1:8b-instruct` | ~5–6 GB | Equivalent alternative default if Qwen's output style is not preferred. |
| `qwen2.5:14b-instruct-q4_K_M` | ~9 GB | Higher quality for messy OCR (misread characters, merged columns); still leaves ~6 GB for context. Roughly 2× slower per recipe. |
| `qwen2.5-vl:7b` / `llama3.2-vision:11b` | ~6–8 GB weights + image tokens | **Vision alternative**: skip Stage 1 and feed the page image directly, letting the model read and structure in one pass. Handles layout better than Tesseract but is slower per page, harder to debug (no intermediate text), and shares the 16 GB with any GPU OCR — an *option to evaluate*, not the default. |

Anything ≥ 30B parameters is out of scope on this hardware. The model name is configuration (`LENA_OLLAMA_MODEL`), so switching between rows is a redeploy, not a code change.

### Warm-up and inference settings
- **Pull on first start** via the `ollama-pull` one-shot service above (or a documented `docker compose exec ollama ollama pull <model>` step in the import runbook). First pull is several GB and is the *only* outbound network access the feature needs.
- **Warm the model** before a batch with a trivial request so the first real recipe does not pay the load latency; `OLLAMA_KEEP_ALIVE` keeps it resident across the batch.
- **Always request structured output** (`"format": <json-schema>`), low temperature, and a generous `num_predict` so long ingredient lists are not truncated mid-JSON.
- Log the model name and digest (`/api/show`) into the work queue for each draft, so results can be attributed when a model upgrade changes behaviour.

## Configuration (proposed additions to `config.Config`)
All optional, empty/zero by default, so the feature is **off** until explicitly configured — same approach as `OTLPEndpoint`:

```go
// Recipe import (OCR + local LLM). All optional; the import tooling is
// disabled unless OllamaBaseURL is set. See docs/recipe-ocr-import.md.
OllamaBaseURL      string        `envconfig:"OLLAMA_BASE_URL" default:""`                 // e.g. http://ollama:11434
OllamaModel        string        `envconfig:"OLLAMA_MODEL" default:"qwen2.5:7b-instruct"`
OllamaTimeout      time.Duration `envconfig:"OLLAMA_TIMEOUT" default:"120s"`
OCRServiceURL      string        `envconfig:"OCR_SERVICE_URL" default:""`                 // e.g. http://ocr:8000
ImportMatchAccept  float64       `envconfig:"IMPORT_MATCH_ACCEPT_THRESHOLD" default:"0.92"`
ImportMatchSuggest float64       `envconfig:"IMPORT_MATCH_SUGGEST_THRESHOLD" default:"0.75"`
```

Resulting env vars (via `envconfig.Process("lena", ...)`): `LENA_OLLAMA_BASE_URL`, `LENA_OLLAMA_MODEL`, `LENA_OLLAMA_TIMEOUT`, `LENA_OCR_SERVICE_URL`, `LENA_IMPORT_MATCH_ACCEPT_THRESHOLD`, `LENA_IMPORT_MATCH_SUGGEST_THRESHOLD`.

Notes:
- If the importer ships as a separate binary (see next section) it can reuse `config.Load()` unchanged, so these belong in the shared `Config` even though the `api` process itself never calls Ollama in v1.
- `LENA_OLLAMA_BASE_URL` and `LENA_OCR_SERVICE_URL` should be validated to be `http://` addresses on the internal network; refuse `0.0.0.0`/public hosts in production builds to prevent accidentally pointing at a cloud endpoint.
- `docker-compose.yml` `api` service would gain the two URL vars only under the `import` profile; the base compose file leaves them unset.

## Proposed integration surface
**Admin-only, reusing existing GraphQL mutations.** No new write path.

- **Ingestion utility** — a new `cmd/recipe-import` Go binary (or `tools/recipe-import`), *not* part of the `api` process. It:
  1. walks an inbox directory of page images/PDFs;
  2. calls the OCR service (Stage 1) and Ollama (Stage 2), caching outputs per page hash;
  3. snapshots the catalog through the **existing GraphQL queries** (`items`, `ingredients`, `units`, `categories`) using an admin bearer token, and runs matching (Stage 3);
  4. writes a review report listing every unmatched/low-confidence ingredient and every unresolvable unit, then **stops** for admin review (or runs interactively);
  5. after approval, optionally calls `createItem` for missing catalog entries, then calls `createRecipe` once per approved recipe (Stage 4), and records the outcome.
- Because it speaks GraphQL as an admin client, it exercises exactly the same authorization (`requireAdmin`), validation (`parseRecipeChildren`), rate limits (`LENA_GRAPHQL_RATE_LIMIT_*` — the importer must pace itself accordingly), and transaction semantics as the web UI. It needs no new BFF interfaces, no changes to `internal/recipe`, and no schema changes.
- **Out of scope for this doc**: a dedicated `importRecipes(pages: [Upload!]!)` GraphQL mutation or a web-UI review screen. Both are natural follow-ups once the CLI pipeline has proven the matching heuristics on real data, and a UI review screen would reuse the same review-queue shape. They are deliberately deferred so that the first iteration does not put multi-minute LLM calls behind an HTTP request or require file-upload support in the BFF.

## Risks, review, and hardening considerations
- **LLM hallucination of quantities/units.** Extraction models will "helpfully" fill in a missing serving count, convert units incorrectly, or turn "2 eggs" into "2 cups eggs". Mitigations: JSON schema enforcement; low temperature; system prompt forbidding inference (`null` when absent); importer-side plausibility checks (e.g. > 10 cups of any spice, `servings` > 50, `prepTimeMinutes` > 24h flagged); side-by-side OCR text in the review report; and — non-negotiable — **no recipe is persisted without an admin approving it**. Auto-accept applies to *matching confidence*, never to skipping review.
- **Ingredient mis-mapping pollutes shared data.** `inventory.item` is global; a wrong `itemId` or a duplicate auto-created item affects every user's grocery lists and nutrition totals. Auto-create is opt-in per line, requires a reviewer-chosen category, and the importer should check for near-duplicate existing items before calling `createItem`. Consider a follow-up to add a server-side `searchItems(term)` query (ILIKE or `pg_trgm`) so matching is not dependent on snapshotting the whole catalog.
- **Keep all traffic local.** `ollama` and `ocr` publish **no host ports**; they are reachable only by service name on the compose network. Do not add them to `Caddyfile`. Ollama has no authentication of its own, so exposing it would allow anyone to run inference (or pull models) on the GPU. This is consistent with the current hardening effort (`docs/hardening20260906.md`, `docs/architecture-hardening.md`) that is reducing the exposed surface to Caddy → `api`/`web`. Model pulls are the one outbound call; in an air-gapped deployment, pre-load `ollama_data` from a trusted machine instead.
- **Idempotency / duplicate detection on re-import.** Two layers: (1) the importer keys its work queue on a content hash of the source page image plus the OCR text, so re-running never re-OCRs, re-prompts, or re-submits an already-persisted page, and stores the resulting `recipe.id`; (2) before `createRecipe`, look the draft name up in the existing recipe list (there is no `GetRecipeByName` today, so v1 pages through `recipes`; adding an exact-name lookup is a small follow-up) and, because `recipe.recipe.name` is `UNIQUE`, treat a constraint violation as "already imported" rather than as a crash. Name collisions between genuinely different recipes ("Pancakes" from two books) are surfaced to the reviewer to rename, not auto-suffixed.
- **Resource contention.** A 16 GB card runs one model at a time. If PaddleOCR/docTR or a vision model is used on GPU, serialize the stages (finish OCR for the batch, then unload, then run structuring) rather than running them concurrently. `OLLAMA_MAX_LOADED_MODELS=1` guards against accidentally loading two models.
- **Rate limiting.** The BFF enforces `LENA_GRAPHQL_RATE_LIMIT_PER_MINUTE`/`_BURST` per user. A bulk import submitting hundreds of recipes must throttle client-side (or the admin temporarily raises the limit for the importing account); do not disable the limiter globally to make import faster.
- **Audit trail.** `created_by` on `recipe.recipe` will be the admin's email, exactly as for manual entry. The importer should additionally keep its own log (page → draft → decisions → recipe id, with model digest) so a bad batch can be identified and the affected recipes deleted via the existing `deleteRecipe` mutation.
- **Copyright.** Bulk-importing published cookbooks into a shared catalog is a content-licensing question for the operator, not a technical one; this doc assumes personal-use import of recipes the household owns.

## Suggested phasing (for when this is picked up)
1. **OCR + structuring spike** — add the `ollama`/`ocr` services under an `import` compose profile, hand-run 20 representative pages through Stage 1–2, measure JSON validity rate and field accuracy per model row in the table above. Decide default model and whether Tesseract is sufficient.
2. **Matching + review CLI** — implement `cmd/recipe-import` through Stage 3 (report only), tune thresholds against the real catalog, build the unit normalization table from `ListUnits`.
3. **Persist** — wire Stage 4 via `createRecipe`/`createItem`, idempotency store, and end-to-end run on the spike pages. Integration test: a fixture OCR text → mocked Ollama response → assert the exact `CreateRecipeInput` that would be sent, and that unresolved units/items block submission.
4. **Follow-ups (optional)** — `searchItems` GraphQL query, `GetRecipeByName`, web review screen, dedicated import mutation.

Each step follows `AGENTS.md`: its own `phase-<N>` branch, PR into `main`, `go build ./...` / `go test ./...` / lint green before merge.
