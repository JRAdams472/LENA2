# Project Rules for Devin

## Git Workflow

- Each major rewrite phase is developed on its own branch named `phase-<N>` (e.g. `phase-5`).
  - The mobile redesign used a self-describing named series instead: `mobile-redesign-p0` through `mobile-redesign-p5`.
- Do not push commits directly to `main`.
- When a phase is complete, open a pull request against `main` and summarize the changes.
- Only merge after the phase has been verified (build, tests, lint).

## Plan Close-Out

After the final phase of any plan merges, before starting the next:

1. Delete merged phase branches locally and on GitHub (verify PR state with `gh` first).
2. Ensure no new feature is left at zero test coverage — every new service, resolver, or page needs at least one unit or integration test.
3. Re-walk corrected audit findings (`audit/summary.md`) for regressions.
4. Note improvements or new features inspired by the completed work (e.g. `docs/newfeatures.md`).
5. Update `README.md` so features and architecture reflect what shipped.
