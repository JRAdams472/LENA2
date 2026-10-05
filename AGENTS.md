# Project Rules for Devin

## Git Workflow

- Each major rewrite phase is developed on its own branch named `phase-<N>` (e.g. `phase-5`).
  - The mobile redesign used a self-describing named series instead: `mobile-redesign-p0` through `mobile-redesign-p5`.
- **Ticket-prefixed names (required):** branches and PR titles must start with the Linear ticket number — `LEN-11-close-out` / `LEN-11: Close-out — docs, wiki, full UAT loop`. Every plan gets a Linear ticket and each phase a subticket; keep them linked.
- Do not push commits directly to `main`.
- Before opening a pull request, run `go mod tidy` and commit any `go.mod`/`go.sum` changes — CI fails the lint job when they are not tidy.
- When a phase is complete, open a pull request against `main` and summarize the changes.
- Only merge after the phase has been verified (build, tests, lint).

## Linear Workflow (required)

- Every plan gets a **parent Linear ticket** (team: `LEN`, MCP server: `linear`). Once planning is done and work begins, the parent moves to **In Progress**.
- **Attach the plan document to the parent ticket before creating any phase sub-issues** — the megaplan goes on the parent first, then the subtickets.
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
- **SonarQube Community Build** (`http://localhost:9000`, project key `lena2`):
  - Start: `docker compose -f docker-compose.sonarqube.yml up -d`; wait for `"status":"UP"` via `curl -s http://localhost:9000/api/system/status`.
  - The `sonarqube` MCP server (`.devin/mcp_config.local.json`) exposes issues/hotspots/quality-gate/measures tools; the API is plain HTTP with token auth (`-u "$TOKEN:"`, token is the username, empty password).
  - Rescan: mount source subdirs only — a whole-repo mount fails on `import/inbox` ACLs and crawls `node_modules`:
    ```bash
    docker run --rm \
      -v "$PWD/internal:/usr/src/internal" -v "$PWD/cmd:/usr/src/cmd" \
      -v "$PWD/clients/web/app:/usr/src/clients/web/app" -v "$PWD/clients/web/lib:/usr/src/clients/web/lib" \
      -v "$PWD/clients/web/__tests__:/usr/src/clients/web/__tests__" -v "$PWD/clients/web/e2e:/usr/src/clients/web/e2e" \
      -v "$PWD/clients/mobile/lib:/usr/src/clients/mobile/lib" -v "$PWD/clients/mobile/test:/usr/src/clients/mobile/test" \
      -v "$PWD/migrations:/usr/src/migrations" -v "$PWD/tools:/usr/src/tools" -v "$PWD/scripts:/usr/src/scripts" \
      -v "$PWD/coverage.out:/usr/src/coverage.out" -v "$PWD/clients/web/coverage/lcov.info:/usr/src/clients/web/coverage/lcov.info" \
      -e SONAR_TOKEN="$TOKEN" sonarsource/sonar-scanner-cli \
      -Dsonar.host.url=http://host.docker.internal:9000 -Dsonar.projectKey=lena2 \
      -Dsonar.sources=. -Dsonar.exclusions='**/sqlc/**,**/mock/**,migrations/seed/**' \
      -Dsonar.tests=clients/web/__tests__,clients/web/e2e,clients/mobile/test -Dsonar.test.inclusions='**/*_test.go' \
      -Dsonar.go.coverage.reportPaths=coverage.out \
      -Dsonar.javascript.lcov.reportPaths=clients/web/coverage/lcov.info
    ```
    (On Git Bash use `"$(pwd -W)/internal:..."` for the mount paths.)
  - **Boy-scout rule:** when a change touches a file that has open low/info-severity SonarQube findings, fix them in the same change when the fix is trivial and low-risk (`replaceAll`, `node:` imports, `Number.isNaN`, optional chaining, etc.). Skip nontrivial findings rather than letting a small fix balloon a diff.
- Linear attachments go through `prepare_attachment_upload` → PUT the file to the signed URL with its exact signed headers → `create_attachment_from_upload`. A `SignatureDoesNotMatch` means the headers drifted — request a fresh URL and PUT immediately (URLs expire in ~60s).

