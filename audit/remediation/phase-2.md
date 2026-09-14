# Remediation Phase 2 — Auth & authorization

- **Branch:** `audit-review-phase2` (cut from latest `main` after the Phase 1 PR is approved)
- **Theme:** Auth & authorization — close the member → global-reference-data privilege escalation, make
  auth failures honest and observable, pair issuers with audiences, scope admin bootstrap to the
  issuer, stop trusting spoofable `X-Forwarded-For`, and add security audit logging.
- **Source reports:** `audit/phase-6-security.md`, `audit/phase-2-bff.md`, `audit/summary.md` (top-15
  #1, #7, #8, #12)

## Findings in scope

| ID | Severity | One-line summary | Target file(s) |
|---|---|---|---|
| A6-01 | High | Members create global `NutrientType` rows via OCR text of their own pending item, bypassing admin-only `createNutrientType`; labels unfiltered | `internal/bff/nutrition_ocr.go:20-60`; `internal/bff/resolver_inventory.go:499`; `internal/inventory/service.go` (`CreateNutrientType`) |
| A2-02 | High | Auth middleware turns every failure (DB/JWKS outages included) into an unlogged `401 invalid token` | `internal/bff/auth.go:81-98,159-165` |
| A6-02 | Medium | Issuer and audience validated as independent flat lists, not pairs; multi-issuer configs cross-accept tokens | `internal/bff/auth.go:107-140`; `internal/platform/config/config.go` (`LENA_AUTH_ISSUERS`/`LENA_AUTH_AUDIENCES`) |
| A6-03 | Medium | Admin bootstrap / protected lists keyed on e-mail while identity is keyed on `(provider, subject)`; any trusted issuer asserting the e-mail gets admin | `internal/bff/auth.go:175-180`; `internal/identity/queries.sql:12-24`; `internal/identity/service.go` |
| A6-04 | Medium | IP rate limiter keyed on `X-Forwarded-For` with no trusted-proxy config; spoofable if `api:8080` is ever reachable directly | `cmd/lena/main.go:176`; `Caddyfile`; `internal/bff/ratelimit.go:36-40` |
| A6-06 | Medium | No security audit logging: auth failures, admin promotion, role/ban changes unrecorded | `internal/bff/auth.go:81-99`; `internal/bff/resolver_identity.go:45-80` |

All six IDs exist in the reports with the severities shown; no corrections were needed.

## Remediation steps

1. **A6-01 — never create reference rows from the member path.**
   1. In `processNutritionPhoto` (`nutrition_ocr.go:20-60`) replace the `GetNutrientTypeByName` →
      `CreateNutrientType` fallback with match-only behaviour: apply nutrients whose type already exists,
      and stage unknown labels for admin review (a `pending_nutrient_type` table, or reuse the existing
      review-JSON pattern from `recipeimport`). Do **not** insert into `inventory.nutrient_type` from
      this path.
   2. If the product decision is instead "auto-create is fine for admins", run the async job with the
      submitting user's identity and require `IsAdmin` before creation; members get the staging path.
   3. Run the existing profanity filter on every label and cap label length (reuse the limit used by
      `createNutrientType`); record the real actor (submitting user's e-mail) instead of the literal
      `"ocr-system"` (the actor half of A2-11 — the `runAsync` half stays in Phase 7).
   4. Prefer moving the whole routine into `inventory.Service.ApplyNutritionLabel(...)` only if it does
      not balloon the diff; otherwise leave that relocation for Phase 6 (A1-09).
2. **A2-02 — typed auth errors, 401 vs 503, and logging.**
   1. Introduce sentinel errors in `auth.go` (e.g. `errTokenInvalid`, `errKeyDiscovery`,
      `errIdentityStore`) and return them from `verifyToken`, the JWKS/discovery path and
      `UpsertUser`/`SetUserRole` respectively.
   2. In the middleware (`auth.go:81-98`) map `errTokenInvalid` → `401`, and `errKeyDiscovery` /
      `errIdentityStore` → `503` with a `Retry-After` header.
   3. Log every non-token failure at `WARN`/`ERROR` with request ID and issuer; log token failures at
      `WARN` with request ID, issuer, subject hash and reason, rate-limited to avoid log flooding
      (shared with A6-06).
   4. Optional: bounded negative cache for repeatedly-seen invalid tokens so garbage-token floods do not
      re-query the IdP.
3. **A6-02 — pair issuers with audiences.**
   1. Replace the two flat lists with an `issuer=audience[,audience]` tuple format (or a JSON list) in
      `platform/config` — e.g. `LENA_AUTH_PROVIDERS` — and keep `LENA_AUTH_ISSUERS`/`LENA_AUTH_AUDIENCES`
      only as a deprecated shim if backwards compatibility is required.
   2. In `auth.go:107-140` resolve the issuer first, then check `aud` against **that issuer's** audience
      set only.
   3. Update `.env.example`, `docs/deployment.md` and `docker-compose*.yml` variable names accordingly
      (compose changes limited to env var names — no other deployment edits in this phase).
4. **A6-03 — scope admin bootstrap and protected identities to the issuer.**
   1. Change `LENA_ADMIN_EMAILS` / `LENA_PROTECTED_EMAILS` semantics to `issuer:email` (or
      `provider:subject` after first login) so a second trusted issuer asserting the same e-mail cannot
      obtain admin.
   2. Never let the bootstrap allowlist promote on more than one provider; once a user row exists,
      promotion should key on `(provider, subject)` in `identity/queries.sql:12-24`.
   3. Log every promotion via the audit events in step 6.
5. **A6-04 — trusted-proxy configuration for the IP extractor.**
   1. In `cmd/lena/main.go:176` replace the bare XFF extractor with
      `echo.ExtractIPFromXFFHeader(echo.TrustLoopback(true), echo.TrustPrivateNet(false),
      echo.TrustIPRange(<caddy subnet>))`, or use `ExtractIPFromRealIPHeader` with Caddy's `X-Real-IP`.
      Make the trusted range configurable (`LENA_TRUSTED_PROXY_CIDRS`).
   2. Add `trusted_proxies private_ranges` to the `Caddyfile`.
   3. Add a test asserting that a spoofed `X-Forwarded-For` from an untrusted peer does not change the
      limiter key (`ratelimit.go:36-40`).
6. **A6-06 — security audit logging.**
   1. Emit structured `audit` log events (actor, target, action, request ID) from
      `resolver_identity.go:45-80` for `setUserRole`, `setUserActive`/ban and bootstrap promotion
      (`auth.go:175-180`), and from `requireAdmin`-gated mutations that change reference data.
   2. Emit auth-failure events per step 2.3 at `WARN`, rate-limited.
   3. Ship them through the existing structured logger to Seq (already in the stack); document retention
      expectations in `docs/deployment.md`.

## Co-located Low policy

Fix a Low finding only if it lives in code already being changed for a High/Medium finding in this
phase; never edit code solely to fix a Low. Lows likely to be co-located here: A6-09 (`UpsertUser`
overwriting `email` regardless of `email_verified` — same `auth.go:159` / `identity/queries.sql` lines as
A6-03), A6-13 (unbounded JWKS bodies / redirects — `auth.go:195-223` if touched for A2-02), A6-10 (CORS
methods — `main.go` only if touched for A6-04), A6-15 (introspection — only if `resolver.go:816-830` is
already being edited).

## Verification

- `go build ./...` passes.
- `go test ./...` passes (integration tests use testcontainers; Docker required).
- `golangci-lint run ./...` and `go vet ./...` report no issues.
- Manual: as a non-admin member, `submitItemNutritionPhoto` on an own pending item whose OCR text
  contains an unknown nutrient label does **not** create a `NutrientType` row (`SELECT count(*) FROM
  inventory.nutrient_type` unchanged); the unknown label appears in the staging location.
- Manual: stop the `db` container (or point JWKS at an unreachable URL) and issue an authenticated
  request — response is `503` with `Retry-After` and a `WARN`/`ERROR` log line, not `401`.
- Manual: with two issuers configured, a token from issuer A carrying issuer B's audience is rejected.
- Manual: `curl -H 'X-Forwarded-For: 1.2.3.4'` directly against `api:8080` from a non-trusted address is
  limited under the real peer IP (observe limiter key in debug log or via burst test).
- Manual: `setUserRole` produces an `audit` log event with actor and target visible in Seq.

## Closing instruction

Open a PR from `audit-review-phase2` into `main` summarising the changes above, then **stop**. Do not
begin Phase 3 until this PR has been reviewed and approved.
