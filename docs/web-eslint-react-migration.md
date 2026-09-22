# `eslint-plugin-react` → `@eslint-react/eslint-plugin` migration

**Status: completed** (Phase 4b, `audit-remediation-phase4b`).

## What changed

`eslint-config-next` was removed because it hard-depends on
`eslint-plugin-react`, which does not support ESLint 10
(`context.getFilename()` was removed; the plugin declares `eslint: ^9.7` max).
The flat config in `clients/web/eslint.config.mjs` now composes the pieces
directly:

| Before (`eslint-config-next`) | After |
|---|---|
| `eslint-plugin-react` | `@eslint-react/eslint-plugin` (`recommended-typescript`, plus `disable-conflict-eslint-plugin-react-hooks`) |
| `eslint-plugin-react` | `eslint-plugin-react-hooks` (`flat/recommended-latest`) — official hooks rules |
| `typescript-eslint` recommended | same, direct dep |
| `@next/eslint-plugin-next` (bundled) | same, direct dep (`recommended` + `core-web-vitals`) |
| `eslint-plugin-jsx-a11y` | `eslint-plugin-jsx-a11y-x`, registered under the `jsx-a11y` namespace so rule IDs are unchanged |
| `eslint-plugin-import` | `eslint-plugin-import-x`, registered under the `import` namespace so rule IDs are unchanged |
| babel parser | `@typescript-eslint/parser` (via `typescript-eslint`); the repo has no babel-only syntax |
| `eslint` `^9` pin + Dependabot ignore | `eslint` `^10`; ignore removed |

The `-x` forks are used **only** for the same rules eslint-config-next
enabled (`import/no-anonymous-default-export`, the six `jsx-a11y` warns) —
no broader ruleset was adopted.

## Code fixes made during the migration

- `key` moved before spread props in the MUI `renderOption` callbacks
  (`@eslint-react/jsx-no-key-after-spread` — real deopt warning).
- Stale `eslint-disable` comments repointed from
  `react-hooks/set-state-in-effect` to `@eslint-react/set-state-in-effect`.

## Known new warnings (accepted, not fixed)

`@eslint-react` reports a handful of warnings the old plugin never had —
`no-context-provider`, `no-use-context`, `naming-convention-ref-name`,
`no-array-index-key`, `set-state-in-effect`. They are warnings only; fix
them opportunistically when touching those files.

## References

- `eslint-plugin-react` peer dependencies at 7.37.5: `eslint: "^3 || ... || ^9.7"`
- `eslint-config-next` 16.3.5 declares `eslint-plugin-react: "^7.37.0"`