## Plan Close-Out

After the final phase of any plan merges, before starting the next:

1. Delete merged phase branches locally and on GitHub (verify PR state with `gh` first).
2. Ensure no new feature is left at zero test coverage — every new service, resolver, or page needs at least one unit or integration test.
3. Re-walk corrected audit findings (`audit/summary.md`) for regressions.
4. File Linear tickets (project **LENA**) for improvements or new features inspired by the completed work — `docs/newfeatures.md` is a shipped-feature history only, not the backlog.
5. Update `README.md` so features and architecture reflect what shipped — including sweeping for stale claims (e.g. a "no refresh tokens" note surviving after sessions shipped).
6. Review every doc listed in `README.md`'s **Documentation** section plus the touched client READMEs (`clients/web/README.md`, `clients/mobile/README.md`) for staleness — env vars, versions, workflow/job names, helper-package names, and endpoint lists drift fast.
7. Update the GitHub wiki (`LENA2.wiki.git`) — it lives outside the repo, so clone it, add/refresh pages for what shipped, and push to `master`.
8. Refresh wiki **screenshots** when user-facing screens changed: run the isolated `lena2shots` stack per `.devin/skills/wiki-screenshots` (`tools/wiki-shots/seed_demo.py` + `capture.mjs`), inspect every PNG for spinners, publish under `images/`, and embed them on the relevant wiki pages — no wiki page should ship prose-only when a screen exists. Evaluate mobile screenshots the same way; if no emulator is available, note that and keep it as a follow-up.
9. **SonarQube rescan gate (required):** if the plan remediated SonarQube findings, run the documented rescan after the final phase merges and confirm **zero new open issues** beyond the plan's accepted `wontfix` list — a finding introduced by the plan's own code is a regression, not backlog. Record the final open-issue count in the close-out proof.

## Code Size & Coverage Report (required on parent-ticket close-out)

When a **top-level** Linear ticket closes (all phase sub-issues merged and verified), post a size-and-shape report as a comment on the parent ticket via `save_comment`. The goal is tracking how the app grows as features ship — report absolute numbers, and deltas vs. the previous report if one exists on an earlier ticket.

Generate it as follows:

1. **Per-language LOC** — tokei (installed via winget; if not on PATH it's at `%LOCALAPPDATA%\Microsoft\WinGet\Packages\XAMPPRocky.Tokei_*\tokei.exe`):
   ```bash
   tokei -e clients/web/node_modules -e clients/mobile/build -e migrations/seed internal cmd clients migrations
   ```
   Note tokei's counts include generated code (`internal/*/sqlc`, `*/mock`) — call that out separately in the report.
2. **Source vs. test split** — bucket tracked files (`git ls-files`) by path: Go tests are `*_test.go`; web tests are `clients/web/__tests__`, `e2e`, `mocks`; mobile tests are `clients/mobile/test`; Go generated is `internal/*/sqlc` + `*/mock`. Everything else is source. Sum `wc -l` per bucket; "executable lines" = non-blank non-comment (`grep -cvE '^\s*(//|$)'` per file).
3. **Test counts** — `grep -c '^func Test'` per Go package; count `it(|test(` in `clients/web/__tests__` + `e2e`; count `test(|testWidgets(` in `clients/mobile/test`.
4. **Coverage** —
   - Go: `go test ./internal/... ./cmd/... -count=1 -cover` — report the per-package table (runs testcontainers; note in the report if container-dependent tests were skipped/failed on environment grounds).
   - Web: `npx jest --coverage` (or read `clients/web/coverage/lcov.info` — always note its date; stale reports must be flagged).
   - Mobile: `flutter test --coverage` if coverage is configured; otherwise state "not measured" — do not estimate.
5. **Post** — `save_comment` on the parent ticket with the report as a markdown table: LOC / executable / test LOC / test count / coverage per feature area, plus a one-line caveats section (generated code excluded, stale reports, skipped suites).
