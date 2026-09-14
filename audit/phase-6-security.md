# Phase 6 — Security & Exploit Review

Branch: `audit` · Base: `main` · Scope: cross-cutting security pass over `internal/bff/auth.go`, `internal/platform/currentuser`, `requireAdmin` / `canModifyItem` gates on every mutation, `internal/bff/ratelimit.go`, GraphQL DoS controls in `cmd/lena/main.go`, SQL parameterisation (`internal/*/queries.sql` + sqlc output), OCR/import file handling (`internal/bff/nutrition_ocr.go`, `recipe_scan.go`, `internal/ocrimport`, `internal/recipeimport`), external clients (`ocrclient`, `ollamaclient`), CORS, and secret/credential handling. Deployment-level security was covered in Phase 5 and is cross-referenced rather than repeated.

No application code was modified. Findings are numbered `A6-xx` and appended to `docs/20260911-audit-findings.md`. The consolidated cross-phase view is in `audit/summary.md`.

## Tooling results

| Tool | Command | Result |
|---|---|---|
| `go vet` | `go vet ./cmd/... ./internal/...` | clean, exit 0 |
| `golangci-lint` v2.13.2 (repo `.golangci.yml`: bodyclose, errcheck, errorlint, gocritic, gosec, govet, ineffassign, misspell, noctx, revive, sqlclosecheck, staticcheck, unused) | `golangci-lint run ./cmd/... ./internal/...` | `0 issues.` |
| `govulncheck` | run in CI (`test.yml:40-43`) | not re-run locally; CI gate exists |
| Secret scan | grep for Google client IDs / API keys across repo and history | none found; only `dummy`/placeholder values |

Static tooling is clean. Everything below was found by manual review; none of it is detectable by the configured linters.

## Authorization matrix (verified)

All 107 `Mutation` fields were mapped to their resolver's first guard:

- 59 call `requireAdmin` (catalog CRUD, wine reference data, identity role/ban, recipe import lifecycle, `SubmitRecipeScan`, `CreateNutrientType`).
- 46 call `userFromContext` and operate on the caller's own rows via `u.UserID` (never a client-supplied user id) or on items gated by `canModifyItem` (admin, or the submitting user while the item is still `pending`).
- `ApproveBrand/RejectBrand/ApproveItem/RejectItem` delegate to `setBrandStatus/setItemStatus`, which call `requireAdmin`.
- No mutation was found without a guard. `/metrics` is behind the same bearer middleware as `/graphql`; `/health` and `/ready` are intentionally public and do not leak error text.
- Protected-admin and last-admin guards exist in `identity.Service` (race noted in A3-05).

## Severity-ranked summary

