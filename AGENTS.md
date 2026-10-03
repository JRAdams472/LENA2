# Project Rules for Devin

## Git Workflow

- Each major rewrite phase is developed on its own branch named `phase-<N>` (e.g. `phase-5`).
  - The mobile redesign used a self-describing named series instead: `mobile-redesign-p0` through `mobile-redesign-p5`.
- Do not push commits directly to `main`.
- When a phase is complete, open a pull request against `main` and summarize the changes.
- Only merge after the phase has been verified (build, tests, lint).

## Linear Workflow (required)

- Every plan gets a **parent Linear ticket** (team: `LEN`, MCP server: `linear`). Once planning is done and work begins, the parent moves to **In Progress**.
- Every phase of the plan gets a **sub-issue** under that parent, named for its branch (e.g. `ingredient-layer-p1`), with a description of the phase's scope and acceptance criteria. **Mark the sub-issue In Progress when its phase starts.**
- When a phase is complete, post **proof of completion** on its sub-issue before asking for merge approval:
  - Relevant run logs (build/test/verification output).
  - **Playwright screenshots** of any user-facing change (the `playwright` MCP server is configured — capture real screens, don't describe them).
  - A link/summary of the **GitHub CI run** showing all checks green.
- **No PR or phase merges without the user's explicit approval.** Present the PR link + proof, then wait — the user reviews the evidence and approves the merge.

## Plan Close-Out

After the final phase of any plan merges, before starting the next:

1. Delete merged phase branches locally and on GitHub (verify PR state with `gh` first).
2. Ensure no new feature is left at zero test coverage — every new service, resolver, or page needs at least one unit or integration test.
3. Re-walk corrected audit findings (`audit/summary.md`) for regressions.
4. Note improvements or new features inspired by the completed work (e.g. `docs/newfeatures.md`).
5. Update `README.md` so features and architecture reflect what shipped — including sweeping for stale claims (e.g. a "no refresh tokens" note surviving after sessions shipped).
6. Review every doc listed in `README.md`'s **Documentation** section plus the touched client READMEs (`clients/web/README.md`, `clients/mobile/README.md`) for staleness — env vars, versions, workflow/job names, helper-package names, and endpoint lists drift fast.
7. Update the GitHub wiki (`LENA2.wiki.git`) — it lives outside the repo, so clone it, add/refresh pages for what shipped, and push to `master`.
8. Refresh wiki **screenshots** when user-facing screens changed: run the isolated `lena2shots` stack per `.devin/skills/wiki-screenshots` (`tools/wiki-shots/seed_demo.py` + `capture.mjs`), inspect every PNG for spinners, publish under `images/`, and embed them on the relevant wiki pages — no wiki page should ship prose-only when a screen exists. Evaluate mobile screenshots the same way; if no emulator is available, note that and keep it as a follow-up.
