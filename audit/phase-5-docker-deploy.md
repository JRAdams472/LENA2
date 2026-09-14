# Phase 5 — Docker, Deployment & CI Configuration Review

Branch: `audit` · Base: `main` · Scope: `Dockerfile`, `clients/web/Dockerfile`, `cmd/testissuer/Dockerfile`, `tools/ocr/Dockerfile`, `docker-compose.yml`, `docker-compose.e2e.yml`, `docker-compose.import.yml`, `Caddyfile`, `.dockerignore`, `.env.example`, `.gitignore`, `scripts/run-local.ps1`, `.github/workflows/test.yml`, `.github/workflows/cleanup.yml`, and the deployment sections of `README.md` / `docs/deployment.md`.

No application or configuration files were modified. Findings are numbered `A5-xx` and appended to `docs/20260911-audit-findings.md`.

**Correction to the task brief.** The brief asked to "explicitly flag that `.github/workflows/` is currently empty". On `main` it is not: `test.yml` (build/vet/govulncheck/gofmt/race tests/coverage gate/golangci-lint/web/e2e/Flutter/image publish) and `cleanup.yml` exist and were verified in Phase 4 to match local results. This report audits the pipeline that exists rather than recommending one from scratch. `README.md:225` still refers to `ci.yml` and `docker.yml`, which do not exist (A5-14).

## Severity-ranked summary

| ID | Severity | Category | Location | Title |
|---|---|---|---|---|
| A5-01 | high | security | `docker-compose.yml:6-8,24,59`; `docs/deployment.md:32,45,104` | Application connects as the PostgreSQL superuser; documented least-privilege `lena_app` role is not implemented |
| A5-02 | high | bug | `Dockerfile:22-26`; `docker-compose.yml:70-71`; `internal/bff/recipe_scan.go:61` | Host bind mount `./import:/data/import` overrides image `chown`; `nobody` cannot write the inbox, breaking recipe-scan uploads |
| A5-03 | medium | security | `docker-compose.yml:60,62`; `.github/workflows/test.yml:265`; `clients/web/Dockerfile:13-16` | `dummy` fallbacks for `LENA_GOOGLE_CLIENT_ID`/`LENA_AUTH_AUDIENCES` let the stack start with a non-functional audience check silently |
| A5-04 | medium | security | `docker-compose.yml:66` | Personal e-mail hard-coded as default `LENA_PROTECTED_EMAILS` |
| A5-05 | medium | poor design | `Dockerfile:2,13`; `cmd/testissuer/Dockerfile:2,13`; `clients/web/Dockerfile:2`; `tools/ocr/Dockerfile:1`; `docker-compose.import.yml:5,30`; `.github/workflows/test.yml` | Base images pinned by mutable tag (`golang:1.27-alpine`, `alpine:3.24`, `python:3.12-slim`, `ollama:latest`); no digests; builder Go minor differs from `go.mod` toolchain |
| A5-06 | medium | poor design | `Dockerfile:9-10`; `.dockerignore` | `COPY . .` ships the whole repo (clients, tools, migrations, audit, import) into the builder; `.dockerignore` misses `clients/mobile`, `tools`, `import`, `audit`, `*.csv` |
| A5-07 | medium | security | `docker-compose.yml:82-88,103-107,127-129,138-139`; `Caddyfile` | Seq UI (`5341`) and GELF UDP (`12201`) published on all host interfaces with no auth; application logs leave the compose network as plaintext UDP via the host |
| A5-08 | medium | poor design | `docker-compose.yml:82-88`; `docker-compose.yml:75-76` | GELF logging driver is mandatory: `api`/`web` fail to start when `seq-gelf` is absent, and daemon-side `localhost:12201` breaks on remote/rootless Docker |
| A5-09 | medium | security | `.github/workflows/test.yml:26,28,61,79,101,158,181,189,203,244,253,259` | Third-party actions pinned by major tag, not SHA; `govulncheck@latest` installed unpinned; no top-level `permissions:` block |
| A5-10 | medium | bug | `docker-compose.yml:32-49`; `migrations/seed/` | Seed job re-runs every `up` and depends on idempotent SQL; no guard; `db-seed`/`db-migrate` have no healthcheck-aware retry |
| A5-11 | low | security | `docker-compose.yml:24,59`; `.env.example:16` | `sslmode=disable` default for DB connections; `OTLPInsecure` defaults `true` |
| A5-12 | low | poor design | `Caddyfile`; `docker-compose.yml:109-120`; `docs/deployment.md:80-94` | No TLS, security headers, request-size limit, or timeouts at the edge; `/ready` and `/metrics` not routed; `/health` unauthenticated through the proxy |
| A5-13 | low | code smell | `docker-compose.yml:4,19,34,55,97,111,124,135` | Fixed `container_name`s prevent multiple stacks (e2e alongside dev) on one host |
| A5-14 | low | code smell | `README.md:225,264`; `docs/deployment.md`; `.env.example` | Deployment docs drift: non-existent workflows, `lena_app`/`LENA_DB_PASSWORD`, `auto_https off`, `AUTH_ISSUERS` names not used by compose |
| A5-15 | low | poor design | `.github/workflows/test.yml:134-195,223-268` | E2E job builds images with Docker Hub pulls and no cache; publish job re-builds instead of promoting tested images; no image scan/SBOM/provenance |
| A5-16 | low | poor design | `tools/ocr/app.py:120-160`; `tools/ocr/Dockerfile` | OCR sidecar runs as root, no upload size limit, PDF pages rendered at 300 dpi unbounded; healthcheck only |
| A5-17 | low | code smell | `scripts/run-local.ps1:35-42,59-66`; `scripts/` | Windows-only helper; `.env` parser exports every key (incl. secrets) into the process; infinite health wait loop; no Linux/macOS equivalent |
| A5-18 | low | poor design | `Dockerfile:15`; `docker-compose.yml:77-81`; `docker-compose.e2e.yml:19-23` | `curl` installed into runtime image solely for healthcheck; no `HEALTHCHECK` in image itself; `/health` does not include DB readiness |