| ID | Severity | Category | Location | Title |
|---|---|---|---|---|
| A6-01 | high | possible exploit | `internal/bff/nutrition_ocr.go:20-60`; `resolver_inventory.go:499` | Any member can create global `NutrientType` reference rows from OCR text of their own pending item, bypassing the admin-only `CreateNutrientType` gate |
| A6-02 | medium | security | `internal/bff/auth.go:107-140` | Issuer and audience are validated independently (flat lists), not as pairs — with more than one issuer configured, issuer A tokens are accepted for issuer B's audience |
| A6-03 | medium | security | `internal/bff/auth.go:175-180`; `identity/queries.sql:12-24` | Admin bootstrap and protected-admin checks are keyed on e-mail, but identity is keyed on `(provider, subject)`; any trusted issuer that asserts the e-mail with `email_verified=true` obtains admin |
| A6-04 | medium | possible exploit | `cmd/lena/main.go:176`; `Caddyfile`; `internal/bff/ratelimit.go:36-40` | IP rate limiter keys on `X-Forwarded-For` via `ExtractIPFromXFFHeader()`; correctness depends on Caddy stripping client-supplied XFF (no `trusted_proxies` configured) and on `api:8080` never being reachable directly |
| A6-05 | medium | security | `cmd/lena/main.go:249-269`; `config.go:42-55`; `internal/bff/resolver_*.go` | No query cost/complexity limit or per-request deadline; authenticated users can issue 120 req/min of depth-15 queries with unbounded nested lists (A2-07, A2-10) |
| A6-06 | medium | security | `internal/bff/auth.go:81-99`; `errors.go:47-66` | Auth failures are unlogged 401s (A2-02); no audit trail for failed logins, JWKS outages, banned-user attempts, admin promotions, or admin mutations |
| A6-07 | medium | security | `internal/bff/recipe_scan.go:55-74`; `nutrition_ocr.go:103-116`; `tools/ocr/app.py:120-146` | Uploads are trusted by declared media type only — no magic-byte sniffing, no image dimension bound before rasterisation; PDFs are rendered by poppler in a root container (A5-16) |
| A6-08 | medium | security | `internal/recipeimport/service.go:364-380`; `internal/platform/ollamaclient` | OCR text → LLM prompt → JSON draft pipeline has no prompt-injection defence; LLM output is trusted into review JSON that becomes recipe rows on `Approve` |
| A6-09 | low | security | `internal/bff/auth.go:157-159`; `identity/queries.sql:23-30` | `UpsertUser` on every request rewrites `email`/`display_name` from the token; unverified e-mails are persisted and shown to admins |
| A6-10 | low | security | `cmd/lena/main.go:278-292` | CORS: `AllowCredentials=true` with an origin allowlist is fine, but `*` mode is a footgun and `PUT/DELETE` are allowed for a POST-only API |
| A6-11 | low | security | `internal/bff/nutrition_ocr.go:116-119`; `resolver.go:95-110` | Fire-and-forget OCR returns `true` before work runs (A2-11); a member can saturate the async worker pool and OCR sidecar for all users with 6 MB images |
| A6-12 | low | security | `config.go:55,68,71` | `GRAPHQL_BODY_LIMIT=4M` makes `NUTRITION_PHOTO_MAX_BYTES` (6 MB) and `RECIPE_SCAN_MAX_BYTES` (20 MB) unreachable via base64 — limits are inconsistent and the effective bound is undocumented |
| A6-13 | low | security | `internal/bff/auth.go:195-223`; `discoverJWKSURI` | OIDC discovery and JWKS fetch follow redirects and read unbounded bodies; issuer list is config-only so SSRF is not client-reachable, but a compromised issuer response can OOM |
| A6-14 | low | security | `clients/web/lib/api.ts:53-59` | ID token stored in `sessionStorage` (with `localStorage` migration path) — readable by any XSS; no CSP set at the edge (A5-12) |
| A6-15 | low | security | `internal/bff/graphql_tracer.go`; `handler` | Introspection enabled in all environments; schema exposes admin mutations to enumerate |
| A6-16 | low | code smell | `internal/inventory/queries.sql:32` | `LIKE '%' || regexp_replace($2, ...) || '%'` is parameterised (safe) but a term of `%`/`_` after stripping is empty and matches everything — full-table scan per keystroke |

Totals: 1 high · 7 medium · 8 low.

## Positive controls observed

- Every SQL statement is sqlc-generated with positional parameters; no `fmt.Sprintf`/concatenation into SQL was found in hand-written code. The single `LIKE` pattern is built server-side from a bound parameter.
- Identity is derived from the authenticated context (`currentuser.User`) everywhere; no resolver accepts a `userId` argument for per-user data.
- JWT validation uses `jwt.Parse` with `WithKeySet` + `WithValidate` (exp/nbf), `kid`-guided refresh is rate-limited (`jwksMinRefreshInterval`), and `alg` is bound to the JWK.
- Admin promotion requires `email_verified=true`; banned users are rejected on every request even with a valid token.
- Uploaded scans are written under the configured inbox with a server-generated filename (`scan-<ms>-<hex>.<ext>`), `0o600`, no client-controlled path component. `ocrimport` work directories are derived from queue ids, not filenames.
- Depth (15), query length (8 KiB), body size (4 MiB), HTTP timeouts, and two-tier (IP, then user) rate limiting are configured.
- Error responses are sanitised to `not found` / `internal server error` with a request id; `/ready` hides DB error text.
- No credentials in the repository or history; `.env*` ignored by git and Docker.

