# Multi-provider authentication (Facebook, Discord, …)

## Problem

Today one provider login **is** one LENA account: `identity.users` is keyed
by `(provider, external_subject)` and every OIDC issuer maps to its own
user row. Adding Facebook or Discord as-is would let the same person end
up with two accounts, two households, and duplicated data — there is no
way to attach a second provider identity to an existing user.

Separately, not every provider speaks OIDC. Discord is OAuth2-only
(`code` exchange + `GET /users/@me`), so the session-exchange endpoint
needs a provider-verifier seam instead of assuming the bearer is always a
verifiable ID token.

## Design

### `identity.user_login` — many logins per user

```sql
CREATE TABLE identity.user_login (
    user_login_id    BIGSERIAL PRIMARY KEY,
    user_id          BIGINT NOT NULL REFERENCES identity.users(user_id) ON DELETE CASCADE,
    provider         VARCHAR(50)  NOT NULL,
    external_subject VARCHAR(255) NOT NULL,
    email            VARCHAR(320) NOT NULL DEFAULT '',
    display_name     VARCHAR(200),
    last_login_at    TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (provider, external_subject)
);
```

Backfilled from `users` so every existing row's primary login appears
there too. `users.provider`/`external_subject` remain as the *primary*
login (unchanged semantics for audit, admin views, e2e issuer).

`UpsertUser` becomes a transaction: resolve `(provider, subject)` in
`user_login` → hit: load user, touch `last_login`; miss: insert user row
+ login row. Resolution for sessions (`iss=lena` → `sub=user_id`) is
unchanged.

### Explicit linking — no auto-merge

Linking is user-initiated and requires **fresh step-up auth on both
sides**: the caller authenticates with a *provider* credential (session
tokens are rejected — a stolen 15-minute access token must not attach an
attacker's provider identity) and supplies the new provider credential in
the body. No email-based merging: provider email claims may be absent or
unverified.

- `GET /auth/identities` — list the caller's linked logins
  (`provider`, `email`, `displayName`, `lastLoginAt`; never subject).
- `POST /auth/link` — body `{credential}`; verified through the provider
  verifier, then bound to the caller. 409 when that login already belongs
  to a (different) account — two existing accounts are never merged.
- `DELETE /auth/link` — body `{provider}`; removes that provider's login.
  The last remaining login cannot be removed.

### Provider verifier seam

`verifyProviderCredential(ctx, provider, credential)` returns
`(subject, email, name)` without persisting anything:

- **OIDC providers** (Google today, Facebook later): parse `iss` →
  allowlist → JWKS verify → extract claims. The `provider` stored on the
  login is the `iss` string, matching existing rows.
- **Discord** (p3): `{provider:"discord", credential:<oauth2 code>}` →
  server-side token exchange (holds `client_secret`) → `GET /users/@me`.

`/auth/session` continues to mint sessions from any verified provider
credential, so a linked Discord login yields a normal LENA session —
the per-request hot path never talks to Discord.

## Phases

- **`auth-multi-p1`** — `user_login` migration + identity service
  (resolve/link/unlink/list, transactional upsert), verify-only OIDC
  seam, link endpoints, tests. No UI yet — nothing to link until a second
  provider ships.
- **`auth-multi-p2`** — Facebook OIDC: issuer/audience config, client
  sign-in buttons (web + mobile), "linked sign-ins" section on web
  Profile, handle missing/unverified email claims.
- **`auth-multi-p3`** — Discord: server-side code exchange,
  `/users/@me` verification, web OAuth redirect flow, link support.
  - Registered redirect URI (dev): `http://localhost/auth/discord/callback`
    — a small Next.js route reads `code`/`state` and posts to
    `/auth/session/discord`; production adds the domain variant.
  - Env: `LENA_DISCORD_CLIENT_ID`, `LENA_DISCORD_CLIENT_SECRET`,
    `LENA_DISCORD_REDIRECT_URI` (server-side only — the code exchange
    holds the secret); web build args `NEXT_PUBLIC_DISCORD_CLIENT_ID`,
    `NEXT_PUBLIC_DISCORD_REDIRECT_URI`.
  - Scopes: `identify` + `email` (email treated as optional/unverified).
  - `POST /auth/session/discord` is unauthenticated — the code is the
    credential — and requires sessions to be enabled (a Discord code
    cannot be a bearer token).
  - Linking while session-authenticated requires `currentCredential`: a
    fresh provider credential resolving to the same account.
  - **Mobile deferred**: Discord's portal rejects custom-scheme redirects,
    so mobile needs the HTTPS callback → `lena://` app-link bounce (or a
    webview intercept). Web sign-in + link ships in p3; a Discord-linked
    user on mobile can meanwhile sign in via any linked OIDC provider —
    a Discord-only user on mobile is the gap to close in a follow-up.

## Non-goals

- Automatic account merging (unsafe without verified-email guarantees).
- Sign-in without any provider (no local passwords — out of scope).
- Per-provider permission scopes beyond identity.
