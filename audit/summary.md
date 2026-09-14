# LENA2 Audit — Consolidated Summary

Branch `audit`, baseline `main`. Six independent phases, 111 findings, no application code changed. Full per-finding detail (file, lines, rationale, remediation) lives in the phase reports; the append-only log is `docs/20260911-audit-findings.md`.

| Phase | Report | Findings | Critical | High | Medium | Low |
|---|---|---|---|---|---|---|
| 1 Architecture & design | `phase-1-architecture.md` | 18 | 0 | 4 | 9 | 5 |
| 2 BFF & GraphQL layer | `phase-2-bff.md` | 22 | 0 | 2 | 10 | 10 |
| 3 Domain services | `phase-3-domains.md` | 21 | 0 | 3 | 10 | 8 |
| 4 Tests | `phase-4-tests.md` | 16 | 0 | 3 | 7 | 6 |
| 5 Docker, deploy & CI | `phase-5-docker-deploy.md` | 18 | 0 | 2 | 8 | 8 |
| 6 Security | `phase-6-security.md` | 16 | 0 | 1 | 7 | 8 |
| **Total** | | **111** | **0** | **15** | **51** | **45** |

Tooling baseline: `go vet` clean, `golangci-lint` (gosec, errcheck, staticcheck, …) 0 issues, `govulncheck` gated in CI, full `go test -race` suite green at 65.7% hand-written coverage. **Nothing below is detectable by the configured static tooling** — the risk is in semantics, not syntax.

## Overall assessment

The codebase is structurally sound: modular-monolith layout, sqlc-only SQL, identity always from context, admin gates on all 59 privileged mutations, two-tier rate limiting, sanitised errors, and a real CI pipeline. The recurring weaknesses are (1) **silent success** — mutations, auth, async work and tests all report OK when they did not do what was asked; (2) **check-then-act without a lock** — pantry quantities, admin guards, brand dedup, import transitions; (3) **the recipe-import/OCR pipeline** is the least mature subsystem by every measure (design, correctness, tests, deployment, security); and (4) **deployment defaults** that are fine for one developer and dangerous for anyone else (`dummy` audiences, superuser DB role, personal e-mail as protected admin).

## Top 15 findings across all phases

Ranked by exploitability × blast radius × likelihood, not strictly by per-phase severity.

| # | ID | Sev | Theme | One-line summary | Phase |
|---|---|---|---|---|---|
| 1 | A6-01 | high | privilege bypass | Any member creates global `NutrientType` rows via OCR text of their own pending item; admin-only mutation bypassed, no filtering | 6 |
| 2 | A3-01 + A4-03 | high | silent success | Every UPDATE/DELETE is `:exec`; not-found and wrong-owner writes return success — and integration tests assert that they do | 3, 4 |
| 3 | A5-01 | high | least privilege | API and migrations run as the Postgres superuser; documented `lena_app` role never created | 5 |
| 4 | A2-01 + A3-06 | high | lost update | Grocery-toggle → pantry adjust is read-modify-write in a tx without `FOR UPDATE`; concurrent toggles double-apply | 2, 3 |
| 5 | A3-02 + A3-03 | high | integrity | Recipe-import `Approve` non-atomic and re-runnable; in-memory queue loses jobs on restart, no per-job timeout | 3 |
| 6 | A5-02 | high | broken feature | `./import` bind mount defeats image `chown`; `nobody` cannot write inbox → recipe-scan upload fails on fresh checkout | 5 |
| 7 | A2-02 + A6-06 | high/med | observability | Auth failures (incl. DB/JWKS outages) are unlogged 401s; no audit trail for admin promotion, role change, ban | 2, 6 |
| 8 | A6-02 + A6-03 + A5-03 | med | auth trust | Issuer and audience validated as flat lists; admin allowlist keyed on e-mail across issuers; compose defaults `dummy` audience | 5, 6 |
| 9 | A6-05 + A2-07 + A2-10 | med | DoS | No cost limit or per-request deadline; nested paths N+1; depth/length limits do not bound cardinality | 2, 6 |
| 10 | A1-01 | high | architecture | Cross-schema SQL joins in `recipe`/`analytics` break the documented module boundary (and the superuser role makes it invisible) | 1 |
| 11 | A1-02 + A2-08 + A3-04 | high/med | error model | No typed domain errors; BFF matches pgx/SQLSTATE strings; unique-violation mapping inconsistent | 1–3 |
| 12 | A6-04 | med | rate limiting | IP limiter keys on `X-Forwarded-For`, correctness rests on Caddy stripping client XFF and `api:8080` never being exposed | 6 |
| 13 | A6-07 + A5-16 | med | upload surface | Media type trusted from data URI; poppler/Pillow parse untrusted bytes in a root container with no limits | 5, 6 |
| 14 | A4-01 + A4-02 | high | test gaps | Transactional grocery path and the entire recipe-import pipeline (20% pkg, 0% resolver coverage) never executed under test | 4 |
| 15 | A5-09 + A5-15 | med | supply chain | Actions pinned by tag, `govulncheck@latest`, no workflow `permissions`; published image ≠ tested image | 5 |