## Findings

### A6-01 — Members can create global nutrient reference data via OCR
- Category: possible exploit · Severity: high
- Location: `internal/bff/nutrition_ocr.go:20-60` (`processNutritionPhoto` → `inv.CreateNutrientType(ctx, label, n.Unit)` for every OCR label not found by name); `resolver_inventory.go:499` (`CreateNutrientType` mutation is `requireAdmin`); `nutrition_ocr.go:83-96` (`SubmitItemNutritionPhoto` only requires `canModifyItem`, i.e. any member on their own pending item).
- Description: a member submits a pending item, then uploads a "nutrition label" image whose text contains arbitrary strings; the OCR parser turns each line into a nutrient label and the async job inserts new rows into the shared `inventory.nutrient_type` table. No profanity filter, length bound, or allowlist is applied to the label; `Unit` is likewise OCR-derived. The row is created with the service's audit identity, not the submitting user's.
- Why it matters: this is a privilege bypass — reference data that the API deliberately restricts to admins can be polluted by any account (spam, offensive names, thousands of junk types per minute within the 120 req/min budget). Polluted types appear in every user's nutrition UI and cannot be attributed to the abuser.
- Remediation: never create reference rows from the member path — match against existing types only and stage unknown labels for admin review (a `pending_nutrient_type` table or the existing review JSON pattern); alternatively run the async job with the submitting user's identity and require `IsAdmin` for creation. Add the profanity filter and a length cap to labels.

### A6-02 — Issuer/audience not validated as a pair
- Category: security · Severity: medium
- Location: `internal/bff/auth.go:107-110` (issuer ∈ `cfg.Issuers`), `:134-140` (audience ∩ `cfg.Audiences` ≠ ∅); `docker-compose.e2e.yml:27-28` (two issuers, two audiences).
- Description: trust is `issuers × audiences`, not `{(issuer, audience)}`. Any token from any trusted issuer bearing any trusted audience passes. In the e2e overlay a `testissuer` token with `aud=<google client id>` is accepted, and a Google token issued to the `lena-e2e-client` audience would be too.
- Why it matters: the design comment says adding a provider "only requires appending to these lists"; doing so silently widens trust across providers. A second-provider client id that is public (client ids are not secret) lets a token minted for a different relying party at provider A be replayed against LENA if provider B's audience happens to be listed.
- Remediation: configure `LENA_AUTH_PROVIDERS` as `issuer=audience[,audience]` tuples (or a JSON list) and check the audience against the matched issuer's set only.

### A6-03 — E-mail-keyed admin bootstrap across issuers
- Category: security · Severity: medium
- Location: `internal/bff/auth.go:175-180`; `identity/service.go:36-41,98-99` (protected e-mails); `identity/queries.sql:12-24` (`ON CONFLICT (provider, external_subject)`).
- Description: identity rows are unique per `(provider, subject)`, so the same e-mail at two issuers is two accounts — each gets admin if the e-mail is in `LENA_ADMIN_EMAILS`, and each is un-bannable if it is in `LENA_PROTECTED_EMAILS`. Whether `email_verified` is trustworthy depends entirely on the issuer.
- Why it matters: with a single issuer (Google) this is acceptable. Combined with A6-02 and any additional or self-hosted issuer (e.g. the e2e `testissuer` left enabled — see A5-03), an attacker who can obtain a `verified` e-mail claim from any trusted issuer becomes a permanent, protected admin.
- Remediation: scope admin/protected lists to `issuer:email` (or `provider:subject` after first login), and never allow the bootstrap allowlist to promote on more than one provider; log every promotion (A6-06).

