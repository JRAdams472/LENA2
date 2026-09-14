# Prompt — Generate phased audit-remediation plans for LENA2

> Reusable prompt. Paste it into a fresh Devin session to (re)generate the plan files in
> `audit/remediation/`. The prompt text below is the one used to produce the files currently in this
> folder; if the audit inputs change, re-run it and review the diff.
>
> Note: the session that produced these files did not receive a verbatim copy of the original prompt,
> so the text below was reconstructed from the task instructions (roadmap table, per-file requirements
> and constraints) given in that session.

---

Repository: `JRAdams472/LENA2`. Work on a branch cut from the latest `main` (suggested name
`audit-review-plans`). This task produces planning documents only — make NO application-code changes.

**Step 1 — Read the audit inputs.** Read every file in the `audit/` folder (`summary.md`,
`phase-1-architecture.md`, `phase-2-bff.md`, `phase-3-domains.md`, `phase-4-tests.md`,
`phase-5-docker-deploy.md`, `phase-6-security.md`) and `docs/20260911-audit-findings.md`. These hold
111 findings (0 critical, 15 High, 51 Medium, 45 Low). Extract, for each finding ID, its severity,
one-line summary, target file(s)/lines, and the report's recommended remediation.

**Step 2 — Save this prompt.** Create the folder `audit/remediation/` and save this prompt, verbatim,
as `audit/remediation/PROMPT.md`, including the roadmap table and all constraints. All branch names use
the spelling `audit-review-phaseN`.

**Step 3 — Generate one plan file per phase:** `audit/remediation/phase-1.md` through
`audit/remediation/phase-8.md`, using this phase → branch → theme → findings roadmap:

| Phase | Branch | Theme | High | Medium |
|---|---|---|---|---|
| 1 | `audit-review-phase1` | Deployment & config security | A5-01, A5-02 | A5-03, A5-04 |
| 2 | `audit-review-phase2` | Auth & authorization | A6-01, A2-02 | A6-02, A6-03, A6-04, A6-06 |
| 3 | `audit-review-phase3` | Error contract & silent-success | A3-01, A1-02, A4-03 | A3-04, A2-08 |
| 4 | `audit-review-phase4` | Concurrency, atomicity & tx seam | A2-01, A1-03, A4-01 | A3-05, A3-06, A3-07, A3-08, A3-12, A4-10 |
| 5 | `audit-review-phase5` | Recipe-import / OCR pipeline | A3-02, A3-03, A4-02 | A3-09, A3-10, A3-13, A6-07, A6-08, A2-09 |
| 6 | `audit-review-phase6` | Architecture boundaries & grocery feature | A1-04, A1-01 | A1-05, A1-07, A1-09, A1-12, A2-06 |
| 7 | `audit-review-phase7` | BFF robustness & DoS controls | — | A2-03, A2-04, A2-05, A2-07, A2-10, A6-05, A2-11, A2-12, A1-08, A1-10, A1-11, A3-21, A6-11 |
| 8 | `audit-review-phase8` | CI/Docker hygiene & test coverage | — | A5-05, A5-06, A5-07, A5-08, A5-09, A5-10, A4-04, A4-06, A4-07, A4-08, A4-09 |

Verify each finding ID against the audit reports while writing; if any ID above does not exist or its
severity differs, correct it to match the reports and note the correction in that phase file. Ensure
every High and Medium finding from `summary.md` (15 High + 51 Medium) is assigned to exactly one phase;
if any are unassigned after building the eight files, add them to the most topically appropriate phase
and list them explicitly.

**Step 4 — Each `phase-N.md` must contain:**

- (a) the branch name `audit-review-phaseN`;
- (b) the theme;
- (c) a findings table with columns ID, severity, one-line summary, target file(s);
- (d) a numbered list of concrete remediation steps drawn from the audit report's remediation text for
  each finding;
- (e) a co-located-Low policy line: "fix a Low finding only if it lives in code already being changed
  for a High/Medium finding in this phase; never edit code solely to fix a Low";
- (f) a Verification section requiring `go build ./...` and `go test ./...` to pass, plus any
  phase-specific manual checks;
- (g) a closing instruction to open a PR from `audit-review-phaseN` into `main` and stop, not beginning
  the next phase until this PR is approved.

**Step 5 — Write `audit/remediation/README.md`** that indexes all eight phase files in order, restates
the approval-gated one-PR-per-phase workflow, states that remediation phases intentionally do not mirror
the audit's own phase numbers, and includes the sequence rationale (Phase 3 typed errors unblock
Phase 4's tx seam, which is a prerequisite for Phase 5's atomic recipe-import Approve; test fixes fold
into the phase that touches their code).

**Step 6 — Open a PR into `main`** containing only the new files under `audit/remediation/`. Confirm no
application code, SQL, Docker, or CI files were modified.

**Constraints**

- Planning documents only; no changes to Go, SQL, GraphQL schema, Docker/compose, Caddy, CI or client
  code.
- One PR per remediation phase, opened from `audit-review-phaseN` into `main`; each PR must be approved
  before the next phase starts.
- Low findings are never a reason to touch code on their own (co-located-Low policy above).
- Every phase PR must pass `go build ./...` and `go test ./...` (integration tests need Docker for
  testcontainers) plus the repo lint (`golangci-lint run ./...`, `go vet ./...`).
- Line numbers in the audit reports refer to `main` @ `c4ad3c7`; re-locate code by symbol name when
  the tree has moved on.
