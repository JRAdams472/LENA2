# Project Rules for Devin

## Git Workflow

- Each major rewrite phase is developed on its own branch named `phase-<N>` (e.g. `phase-5`).
  - The mobile redesign used a self-describing named series instead: `mobile-redesign-p0` through `mobile-redesign-p5`.
- Do not push commits directly to `main`.
- When a phase is complete, open a pull request against `main` and summarize the changes.
- Only merge after the phase has been verified (build, tests, lint).

## Linear Workflow (required)

- Every plan gets a **parent Linear ticket** (team: `LEN`, MCP server: `linear`). Once planning is done and work begins, the parent moves to **In Progress**.
- Every phase of the plan gets a **sub-issue** under that parent, named for its branch (e.g. `ingredient-layer-p1`), with a description of the phase's scope and acceptance criteria. **Mark the sub-issue In Progress when its phase starts.**
- When a phase is complete, post **proof of completion** on its sub-issue before asking for merge approval:
  - Relevant run logs (build/test/verification output).
  - **Playwright screenshots** of any user-facing change (the `playwright` MCP server is configured — capture real screens, don't describe them).
  - A link/summary of the **GitHub CI run** showing all checks green.
- **No PR or phase merges without the user's explicit approval.** Present the PR link + proof, then wait — the user reviews the evidence and approves the merge.

## Evidence Gathering (Observability)

Seq and Jaeger run locally in the compose stack and are both queryable over plain HTTP — no MCP needed. Use them to attach runtime evidence (logs, traces) to phase proof-of-completion.

- **Seq** (structured logs, `http://localhost:5341`):
  ```bash
  # Recent events as JSON (Properties include level, request_id, latency_ms, source file/line)
  curl -s 'http://localhost:5341/api/events?count=50'
  # Filtered — e.g. errors only (Seq filter expression syntax)
  curl -s 'http://localhost:5341/api/events?count=50&filter=level%20%3D%20%27ERROR%27'
  # Match a specific request seen in the UI/logs
  curl -s "http://localhost:5341/api/events?filter=request_id%20%3D%20%27<req-id>%27"
  ```
- **Jaeger** (traces, `http://localhost:16686` — the v2 API is under `/api/v3/`, NOT `/api/`):
  ```bash
  curl -s 'http://localhost:16686/api/v3/services'                     # -> {"services":["lena2","jaeger"]}
  # start_time_min/max are REQUIRED and must be RFC3339 (unix micros silently returns nothing)
  curl -sG 'http://localhost:16686/api/v3/traces' \
    --data-urlencode 'query.service_name=lena2' \
    --data-urlencode 'query.limit=10' \
    --data-urlencode 'query.start_time_min=2026-01-01T00:00:00Z' \
    --data-urlencode 'query.start_time_max=2030-01-01T00:00:00Z'
  ```
- **Raw fallback**: `docker logs lena2-api-1 --since 30m` when Seq's structured view isn't needed.
- Linear attachments go through `prepare_attachment_upload` → PUT the file to the signed URL with its exact signed headers → `create_attachment_from_upload`. A `SignatureDoesNotMatch` means the headers drifted — request a fresh URL and PUT immediately (URLs expire in ~60s).

## Plan Close-Out

After the final phase of any plan merges, before starting the next:

1. Delete merged phase branches locally and on GitHub (verify PR state with `gh` first).
2. Ensure no new feature is left at zero test coverage — every new service, resolver, or page needs at least one unit or integration test.
3. Re-walk corrected audit findings (`audit/summary.md`) for regressions.
4. Note improvements or new features inspired by the completed work (e.g. `docs/newfeatures.md`).
5. Update `README.md` so features and architecture reflect what shipped — including sweeping for stale claims (e.g. a "no refresh tokens" note surviving after sessions shipped).
6. Review every doc listed in `README.md`'s **Documentation** section plus the touched client READMEs (`clients/web/README.md`, `clients/mobile/README.md`) for staleness — env vars, versions, workflow/job names, helper-package names, and endpoint lists drift fast.
7. Update the GitHub wiki (`LENA2.wiki.git`) — it lives outside the repo, so clone it, add/refresh pages for what shipped, and push to `master`.
8. Refresh wiki **screenshots** when user-facing screens changed: run the isolated `lena2shots` stack per `.devin/skills/wiki-screenshots` (`tools/wiki-shots/seed_demo.py` + `capture.mjs`), inspect every PNG for spinners, publish under `images/`, and embed them on the relevant wiki pages — no wiki page should ship prose-only when a screen exists. Evaluate mobile screenshots the same way; if no emulator is available, note that and keep it as a follow-up.