### A6-04 — XFF-based rate-limit key trusts the proxy chain implicitly
- Category: possible exploit · Severity: medium
- Location: `cmd/lena/main.go:172-176` (`e.IPExtractor = echo.ExtractIPFromXFFHeader()`); `internal/bff/ratelimit.go:36-40`; `Caddyfile:2-4` (no `trusted_proxies`); `docker-compose.yml:51-88` (`api` not published on the host, good).
- Description: Echo's XFF extractor walks the header right-to-left skipping private/loopback ranges, so the key is the first non-private hop. Whether a client can forge the key depends on Caddy removing incoming `X-Forwarded-For` (Caddy ≥ 2.5 does so by default unless `trusted_proxies` is set — this is my understanding of Caddy's behaviour; it should be verified against the pinned `caddy:2.8.4`). If a deployment exposes `api:8080` directly (e.g. a `ports:` override, the `run-local.ps1` `-ApiUrl ...:8080` example), a client sends `X-Forwarded-For: 1.2.3.<n>` and gets a fresh 300 req/min budget per value, neutralising the pre-auth limiter that protects JWKS refresh and the DB `UpsertUser`.
- Why it matters: the IP limiter is the only pre-authentication defence; the code comment acknowledges reliance on Caddy but nothing enforces it.
- Remediation: use `echo.ExtractIPFromXFFHeader(echo.TrustLoopback(true), echo.TrustPrivateNet(false), echo.TrustIPRange(<caddy subnet>))`, or `ExtractIPFromRealIPHeader` with Caddy's `X-Real-IP`; add `trusted_proxies private_ranges` to the Caddyfile; add a test that a spoofed XFF does not change the limiter key.

### A6-05 — No query-cost budget or per-request deadline
- Category: security · Severity: medium
- Location: `cmd/lena/main.go:249-251` (depth 15, length 8 KiB only); `config.go:58-61` (server timeouts, but `WriteTimeout` 30 s does not cancel resolver work); resolvers with nested list fields (`recipes { items { item { nutrients … } } }`, `mealPlans { slots { items { … } } }`).
- Description: depth and byte length do not bound cardinality. A member can request every recipe with every item with every nutrient in one 8 KiB query and repeat it 120×/min; nested paths still N+1 (A2-07). There is no `graphql.MaxParallelism`-style cost limit, no `context.WithTimeout` per request, and list limits are clamped only on some resolvers (`clamp(args.Limit, 1, 50|100)` in inventory/recipe; identity uses 100; several lists have no `limit` argument at all).
- Why it matters: cheap authenticated DoS against Postgres; an admin's own account is enough to take the service down accidentally.
- Remediation: add a per-request `context.WithTimeout` middleware (e.g. 10 s), a complexity estimator (graph-gophers supports custom validation via `graphql.MaxParallelism`/`Tracer` hooks, or pre-parse with `graphql-go/graphql/language`), and clamp every list resolver.

### A6-06 — No security audit logging
- Category: security · Severity: medium
- Location: `internal/bff/auth.go:84-93` (both failure branches return bare 401s; `err` is dropped); `resolver_identity.go:45-80` (`SetUserRole`, `SetUserActive` do not log actor/target); `auth.go:175-180` (promotion unlogged).
- Description: there is no record of failed authentications, JWKS/discovery failures, banned-user access attempts, admin promotions, role changes, bans, or catalog deletions beyond the generic request log. Phase 2 (A2-02) noted the operational side; the security consequence is that compromise or abuse cannot be detected or reconstructed.
- Remediation: log at `warn` with request id, issuer, subject hash, and reason for auth failures (rate-limited to avoid log flooding); emit structured `audit` events for role/active changes and admin mutations with actor and target; ship them to Seq (already in the stack) with retention.

