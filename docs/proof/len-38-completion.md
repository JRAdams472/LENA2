# LEN-38 — LEN-29 P4 OAuth PKCE: proof of completion

PR: https://github.com/JRAdams472/LENA2/pull/TBD
Parent: LEN-29 (OWASP Top 10 review). Finding: #4 (OAuth code flows lack PKCE and server-side state binding).

## What shipped

### Web client (`clients/web/lib/oauth.ts`)
- `generateCodeVerifier()` — 32 bytes via `crypto.getRandomValues` → 43-char base64url verifier
- `deriveCodeChallenge()` — S256 via `crypto.subtle.digest`
- `authorizeUrl` now sends `code_challenge` + `code_challenge_method=S256` on **all three** providers (Discord, Microsoft, Facebook — providers without PKCE support ignore the params)
- `startOAuthSignIn` is async: generates verifier → challenge, stores verifier in `sessionStorage` under `lena_oauth_verifier_{provider}` alongside state/nonce

### Callback + session plumbing
- `app/auth/[provider]/callback/page.tsx` — reads + clears the verifier; a missing verifier fails the same as a state mismatch
- `AuthProvider.signInWithProvider` / `api.createProviderSession` — accept and forward `codeVerifier` in the `POST /auth/session/{provider}` body
- `LoginScreen` — wraps the now-async `startOAuthSignIn` with a catch that surfaces a generic start error

### BFF
- `providerSessionRequest.CodeVerifier` — **required**; missing → `400 codeVerifier is required`
- `CodeVerifier` interface: `verify(ctx, code, nonce, codeVerifier)`; `OAuthOIDCVerifier` and `DiscordVerifier` forward it as `code_verifier` in the token-exchange form (only when non-empty — verified by form-field absence tests)
- `/auth/link` — `codeVerifier`/`currentCodeVerifier` fields threaded through `verifyCredential` for code-exchange credentials; optional there since older clients may not send one
- `docs/auth-oidc.md` — session-exchange endpoint documented with the PKCE requirement

## Tests

- `session_exchange_test.go` — verifier forwarded to `verify()`, missing verifier → 400 table case
- `discord_test.go` / `oauthoidc_test.go` — httptest fakes assert `code_verifier` lands in the exchange form, and is **absent** when the caller supplies none
- `oauth.test.ts` — `code_challenge`/`code_challenge_method` asserted on all three authorize URLs; RFC 7636 Appendix B vector verifies the S256 derivation; verifier shape test (43-char base64url, unique per call)
- `session.test.tsx` — exchange body asserted to carry `codeVerifier`

## Verification

- `go test ./internal/bff/` green incl. testcontainers integration
- `golangci-lint` 0 issues; `tsc --noEmit` clean; eslint clean on touched files
- Jest 300/300 across `__tests__/lib/` + `__tests__/app/auth/`
- Note: `crypto.subtle`/`TextEncoder` polyfilled in jest (jsdom lacks them); browsers provide both natively

## Not in scope

- Google GIS + mobile have no authorization-code flow — nothing to protect there
- `/auth/link` keeps the verifier optional for client compatibility
