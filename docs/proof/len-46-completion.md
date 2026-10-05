# LEN-46 — P6 completion proof

**Phase:** LEN-28 P6 — low/info sweep + close-out
**Branch:** `len-46-p6-sweep-closeout`

## Rescan result

- **Open issues: 0** (59 → 0)
- Accepted `wontfix` list: 1 — `javascript:S4036` `tools/wiki-shots/capture.mjs` (PATH search in a local dev screenshot tool; justified in P1).
- No new issues introduced by P6 changes.

## Findings cleared (59)

| Rule | Count | Fix |
|---|---|---|
| typescript:S7781 | 23 | `.replace(/x/g, y)` → `.replaceAll("x", "y")` in `auth-gate/session/login/AdminLayout` test JWT helpers + `AuthProvider.decodeJwtPayload` |
| typescript:S7773 | 12 | `isNaN` → `Number.isNaN`, `parseInt` → `Number.parseInt` (QuantityDialog, events/[id], inventory/items, recipes/[id]) |
| typescript:S6551 | 6 | `String(unknown)`/`.toString()` → narrowed types / `cellText` reuse (DataTable, AdminLayout.test, ingredients) |
| typescript:S7772 | 4 | `node:` import prefixes (`oauth.test.ts` crypto/util, `auth.setup.ts` fs/path) |
| typescript:S7741 | 2 | `typeof x === "undefined"` → `x === undefined` (`oauth.test.ts`, `capabilities.ts`) |
| typescript:S6582 | 2 | optional chaining (`IngredientAutocomplete`, `household/page`) |
| typescript:S6606 | 2 | `updateMealSlot` merge — `??` was **unsafe** here (`null` clears the field), so conditional assignment preserves semantics while clearing the ternary (`lib/api.ts`) |
| typescript:S7758 | 1 | `String.fromCharCode` → `String.fromCodePoint` (`oauth.ts` base64url) |
| typescript:S7749 | 1 | `3600_000` → `3_600_000` (`notifications/page.test.tsx`) |
| typescript:S7750 | 1 | `.filter(...).at(-1)` → `.findLast(...)` (`agent.test.ts`) |
| typescript:S7780 | 1 | escaped-quote template → `String.raw` (`protocol.test.ts`) |
| typescript:S7737 | 1 | default object-literal param → `DEFAULT_PAGE` const (`api-items.test.ts`) |
| typescript:S7765 | 1 | `indexOf(...) === -1` → `.includes(...)` (`semantic-search.spec.ts`) |
| typescript:S7755 | 1 | `messages[len-1]` → `.at(-1)` + undefined-safe fallback (`nano.ts`) |
| typescript:S7778 | 1 | consecutive `push()` → single `push(a, b)` (`structured.ts`) |

## AGENTS.md directives added

- **Boy-scout rule** (Evidence Gathering / SonarQube): touching a file with open low/info findings → fix trivial low-risk ones in the same change.
- **Close-out rescan gate** (Plan Close-Out #9): plans that remediate SonarQube findings end with a rescan showing zero new open issues beyond the accepted wontfix list; final count recorded in close-out proof.

## Verification

- `npx tsc --noEmit` — clean
- `npx eslint app lib __tests__ e2e` — 0 issues
- `npx jest` — 571/571 pass
- SonarQube rescan (SonarScanner CLI 8.1.0.6389, CE task `3e03cfc4`) — **0 open issues**

## LEN-28 plan totals

194 open at baseline → **0 open** across six phases:

| Phase | PR | Delta |
|---|---|---|
| P1 bugs/vulns/regexes | #263 | 194 → 179 |
| P2 bff Go complexity | #264 | 179 → 145 |
| P3 remaining Go | #265 | 145 → 109 |
| P4 web complexity | #266 | 109 → 74 |
| P5 tools/Python/Docker | #267 | 74 → 59 |
| P6 low/info sweep | this PR | 59 → 0 |