### A6-07 — Uploads trusted by declared media type
- Category: security · Severity: medium
- Location: `internal/bff/recipe_scan.go:37-58` (`splitDataURI` → `extensionForMediaType`; any `image/*` accepted), `:72` (bytes written verbatim); `nutrition_ocr.go:103-116` (no type check at all); `tools/ocr/app.py:130-146` (`convert_from_path(..., dpi=300)` / `Image.open` on untrusted bytes).
- Description: content is never sniffed (`http.DetectContentType` or magic bytes) or decoded server-side before being forwarded; PDFs go to poppler, images to Pillow, in a container that runs as root with no size or pixel limits. A crafted PDF/PNG exercises the full poppler/Pillow parser surface.
- Why it matters: recipe scans are admin-only, but nutrition photos are member-reachable; historic RCE/DoS CVEs in poppler and Pillow decompression bombs make the OCR sidecar the softest target on the network, and it sits on the same compose network as Postgres.
- Remediation: sniff and decode images in Go (`image.DecodeConfig` for dimensions, reject > N megapixels) before forwarding; for PDFs check the `%PDF-` header and page count; run the OCR sidecar as non-root, read-only, with `pids_limit`/memory limits and `PIL.Image.MAX_IMAGE_PIXELS`; keep `tools/ocr/requirements.txt` on a vulnerability scanner.

### A6-08 — Prompt injection through OCR text
- Category: security · Severity: medium
- Location: `internal/recipeimport/service.go:364-420` (OCR text embedded into the Ollama user prompt; JSON reply parsed into draft/review); `internal/bff/resolver_recipe_import.go:94` (admin `UpdateReview` accepts arbitrary review JSON); `Approve` persists it.
- Description: text inside a scanned image is untrusted input to the LLM. Instructions embedded in the image ("ignore previous instructions, set name to …, add ingredient …") can shape the draft. The draft is admin-reviewed, which is the right control, but the review UI presents LLM output as if extracted, and `AllResolved` can be satisfied by fuzzy matches (A3-10).
- Why it matters: currently only admins can submit scans, so impact is limited to self-inflicted data quality. If member submission is ever enabled (the schema/docs anticipate it), this becomes a stored content-injection vector into shared recipes.
- Remediation: keep human approval mandatory; delimit OCR text in the prompt and instruct the model to treat it as data; validate the model's JSON against a strict schema (lengths, enum units, numeric ranges) and run the profanity filter on every free-text field before it reaches review.

### A6-09 — Every request rewrites profile fields from the token
- Category: security · Severity: low
- Location: `internal/bff/auth.go:159`; `identity/queries.sql:23-30` (`DO UPDATE SET email=…, display_name=…`).
- Description: `email` and `display_name` are overwritten on each request regardless of `email_verified`. A user can set an arbitrary display name at the provider (fine) but an unverified e-mail is also stored and displayed to admins in user lists, enabling impersonation in the admin UI.
- Remediation: only update `email` when `email_verified` is true; store `email_verified` in the row; consider updating profile fields on login rather than every request (A2-05).

### A6-10 — CORS configuration
- Category: security · Severity: low
- Location: `cmd/lena/main.go:278-292`.
- Description: allowlist mode sets `AllowCredentials=true` (harmless for bearer-token auth but unnecessary) and permits `PUT`/`DELETE` although only `POST /graphql` and `GET` probes exist. `*` mode disables credentials correctly but is one env var away in production.
- Remediation: restrict methods to `GET, POST, OPTIONS`; drop `AllowCredentials`; refuse `*` unless `LENA_ENV=dev`.

### A6-11 — Member-triggered async work is unbounded per user
- Category: security · Severity: low
- Location: `internal/bff/nutrition_ocr.go:116-119`; `resolver.go:95-110` (`runAsync` pool); `ratelimit.go` (per-user 120/min).
- Description: each `SubmitItemNutritionPhoto` costs the caller one request but costs the server an OCR round-trip (up to `OCR_TIMEOUT`=20 s) and DB writes. 120 calls/min from one account keep the worker pool and the single OCR sidecar busy for everyone; when the pool is full the request still returns `true` (A2-11).
- Remediation: per-user concurrency cap (one in-flight OCR per user), queue with backpressure returning a `PENDING`/`REJECTED` status, and a dedicated lower rate limit for upload mutations.

