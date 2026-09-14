# Remediation Phase 1 — Deployment & config security

- **Branch:** `audit-review-phase1` (cut from latest `main`)
- **Theme:** Deployment & config security — stop the API running as the Postgres superuser, make the
  recipe-scan inbox writable, and remove the `dummy` / personal-e-mail defaults from `docker-compose.yml`.
- **Source reports:** `audit/phase-5-docker-deploy.md`, `audit/summary.md` (top-15 #3, #6, #8; theme
  "Deployment defaults")

## Findings in scope

| ID | Severity | One-line summary | Target file(s) |
|---|---|---|---|
| A5-01 | High | API and migrations run as the PostgreSQL superuser; documented `lena_app` least-privilege role is never created | `docker-compose.yml:6-8,24,59`; `docs/deployment.md:32,45,102-104`; `migrations/` |
| A5-02 | High | `./import:/data/import` bind mount overrides the image `chown`; `nobody` cannot write the inbox, so recipe-scan uploads fail on a fresh checkout | `Dockerfile:22-26`; `docker-compose.yml:70-71`; `internal/bff/recipe_scan.go:61`; `.gitignore` |
| A5-03 | Medium | `dummy` fallbacks for `LENA_GOOGLE_CLIENT_ID` / `LENA_AUTH_AUDIENCES` let a non-functional auth config start silently | `docker-compose.yml:60,62,96`; `.github/workflows/test.yml:15,265`; `clients/web/Dockerfile:13-16`; `internal/platform/config/config.go` |
| A5-04 | Medium | Personal e-mail hard-coded as the default `LENA_PROTECTED_EMAILS` | `docker-compose.yml:66`; `.env.example` |

All four IDs exist in the reports with the severities shown; no corrections were needed.

## Remediation steps

1. **A5-01 — create and use a least-privilege application role.**
   1. Add a migration (or a Postgres init script under the compose `db` service) that creates
      `lena_app` with `LOGIN`, grants `USAGE` on every domain schema (`identity`, `inventory`, `wine`,
      `recipe`, `mealplan`, `grocery`, `analytics`), `SELECT/INSERT/UPDATE/DELETE` on their tables and
      `USAGE` on their sequences. Keep DDL rights with the owner role only.
   2. Introduce `LENA_DB_PASSWORD` (hard-fail `:?` in compose) and build a second connection string for
      `api` that uses `lena_app`; keep `db-migrate` / `db-seed` on the owner role.
   3. Because new tables created by later migrations will not automatically carry grants, either add
      `ALTER DEFAULT PRIVILEGES … IN SCHEMA <s> GRANT … TO lena_app` for each schema in the same
      migration, or document that every new migration must grant to `lena_app`.
   4. Update `docs/deployment.md` (`:32,45,102-104`) so the documented role, password variable and
      connection strings match what compose actually does (this also retires part of Low A5-14, which is
      co-located in the same doc section).
2. **A5-02 — make the import inbox writable by UID 65534.**
   1. Replace the bind mount `./import:/data/import` in `docker-compose.yml:70-71` with a named volume
      (`import_data:/data/import`) so the image's `chown -R 65534:65534 /data/import` applies. If the
      host-side CLI overlay (`docker-compose.import.yml`) genuinely needs the directory on the host,
      add an init step / entrypoint that `chown`s the mount instead, and document `mkdir -p import &&
      chown 65534 import` in `docs/deployment.md`.
   2. Remove or rewrite the compose comment at `:67-68` that describes the host-side CLI sharing the
      directory.
   3. Add an e2e case (`docker-compose.e2e.yml` / `.github/workflows/test.yml` e2e job) that performs
      one `submitRecipeScan` upload so CI catches a non-writable inbox.
3. **A5-03 — hard-fail on missing auth configuration.**
   1. In `docker-compose.yml` change `LENA_GOOGLE_CLIENT_ID` and `LENA_AUTH_AUDIENCES` (and the web
      build arg at `:96`) from `${VAR:-dummy}` to `${VAR:?…}`; keep `dummy` only in
      `docker-compose.e2e.yml` where the `testissuer` is used.
   2. Extend the `clients/web/Dockerfile:13-16` guard so it rejects the literal `dummy` as well as the
      empty / `__YOUR_GOOGLE_CLIENT_ID__` placeholders.
   3. In `internal/platform/config` `Validate`, refuse an audience list containing `dummy` unless a test
      issuer is configured (the e2e overlay sets one).
4. **A5-04 — drop the personal e-mail default.**
   1. Change `docker-compose.yml:66` to `LENA_PROTECTED_EMAILS: ${LENA_PROTECTED_EMAILS:-}`.
   2. Confirm `.env.example` documents the variable (already present as a comment) and move the
      operator's real value into the untracked `.env`.
5. **Optional structural follow-through (from `summary.md`, "Deployment defaults").** If it keeps the
   diff reviewable, split `docker-compose.yml` into a base file with `:?` for every auth/DB variable and
   a `docker-compose.dev.yml` overlay carrying the convenient defaults. If not, leave this for Phase 8
   and only do steps 1–4.

## Co-located Low policy

Fix a Low finding only if it lives in code already being changed for a High/Medium finding in this
phase; never edit code solely to fix a Low. Lows likely to be co-located here: A5-14 (deployment doc
drift — same `docs/deployment.md` sections as A5-01), A5-11 (`sslmode=disable` default on the same
compose lines as A5-01).

## Verification

- `go build ./...` passes.
- `go test ./...` passes (integration tests use testcontainers; Docker required).
- `golangci-lint run ./...` and `go vet ./...` report no issues.
- Manual: `docker compose up --build` on a fresh clone **without** a `.env` fails fast with the `:?`
  messages for the auth/DB variables (not a silent start).
- Manual: with a valid `.env`, `psql` as `lena_app` can `SELECT` from `inventory.item` but cannot
  `CREATE TABLE` or read `pg_authid`; `SELECT current_user` from the API's connection returns
  `lena_app`.
- Manual: `submitRecipeScan` (admin token) succeeds on a fresh checkout; `docker compose exec api ls -la
  /data/import/inbox` shows files owned by UID 65534.
- Manual: `docker compose config` shows no `dummy` values and no `aipaloovik@gmail.com` in the rendered
  base stack.

## Closing instruction

Open a PR from `audit-review-phase1` into `main` summarising the changes above, then **stop**. Do not
begin Phase 2 until this PR has been reviewed and approved.