Totals: 2 high · 8 medium · 8 low.

## Findings

### A5-01 — Application runs as the PostgreSQL superuser
- Category: security · Severity: high
- Location: `docker-compose.yml:6-8` (`POSTGRES_USER` is the cluster superuser), `:24` (migrate uses it), `:59` (`LENA_DATABASE_URL` uses it); `docs/deployment.md:32,45,102-104` (documents a separate `lena_app` role and `LENA_DB_PASSWORD`).
- Description: the same credential bootstraps the cluster, runs migrations, seeds data, and serves application traffic. The deployment guide describes a least-privilege application role, but nothing in `migrations/` or compose creates it.
- Why it matters: any SQL-level bug or injection (Phase 6) executes with superuser rights: `COPY ... TO PROGRAM`, reading `pg_authid`, dropping schemas. It also defeats the documented cross-domain-schema boundary (A1-01) — nothing prevents a domain from reading another schema.
- Remediation: add a migration (or init script) that creates `lena_app` with `LOGIN`, grants `USAGE` on domain schemas and `SELECT/INSERT/UPDATE/DELETE` on their tables, and `USAGE` on sequences; run `db-migrate` as the owner role and `api` as `lena_app` via a separate `LENA_DB_PASSWORD`; update `docs/deployment.md` to match reality or vice-versa.

### A5-02 — Bind-mounted inbox is not writable by `nobody`
- Category: bug · Severity: high
- Location: `Dockerfile:22` (`chown -R 65534:65534 /data/import` in the image), `:26` (`USER nobody`); `docker-compose.yml:70-71` (`./import:/data/import` bind mount); `internal/bff/recipe_scan.go:61` (`os.MkdirAll(inbox, 0o700)`); `.gitignore` (`/import/` is untracked, so Docker creates it).
- Description: a bind mount replaces the image directory, so the `chown` in the image has no effect. When `./import` does not exist on the host, the Docker daemon creates it owned by `root:root` `0755`; when it does exist it carries the host user's UID. In both cases UID 65534 cannot create `inbox/` or write files, so `SubmitRecipeScan` fails with `create import inbox: permission denied` (or `open ...: permission denied`) on a fresh checkout. The comment at `:67-68` describes a host-side CLI pipeline that shares the directory, reinforcing the mismatch.
- Why it matters: the recipe-scan feature (and the `--profile import` overlay) is broken out of the box on Linux hosts and on CI runners; the "least privilege" of `USER nobody` is nominally satisfied but the feature it guards does not work.
- Remediation: use a named volume (`import_data:/data/import`) so the image's ownership applies, or add an init container / entrypoint step that `chown`s the mount, or document `mkdir -p import && chown 65534 import` in `docs/deployment.md`; add an e2e case that uploads a scan so CI catches it.

