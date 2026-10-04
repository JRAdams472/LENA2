# LEN-39 — P5: Nonce-based CSP — Completion Proof

PR: https://github.com/JRAdams472/LENA2/pull/261
Ticket: LEN-39 (parent LEN-29, OWASP Top 10 2021 remediation — finding 7)
Branch: `len-39-p5-csp`

## What shipped

### `clients/web/proxy.ts` (new) — per-request nonce CSP

Next 16.3.6 `proxy.ts` (the renamed `middleware.ts` — the old convention is
deprecated). For every page request:

- Generates a fresh nonce (`crypto.randomUUID()` → base64).
- Sets the CSP on the **request** headers so Next extracts the nonce and
  stamps it on every framework script tag automatically; also exposes it
  as `x-nonce` for Server Components needing a manual `<Script nonce>`.
- Sets the same CSP on the **response**.

Policy (rest carried over verbatim from the old Caddyfile line):

```
default-src 'self';
script-src 'self' 'nonce-<n>' 'strict-dynamic'   (+ 'unsafe-eval' in dev only)
style-src 'self' 'unsafe-inline' https://accounts.google.com/gsi/;
img-src 'self' data: blob:;
font-src 'self' data:;
connect-src 'self' https://accounts.google.com/gsi/;
frame-src https://accounts.google.com/gsi/;
object-src 'none'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'
```

- `'unsafe-inline'` dropped from `script-src`; `'strict-dynamic'` propagates
  trust to dynamically inserted scripts, which is how
  `@react-oauth/google` loads `accounts.google.com/gsi/client`.
- `style-src` keeps `'unsafe-inline'` — Emotion (MUI) injects `<style>`
  tags and inline styles are a far weaker injection surface.
- Matcher skips `_next/static`, `_next/image`, `favicon.ico`, `icon.svg`,
  and prefetch requests — the CSP belongs on HTML documents.

### `clients/web/app/layout.tsx`

`export const dynamic = "force-dynamic"` — nonce CSP requires every page
to render per-request; a static shell has no request nonce to stamp.

### `Caddyfile`

Removed the static page-CSP line (Next owns it now — a static header here
would shadow/conflict). All other security headers unchanged: XFO,
Referrer-Policy, XCTO, Permissions-Policy, HSTS, `Server`/`X-Powered-By`
stripping.

### Docs

- `docs/deployment.md` — §3 now points CSP ownership at `proxy.ts`.
- `docs/security-review-20261003.md` — F2 fix text and the
  `script-src 'unsafe-inline'` residual-risk bullet annotated as resolved.

## Live verification (dev stack, `web` rebuilt + `caddy` restarted)

Response through Caddy:

```
Content-Security-Policy: default-src 'self';
  script-src 'self' 'nonce-ODRjY2Yx…' 'strict-dynamic';
  style-src 'self' 'unsafe-inline' https://accounts.google.com/gsi/; …
X-Frame-Options: DENY        X-Content-Type-Options: nosniff
Referrer-Policy: strict-origin-when-cross-origin
```

Playwright against `http://localhost/login`:

- Header nonce == `nonce` attribute on every `<script>` tag in the DOM.
- Google Identity Services script loaded and executed (its
  `accounts.google.com/gsi/client` insert is trusted via
  `strict-dynamic`) — proof: the only console message was GSI's own
  "Provider's accounts list is empty" (expected in dev).
- MUI/Emotion rendered (computed styles applied).
- **Zero CSP violation console errors.**
- Nonces differ across requests.

## Tests

`__tests__/proxy.test.ts` (5 tests, node env): fresh nonce per request,
`script-src` drops `'unsafe-inline'` + keeps `'strict-dynamic'`,
`style-src` keeps `'unsafe-inline'`, all other directives verbatim,
nonce/CSP propagated through `x-middleware-request-*` headers.

## Verification

- `npx jest` — 564/564 (50 suites)
- `npx tsc --noEmit` — clean
- `npx eslint proxy.ts app/layout.tsx __tests__/proxy.test.ts` — 0 issues
- `npm run build` — all routes `ƒ` (dynamic), `ƒ Proxy (Middleware)` registered
- CI e2e job runs the production stack through this Caddyfile — green on
  the PR is the end-to-end gate.
