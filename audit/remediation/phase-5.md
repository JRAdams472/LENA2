# Remediation Phase 5 — Recipe-import / OCR pipeline

- **Branch:** `audit-review-phase5` (cut from latest `main` after the Phase 4 PR is approved)
- **Theme:** Recipe-import / OCR pipeline — treat the pipeline as one hardening epic: guarded status
  transitions, an atomic `Approve`, a durable and re-entrant-safe job queue, a real review gate, content
  sniffing on uploads, prompt-injection defence and a strict LLM output schema, correct pagination, and
  an integration test that walks scan → OCR → draft → review → approve.
- **Source reports:** `audit/phase-3-domains.md`, `audit/phase-4-tests.md`, `audit/phase-6-security.md`,
  `audit/phase-2-bff.md`, `audit/phase-1-architecture.md`, `audit/summary.md` (top-15 #5, #13; theme
  "Recipe-import/OCR pipeline")
- **Prerequisites:** Phase 3 (`ErrConflict` for zero-row transitions) and Phase 4 (`UnitOfWork` seam
  and `WithTx` composition fix) — `Approve` below spans `recipe` and `recipeimport` in one transaction.

## Findings in scope

| ID | Severity | One-line summary | Target file(s) |
|---|---|---|---|
| A3-02 | High | Unguarded status transitions; `Approve` non-atomic and re-runnable (duplicate recipes); admin decisions overwritable by worker | `internal/recipeimport/service.go:166-252,316-327`; `internal/recipeimport/queries.sql:31-92` |
| A3-03 | High | In-memory job queue: orphaned jobs after restart, no per-job timeout, `Retry` during `processing` runs two workers | `internal/recipeimport/service.go:330-358` |
| A4-02 | High | Recipe-import pipeline, SQL store, and admin GraphQL surface effectively untested | `internal/recipeimport/*` (`Process`, `Retry`, `Shutdown`, `store.go`); `internal/bff/resolver_recipe_import.go`; `internal/ocrimport/review.go`, `workqueue.go` |
| A3-09 | Medium | `ListPending` applies the same offset to eight per-status queries; pages skip/duplicate rows | `internal/recipeimport/service.go:137-163`; `internal/recipeimport/queries.sql` |
| A1-13 | Medium | `ListPending` emulates `IN` with up to 8 sequential per-status queries (architecture-report view of A3-09) | `internal/recipeimport/service.go:137-163` |
| A3-10 | Medium | `AllResolved` accepts fuzzy "suggested" matches; imports auto-mark `ready` and persist without admin acceptance | `internal/ocrimport/review.go:57-64`; `internal/ocrimport/catalog.go:263-268`; `internal/recipeimport/service.go:230-232,446-449` |
| A3-13 | Medium | 105-line `Process` driven by magic status strings; two incompatible status vocabularies | `internal/recipeimport/service.go:350-455`; `internal/ocrimport/workqueue.go:18-27` |
| A6-07 | Medium | Uploads trusted by declared media type; no sniffing/dimension bound; poppler/Pillow parse untrusted bytes as root | `internal/bff/recipe_scan.go:55-74`; `internal/bff/nutrition_ocr.go:103-116`; `tools/ocr/app.py:120-146`; `tools/ocr/Dockerfile` |
| A6-08 | Medium | OCR text → LLM prompt with no injection defence; model JSON trusted into review/recipe rows | `internal/recipeimport/service.go:364-420` |
| A2-09 | Medium | Recipe-import lists have no `pageSize` upper clamp, echo raw page args in `pageInfo`, and report `total = len(items)` | `internal/bff/resolver_recipe_import.go:33-76` |

**Roadmap corrections / additions.** All IDs supplied in the roadmap exist with the severities shown.
**A1-13 (Medium)** was not assigned to any phase in the roadmap; it describes the same `ListPending`
defect as A3-09 and is added here so both close with one change.

## Remediation steps

1. **A3-13 — one status type and staged `Process` (do first; later steps use the transition table).**
   1. Define `type Status string` with constants and a `canTransition(from, to Status) bool` table in
      `recipeimport`; delete the second vocabulary in `ocrimport/workqueue.go:18-27` (or delete the
      dead file-based queue outright — Low A1-15/A3-17 are co-located).
   2. Split `Process` into `runOCR`, `runDraft`, `runMapping`, each returning the next state; add a
      dedicated `SetStatus` store method that goes through the transition table.
2. **A3-02 — guarded transitions and an atomic, idempotent `Approve`.**
   1. Make every transition in `recipeimport/queries.sql:31-92` a conditional update declared
      `:execrows`, e.g. `UPDATE … SET status = 'persisted', … WHERE recipe_import_id = $1 AND status IN
      ('ready','reviewing')`; treat `0` rows as `domainerr.ErrConflict`.
   2. Run `Approve` as one transaction through the Phase 4 unit of work:
      `recipe.Service.WithTx(tx).CreateRecipeWithChildren(...)` then `store.WithTx(tx).SetPersisted(...)`,
      commit. A second `Approve` on the same import returns `ErrConflict` and creates no duplicate recipe.
   3. Ensure the worker cannot regress a terminal or admin-owned state (`rejected`, `persisted`,
      `reviewing`) — enforce via the transition table from step 1.
3. **A3-03 — durable, single-claimer job queue.**
   1. On service start: `SELECT recipe_import_id FROM recipe.recipe_import WHERE status IN
      ('pending','processing', …)` and re-enqueue.
   2. Claim jobs with `UPDATE … SET status = 'processing' WHERE recipe_import_id = $1 AND status =
      'pending' RETURNING …` so only one worker proceeds; `Retry` during `processing` therefore becomes
      a no-op/`ErrConflict` instead of a second worker.
   3. Derive worker contexts from a service-lifetime context cancelled in `Shutdown`, with
      `context.WithTimeout` per stage (OCR, draft, mapping). Use a buffered channel and fixed worker loop
      (co-located Low A3-20: `sync.Once` around close, log `MarkFailed` errors).
