# Remediation Phase 8 — CI/Docker hygiene & test coverage

- **Branch:** `audit-review-phase8` (cut from latest `main` after the Phase 7 PR is approved)
- **Theme:** CI/Docker hygiene & test coverage — pin images and actions, narrow the build context,
  stop exposing observability ports, make logging optional, version the seed step, and close the
  remaining test-coverage gaps (non-admin authorization, honest GraphQL test helper, OCR/upload paths,
  brand moderation, mock-layer diet, `recipeimport` fake store).
- **Source reports:** `audit/phase-5-docker-deploy.md`, `audit/phase-4-tests.md`, `audit/summary.md`
  (top-15 #15; themes "Deployment defaults", "Test suite")
- **Note:** this phase is last because none of its findings block correctness work, and several of its
  test items (A4-07, A4-09) are cheaper once Phases 4 and 5 have landed the code they test.

## Findings in scope

| ID | Severity | One-line summary | Target file(s) |
|---|---|---|---|
| A5-05 | Medium | Mutable image tags, no digests, `ollama:latest`; builder Go 1.27 vs `go.mod` 1.26.6; mixed Alpine bases | all `Dockerfile`s; `docker-compose.import.yml:5,30`; `go.mod:3` |
| A5-06 | Medium | `COPY . .` ships whole repo into builder; `.dockerignore` misses `clients/mobile`, `tools`, `import`, `audit`, CSVs | `Dockerfile:9-10`; `.dockerignore` |
| A5-07 | Medium | Seq UI (5341) and GELF UDP (12201) published on all interfaces, unauthenticated; logs leave network as plaintext UDP | `docker-compose.yml:82-88,103-107,127-129,138-139` |
| A5-08 | Medium | GELF driver + `seq-gelf` mandatory for `api`/`web`; daemon-side `localhost` breaks on Desktop/rootless/remote Docker | `docker-compose.yml:75-76,82-88,100-107` |
| A5-09 | Medium | Actions pinned by major tag not SHA; `govulncheck@latest`; no top-level `permissions:` | `.github/workflows/test.yml` (all `uses:`), `:42`; `.github/workflows/cleanup.yml` |
| A5-10 | Medium | Seed job re-runs every `up`, outside migration versioning; API blocked on seed success; CSV unreferenced | `docker-compose.yml:32-49`; `migrations/seed/` |
| A4-04 | Medium | E2E suite runs every mutation as admin; no HTTP-level non-admin authorization test | `internal/bff/bff_integration_test.go:51,148` |
| A4-05 | Medium | `doGraphQL` discards decode errors and only logs GraphQL errors; partial-success responses pass | `internal/bff/bff_integration_test.go:684-711` |
| A4-06 | Medium | OCR/scan upload paths (data-URI parsing, inbox writes, async fan-out) 0% covered | `internal/bff/nutrition_ocr.go`; `internal/bff/recipe_scan.go`; `internal/platform/ocrclient/` |
| A4-07 | Medium | Brand moderation flow (`SubmitBrand`, `SearchBrands`, `PendingBrands`, `Approve/RejectBrand`, `brandWriteError`) 0% covered | `internal/inventory/service.go:79-194`; `internal/bff/resolver_inventory.go:73-175,1014` |
| A4-08 | Medium | Two mock layers mirror the implementation; SQL predicates and `:exec` semantics never observed by unit tests | `internal/*/service_test.go`; `internal/bff/resolver_*_test.go` (353 `gomock.Any()`) |
| A4-09 | Medium | Hand-rolled `memoryStore` diverges from SQL store; `Approve` tested only from pre-set `ready`; setup errors discarded | `internal/recipeimport/service_test.go:18-201,252-270` |

**Roadmap corrections / additions.** All IDs supplied in the roadmap exist with the severities shown.
**A4-05 (Medium)** was not assigned to any phase in the roadmap; it is added here because it is a
prerequisite for A4-04 (a non-admin authorization test is meaningless if the helper swallows GraphQL
errors) and lives in the same test file.

## Remediation steps

1. **A4-05 — make the GraphQL test helper honest (do first; every later test relies on it).**
   1. In `bff_integration_test.go:684-711` make `doGraphQL` `require.NoError` on decode and
      `require.Empty(gr.Errors)` by default; add an explicit `doGraphQLExpectErrors` variant for negative
      tests. Fix any existing test that was passing only because errors were ignored.
2. **A4-04 — non-admin authorization at the HTTP level.**
   1. Add a `tokB`-driven block (`bff_integration_test.go:51,148` already mints tokens) that attempts one
      admin mutation per domain (`approveItem`, `createNutrientType`, `setUserRole`,
      `approveRecipeImport`, `approveBrand`, …) and asserts the GraphQL error code.
   2. Add one case asserting the code is `FORBIDDEN`, not `UNAUTHENTICATED`.
3. **A4-06 — OCR/upload path tests.**
   1. Table tests for `splitDataURI`/`extensionForMediaType`: malformed URI, unsupported media type,
      oversized payload, media type with parameters, sniffed-type mismatch (Phase 5 behaviour).
   2. `httptest`-backed tests for `platform/ocrclient`: non-200, timeout, malformed JSON.
   3. Resolver test with a fake OCR client and a `t.TempDir()` inbox asserting the stored file name is not
      client-controlled and that saturation returns `BUSY`/`PENDING` (Phase 7 behaviour).
4. **A4-07 — brand moderation coverage.**
   1. Mirror the item-moderation tests for brands: `SubmitBrand`, `SearchBrands`, `PendingBrands`,
      `ApproveBrand`, `RejectBrand`.
   2. Integration test submitting a duplicate brand under different casing/whitespace and asserting the
      Phase 4 outcome (one row, `CONFLICT` or existing-row return); test `brandWriteError`/central
      mapping of `23505`.
5. **A4-09 — retire the diverging `recipeimport` fake store.**
   1. Replace `memoryStore` (`service_test.go:18-201`) with the real store over Testcontainers for
      transition tests, or contract-test both implementations against a shared table of transitions.
   2. Add negative `Approve` cases (from `pending`, `processing`, `rejected`, `persisted`) and a
      writer-failure case asserting rollback; use `require.NoError` on all setup calls
      (`service_test.go:252-270`).
6. **A4-08 — mock-layer diet.**
   1. Keep a thin set of gomock tests for error wrapping only; move behavioural coverage to integration
      tests using a shared container per package (`TestMain`, co-located Low A4-12).
   2. In BFF tests replace `gomock.Any()` with `gomock.Eq`/custom matchers on the fields the resolver is
      responsible for computing (start with the 353 occurrences in the resolvers touched by Phases 3–7).
7. **A5-09 — CI supply-chain hygiene.**
   1. Pin every `uses:` in `test.yml` and `cleanup.yml` to a commit SHA (with the version as a comment);
      enable Dependabot `github-actions` ecosystem to keep them current.
   2. Add `permissions: contents: read` at workflow level (grant more per job only where needed, e.g.
      `packages: write` for publish).
   3. Pin `govulncheck@vX.Y.Z` (`test.yml:42`); consider `step-security/harden-runner`.
8. **A5-05 — pin images and align the toolchain.**
   1. Pin `FROM` lines in every `Dockerfile` with `@sha256:` digests (Dependabot/Renovate `docker`
      ecosystem bumps them); use one Alpine base.
   2. Align the builder Go version with `go.mod` (or use `GOTOOLCHAIN=auto` with the `toolchain`
      directive).
   3. Pin `ollama/ollama` to a version in `docker-compose.import.yml:5,30`.
9. **A5-06 — narrow the build context.**
   1. `COPY cmd/ internal/ migrations/ ./` (plus `go.mod`/`go.sum`) instead of `COPY . .` in
      `Dockerfile:9-10`; add `--mount=type=cache,target=/go/pkg/mod`.
   2. Extend `.dockerignore` with `clients/`, `tools/`, `import/`, `audit/`, `scripts/`, `**/*.csv`.
10. **A5-07 / A5-08 — observability off the public interface and optional.**
    1. Move the GELF driver and `seq`/`seq-gelf` services to an optional `docker-compose.logging.yml`
       overlay; default `api`/`web` to `json-file` with `max-size`/`max-file` (or an `x-logging` anchor
       operators can switch).
    2. In the overlay bind `127.0.0.1:5341:80` and `127.0.0.1:12201:12201/udp`, and enable Seq
       authentication (`SEQ_FIRSTRUN_ADMINPASSWORDHASH`); alternatively drop the host hop entirely with a
       Seq/Vector agent on the compose network.
11. **A5-10 — versioned, explicit seeding.**
    1. Convert `migrations/seed/` into numbered migrations (or a `schema_seed` version table) so seeds run
       once; delete the unreferenced CSV or wire it in.
    2. Make `api` depend on `db-migrate` only; run the seed as an explicit `--profile seed` job.

## Co-located Low policy

Fix a Low finding only if it lives in code already being changed for a High/Medium finding in this
phase; never edit code solely to fix a Low. Lows expected to be co-located here: A4-12 (shared container
per package — part of A4-08), A4-13 (per-package coverage floors — same `test.yml` edited for A5-09),
A5-15 (build-once/publish-same-digest, `needs: lint`, SBOM/provenance, authenticated pulls — same
workflow), A5-13 (`container_name` — same compose blocks as A5-07/A5-08), A5-18 (`/ready` probe,
`HEALTHCHECK` — same `Dockerfile`/compose lines as A5-05/A5-06), A5-11/A5-12 (`sslmode`, Caddy
hardening — only if those lines are already in the diff), A5-14 (docs drift — update `docs/deployment.md`
for whatever compose changes land here), A4-14/A4-15 (clock injection, bootstrap tests — only if
those test files are touched).

## Verification

- `go build ./...` passes.
- `go test ./...` passes; coverage gate in CI still ≥ 60% (expect it to rise).
- `golangci-lint run ./...` and `go vet ./...` report no issues.
- Manual: `docker compose config` on the base stack shows no `gelf` driver, no `seq*` services and no
  published `5341`/`12201`; `docker compose -f docker-compose.yml -f docker-compose.logging.yml config`
  shows them bound to `127.0.0.1` only.
- Manual: `docker compose up` twice in a row runs the seed once (check `schema_seed`/migration table);
  `api` starts even if the seed profile is not invoked.
- Manual: `docker build .` succeeds and `docker history` / build log shows only `cmd/`, `internal/`,
  `migrations/`, `go.mod`, `go.sum` copied; image base layers reference `@sha256:` digests.
- Manual: `grep -n "uses:" .github/workflows/*.yml` shows only SHA-pinned refs; workflow YAML has a
  top-level `permissions:` block; CI run is green on the PR.
- Manual: `go test -run TestIntegration ./internal/bff/ -v` shows the non-admin block executing and a
  `FORBIDDEN` assertion; `grep -c "gomock.Any()" internal/bff/*_test.go` is materially below 353.

## Closing instruction

Open a PR from `audit-review-phase8` into `main` summarising the changes above, then **stop**. This is
the final remediation phase; after approval, re-run the audit checklist in `audit/summary.md` to confirm
every High/Medium finding is closed or explicitly deferred.
