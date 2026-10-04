# LEN-41 — LEN-28 P1: bugs, security & reliability — Completion Proof

PR: https://github.com/JRAdams472/LENA2/pull/263
Ticket: LEN-41 (parent LEN-28, SonarQube megaplan)
Branch: `len-41-p1-correctness`

## Rescan result

Fresh scan after changes: **194 → 179 open issues; BUG: 0, VULNERABILITY: 0**
(was 2 + 5). One wontfix: `javascript:S4036` on `tools/wiki-shots/capture.mjs`
(local dev-only tool; justification recorded on the issue).

## Fixes

| Rule | Site | Fix |
|---|---|---|
| typescript:S6544 (BUG) | `lib/ai/engineStore.ts:39` | `if (probing !== null)` — Promise was always truthy in the conditional |
| typescript:S2245 (VULN) ×2 | `lib/api.ts:253-254` | Idempotency fallback `Math.random` → `crypto.getRandomValues(16B)` (works in non-secure contexts where `randomUUID` is gated); last resort is a module counter |
| typescript:S2245 (VULN) | `e2e/helpers.ts:74` | `uniqueCode` → `node:crypto` `randomInt` |
| typescript:S2245 (VULN) | `__tests__/components/CrudPage.test.tsx:31` | Deterministic `qk${counter}` queryKey |
| typescript:S8786 ×3 | `lib/oauth.ts:36`, `lib/format.ts:30`, `lib/ai/protocol.ts:57` | `base64url` → pure `replaceAll` chain; `stripSize` edges → `stripEdges` char-set loop; `stripFences` → `indexOf`-based (```json contract preserved) |
| python:S7493 (BUG) | `tools/ocr/app.py:145` | `tempfile` write off the event loop via `asyncio.to_thread` + `_write_temp` helper |
| javascript:S4036 (VULN) | `tools/wiki-shots/capture.mjs:212` | **wontfix** — dev-only tool, PATH is the dev shell's by design |

## In-phase low/info cleared (files already touched)

- `oauth.ts` — two S7781 (`.replace` → `replaceAll`) cleared by the base64url rewrite
- `protocol.ts` — S6571 (`unknown | undefined` union → `unknown`), S6582 (`?` optional chain), S6594 (`.match` → `.exec`)
- `format.ts` — S6594 (`.match` → `SIZE_RE.exec`)
- `__tests__/proxy.test.ts` — S6594 (`.match` → `.exec`)

## Tests added

`__tests__/lib/format.test.ts` — 7 tests pinning `fmtQty`, `sizeBadge`,
and `stripSize` edge-strip behavior (no format coverage existed before).

## Verification

- jest 571/571 (51 suites) · tsc clean · eslint clean on touched files
- SonarQube rescan confirms all targeted issues closed; no new issues
  introduced by the changes