## Cross-cutting themes

**Silent success.** `:exec` mutations (A3-01), `runAsync` returning `true` when work is dropped (A2-11), auth swallowing causes (A2-02), grocery generation returning an empty list (A1-04/A3-11), seed job re-running (A5-10), tests encoding the wrong behaviour (A4-03/A4-05). Fix pattern: `:execrows` + `ErrNotFound`, return job status not `true`, log failures, make tests assert the *intended* contract.

**Check-then-act races.** Pantry adjust (A2-01/A3-06), last-admin guard (A3-05), brand dedup (A3-08), import transitions (A3-02), nutrient-type creation (A2-11/A6-01). Fix pattern: single-statement atomic SQL (`UPDATE … SET qty = GREATEST(0, qty - $1)`), `WHERE status = $expected` guards, `ON CONFLICT`, `SELECT … FOR UPDATE`.

**Recipe import / OCR.** Design (A1-09, A1-13), correctness (A3-02/03/09/10/13), tests (A4-02), deployment (A5-02, A5-16), security (A6-01, A6-07, A6-08, A6-11). Recommend treating the pipeline as a single hardening epic: persistent queue with status guards, Go-side content sniffing, non-root sidecar, prompt delimiting, strict draft schema, and an integration test that walks scan → OCR → draft → review → approve.

**Deployment defaults.** A5-01, A5-03, A5-04, A5-07, A5-11 all stem from `docker-compose.yml` doubling as dev convenience and "production-like" reference. Split into a base file with hard-fail (`:?`) for every auth/DB variable and a `docker-compose.dev.yml` overlay carrying the convenient defaults.

**Boundaries and error model.** A1-01/02/03/08/10, A2-08, A3-04/12. Typed errors and small role-scoped interfaces unblock most BFF simplification and make the ownership bugs above testable.

## Recommended sequencing

1. **Week-1 fixes (small diffs, high impact):** A6-01 (stop member-path `CreateNutrientType`), A5-03/A5-04 (`:?` for auth vars, drop personal e-mail default), A5-02 (named volume for `/data/import`), A6-04 (trusted-proxy config), A6-06 (log auth failures + admin actions).
2. **Correctness sprint:** A3-01 → `:execrows` + `ErrNotFound` across all domains (then fix A4-03 tests to expect errors); A2-01/A3-06 atomic pantry SQL; A3-05 `FOR UPDATE`; A3-08 `ON CONFLICT`.
3. **Least privilege & auth:** A5-01 `lena_app` role migration; A6-02/A6-03 issuer-scoped audiences and admin lists; A6-09 verified-e-mail-only updates.
4. **Recipe-import epic:** A3-02/03/09/10/13, A6-07/08/11, A5-16, plus A4-02 coverage.
5. **Platform hygiene:** A6-05 request deadline + cost limit, A1-02/A3-04 typed errors, A5-05/06/09/15 pinning and build-once-promote, A5-12/A6-14 edge headers/CSP, docs refresh (A1-18, A5-14).

## Coverage of the original brief

- All six phases delivered as independent commits on `audit`; every finding has category, severity, path+lines, description, impact, remediation, and appears in the append-only log.
- Deviations from the brief, documented in the reports: `docs/go-rewrite-spec.md` and `lena-go-postgres-rewrite-plan.md` do not exist on `main` (Phase 1); `.github/workflows/` is **not** empty — the existing CI was audited instead of recommended from scratch (Phase 5); the first Phase 4 test run was blocked by a Docker Hub 429 and re-run via a registry mirror.
- No PR was opened, per the commit-and-stop instruction. `AGENTS.md` expects a PR into `main` when a phase completes; open `audit` → `main` when ready to land the reports.
