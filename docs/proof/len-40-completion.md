# LEN-40 — P6: LEN-29 close-out — Completion Proof

PR: https://github.com/JRAdams472/LENA2/pull/TBD
Ticket: LEN-40 (parent LEN-29, OWASP Top 10 2021 remediation)
Branch: `len-40-p6-closeout`

## AGENTS.md plan close-out checklist

1. **Branches** — all five merged phase branches deleted locally and
   remotely; `git ls-remote --heads origin` shows no `len-*`/`phase-*`
   branches.
2. **Coverage** — every phase shipped tests (enumerated in
   `docs/len-29-remediation.md` § Coverage check): config/hashing/
   invalidation unit tests, enumeration + rate-limit tables, member-
   rejection resolver test, member-hides-picker UI test, PKCE vector +
   verifier-plumbing tests, proxy CSP tests, and the `pip-audit` CI gate
   itself.
3. **Audit re-walk** — all ten LEN-29 findings re-verified against
   `main` with grep citations (`docs/len-29-remediation.md` table). The
   111-finding `audit/summary.md` backlog remains intentionally out of
   scope and is tracked by `audit/remediation/README.md`'s roadmap.
4. **`docs/newfeatures.md`** — Security hardening section appended.
5. **README sweep** — providers line now lists all four sign-in options
   + PKCE; sessions paragraph notes the ≥32-byte secret minimum and
   immediate deactivation; new "Nonce CSP" architecture bullet.
6. **Docs review** (README's Documentation section + client READMEs):
   - `docs/deployment.md` — CSP ownership moved to `proxy.ts` (done in P5).
   - `docs/auth-oidc.md` — PKCE documented (done in P4).
   - `docs/graphql-schema.md` — recipe mutations marked `@admin`,
     `setRecipeCategories` + invite/accept documented with the new
     contract; `UpdateProfileInput` fields corrected (`birthdate`,
     `isSearchable`); `setUserActive` notes immediate eviction/revocation.
   - `docs/testing.md` — `ocr-import` job notes the `pip-audit` gate.
   - `docs/security-review-20261003.md` — F2 + `unsafe-inline` residual
     annotated resolved (done in P5).
   - `clients/web/README.md` — OAuth flows now mention PKCE S256.
   - `clients/mobile/README.md` — accurate as-is (Google-only; OAuth
     deferred, no PKCE needed).
7. **Wiki** — `Getting-Started.md` updated: all four sign-in providers
   (was Google + Discord only). No other page touches security internals;
   the wiki is user-facing feature docs.
8. **Screenshots** — skipped deliberately: LEN-29 changed headers,
   dependencies, and resolver gates only. No user-facing screen differs
   visually; the login page renders the same buttons.

## Deliverable

`docs/len-29-remediation.md` — per-finding fix → PR → on-main
verification + accepted residuals + coverage check.
