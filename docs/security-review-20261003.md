# Security Review — 2026-10-03

Pre-launch offensive review of the Docker stack (Caddy → api → db → web →
ocr → ollama). Findings were confirmed by live probes against the running
stack plus code review. All confirmed findings are remediated on
`uat-fixes-2`.

## Findings and remediations

### F1 — IP rate limiter bypass via spoofed `X-Forwarded-For` (fixed)

**Probe**: 80 requests with a unique `X-Forwarded-For` each produced zero
HTTP 429s; the limiter keyed on the spoofed value.

**Root cause**: `servers { trusted_proxies static private_ranges }` made
Caddy preserve inbound XFF for private-range peers, and the API trusted
broad compose-private CIDRs, so the spoofed head of the chain became the
limiter key. Worst victim: `/auth/session/{provider}` — each attempt
triggers an outbound call to a third-party token endpoint.

**Fix**: removed `trusted_proxies` from the Caddyfile entirely (correct
for the direct-to-internet topology — Caddy now rewrites XFF from the
real peer); pinned the compose subnet and Caddy's address; narrowed
`LENA_TRUSTED_PROXY_CIDRS` to `172.31.0.10/32`; added a global (non-IP)
token bucket on the code-exchange endpoint
(`LENA_AUTH_CODE_EXCHANGE_RATE_LIMIT_*`, default 60/min burst 20); added
`remote_ip` to request logs.

**Verified**: spoofed-IP floods now draw 429s; `remote_ip` logs the real
peer; the global bucket throttles code-exchange floods regardless of
source IPs.

### F2 — No HTTP security headers (fixed)

Web responses exposed `X-Powered-By: Next.js` with no CSP, XFO,
Referrer-Policy, XCTO, Permissions-Policy, or HSTS.

**Fix**: `header` block in the Caddyfile adds the full set. CSP allows
`accounts.google.com/gsi/` in script/style/connect/frame-src for Google
Identity Services; `script-src` keeps `unsafe-inline` because the
nonstandard Next.js build emits inline bootstrap scripts. `Server` and
`X-Powered-By` are stripped. HSTS is emitted unconditionally (browsers
ignore it on plain HTTP; it activates under TLS).

### F3 — GraphQL introspection open to all authenticated users (fixed)

Any signed-in member could enumerate the full schema including admin-only
mutations (133 mutations enumerated live).

**Fix**: `graphql.RestrictIntrospection(bff.AllowIntrospectionForAdmins(...))`
— introspection runs for admin role only, and
`LENA_GRAPHQL_DISABLE_INTROSPECTION=true` disables it entirely.
graphql-go drops `__schema`/`__type` selections for disallowed callers
(`data: {}`) rather than erroring; `__typename` still works, as required
by union/interface resolution. Verified live: member gets `{"data":{}}`,
admin gets the full schema.

### F4 — `is_searchable` defaulted to TRUE, documented as opt-in (fixed)

`migrations/0028` created `is_searchable BOOLEAN NOT NULL DEFAULT TRUE`
while the code comments describe opt-in behavior — every account was
enumerable via `searchHouseholdUsers` by default.

**Fix**: `migrations/0044_searchable_optin` sets `DEFAULT FALSE` and
backfills all existing rows to `FALSE` (intentional: users re-enable via
`updateMyProfile { isSearchable }`). Verified live: opted-in user is
findable, opted-out is not; the toggle persists.

### F5 — Refresh token in `localStorage` (fixed)

The 30-day refresh credential sat in `localStorage`, so any XSS would
yield persistent account takeover even after the XSS was patched.

**Fix**: browser refresh tokens moved to an `HttpOnly; SameSite=Strict`
cookie path-scoped to `/auth/session`, `Secure` when the request is HTTPS
(direct TLS or `X-Forwarded-Proto: https` from a trusted edge). The JSON
`refreshToken` field remains for mobile clients, and the refresh/revoke
endpoints accept either form (body wins when both present). Rotation
re-issues the cookie; invalid/reused credentials and revoke clear it. The
web client stores only a non-secret `lena_session_hint` marker and
migrates legacy `localStorage` tokens on the next refresh. Verified live:
cookie-only refresh returns 200 + rotated cookie; revoke returns 204 +
`Max-Age=0` clear; cookie attrs confirmed in `Set-Cookie`.

## Defense-in-depth already present (verified during review)

- Household scoping enforced in SQL, not just resolvers; invite accept/
  decline/cancel verify party membership; restricted `HouseholdUser`
  projection does not leak email/role.
- Admin gating via `@admin` schema directive — member gets
  `forbidden: admin role required`.
- Session rotation with family revocation on refresh-token reuse;
  sessions endpoint rejects session-token-created sessions (step-up).
- Upload pipeline: media sniffing, pixel/page/size caps, hardened OCR
  container (read-only root, tmpfs, pids/mem limits, no-new-privileges),
  prompt-injection delimiters treating OCR text as untrusted data.
- GraphQL cost controls: depth ≤15, query length cap, field-resolution
  budget, 4 MB body limit, per-user rate/concurrency limiters, AI
  timeout isolation, sanitized error codes.
- Edge minimality: only Caddy publishes a host port; db/api/ollama/seq
  are compose-internal.
- AI assistant tools are read-only; mutations go through normal
  resolvers.
- No `dangerouslySetInnerHTML`/`eval`/raw `innerHTML` sinks in the web
  client; no app-level middleware (CVE-2025-29927 n/a).

## Residual risks / follow-ups

- **TLS by default**: local dev runs plain HTTP; production must set
  `CADDY_ADDR=<domain>` for automatic Let's Encrypt (cookie `Secure` and
  effective HSTS depend on it).
- **`script-src 'unsafe-inline'`** in CSP is required by the current
  Next.js build; a nonce/hashed-script build would allow removing it.
- **Searchability backfill is one-way**: opting out every existing user
  is privacy-correct but means re-opt-in is required for invite search.
- **Member test account** (`member@lena.local`, id 2, household 2) exists
  in the local UAT database only.
- Provider code-exchange endpoint has no CSRF `state` check beyond
  per-provider requirements; the SameSite=Strict cookie and JSON-only
  contract limit the practical exposure.