### A5-03 — `dummy` audience/client-ID fallbacks
- Category: security · Severity: medium
- Location: `docker-compose.yml:60` (`LENA_GOOGLE_CLIENT_ID: ${...:-dummy}`), `:62` (`LENA_AUTH_AUDIENCES: ${...:-dummy}`), `:96`; `.github/workflows/test.yml:15,265`; `clients/web/Dockerfile:13-16` (build fails only for empty or the literal `__YOUR_GOOGLE_CLIENT_ID__`, so `dummy` passes).
- Description: with no `.env`, the API starts trusting `https://accounts.google.com` tokens whose `aud` is `dummy` — none will validate, so the deployment is silently non-functional rather than failing fast. The web image accepts `dummy` as a valid client ID.
- Why it matters: `POSTGRES_PASSWORD` correctly uses `:?` to hard-fail; the auth-critical variables do not. An operator can ship a stack that looks healthy (`/health` 200) but rejects every login, or — if a real issuer is later added to `LENA_AUTH_ISSUERS` while audiences remain `dummy` — misconfigure trust boundaries without noticing.
- Remediation: use `${LENA_AUTH_AUDIENCES:?...}` in `docker-compose.yml`, keep `dummy` only in `docker-compose.e2e.yml`; extend the web Dockerfile guard to reject `dummy`; have `config.Validate` refuse an audience list containing `dummy` unless a test issuer is configured.

### A5-04 — Personal e-mail as default protected admin
- Category: security · Severity: medium
- Location: `docker-compose.yml:66` (`LENA_PROTECTED_EMAILS: ${LENA_PROTECTED_EMAILS:-aipaloovik@gmail.com}`).
- Description: every deployment that does not override the variable makes this address un-demotable and un-bannable (`identity.IsProtected`). Combined with `LENA_ADMIN_EMAILS` promotion on first sign-in, anyone who controls a Google account with that e-mail (or a look-alike issuer accepted via A5-03) gains a permanent admin on third-party deployments.
- Why it matters: hard-coded identities in infrastructure config are a backdoor pattern and leak PII into the public repository.
- Remediation: default to empty (`${LENA_PROTECTED_EMAILS:-}`), document in `.env.example` (already commented there), and move the operator's value to `.env`.

### A5-05 — Mutable image tags and toolchain skew
- Category: poor design · Severity: medium
- Location: `Dockerfile:2` (`golang:1.27-alpine`), `:13` (`alpine:3.24`); `cmd/testissuer/Dockerfile:2,13` (`alpine:3.21` — different from the API image); `clients/web/Dockerfile:2,20` (`node:26.8.1-alpine`); `tools/ocr/Dockerfile:1` (`python:3.12-slim`); `docker-compose.import.yml:5,30` (`ollama/ollama:latest`); `docker-compose.yml:3,18,110,123,134` (patch-pinned but no digest); `go.mod:3` (`go 1.26.6`).
- Description: no image is pinned by digest; `ollama:latest` is fully floating; the two Go images use `1.27-alpine` while `go.mod` declares `1.26.6` and CI uses `go-version-file: go.mod` — so CI tests one toolchain and the shipped binary is built with another. The API and testissuer runtime images use different Alpine versions.
- Why it matters: builds are not reproducible and a compromised or regressed upstream tag propagates on the next `--build`; toolchain skew can change runtime behaviour (GC, `net/http` defaults) between what CI verified and what runs.
- Remediation: pin `FROM` lines with `@sha256:` digests and let Dependabot/Renovate bump them; align builder Go with `go.mod` (or use `GOTOOLCHAIN=auto` with the `toolchain` directive); pin `ollama/ollama` to a version; use one Alpine base.

