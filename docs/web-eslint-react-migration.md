# Future migration: `eslint-plugin-react` → `@eslint-react/eslint-plugin`

## Current state

- `clients/web` pins `eslint` to `^9.39.5`.
- `.github/dependabot.yml` ignores `eslint` major-version updates for `clients/web`.
- `eslint-plugin-react` 7.37.5 is not compatible with ESLint 10; it uses `context.getFilename()`, which ESLint 10 removed.

## Trigger for this migration

Migrate when one of the following is true:

1. `eslint-plugin-react` releases a version that officially supports ESLint 10 and `eslint-config-next` upgrades to it.
2. We decide to stop using `eslint-config-next` and can fully replace it with a custom flat config.

## Migration outline

1. **Remove the `eslint` pin** in `clients/web/package.json` (or let Dependabot bump it after removing the ignore).
2. **Remove the Dependabot ignore** in `.github/dependabot.yml`.
3. **Replace or remove `eslint-config-next`** in `clients/web/eslint.config.mjs`.
   - `eslint-config-next` depends on `eslint-plugin-react`, so you cannot simply swap the plugin underneath it.
4. **Build a new flat config** using:
   - `@eslint-react/eslint-plugin` (peer deps allow `eslint: '*'`)
   - `typescript-eslint`
   - `eslint-plugin-react-hooks`
   - `eslint-plugin-jsx-a11y` and `eslint-plugin-import` (if the project wants equivalent coverage)
5. **Map existing rule overrides** from `react/*` to `@eslint-react/*` rule names; they are not 1:1.
6. **Run `npm run lint` and `npm run build`** until clean, then update CI.

## References

- `eslint-plugin-react` peer dependencies at 7.37.5: `eslint: "^3 || ^4 || ^5 || ^6 || ^7 || ^8 || ^9.7"`
- `eslint-config-next` 16.3.5 declares `eslint-plugin-react: "^7.37.0"`
