# LENA2 Code Audit — Status & Resume Guide

Six-phase review/audit of `JRAdams472/LENA2`. **Audit only: no application logic is changed.** The only
files produced are Markdown reports in `audit/` plus the running findings log
`docs/20260911-audit-findings.md`.

- Branch: `audit` (branched from `main` @ `c4ad3c7`)
- Origin session: https://app.devin.ai/sessions/2b5cbafd5dde47dca75ee08458aee55c
- Last updated: 2026-09-11

## Status

| Phase | Scope | Report | Status | Commit |
|---|---|---|---|---|
| 1 | Architecture & design (docs vs. code, modular-monolith boundaries, BFF orchestration, `internal/platform/`) | `audit/phase-1-architecture.md` | done — 18 findings (4 high / 9 med / 5 low) | `66c87b5` |
| 2 | BFF & GraphQL API layer (`internal/bff/*.go`, `cmd/lena/main.go`) | `audit/phase-2-bff.md` | done — 22 findings (2 high / 10 med / 10 low) | `6f9472f` |
| 3 | Domain services (`inventory`, `recipe`, `mealplan`, `grocery`, `wine`, `identity`, `userprefs`, `analytics`, `ocrimport`, `recipeimport`) | `audit/phase-3-domains.md` | **not started — next** | — |
| 4 | Unit & integration tests (`*_test.go`, `internal/bff/mock/`, `generate.go`; run `go test ./...` + coverage) | `audit/phase-4-tests.md` | not started | — |
| 5 | Docker, deployment & CI (`Dockerfile`, `docker-compose*.yml`, `Caddyfile`, `.dockerignore`, `.env.example`, `scripts/`; flag empty `.github/workflows/`) | `audit/phase-5-docker-deploy.md` | not started | — |
| 6 | Security & exploit review (auth/authz, rate limiting, GraphQL DoS, SQL injection surface, OCR/import file handling, SSRF, CORS, secrets; run `go vet ./...` + `golangci-lint`) | `audit/phase-6-security.md` + `audit/summary.md` | not started | — |

Running findings log (append-only, IDs `A<phase>-<nn>`): `docs/20260911-audit-findings.md`
(currently `A1-01`–`A1-18`, `A2-01`–`A2-22`).

## Working rules (from the original task)

1. Each phase is independent and ends with a committed report; stop after each phase and wait for
   explicit approval before starting the next.
2. Every finding records: category (poor design / bug / antipattern / code smell / cognitive complexity /
   security / possible exploit), severity (critical / high / medium / low), file path + line numbers,
   description, why it matters, concrete remediation.
3. Each report starts with a severity-ranked summary table.
4. Append each phase's findings to `docs/20260911-audit-findings.md`; never renumber earlier entries.
5. Commit to `audit`; no PR has been opened yet (per "commit and stop"). `AGENTS.md` asks for a PR against
   `main` when a phase is complete — open one at the end (or when the user asks).

## Phase-by-phase plan for the remaining work

### Phase 3 — Domain services
- For each domain package: read `service.go` (hand-written), sqlc output (`db.go`, `models.go`,
  `queries.sql.go`) and `queries.sql`; note generated vs. hand-written separately.
- Focus: business-logic bugs, transaction handling via `internal/platform/dbtx`, input validation,
  concurrency (read-modify-write, missing `FOR UPDATE`), complexity hotspots.
- Leads already captured in earlier phases to confirm at the domain level: A1-01 (cross-schema SQL in
  `recipe`/`analytics`), A1-04 (`grocery.Generate` creates an empty list), A1-06 (`inventory.DeleteItem`
  two deletes without tx), A1-13 (`recipeimport.ListPending` per-status loop), A2-01 (no atomic
  quantity updates in `userprefs`/`grocery`), A2-06 (nutrient amount has no basis), A2-11
  (`GetNutrientTypeByName`→`CreateNutrientType` race).
- Output `audit/phase-3-domains.md`; append `A3-xx` to the log; commit; stop.

### Phase 4 — Tests
- Inventory all `*_test.go`, `internal/bff/mock/`, `generate.go`; run `go test ./... -cover` (integration
  tests may need Postgres via `internal/platform/testenv` — check docker-compose).
- Focus: coverage gaps, tautological assertions, missing error-path tests, over-mocking (note the
  test-only non-transactional fallbacks in `resolver_grocery.go` / `resolver_userprefs.go`), flakiness,
  whether integration tests exercise real behaviour.
- Output `audit/phase-4-tests.md`; append `A4-xx`; commit; stop.

### Phase 5 — Docker, deployment & CI
- Review `Dockerfile`, `docker-compose.yml`, `docker-compose.e2e.yml`, `docker-compose.import.yml`,
  `Caddyfile`, `.dockerignore`, `.env.example`, `scripts/`.
- Focus: image pinning, reproducibility, secret handling (`POSTGRES_PASSWORD` defaults), least privilege
  (`nobody` user, `/data/import` permissions — relates to A2-20 / recipe-scan inbox), healthchecks,
  exposed ports/networks, logging.
- Explicitly flag empty `.github/workflows/` vs. `AGENTS.md` verification requirement; recommend a CI
  pipeline (build, `go test`, `golangci-lint`, `go vet`, web client lint/build).
- Output `audit/phase-5-docker-deploy.md`; append `A5-xx`; commit; stop.

### Phase 6 — Security & exploit review + summary
- Cross-cutting pass: `internal/bff/auth.go` + `internal/platform/currentuser` (issuers/audiences,
  `requireAdmin`, protected/admin emails), `ratelimit.go` (XFF trust — see A2-21), GraphQL
  depth/complexity/DoS (see A2-10), SQL injection surface (confirm all queries are sqlc-parameterised),
  OCR/import file paths (`internal/ocrimport`, `nutrition_ocr.go`, `recipe_scan.go`, inbox), SSRF via
  `ocrclient`/`ollamaclient`, CORS, secret exposure. Items deferred from Phase 2 to here: A2-02 (401 vs
  503 semantics), A2-16 (popularity-count inflation), A2-21 (IP spoofing via XFF).
- Run `go vet ./...` and `golangci-lint run` (config `.golangci.yml`); include relevant output.
- Output `audit/phase-6-security.md` and `audit/summary.md` (top findings across all phases, prioritised);
  append `A6-xx`; commit; stop; then open the PR `audit` → `main`.

## How to resume

```sh
git fetch origin && git checkout audit && git pull
cat audit/README.md docs/20260911-audit-findings.md   # status + all findings so far
go build ./...                                        # baseline built cleanly at c4ad3c7
```

Then start the next "not started" phase above, following the working rules.

## Notes / caveats recorded so far

- `docs/go-rewrite-spec.md` and `docs/lena-go-postgres-rewrite-plan.md` (named in the task) do not exist
  on `main`; `docs/graphql-bff-orchestration.md` and the other `docs/` files were used as the baseline.
  No ADRs exist.
- Line numbers in the reports refer to `main` @ `c4ad3c7`.
- `go build ./...` passed; no tests, `go vet`, or lint have been run yet (scheduled for Phases 4 and 6).