### A5-06 — Over-broad build context
- Category: poor design · Severity: medium
- Location: `Dockerfile:9-10` (`COPY . .` before `go build`); `.dockerignore` (excludes `.git`, docs, node_modules, but not `clients/mobile`, `tools/`, `import/`, `audit/`, `migrations/seed/*.csv`, `*.ps1`).
- Description: the entire repository — including the Flutter client, the Python OCR tool, seed CSVs, and any local `import/` uploads — is sent to the daemon and copied into the builder layer. `go build` only needs `cmd/`, `internal/`, `go.mod`, `go.sum` (and `migrations/` if embedded).
- Why it matters: slow builds and cache invalidation on unrelated changes; risk of copying user-uploaded scans or local secrets not matched by the `.env*` patterns (e.g. `*.pem`, `*.json` credentials) into an intermediate layer.
- Remediation: `COPY cmd/ internal/ migrations/ ./` explicitly; extend `.dockerignore` with `clients/`, `tools/`, `import/`, `audit/`, `scripts/`, `**/*.csv`; consider `--mount=type=cache,target=/go/pkg/mod`.

### A5-07 — Observability ports exposed on all interfaces without auth
- Category: security · Severity: medium
- Location: `docker-compose.yml:127-129` (`5341:80` Seq UI), `:138-139` (`12201:12201/udp` GELF), `:82-88,103-107` (log driver targets `udp://localhost:12201` via the host).
- Description: Seq (full log search UI/API) and the GELF ingest port bind to `0.0.0.0` on the host. Seq is started without authentication (`ACCEPT_EULA` only). Application logs — which include request IDs, e-mails, resolver errors with SQL state (A1-02) — travel host-loopback as unencrypted UDP.
- Why it matters: on any host with a public interface (the stack is described as "production-like"), anyone can read logs or inject forged log records. The OCR/Ollama overlay correctly binds to `127.0.0.1`; the logging stack does not.
- Remediation: bind `127.0.0.1:5341:80` and `127.0.0.1:12201:12201/udp`; enable Seq authentication (`SEQ_FIRSTRUN_ADMINPASSWORDHASH`); alternatively drop the host hop by using the `json-file` driver plus a Seq/Vector agent on the compose network.

### A5-08 — Hard dependency on GELF driver and `seq-gelf`
- Category: poor design · Severity: medium
- Location: `docker-compose.yml:75-76,82-88,100-107`.
- Description: `api` and `web` declare `logging.driver: gelf` and `depends_on: seq-gelf`. If Seq is not wanted (CI e2e, small deployments), the containers cannot start; `docker compose logs` shows nothing because the daemon ships logs away. `gelf-address: udp://localhost:12201` is resolved by the daemon, which is wrong for Docker Desktop, rootless, or remote contexts.
- Why it matters: couples application availability to the log sink; hides logs from operators debugging locally; the e2e job (`test.yml:141-156`) therefore also starts two Seq containers on every CI run.
- Remediation: move the logging stack to an optional overlay (`docker-compose.logging.yml`) and default to `json-file` with `max-size`/`max-file`; or use a `x-logging` anchor operators can switch.

### A5-09 — CI supply-chain hygiene
- Category: security · Severity: medium
- Location: `.github/workflows/test.yml:26` (`actions/checkout@v7`), `:28` (`setup-go@v5`), `:61,128,181,189` (`upload-artifact@v7`), `:79` (`golangci-lint-action@v9`), `:101,158` (`setup-node@v4`), `:203` (`subosito/flutter-action@v2`), `:244,253,259` (`docker/*`), `:42` (`go install golang.org/x/vuln/cmd/govulncheck@latest`); file has no top-level `permissions:`; `cleanup.yml:10` correctly scopes `actions: write`.
- Description: all actions are pinned to floating major tags; `govulncheck` is installed from `@latest` on each run; the default `GITHUB_TOKEN` scope applies to `go`, `lint`, `web`, `e2e`, `mobile` jobs (the `docker` job scopes correctly).
- Why it matters: tag-based pinning was the vector for the 2025 `tj-actions` compromise; a hijacked tag executes with repository token in every job.
- Remediation: pin actions to commit SHAs (Dependabot `github-actions` ecosystem keeps them current); add `permissions: contents: read` at workflow level; pin `govulncheck@vX.Y.Z`; consider `step-security/harden-runner`.

