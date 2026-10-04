# LEN-29 — OWASP Top 10 2021 Security Review: Remediation Report

Parent ticket: LEN-29 · Phases: LEN-35…LEN-40 · PRs: #257–#261 + close-out.

Every finding is verified against `main` — the re-walk below cites the
landed code, not the branch.

## Findings → status

| # | Finding | Fix | PR | Verification on main |
|---|---------|-----|----|----------------------|
| 1 | OCR service deps carried 65 known CVEs | Pillow 12.3.0, fastapi 0.142.2 (starlette 1.7.0), python-multipart 0.0.32, uvicorn 0.54.0; **new `pip-audit` gate** in the `ocr-import` CI job | #257 | `pip-audit` inside the rebuilt image: 0 vulnerabilities; gate green since its first run |
| 2 | `setRecipeCategories` was member-writable — global catalog writes across households | `@admin` in schema + `requireAdmin` in resolver; web hides picker + skips `recipeCategoryGroups` fetch for members (read-only chips remain); mobile reverts optimistic toggle on FORBIDDEN | #259 | `schema.graphqls:323` `@admin`; member test asserts rejection |
| 3 | Invite accepted arbitrary user IDs — bypassed `is_searchable`/`is_active`, enumerable errors, unlimited probing | Target must be `IsActive && IsSearchable` (mirrors `searchUsers`); all failure classes return identical `BAD_USER_INPUT: cannot invite this user`; per-user `userRateLimiter` (10/min) | #259 | `resolver_household.go:173` gate; identical-error table test |
| 4 | OAuth authorization-code flows had no PKCE | S256 PKCE end-to-end: 32-byte verifier + `code_challenge` on Discord/Microsoft/Facebook authorize URLs; sessionStorage lifecycle; `codeVerifier` **required** on `POST /auth/session/{provider}` → `code_verifier` forwarded to token endpoints; `/auth/link` passthrough | #260 | `oauth.ts` + `auth_links.go`; RFC 7636 Appendix-B vector test |
| 5 | `SESSION_SECRET` accepted any length | `ValidateServer` rejects `<32` bytes at boot (empty still allowed = OIDC-only mode); e2e secret bumped; `.env.example` documents the minimum | #258 | `internal/platform/config/config.go:269` |
| 6 | `hashSubject` logged the raw provider subject on auth failures | `hex(sha256(subject))[:16]` | #258 | `internal/bff/auth.go:329`; test asserts the real digest |
| 7 | Static CSP with `script-src 'unsafe-inline'` — any injected inline script executed | `clients/web/proxy.ts` (Next 16's renamed middleware): per-request nonce, `script-src 'self' 'nonce-…' 'strict-dynamic'` (+`'unsafe-eval'` dev-only), `style-src` keeps `'unsafe-inline'` for Emotion/MUI, all other directives verbatim; `force-dynamic` root layout; Caddyfile drops its CSP line | #261 | nonce == script attr on every tag; GSI loads via strict-dynamic; zero console violations (Playwright) |
| 8 | SonarQube dev overlay exposed port 9000 on all interfaces | `127.0.0.1:9000:9000` bind | #257 | `docker-compose.sonarqube.yml:19` |
| 9 | `google.golang.org/grpc` vulnerable (GO-2026-6443) | bumped to v1.83.2 + `go mod tidy` | #257 | `go.mod` |
| 10 | Admin deactivation was not immediate — up to 2-min identity-cache window and refresh tokens kept minting access tokens | `SetUserActive` evicts the target's cached identity via `AuthInvalidator`; `AdminSetActive` revokes all refresh-token families inside the same transaction | #258 | `service.go:613` `RevokeUserSessions` + resolver invalidator call |

## Residuals (accepted, documented)

- **`x/crypto/openpgp`** — `govulncheck` still flags it, but it's a
  transitive dependency with no reachable call path; upstream pins block
  the fix. Re-checked each release.
- **npm dev-only advisories** — remaining audit findings are all in the
  build toolchain (`next build` deps), unreachable at runtime; the
  published image ships `next start` only.
- **`sessionStorage` session artifacts** — the PKCE verifier and OAuth
  state live in sessionStorage; tab-scoped by design and the residual
  XSS-read risk is now mitigated by the nonce CSP (finding 7).
- **SonarQube** remains a localhost-only dev tool — it never deploys to
  production.

## Coverage check

Every new code path shipped with tests:

- P1 — `pip-audit` gate is the test (CI job)
- P2 — `TestValidateServer_SessionSecret` (5-case), `TestHashSubject`,
  `TestResolver_SetUserActive` (eviction), `TestAdminSetActive` (revoke)
- P3 — invite enumeration table test (byte-identical errors), rate-limit
  test, `setRecipeCategories` member-rejection test, member-hides-picker
  web test; bff integration flows opt targets into `isSearchable`
- P4 — RFC 7636 vector test, `code_verifier` present/absent assertions
  on httptest token fakes, missing-verifier → 400, PKCE params on all
  three authorize URLs
- P5 — `__tests__/proxy.test.ts`: nonce freshness, directive contents,
  `x-middleware-request-*` propagation

## Regression re-walk

All ten findings re-verified on `main` post-merge (grep citations in the
table). The broader audit backlog (`audit/summary.md`, 111 findings) is
intentionally out of scope — LEN-29 closed the OWASP-targeted subset; the
remaining items follow `audit/remediation/README.md`'s 8-phase roadmap.
