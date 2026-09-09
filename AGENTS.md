# Project Rules for Devin

## Git Workflow

- Each major rewrite phase is developed on its own branch named `phase-<N>` (e.g. `phase-5`).
  - The mobile redesign used a self-describing named series instead: `mobile-redesign-p0` through `mobile-redesign-p5`.
- Do not push commits directly to `main`.
- When a phase is complete, open a pull request against `main` and summarize the changes.
- Only merge after the phase has been verified (build, tests, lint).