### A5-10 — Seed step re-runs unconditionally
- Category: bug · Severity: medium
- Location: `docker-compose.yml:32-49` (`db-seed` loops over `/seed/*.sql` on every `up`), `migrations/seed/0001_reference_data.sql`, `0002_inventory_seed.sql`, `Grocery_UPC_Database.csv` (not referenced by the loop).
- Description: seeding is outside the migration framework, so it runs on every stack start and relies on every statement being idempotent (`ON CONFLICT`). The CSV in the seed directory is not loaded by anything in compose. `api` waits for `db-seed` to complete, so a seed failure blocks the API even if the schema is fine.
- Why it matters: a future non-idempotent seed will fail on second start and take the API down with it; seed order is lexical and untracked.
- Remediation: convert seeds into numbered migrations (or a `schema_seed` version table), or make `api` depend on `db-migrate` only and run seed as an explicit `--profile seed` job.

### A5-11 — Plaintext DB and OTLP transport defaults
- Category: security · Severity: low
- Location: `docker-compose.yml:24,59` (`sslmode=${POSTGRES_SSLMODE:-disable}`); `.env.example:16`; `internal/platform/config/config.go:36-39` (`OTLPInsecure` default `true`).
- Description: inside a single compose network this is acceptable, but the same file is the documented production path and the variable makes it easy to point `LENA_DATABASE_URL` at a remote DB while keeping `sslmode=disable`.
- Remediation: default `POSTGRES_SSLMODE` to `prefer`/`require` in docs and require an explicit opt-out; default `OTLPInsecure=false` when an endpoint is set.

### A5-12 — Edge proxy provides no hardening
- Category: poor design · Severity: low
- Location: `Caddyfile:1-13`; `docker-compose.yml:112-113` (only `80:80`); `docs/deployment.md:80-94` (`auto_https off` guidance).
- Description: Caddy proxies `/graphql`, `/health`, and everything else to `web`. No `request_body max_size` (the API relies on its own limits), no `header` directives (HSTS, `X-Content-Type-Options`, CSP), no rate-limit or timeout, no port 443. `/ready` and `/metrics` are not routed (fine for `/metrics`, but readiness probes cannot reach the API through the proxy). `/health` is public through the proxy, which is acceptable but undocumented.
- Remediation: add `request_body { max_size 25MB }`, a `header` block with standard security headers, and a `:443` site with automatic TLS for production; route `/ready` if an external orchestrator needs it.

### A5-13 — Fixed container names
- Category: code smell · Severity: low
- Location: `docker-compose.yml:4,19,34,55,97,111,124,135`; `docker-compose.e2e.yml:11`; `docker-compose.import.yml:7,31,44`.
- Description: `container_name` disables compose project namespacing; a second stack (e.g. e2e alongside dev, or two branches) fails with name conflicts.
- Remediation: remove `container_name` and rely on `COMPOSE_PROJECT_NAME`.

### A5-14 — Deployment documentation drift
- Category: code smell · Severity: low
- Location: `README.md:225` (refers to `ci.yml`, `docker.yml`), `:264`; `docs/deployment.md:18-47` (`lena_app`, `LENA_DB_PASSWORD`, `DATABASE_URL`, `AUTH_ISSUERS` without `LENA_` prefix), `:80,94` (`auto_https off` not present in `Caddyfile`); `.env.example:11` (`LENA_ADMIN_EMAILS` documented as commented but compose defaults it to empty, fine) vs `docker-compose.yml:66` (protected e-mail default not mentioned).
- Description: the guide describes a stack that differs from the checked-in compose in credentials, variable names, and TLS handling. Operators following it will produce a non-starting stack.
- Remediation: regenerate `docs/deployment.md` from the actual compose files; add a CI step that greps documented env names against `config.go` tags.

### A5-15 — CI build/publish flow
- Category: poor design · Severity: low
- Location: `.github/workflows/test.yml:141-142` (e2e `up -d --build`, no registry cache, Docker Hub pulls for postgres/caddy/seq), `:223-268` (publish job rebuilds from source after tests pass; `push` only on `main`; no cache, no `provenance`/`sbom`, no image scan), `:225` (`needs` omits `lint` and `ocr-import`).
- Description: images published to GHCR are not the images that passed e2e; the publish job can succeed while `lint` fails; unauthenticated Docker Hub pulls in e2e are subject to the same `429` observed in Phase 4 (A4-12).
- Remediation: build once with `docker/build-push-action` (`load: true`, `cache-from/to: gha`), run e2e against those tags, then push the same digests; add `lint` and `ocr-import` to `needs`; enable `provenance: true`, `sbom: true`, and a Trivy/Grype scan step; log in to Docker Hub or mirror base images to GHCR.

