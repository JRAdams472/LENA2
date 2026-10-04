# LENA — Authentication & OIDC

## 1. Overview

LENA uses **OIDC ID tokens** for authentication. The first release supports **Google**, but the schema and middleware are designed for multi-provider.

## 2. Schema

```sql
CREATE TABLE identity.users (
    user_id             BIGSERIAL PRIMARY KEY,
    provider            VARCHAR(50) NOT NULL DEFAULT 'google',
    external_subject    VARCHAR(255) NOT NULL,
    email               VARCHAR(320) NOT NULL,
    display_name        VARCHAR(200),
    is_active           BOOLEAN NOT NULL DEFAULT TRUE,
    last_login_at       TIMESTAMPTZ,
    created_by          VARCHAR(100) NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by          VARCHAR(100),
    updated_at          TIMESTAMPTZ,
    UNIQUE (provider, external_subject)
);
```

- `provider` + `external_subject` is the stable key.
- `email` is mutable and only for display/audit.

## 3. Token Flow

1. Client (Flutter or Next.js) obtains an ID token from Google.
2. Client sends `Authorization: Bearer <id_token>` on every request.
3. Auth middleware validates the token:
   - `iss` is in the configured allowlist (e.g. `https://accounts.google.com`).
   - `aud` matches the configured client ID.
   - `exp` is in the future.
   - Signature is verified with provider JWKS.
4. Middleware extracts `sub`, `email`, `name`.
5. Middleware calls `identity.UpsertUser(provider, sub, email, name)` to get `user_id`.
6. `user_id`, `email`, `subject`, and `provider` are stored in the request `context`.

## 4. Go Middleware

```go
func AuthMiddleware(cfg AuthConfig, users UserStore) echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            token, err := extractBearer(c.Request())
            if err != nil { return err }

            claims, err := validateJWT(token, cfg)
            if err != nil { return echo.NewHTTPError(401, err) }

            u, err := users.Upsert(ctx, claims.Issuer, claims.Subject, claims.Email, claims.Name)
            if err != nil { return err }

            ctx := context.WithValue(c.Request().Context(), currentUserKey, u)
            c.SetRequest(c.Request().WithContext(ctx))
            return next(c)
        }
    }
}
```

## 5. Multi-OIDC Ready

- Configuration is a list of allowed issuers with JWKS URLs and audiences.
- Example `.env`:
  ```
  AUTH_ISSUERS=https://accounts.google.com
  AUTH_AUDIENCES=<google-client-id>
  ```
- Adding a provider later only requires a config change and updating the sign-in button on the client.

## 6. Current User

GraphQL resolvers access the user from context:

```go
u := currentuser.FromContext(ctx)
```

If missing, the request is rejected. No resolver accepts `userId` from the client.

## 7. Audit

- `created_by` / `updated_by` columns are the user's `email` (human-readable).
- `user_id` is the scoping/ownership key for per-user data.

## 8. Sessions (refresh tokens)

Google ID tokens expire in ~1 hour. To keep users signed in, LENA issues
its own session on top of the provider credential:

- `POST /auth/session` — authenticated by the provider token (middleware);
  returns `{accessToken, refreshToken, expiresAt}`.
- `POST /auth/session/refresh` — `{refreshToken, device}` → rotated pair.
  Unauthenticated by design: this is called *after* the access token
  expires. IP-rate-limited.
- `POST /auth/session/revoke` — `{refreshToken}` → sign out.
- `POST /auth/session/{provider}` — OAuth2 code-exchange sign-in for
  providers whose credential can't be a bearer token: `discord`
  (`code` → `users/@me`), `microsoft` and `facebook` (`code` → OIDC
  `id_token`, verified by the §3 machinery; `nonce` binds the authorize
  request to the token and is required for Facebook). The request must
  also carry `codeVerifier` — the browser sends an S256 `code_challenge`
  on the authorize redirect and the server forwards the verifier to the
  token exchange, so an intercepted code alone cannot be redeemed
  (PKCE, RFC 7636). Unauthenticated — the code + verifier are the
  credential — IP-rate-limited, 8K body.

**Access token** — HS256 JWT signed with `LENA_SESSION_SECRET`,
`iss=lena`, `sub=<user_id>`, ~15 min TTL (`LENA_SESSION_ACCESS_TTL`).
The auth middleware routes on `iss`: `lena` → session validation → load
the identity row by `sub` (no upsert); anything else → the JWKS path in
§3–4. Session-authenticated requests cannot call `/auth/session` — a
stolen short-lived token must not mint refresh tokens.

**Refresh token** — opaque (32 random bytes, base64url), stored only as a
SHA-256 hash in `identity.session`. Each refresh rotates into a new row in
the same `family_id` and marks the old row `replaced_by`/`revoked_at`.
Replaying a rotated or revoked token is treated as theft: the whole family
is revoked (`ErrSessionReuse`). Expiry slides on each rotation, bounded by
`LENA_SESSION_REFRESH_TTL` (default 720 h / 30 d).

**Clients** — web: refresh token in `localStorage`, access token in
`sessionStorage`, single-flight refresh + one retry on 401
(`lib/api.ts`), fresh-tab restore via rotation + `me`. Mobile: refresh
token in `flutter_secure_storage`, proactive refresh in the auth link, one
retry on 401 (`lib/graphql_config.dart`).

**Disabled mode** — empty `LENA_SESSION_SECRET` leaves the service
unconfigured: session endpoints return 503, `iss=lena` tokens are
rejected, and clients fall back to passing the provider token as the
bearer (OIDC-only mode, the pre-session behavior).

Known limit: an in-flight access token stays valid until its ~15-minute
expiry even after the family is revoked — revocation bounds the refresh
channel, which is where persistence lives.

## 9. Security Notes

- Provider ID tokens are short-lived; sessions ride on the rotating
  refresh token described above.
- Tokens are never logged.
- All authentication errors return `401` with a generic message; detailed
  causes are logged at `debug` level.