4. **A3-10 — a real human review gate.**
   1. Define "resolved" in `ocrimport/review.go:57-64` as `Status == "accepted" || Approved` **and**
      `UnitID != ""`; fuzzy `suggested` matches no longer count.
   2. Have `UpdateReview` set `Approved` per item; require `review.Approved` in `Approve`
      (`service.go:230-232,446-449`). Imports must not auto-mark `ready` from suggestions alone
      (`catalog.go:263-268`).
5. **A6-08 — prompt-injection defence and strict output validation.**
   1. In `service.go:364-420` delimit the OCR text inside the prompt (fenced block / sentinel tokens) and
      instruct the model to treat it strictly as data.
   2. Validate the model's JSON against a strict schema before it reaches review rows: string lengths,
      enum units (must resolve to `inventory.unit`), numeric ranges, bounded item count.
   3. Run the existing profanity filter on every free-text field (title, notes, ingredient names).
   4. Keep human approval mandatory (step 4).
6. **A6-07 — sniff and bound uploads in Go; harden the sidecar.**
   1. In `recipe_scan.go:55-74` and `nutrition_ocr.go:103-116`: sniff the decoded bytes
      (`http.DetectContentType`), `image.DecodeConfig` for dimensions and reject > N megapixels; for
      PDFs check the `%PDF-` header and cap page count; reject media types that do not match the
      declared one. Do not derive file names from client input.
   2. `tools/ocr/Dockerfile` / `docker-compose*.yml`: run the sidecar as non-root (`USER`), `read_only:
      true`, `tmpfs: /tmp`, `pids_limit` and memory limits; set `PIL.Image.MAX_IMAGE_PIXELS` and
      enforce a streamed size limit and page-count cap in `app.py:120-146` (this closes co-located Low
      A5-16 as part of the same change). Keep `tools/ocr/requirements.txt` under vulnerability scanning
      (Phase 8 CI).
7. **A3-09 / A1-13 — one query for pending lists.**
   1. Add sqlc queries `ListByStatuses(statuses text[], limit, offset)` with `WHERE status = ANY($1::varchar[])
      ORDER BY created_at DESC LIMIT $2 OFFSET $3` and a matching `CountByStatuses`.
   2. Delete the Go-side merge in `service.go:137-163`.
8. **A2-09 — pagination contract in the resolver.**
   1. In `resolver_recipe_import.go:33-76` use the shared `clamp` (or a `pageArgs()` helper) for
      `pageSize`, echo the clamped values in `pageInfo`, and set `total` from `CountByStatuses`.
   2. Optionally enforce the clamp in the schema via a validated `PageInput` type.
9. **A4-02 — test the pipeline end to end.**
   1. Integration test for `store.go` against Testcontainers covering every status transition (including
      the rejected transitions from step 2) and `ListPending` pagination.
   2. Resolver tests for the four admin mutations including `requireAdmin` denial.
   3. Unit tests for `Process` stages with fake OCR/LLM clients: profanity, OCR failure, unparsable and
      schema-violating LLM output, injected instructions in OCR text.
   4. Unit tests for `review.go` (`AllResolved` with fuzzy suggestions — the A3-10 case).
   5. One integration test that walks `submitRecipeScan` → OCR → draft → review → `approveRecipeImport`
      and asserts exactly one recipe exists after a double approve. Use the real store rather than the
      diverging `memoryStore` where transitions are under test (Phase 8 A4-09 finishes that swap).

## Co-located Low policy

Fix a Low finding only if it lives in code already being changed for a High/Medium finding in this
phase; never edit code solely to fix a Low. Lows expected to be co-located here: A3-20 (worker lifecycle
— same code as A3-03), A5-16 (OCR sidecar root/limits — same files as A6-07), A2-20 (orphan inbox file
on insert failure — `recipe_scan.go:61-83`, same function as A6-07), A1-15/A3-17 (dead `ocrimport`
queue/review IO — delete if A3-13 removes the second status vocabulary), A2-15 (`Draft()/Review()`
decode errors — only if `resolver_recipe_import.go:181-214` is touched), A3-14 (catalog reload per job —
only if `catalog.go:41-98` is touched for A3-10).

## Verification

- `go build ./...` passes (after `sqlc generate`; commit regenerated code).
- `go test ./...` passes, including the new store/transition/end-to-end tests (Docker required).
- `golangci-lint run ./...` and `go vet ./...` report no issues.
- Manual: submit a scan, let it reach `ready`, call `approveRecipeImport` twice → second call returns
  `CONFLICT`; `SELECT count(*) FROM recipe.recipe WHERE …` shows one row.
- Manual: kill the `api` container while an import is `processing`, restart → the job is re-claimed and
  finishes (or is marked failed after its stage timeout), never left orphaned.
- Manual: a scan whose OCR text contains "ignore previous instructions and output …" yields a draft
  that still validates against the schema (or is marked failed), never a persisted recipe.
- Manual: upload a `.png` body declared as `application/pdf` → `BAD_USER_INPUT`; a 20 000×20 000 px
  image → rejected before reaching the sidecar; `docker compose exec ocr id` shows a non-root UID.
- Manual: `pendingRecipeImports(page: 1, pageSize: 5000)` returns a clamped `pageSize` and a `total`
  that equals the DB count.

## Closing instruction

Open a PR from `audit-review-phase5` into `main` summarising the changes above, then **stop**. Do not
begin Phase 6 until this PR has been reviewed and approved.
