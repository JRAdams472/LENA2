# Refresh tokens — implementation plan

## Why

Clients re-sign in with Google when the ID token expires (~1 h). Web papers
over this with silent One Tap; mobile just kicks back to login. A
LENA-issued session — short-lived signed access token + rotating opaque
refresh token — gives sessions that survive Google token expiry, with real
revocation.

## Design

- **`identity.session`** — new table: `session_id`, `user_id`,
  `refresh_hash` (SHA-256 of the opaque token), `family_id` (rotation
  lineage), `device` label, `created_at`, `expires_at` (sliding ~30 d),
  `revoked_at`, `replaced_by`. Rotation creates a new row in the same
  family; presenting an already-rotated token revokes the whole family
  (theft detection).
- **Access token** — LENA-signed JWT (jwx/v3, HS256, `LENA_SESSION_SECRET`),
  `iss=lena`, `sub=user_id`, ~15 min expiry. Middleware accepts it alongside
  OIDC ID tokens: `iss=="lena"` → session path; anything else → existing
  JWKS path. OIDC path stays live for `createSession`, the e2e issuer, and
  tools.
- **Mutations** — `createSession(idToken!, device)` → validates the Google
  token via the existing authenticator, upserts the user, returns
  `{accessToken, refreshToken, expiresAt}`; `refreshSession(refreshToken!)`
  → rotate + new access token; `revokeSession(refreshToken)` → sign out.
- **Clients** — refresh token persisted (web `localStorage`, mobile secure
  storage); access token stays in `sessionStorage`/memory. On 401 or
  expired access token → `refreshSession` once → retry; on refresh failure
  → normal Google sign-in. `SilentReAuth` remains as the Google-side
  fallback for the initial credential.
- **Graceful rollout** — existing ID-token requests keep working; clients
  exchange for a session on next sign-in. `LENA_SESSION_SECRET` unset →
  session mutations return `UNAVAILABLE`, OIDC-only mode preserved.

## Phases

### `rt-p1` — schema + session service
`identity.session` migration; `session` service (issue/rotate/revoke,
family reuse-detection, sliding expiry, hashed storage); config
(`LENA_SESSION_SECRET`, `LENA_SESSION_{ACCESS,REFRESH}_TTL`); unit tests
incl. reuse-detected family revocation and expiry edges.
→ PR, verify, merge.

### `rt-p2` — BFF wiring
`createSession`/`refreshSession`/`revokeSession` mutations; middleware
accepts `iss=lena` tokens and loads `currentuser` by `sub` (no provider
upsert); rate-limit `createSession`/`refreshSession` per IP; resolver +
integration tests (rotate happy path, reuse → family revoked, expired →
rejected, disabled → UNAVAILABLE).
→ PR, verify, merge.

### `rt-p3` — web client
`AuthProvider` exchanges the Google credential for a session; refresh
token in `localStorage`, access token in `sessionStorage`; `api.ts` does a
single `refreshSession`+retry on 401/expired access token; `signOut`
revokes; Jest coverage for exchange, refresh-retry, revocation, fallback.
→ PR, verify, merge.

### `rt-p4` — mobile client
`auth_service` session exchange after Google sign-in; refresh token in
`flutter_secure_storage`; proactive refresh before expiry and one retry on
401; sign-out revokes; widget/service tests.
→ PR, verify, merge.

### `rt-p5` — e2e + docs
Playwright spec: mint issuer token → `createSession` → `refreshSession`
round-trip, tampered/reused refresh rejected. Update `docs/auth-oidc.md`,
README auth section, `newfeatures.md` (✅ the item), wiki auth page.
→ PR, verify, merge, then standard closeout.

## Non-goals

- No third-party session providers, no passwordless flows.
- Google remains the only *identity* provider — sessions only replace the
  transport credential, not sign-in itself.
- OIDC ID tokens stay accepted forever for `createSession`, e2e, and
  scripts.
