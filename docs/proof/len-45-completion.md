# LEN-45 / P5 — tools: Python, Docker, dev scripts

Phase 5 of the LEN-28 SonarQube megaplan. All tools-side findings cleared:
**SonarQube rescan reports 0 Python, Docker, and JavaScript findings in
`tools/` and `cmd/testissuer/`**. Repo-wide open issues dropped **74 → 59**
(remaining are all TypeScript, scoped to the P6 sweep).

## What changed

### `tools/ocr/app.py` (S5886 ×4, S8410)

- `ocr()` return annotation is now `dict | JSONResponse` — the 4 error-path
  `JSONResponse` returns no longer contradict the `-> dict` hint.
- `image` param uses `Annotated[UploadFile, File()]` per the FastAPI DI rule.

### `tools/wiki-shots/seed_demo.py` (S1192 ×2, S1481)

- `OLIVE_OIL` and `STORE_NAME` module constants replace the duplicated
  `"olive oil"` / `"Corner Market"` literals (3 sites each).
- Removed the unused `found_any` variable in `find_items`.

### `tools/wiki-shots/capture.mjs` (S7772 ×3, S7749)

- `module`, `fs`, `child_process` imports use the `node:` prefix.
- `86400_000` → `86_400_000` (correct 3-digit separator grouping; same value).

### Dockerfiles (S8431 ×3)

- `cmd/testissuer/Dockerfile` (builder + runtime) and `tools/ocr/Dockerfile`:
  `FROM image:tag@sha256:…` → digest-only `FROM image@sha256:…` with the
  version recorded in the adjacent comment (still pinned, no drift).

## Verification

- `python -m py_compile tools/ocr/app.py tools/wiki-shots/seed_demo.py` ✓
- `node --check tools/wiki-shots/capture.mjs` ✓
- SonarQube rescan: 0 findings in `tools/` + `cmd/testissuer/`; total open
  **74 → 59**; no new issues introduced
- OCR import CI job covers the app.py endpoint contract (the union return
  type and `Annotated` param are annotation-only changes)
