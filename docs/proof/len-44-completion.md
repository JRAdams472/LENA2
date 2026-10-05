# LEN-44 / P4 — web complexity

Phase 4 of the LEN-28 SonarQube megaplan. All web-client complexity and
readability findings cleared: **SonarQube rescan reports 0 `typescript:S3776`,
`S3358`, `S4624`, and `S6772` findings**. Repo-wide open issues dropped
**109 → 74** (remaining are P5 tools/Docker and P6 sweep scope).

## What changed (pure refactor — behavior unchanged)

### S3776 cognitive-complexity splits (all → ≤15)

| Function | Was | Approach |
|---|---|---|
| `lib/ai/validate.ts` `validatePairings` | 20 | `toPairing` per-item conversion |
| `lib/ai/validate.ts` `validateEventFixes` | 35 | `eventFixIndex` lookup build, `eventFixPayloadOk` per-action checks, `toEventFix` record build |
| `lib/ai/protocol.ts` `parseToolCalls` | 21 | `toToolCall` + `parseCallArguments` (object-or-stringified args) |
| `lib/ai/useAssistant.ts` `useAssistant` | 16 | `resolveStatus` lifecycle→status mapping |
| `app/components/AdminLayout.tsx` `renderNavEntry` | 25 | `navGroupIcon` + `renderNavGroup`/`renderNavLeaf` split |

### S3358 nested ternaries → statements/helpers (23)

- `AdminLayout.tsx` — `notificationHref` deep-link helper; nav icon chain →
  `navGroupIcon` switch
- `DataTable.tsx` — `columnDefs` if/else; `cellText` fallback renderer
- `events/[id]/page.tsx` — ingredients heading `note` built with if/else
- `inventory/allergen-suggestions/page.tsx` — `statusColor` if-chain;
  plural suffix lifted out of the notice ternary
- `meal-plans/[id]/page.tsx` — `itemSearchEmptyText`, `brandItemName`
- `notifications/page.tsx` — `prefStatusText`
- `recipes/[id]/page.tsx` — `ratingSummary`, `recipeItemLabel`
- `recipes/pending/[id]/page.tsx` — suggestion sort comparator → block body
- `lib/api.ts` — `usualBrandLabel`
- `lib/ai/useAssistant.ts` — status chain → `resolveStatus` (with S3776)

### S4624 nested template literals (6)

- `grocery-lists/[id]/page.tsx` — `brandItemLabel` (×2)
- `meal-plans/[id]/page.tsx` — `slotItemChipLabel`
- `recipes/[id]/page.tsx` — `brandItemLabel`, `recipeItemLabel` (×2)

### S6772 ambiguous JSX spacing (1)

- `recipes/page.tsx` — explicit `{" "}` before the hidden file input

## Verification

- `npx tsc --noEmit` — clean
- `npx eslint` on all touched files — clean
- `npx jest` — 571/571 tests pass
- SonarQube rescan: `typescript:S3776`/`S3358`/`S4624`/`S6772` **0 findings**;
  total open **109 → 74**; no new issues introduced