### A6-12 — Inconsistent upload limits
- Category: security · Severity: low
- Location: `config.go:55` (`GRAPHQL_BODY_LIMIT=4M`), `:68` (`NUTRITION_PHOTO_MAX_BYTES=6291456`), `:71` (`RECIPE_SCAN_MAX_BYTES=20971520`); `.env.example:44`.
- Description: base64 inflates by 4/3, so the largest decodable payload through `/graphql` is ≈3 MB; the documented 6 MB / 20 MB limits can never be reached, and raising the body limit to honour them would raise the DoS surface of every other query.
- Remediation: either move uploads to a dedicated multipart endpoint with its own body limit and the same auth middleware, or set the documented limits to match the body cap.

### A6-13 — Unbounded reads from issuer endpoints
- Category: security · Severity: low
- Location: `internal/bff/auth.go:215` (`jwk.Fetch`), `:256-275` (discovery body decoded without `io.LimitReader`; default redirect following).
- Description: issuers are config-controlled, so this is not client-reachable SSRF. But a compromised/misbehaving issuer (or DNS hijack in a non-TLS issuer like the e2e `http://testissuer`) can return a multi-GB body and exhaust memory while the global auth mutex is held (A2-04).
- Remediation: wrap bodies in `io.LimitReader(…, 1<<20)`, use `jwk.Fetch` with a custom `http.Client` that disables redirects, and require `https://` issuers outside e2e.

### A6-14 — Token storage in web client
- Category: security · Severity: low
- Location: `clients/web/lib/api.ts:53-59`; `clients/web/app/auth/AuthProvider.tsx`; `Caddyfile` (no CSP).
- Description: the Google ID token is kept in `sessionStorage` (per-tab, cleared on close — a reasonable choice) with a `localStorage` fallback for migration. Any XSS in the Next.js app reads it. No `Content-Security-Policy`, `X-Frame-Options`, or `Referrer-Policy` are set at the edge or by Next.
- Remediation: add a strict CSP via Caddy `header` or `next.config.ts` headers; remove the `localStorage` fallback after migration; consider a BFF session cookie (`HttpOnly`, `SameSite=Lax`) exchanged for the ID token so the browser never holds a bearer token.

### A6-15 — Introspection always on
- Category: security · Severity: low
- Location: `internal/bff/resolver.go:816-830` (`NewGraphQLHandler` schema options); no environment switch.
- Description: authenticated members can introspect the full schema including all admin mutations and argument shapes. Not a vulnerability by itself (auth gates hold) but it lowers the cost of probing.
- Remediation: disable introspection outside dev, or restrict it to admins via the existing context.

### A6-16 — Empty search term matches everything
- Category: code smell · Severity: low
- Location: `internal/inventory/queries.sql:32` (`LIKE '%' || lower(regexp_replace($2, '[^a-zA-Z0-9]', '', 'g')) || '%'`); `resolver_inventory.go:81` (`clamp(args.Limit, 1, 50)`).
- Description: the query is safely parameterised, and `regexp_replace` strips `%`/`_`, so injection is not possible. But a term consisting only of stripped characters becomes `LIKE '%%'`, scanning the whole table per call; the resolver does not reject terms that are empty after normalisation.
- Remediation: normalise in Go, reject terms shorter than 2 characters, and add a trigram index if fuzzy search is required.

## Suggested remediation order

1. A6-01 — close the reference-data bypass (small code change, high impact).
2. A6-02 / A6-03 — pair issuer with audience and scope admin lists to an issuer before any second provider is added.
3. A6-04 / A6-05 / A6-06 — trusted-proxy configuration, per-request deadline + cost limit, security audit logging.
4. A6-07 / A6-08 — content sniffing and OCR sidecar hardening; prompt delimiting and strict draft validation.
5. Remaining low items with the Phase 5 edge-hardening work (A5-12).
