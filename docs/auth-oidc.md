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

## 8. Security Notes

- ID tokens are short-lived. LENA does **not** implement refresh tokens; the client must re-sign in with Google.
- Tokens are never logged.
- All authentication errors return `401` with a generic message; detailed causes are logged at `debug` level.