### A5-16 — OCR sidecar hardening
- Category: poor design · Severity: low
- Location: `tools/ocr/Dockerfile` (no `USER`, runs as root); `tools/ocr/app.py:120-146` (`await image.read()` with no size cap; `convert_from_path(..., dpi=300)` for every page of an arbitrary PDF; temp file written with default perms); `docker-compose.import.yml:46-47` (bound to `127.0.0.1`, good).
- Description: the API enforces `LENA_RECIPE_SCAN_MAX_BYTES` before forwarding, but the sidecar trusts any caller on the compose network. A large multi-page PDF is rasterised at 300 dpi in memory.
- Remediation: add `USER` to the image; enforce `Content-Length`/streamed size limit and page-count cap in `app.py`; pin `requirements.txt` hashes; run with `read_only: true` and `tmpfs: /tmp`.

### A5-17 — `scripts/run-local.ps1`
- Category: code smell · Severity: low
- Location: `scripts/run-local.ps1:35-42` (exports every `.env` key into the process, including `POSTGRES_PASSWORD`), `:59-66` (`do { } while ($health.status -ne 'ok')` with no timeout), `:100` (passes Google client ID via `--dart-define`, visible in process list).
- Description: single Windows-only convenience script; no POSIX equivalent although the README's primary path is Docker Compose on Linux/macOS.
- Remediation: add a timeout to the health loop; only export the variables actually needed; add a `Makefile`/`run-local.sh` counterpart or drop the script from the deployment surface.

### A5-18 — Healthcheck design
- Category: poor design · Severity: low
- Location: `Dockerfile:15` (`apk add curl` in the runtime image), no `HEALTHCHECK` instruction; `docker-compose.yml:77-81` (`curl -f http://localhost:8080/health`); `cmd/lena/main.go:224-226` (`/health` always 200), `:227-234` (`/ready` checks DB).
- Description: compose health uses `/health`, which does not verify the database, so `api` is "healthy" while `/ready` is 503. `curl` (and its libcurl/OpenSSL surface) is in the production image only for this probe.
- Remediation: probe `/ready` in compose (or make `/health` include a cheap DB ping with caching); replace `curl` with a tiny `-healthcheck` subcommand in the Go binary or a static `wget`; add `HEALTHCHECK` to the image for non-compose runtimes.

## Observations not raised as findings

- Positive: `POSTGRES_PASSWORD` uses `:?` so compose fails fast without a secret; `.env*` is git-ignored and docker-ignored; the OCR and Ollama overlay binds host ports to `127.0.0.1`; `db-migrate` uses a pinned `migrate/migrate:v4.18.2` and mounts migrations read-only; `Caddyfile` is mounted read-only; the API runtime image is a small two-stage `alpine` build with `CGO_ENABLED=0` and runs as `nobody`; the web image runs as `node`; `/metrics` is behind the same bearer auth as `/graphql`; `/ready` deliberately hides DB error text; `cleanup.yml` correctly scopes permissions.
- CI is substantive: build, vet, govulncheck, gofmt, race tests with a coverage gate, golangci-lint v2, web TS/lint/jest/build, full-stack Playwright e2e with a local OIDC issuer, and Flutter analyze/test. Gaps are in supply-chain hygiene and publish flow (A5-09, A5-15), not in absence.
- `docker-compose.e2e.yml` disables both rate limiters and seeds `e2e@example.com` as admin — appropriate for tests, and confined to the overlay.
- GPU reservation in `docker-compose.import.yml:21-27` makes the overlay fail on hosts without the NVIDIA runtime; acceptable for an opt-in profile but worth documenting.

## Suggested remediation order

1. A5-01 (application DB role) and A5-02 (inbox permissions) — one breaks least privilege, the other breaks a shipped feature.
2. A5-03 / A5-04 — remove `dummy` and personal-e-mail defaults from `docker-compose.yml`.
3. A5-07 / A5-08 — bind observability ports to loopback and make the logging stack optional.
4. A5-09 / A5-15 — SHA-pin actions, workflow `permissions`, build-once-promote, base-image mirror.
5. A5-05 / A5-06 — digest pinning, toolchain alignment, narrower build context.
6. Remaining low items alongside a docs refresh (A5-14).
